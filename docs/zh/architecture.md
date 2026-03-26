# 架构与设计模式

## 概述

openclaw-qqbot 是一个使用 Go 构建的多账号 QQ Bot 服务。架构以 **BotManager** 为顶层协调器，每个账号拥有完全隔离的依赖。系统通过 RESTful HTTP API 暴露消息能力，并通过与 QQ Bot 网关的持久 WebSocket 连接接收消息。

```
                    ┌─────────────────────┐
                    │     Health Server    │ (separate port)
                    │   /health /healthz   │
                    └─────────────────────┘

                    ┌─────────────────────┐
                    │     API Server       │ (separate port)
                    │  /api/v1/accounts/.. │
                    └──────────┬──────────┘
                               │ BotAPI interface
                    ┌──────────▼──────────┐
                    │     BotManager      │
                    │  (multi-account)    │
                    └──────────┬──────────┘
                               │
              ┌────────────────┼────────────────┐
              │                │                │
      ┌───────▼──────┐ ┌──────▼──────┐ ┌──────▼──────┐
      │  Account A   │ │  Account B  │ │  Account C  │
      │  (isolated)  │ │  (isolated) │ │  (isolated) │
      └──────────────┘ └─────────────┘ └─────────────┘
```

## 组件架构

### BotManager (`internal/qqbot/botmanager.go`)

核心协调器。并发管理多个账号，每个账号拥有隔离的依赖。

**职责：**
- 创建并管理所有账号及其隔离的依赖
- 为 HTTP API 服务器提供 `BotAPI` 接口
- 处理优雅关闭（调度器 → 网关 → 存储刷新 → 数据库关闭）
- 协调 webhook 调度器进行事件转发

**账号生命周期：**

```
AddAccount() → Start() → [gateway connects] → Stop()
                   │
                   ├─ Client.Init()        (start token refresh)
                   ├─ Scheduler.Start()    (load persisted reminders)
                   └─ Gateway.Connect()    (background goroutine per account)
```

### 账号隔离

每个账号拥有完整的独立依赖集。除底层 SQLite 数据库外，账号之间不共享任何状态（数据库通过 `account_id` 列实现按账号分区）。

| 组件 | 包 | 用途 |
|------|-----|------|
| `APIClient` | `internal/api` | QQ Bot REST API 客户端，带 token 缓存 |
| `Gateway` | `internal/gateway` | WebSocket 连接、心跳、重连 |
| `OutboundHandler` | `internal/outbound` | 消息发送，支持限流和媒体标签解析 |
| `ProactiveManager` | `internal/proactive` | 主动消息和广播 |
| `Scheduler` | `internal/proactive` | 定时任务执行（100ms tick 循环） |
| `KnownUsersStore` | `internal/store` | 用户追踪，支持 upsert |
| `RefIndexStore` | `internal/store` | 消息引用索引，用于上下文 |
| `SessionStore` | `internal/store` | WebSocket 会话持久化 |
| `TTSProvider` | `internal/audio` | 文本转语音合成 |

### 接口适配器模式

HTTP API 服务器（`internal/httpapi/`）通过 `BotAPI` 接口与 `BotManager` 通信，该接口由 `internal/qqbot/run.go` 中的 `botAPIAdapter` 实现。这避免了循环导入，因为 `httpapi` 只导入接口，而不导入具体的 `BotManager`。

```
httpapi.APIServer → BotAPI interface ← botAPIAdapter wraps *BotManager
```

---

## 数据流

### 入站消息流

```
QQ Bot Gateway (WebSocket)
    │
    ▼
Gateway.readLoop()           # reads WSPayload
    │
    ▼
Gateway.handleDispatch()     # routes by event type
    │
    ├─ READY/RESUMED → session persistence, ready signal
    │
    └─ Message events → MessageQueue.Enqueue()
                            │
                            ▼
                       MessageQueue      # per-user serialization
                            │
                            ▼
                       EventHandler (BotManager)
                            │
                            ├─ Record user in KnownUsersStore (C2C/GROUP events)
                            │
                            └─ WebhookDispatcher.Dispatch()
                                    │
                                    ▼
                               HTTP POST (async, with retry)
                               to configured webhook URL
```

### 出站消息流

```
HTTP API Request
    │
    ▼
APIServer handler
    │
    ▼
BotManager (lookup account)
    │
    ▼
OutboundHandler.SendText/SendImage/SendVoice/...
    │
    ├─ Rate limit check (ReplyLimiter)
    │   └─ If exceeded/expired → clear msgID (fallback to proactive)
    │
    ├─ Media tag parsing (ParseMediaTags)
    │   └─ If tags found → sendMixedContent (text + media in order)
    │
    ├─ Image URL resolution (prefix with image server base URL)
    │
    └─ APIClient.Send*Message()
            │
            ├─ Get access token (TokenCache with singleflight)
            │
            ├─ Upload media if needed (UploadCache with MD5 key)
            │   └─ Retry on server errors (2 retries, exponential backoff)
            │
            └─ POST to QQ Bot API
                └─ If response has ref_idx → onMessageSent hook
```

---

## 并发模型

### 按用户消息队列 (`internal/gateway/queue.go`)

来自 WebSocket 网关的消息通过**按用户串行化、跨用户并行**的模型处理：

| 参数 | 默认值 | 描述 |
|------|--------|------|
| `maxConcurrentUsers` | 10 | 最大并行处理用户数（信号量） |
| `perUserQueueSize` | 20 | 每个用户的最大待处理消息数 |
| `globalQueueSize` | 1000 | 分发通道的缓冲区大小 |

- 每个用户拥有独立的 goroutine（`userWorker`），**串行**处理消息——保证同一用户的消息顺序。
- 信号量限制同时处理消息的用户总数为 10。
- 按用户队列溢出时静默丢弃消息（`Enqueue` 返回 `false`）。
- 处理器错误仅记录日志，不会中断处理。

### Token 缓存去重 (`internal/api/token.go`)

使用 `golang.org/x/sync` 包中的 `singleflight.Group` 对同一 appId 的并发 token 请求进行去重。当多个 goroutine 同时需要 token 时，只发起一次 HTTP 请求，结果在所有请求间共享。

**Token 生命周期：**
- 后台刷新在 `Init()` 时启动，直到 `Close()` 时停止。
- Token 在实际过期前 5 分钟被视为已过期（`earlyExpiry`）。
- 后台刷新在实际过期前 5 分钟触发（`refreshAhead`）。
- 最小刷新间隔：60 秒。
- 刷新失败后 5 秒重试。
- QQ Bot token 默认 TTL：约 7200 秒（2 小时）。

### 存储线程安全

所有 store 实现使用 `sync.Mutex` 保证线程安全。SQLite 数据库使用：
- **WAL 模式**以提升并发读取性能。
- **单连接**（`MaxOpenConns=1`）保证写入安全。
- **5 秒忙等待超时**处理竞争。

### 回复限流器 (`internal/outbound/ratelimit.go`)

执行 QQ Bot 平台的**每个 msg_id 每小时最多 4 条被动回复**限制：

- 使用内存中的 `map[string]*replyRecord`，通过 `sync.Mutex` 保护。
- 追踪数量超过 10,000 条时自动清理（清除已过期的条目）。
- 超过限制或过期（>1 小时）时，透明地回退为主动消息。

---

## 网关与重连

### WebSocket 生命周期 (`internal/gateway/gateway.go`)

```
Connect() → connectLoop()
    │
    └─ connectOnce()
        │
        ├─ getGatewayURLAndToken()    # HTTP GET /gateway with auth
        │
        └─ tryConnect() [for each intent level]
            │
            ├─ Dial WebSocket
            ├─ Read Hello (op=10) → get heartbeat_interval
            │
            ├─ If session exists → send Resume (op=6)
            │   └─ On RESUMED → mark connected
            │
            ├─ If new session → send Identify (op=2) with intents
            │   └─ On READY → save session, mark connected
            │
            └─ readLoop() → handlePayload() for each message
                ├─ Dispatch (op=0) → route events
                ├─ Heartbeat ACK (op=11) → acknowledged
                ├─ Reconnect (op=7) → disconnect + reconnect
                └─ Invalid Session (op=9)
                    ├─ canResume=true → reconnect with resume
                    └─ canResume=false → clear session, try next intent level
```

### 权限回退

QQ Bot API 可能拒绝某些权限组合。网关实现了 3 级权限回退：

| 级别 | 名称 | 权限 | 覆盖范围 |
|------|------|------|----------|
| 0 | `full` | Guilds + GuildMembers + DirectMessage + GroupAndC2C + PublicGuildMessages | 所有频道 |
| 1 | `group_and_guild` | Guilds + PublicGuildMessages + GroupAndC2C | 群聊 + 频道公开消息 |
| 2 | `guild_only` | Guilds + PublicGuildMessages | 仅频道公开消息 |

当收到 `INVALID_SESSION` 且 `canResume=false` 时，网关清除会话并递增 `intentIndex`，尝试下一个（更低的）权限级别。当前权限级别持久化存储在会话存储中。

### 重连配置 (`internal/gateway/reconnect.go`)

| 参数 | 默认值 | 描述 |
|------|--------|------|
| 退避延迟 | `[1s, 2s, 5s, 10s, 30s, 60s]` | 重试之间的指数退避 |
| 限流延迟 | 60s | 被服务器限流时的额外延迟 |
| 最大尝试次数 | 100 | 放弃前的最大重连尝试次数 |
| 快速断连阈值 | 5s | "快速"断连检测的时间窗口 |
| 最大快速断连次数 | 3 | 触发停止的连续快速断连次数 |

`GetDelay()` 方法根据尝试次数从延迟数组中选取值（对于高尝试次数，钳位到最后一个值）。`ShouldQuickStop()` 方法检测快速重连失败，这种情况通常表示配置或凭据问题。

### 会话持久化

WebSocket 会话状态持久化到 SQLite，以便在重启后恢复：

| 字段 | 描述 |
|------|------|
| `session_id` | READY 事件中的 QQ Bot 会话 ID |
| `last_seq` | 最后接收的序列号 |
| `intent_level_index` | 当前权限回退级别 |
| `app_id` | 账号 AppID（加载时用于验证） |

会话数据在 READY 时保存，每接收一条消息后更新 seq。加载时，超过 5 分钟的会话被视为过期。如果持久化的 `app_id` 与当前账号不匹配，会话将被失效。

---

## 存储

### 数据库 (`internal/store/db.go`)

使用**纯 Go SQLite**（`modernc.org/sqlite`）——无需 CGO。数据库文件：`{dataDir}/qqbot.db`。

**数据表**（4 张表）：

| 表 | 用途 | 关键字段 | TTL/淘汰 |
|----|------|----------|----------|
| `known_users` | 追踪消息发送者 | `account_id`、`open_id`、`type`、`group_open_id` | 无（持久化） |
| `ref_index` | AI 上下文的消息引用索引 | `ref_key`、`content`、`sender_id` | 7 天，最多 50,000 条 |
| `sessions` | WebSocket 会话状态 | `account_id`（主键）、`session_id`、`last_seq` | 5 分钟，appID 不匹配 |
| `reminders` | 定时提醒任务 | `id`（主键）、`account_id`、`schedule`、`next_run` | 一次性执行后删除 |

**索引：**
- `known_users(account_id)`、`known_users(account_id, type)`
- `reminders(account_id)`

### 旧版迁移

首次启动时，存储会自动从旧版 JSON 文件迁移数据：
- `known-users.json`（JSON 数组）→ `known_users` 表
- `ref-index.jsonl`（JSONL 格式）→ `ref_index` 表
- `session-*.json`（按账号的文件）→ `sessions` 表

迁移成功后，每个源文件会被重命名为 `.bak`，以防止重复迁移。

### 数据目录

所有持久化数据存储在 `data/` 目录下（运行时自动创建）：

```
data/
├── qqbot.db              # SQLite database (all stores)
├── known-users.json.bak  # Migrated legacy data
├── ref-index.jsonl.bak
└── session-default.json.bak
```

---

## API 客户端 (`internal/api`)

### Token 管理

- **接口地址：** `https://bots.qq.com/app/getAppAccessToken`
- **缓存：** 内存中的 `TokenCache`，按 appId 分条目
- **去重：** `singleflight.Group` 防止并发 token 请求
- **后台刷新：** 在过期前 5 分钟自动刷新
- **提前过期：** Token 在实际 TTL 前 5 分钟被视为已过期

### 上传缓存

媒体上传通过 `UploadCache` 缓存，避免重复上传：

- **缓存键：** `md5(data):scope:targetId:fileType`
- **最大容量：** 500 条
- **TTL：** 基于服务器响应 TTL 减去 60 秒安全余量（最少 10 秒）
- **淘汰策略：** 优先清除过期条目，容量满时清除较旧的一半

### 请求重试

文件上传请求使用指数退避重试：
- **最大重试次数：** 2 次
- **基础延迟：** 1 秒（每次重试翻倍：1s、2s）
- **不可重试：** 400/401/Invalid/Timeout 错误立即失败
- **超时：** 标准 API 30 秒，文件上传 120 秒

---

## 调度器 (`internal/proactive/scheduler.go`)

定时提醒任务由内存调度器管理，使用 SQLite 进行持久化。

**执行模型：**
- 基于 Ticker：每 **100ms** 检查到期任务。
- 到期任务在锁的保护下收集，然后顺序执行。
- 执行后，根据调度规则重新计算 `NextRun`。
- 一次性任务（空调度规则或无下次运行时间）在执行后删除。
- 使用 `sync.Mutex` 保证线程安全。

**任务生命周期：**

```
AddReminder() → persisted to store + in-memory map
    │
    ▼
Scheduler.run() [100ms ticker]
    │
    ▼
checkDue() → collect due jobs → executeJob()
    │
    ├─ Recurring → update NextRun
    └─ One-time → delete from map + store
```

**调度语法：**
- `@every Xs/Xm/Xh` —— Go `time.ParseDuration` 间隔
- 5 字段 cron —— `minute hour day month weekday`，支持范围和步长

---

## 图片托管 (`internal/image`)

一个本地 HTTP 服务器，用于提供 bot 上传的图片：

- 通过 `/images/{uuid}.{ext}` 提供图片，其中 UUID 为 32 个十六进制字符。
- 通过魔数自动检测格式（PNG、JPEG、GIF、WebP）。
- 启用 CORS（`Access-Control-Allow-Origin: *`）。
- 基于 TTL 的过期机制（过期图片返回 HTTP 410 Gone）。
- 路径遍历保护（验证解析后的路径在图片目录内）。

图片尺寸解析器（`internal/image/size.go`）直接读取二进制文件头（无需图片解码），支持 PNG、JPEG、GIF 和 WebP 格式。

---

## 依赖项

### Go 模块

| 包 | 版本 | 用途 |
|----|------|------|
| `github.com/gorilla/websocket` | v1.5.3 | 网关连接的 WebSocket 客户端 |
| `golang.org/x/sync` | v0.20.0 | `singleflight` 用于 token 请求去重 |
| `gopkg.in/yaml.v3` | v3.0.1 | YAML 配置解析 |
| `modernc.org/sqlite` | v1.47.0 | 纯 Go SQLite 驱动（无 CGO） |

### 运行时依赖

| 工具 | 用途 |
|------|------|
| `ffmpeg` | 音频处理（SILK 编解码、格式转换） |
| `ffprobe` | 音频格式检测 |

除 ffmpeg/ffprobe 外，没有其他外部运行时依赖。二进制文件是完全自包含的。

## 启动与关闭

### 启动流程

```
1. Parse CLI flags (-config, -health, -api)
2. Load and validate YAML config
3. Create data/ directory
4. Open SQLite database (schema + migration)
5. For each configured account:
   a. ResolveAccount() (apply defaults, resolve secrets)
   b. Skip if disabled
   c. Create APIClient, stores, Gateway, OutboundHandler, etc.
   d. Register webhook URL if configured
6. Start webhook dispatcher
7. Start BotManager:
   a. Client.Init() for each account (token refresh)
   b. Scheduler.Start() for each account (load persisted reminders)
   c. Gateway.Connect() for each account (background goroutines)
8. Start API server (if -api specified)
9. Start health server (if -health specified)
10. Wait for SIGINT/SIGTERM
```

### 关闭流程

```
SIGINT/SIGTERM received
    │
    ▼ (10s timeout)
    │
    ├─ API server stop (5s graceful shutdown)
    ├─ Health server stop
    └─ BotManager.Stop():
        ├─ Cancel context (stops all background goroutines)
        ├─ Scheduler.Stop() for each account
        ├─ Gateway.Close() for each account
        ├─ KnownUsers/RefIndex/Sessions.Flush() for each account
        ├─ KnownUsers/RefIndex/Sessions.Close() for each account
        ├─ APIClient.Close() for each account
        └─ DB.Close()
```
