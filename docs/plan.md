# Go 实现计划：openclaw-qqbot

## 需求重述

将现有的 TypeScript QQ Bot 通道插件用 Go 语言重新实现。该插件作为 AI Agent 框架（OpenClaw）与 QQ 消息平台之间的桥梁，提供：

- **消息收发**：C2C 私聊、群聊 @mention、频道消息
- **富媒体**：图片、语音、视频、文件的上传和发送
- **语音 STT/TTS**：SILK 格式转换、语音转文字、文字转语音
- **主动消息**：定时提醒、主动推送、广播
- **限流与重连**：消息回复限流（4次/小时）、指数退避重连
- **图床服务**：本地图片托管 HTTP 服务
- **多账号**：支持多个 QQ Bot 账号并发运行

---

## 实现阶段

### Phase 1: 项目骨架与基础类型定义

**目标**：搭建 Go 项目结构，定义核心数据类型和配置解析。

| 步骤 | 内容 |
|------|------|
| 1.1 | 创建 Go module，设计目录结构（`cmd/`, `internal/`, `pkg/`） |
| 1.2 | 定义核心类型（对应 `types.ts`）：`QQBotConfig`, `ResolvedQQBotAccount`, `C2CMessageEvent`, `GuildMessageEvent`, `GroupMessageEvent`, `MessageAttachment`, `WSPayload`, `AudioFormatPolicy` |
| 1.3 | 实现配置解析（对应 `config.ts`）：从 JSON/YAML 配置文件解析多账号配置，支持 secret 从文件/环境变量读取 |
| 1.4 | 实现日志系统：结构化日志（`slog`），按 accountId 隔离日志上下文 |

**目录结构设计**：

```
qqbot/
├── cmd/
│   └── qqbot/
│       └── main.go              # 入口
├── internal/
│   ├── config/
│   │   └── config.go            # 配置解析
│   ├── types/
│   │   └── types.go             # 核心类型定义
│   ├── api/
│   │   ├── client.go            # QQ Bot API 客户端
│   │   ├── token.go             # Token 管理（缓存 + singleflight）
│   │   ├── media.go             # 富媒体上传/发送
│   │   └── message.go           # 消息发送接口
│   ├── gateway/
│   │   ├── gateway.go           # WebSocket 网关
│   │   ├── intents.go           # 权限级别定义
│   │   ├── reconnect.go         # 重连策略
│   │   └── queue.go             # 消息队列与并发控制
│   ├── outbound/
│   │   ├── outbound.go          # 出站消息处理
│   │   ├── ratelimit.go         # 消息回复限流器
│   │   └── tags.go              # 富媒体标签解析
│   ├── audio/
│   │   ├── silk.go              # SILK 编解码（调用 silk-sdk CGO）
│   │   ├── convert.go           # 音频格式转换
│   │   └── stt.go               # 语音转文字（OpenAI 兼容 API）
│   ├── image/
│   │   ├── server.go            # 本地图床 HTTP 服务
│   │   └── size.go              # 图片尺寸解析
│   ├── store/
│   │   ├── known_users.go       # 已知用户存储
│   │   ├── ref_index.go         # 消息引用索引存储
│   │   ├── session.go           # WebSocket 会话持久化
│   │   └── upload_cache.go      # 文件上传缓存
│   ├── proactive/
│   │   └── proactive.go         # 主动消息管理
│   └── utils/
│       ├── fileutil.go          # 文件工具
│       ├── platform.go          # 平台检测
│       ├── payload.go           # 结构化消息编解码
│       └── mediatags.go         # 媒体标签模糊匹配
├── go.mod
├── go.sum
└── configs/
    └── config.example.yaml      # 示例配置
```

**Go 依赖选型**：

| 功能 | Go 库 |
|------|-------|
| WebSocket | `github.com/gorilla/websocket` |
| YAML 配置 | `gopkg.in/yaml.v3` |
| HTTP 客户端 | `net/http`（标准库） |
| HTTP 服务器 | `net/http`（标准库） |
| SILK 编解码 | CGO 封装腾讯 silk-sdk 或调用 ffmpeg |
| 日志 | `log/slog`（Go 1.21+ 标准库） |
| 单元测试 | `testing`（标准库） |
| 并发控制 | `golang.org/x/sync/singleflight` |

---

### Phase 2: API 客户端与 Token 管理

**目标**：实现 QQ Bot REST API 客户端，包括鉴权、消息发送、媒体上传。

| 步骤 | 内容 |
|------|------|
| 2.1 | **Token 管理**（对应 `api.ts:60-200`）：`singleflight` 模式获取 access_token，按 appId 隔离缓存，提前 5 分钟刷新 |
| 2.2 | **后台 Token 刷新**（对应 `api.ts:670-786`）：goroutine 循环刷新，支持 AbortController 语义（`context.Context`） |
| 2.3 | **API 请求封装**（对应 `api.ts:208-288`）：通用 HTTP 请求方法，超时控制，JSON 编解码，错误处理 |
| 2.4 | **消息发送接口**：`SendC2CMessage`, `SendGroupMessage`, `SendChannelMessage`, `SendProactiveC2CMessage` |
| 2.5 | **富媒体上传**（对应 `api.ts:478-665`）：`UploadC2CMedia`, `UploadGroupMedia`，支持 URL 和 Base64，指数退避重试 |
| 2.6 | **Markdown 消息支持**：`msg_type=2` markdown 格式 vs `msg_type=0` 纯文本自动切换 |
| 2.7 | **上传缓存**（对应 `upload-cache.ts`）：MD5 哈希缓存 file_info，避免重复上传 |

---

### Phase 3: WebSocket 网关

**目标**：实现与 QQ Bot WebSocket 网关的连接、事件接收、消息路由。

| 步骤 | 内容 |
|------|------|
| 3.1 | **网关连接**：获取 gateway URL，建立 WebSocket 连接，发送 Identify/Resume 握手 |
| 3.2 | **Intent 权限分级**（对应 `gateway.ts:100-131`）：三级 intent 策略（完整/群聊+频道/仅频道），自动降级 |
| 3.3 | **心跳机制**：定时发送 heartbeat，处理 heartbeat ACK 超时 |
| 3.4 | **事件分发**：解析 `WSPayload`，根据 `op` 和 `t` 分发到对应处理器（C2C/Group/Guild） |
| 3.5 | **消息队列与并发控制**（对应 `gateway.ts:146-148`）：全局队列 1000 条、单用户队列 20 条、最大 10 并发用户 |
| 3.6 | **重连策略**（对应 `gateway.ts:134-138`）：指数退避 [1s, 2s, 5s, 10s, 30s, 60s]，最大 100 次，快速断开检测 |

---

### Phase 4: 出站消息处理与限流

**目标**：实现 AI 回复的发送逻辑，包括文本、富媒体、限流和主动消息降级。

| 步骤 | 内容 |
|------|------|
| 4.1 | **限流器**（对应 `outbound.ts:29-100`）：同一 message_id 1 小时最多 4 次被动回复，超期自动降级为主动消息 |
| 4.2 | **富媒体标签解析**（对应 `outbound.ts`, `media-tags.ts`）：解析 `<qqimg>`, `<qqvoice>`, `<qqvideo>`, `<qqfile>` 标签，支持 ~40 种模糊匹配变体 |
| 4.3 | **图片消息**：下载远程图片 → 解析尺寸 → 上传到 QQ → 发送，支持本地文件和 URL |
| 4.4 | **语音消息**：TTS 合成 → 音频格式转换 → SILK 编码 → Base64 → 上传发送 |
| 4.5 | **视频/文件消息**：文件大小校验（20MB 限制） → 上传发送 |
| 4.6 | **主动消息降级**：当被动回复次数用完时，自动切换为主动消息 API |

---

### Phase 5: 语音处理（SILK 编解码 + STT）

**目标**：实现 QQ 语音消息的接收转写和发送合成。

| 步骤 | 内容 |
|------|------|
| 5.1 | **SILK 解码**：接收 QQ 语音附件 → SILK → WAV 转换 |
| 5.2 | **SILK 编码**：WAV/MP3 → SILK 编码（用于发送语音） |
| 5.3 | **STT 语音转文字**（对应 `gateway.ts:32-98`）：调用 OpenAI 兼容 STT API（Whisper），支持自定义 baseUrl/apiKey |
| 5.4 | **音频格式策略**（对应 `types.ts:61-75`）：STT 直通格式和上传直通格式的可配置化 |
| 5.5 | **MP3 解码**（对应 `audio-convert.ts`）：MP3 → PCM 用于 TTS 管道 |

**SILK 编解码方案选择**：

| 方案 | 优点 | 缺点 |
|------|------|------|
| CGO 封装腾讯 silk-sdk | 性能最佳，官方实现 | 交叉编译困难，需要 C 编译器 |
| 调用 ffmpeg | 生态成熟，无需 CGO | 额外依赖，需要系统安装 ffmpeg |
| Go 纯实现 | 无 CGO | 性能较差，维护成本高 |

**推荐**：优先使用 ffmpeg（通过 `os/exec`），可选 CGO 方案作为高性能后备。

---

### Phase 6: 存储层

**目标**：实现文件持久化存储，替代 TS 版本的 JSON/JSONL 文件存储。

| 步骤 | 内容 |
|------|------|
| 6.1 | **已知用户存储**（对应 `known-users.ts`）：JSON 文件持久化，节流写入，支持过滤和统计 |
| 6.2 | **消息引用索引**（对应 `ref-index-store.ts`）：JSONL 追加写入，内存缓存，7 天 TTL，定期压缩，最大 5 万条 |
| 6.3 | **会话持久化**（对应 `session-store.ts`）：JSON 文件保存 sessionId + lastSeq，支持进程重启后恢复连接 |
| 6.4 | **上传缓存**（对应 `upload-cache.ts`）：MD5 哈希 + file_info 缓存，带 TTL 过期 |

**可选改进**：Go 版本可考虑使用 SQLite（`modernc.org/sqlite`，纯 Go 无 CGO）替代纯 JSON 文件，提升查询性能和数据完整性。

---

### Phase 7: 图床服务与辅助功能

**目标**：实现本地图片托管服务和各种工具函数。

| 步骤 | 内容 |
|------|------|
| 7.1 | **图床 HTTP 服务**（对应 `image-server.ts`）：端口 18765，TTL 过期清理，CORS 支持，路径安全检查 |
| 7.2 | **图片尺寸解析**（对应 `image-size.ts`）：读取 PNG/JPEG/GIF/WebP 头部获取尺寸 |
| 7.3 | **文件工具**（对应 `file-utils.ts`）：文件大小校验（20MB），MIME 类型检测 |
| 7.4 | **平台检测**（对应 `platform.ts`）：跨平台路径处理，ffmpeg 检测，启动诊断 |
| 7.5 | **Payload 编解码**（对应 `payload.ts`）：`QQBOT_PAYLOAD:` 和 `QQBOT_CRON:` 结构化消息处理 |
| 7.6 | **媒体标签模糊匹配**（对应 `media-tags.ts`）：~40 种 LLM 输出的标签变体正则化 |

---

### Phase 8: 主动消息与定时任务

**目标**：实现主动消息推送和定时提醒功能。

| 步骤 | 内容 |
|------|------|
| 8.1 | **主动消息发送**（对应 `proactive.ts`）：向 C2C/群发送主动消息 |
| 8.2 | **用户管理**：查询已知用户列表，统计信息 |
| 8.3 | **广播消息**：向所有已知用户或指定群发送广播 |
| 8.4 | **定时提醒集成**：cron 任务调度，到期后通过 QQ 发送提醒消息 |

---

### Phase 9: 多账号与集成测试

**目标**：实现多账号并发支持和端到端测试。

| 步骤 | 内容 |
|------|------|
| 9.1 | **多账号隔离**：Token 缓存、WebSocket 连接、消息队列、会话存储均按 accountId 隔离 |
| 9.2 | **优雅关闭**：处理 SIGINT/SIGTERM，停止所有 goroutine，保存会话状态 |
| 9.3 | **API 集成测试**：Mock QQ Bot API，测试消息收发全流程 |
| 9.4 | **配置验证**：启动时检查 appId/clientSecret 有效性 |
| 9.5 | **健康检查**：HTTP health endpoint，暴露各账号连接状态 |

---

## 依赖关系

```
Phase 1 (骨架/类型)
  └─> Phase 2 (API 客户端)
        └─> Phase 3 (WebSocket 网关)
              ├─> Phase 4 (出站消息)
              ├─> Phase 5 (语音处理)
              └─> Phase 6 (存储层)
  └─> Phase 7 (图床/工具)
  └─> Phase 8 (主动消息/定时) <-- 依赖 Phase 2 + Phase 6
Phase 9 (多账号/测试) <-- 依赖所有前置阶段
```

## 风险评估

| 风险 | 级别 | 缓解措施 |
|------|------|----------|
| SILK 编解码（无纯 Go 库） | **高** | 优先 ffmpeg 方案，备选 CGO 封装 silk-sdk |
| QQ Bot API 文档不完善 | **中** | 参考 TS 原版实现的 API 调用细节 |
| WebSocket 协议细节兼容性 | **中** | 严格复刻 TS 版本的握手、心跳、Resume 逻辑 |
| 多账号并发状态隔离 | **低** | Go 的 goroutine + context 天然适合隔离 |
| 文件存储并发写入安全 | **低** | Go 的 `sync.Mutex` + 节流写入 |

## 复杂度评估：中高

- 核心逻辑清晰（消息收发），但分支场景多（C2C/Group/Channel 三种场景）
- `gateway.ts` 单文件 3400 行是最大挑战，建议在 Go 中拆分为 6-7 个文件
- SILK 音频编解码是技术风险最高的部分

## 原始项目参考

基于 `/data/tmp/openclaw-qqbot` 项目（TypeScript 实现），版本 1.5.7。

关键源文件映射：

| TS 源文件 | Go 目标模块 | 行数 |
|-----------|-------------|------|
| `src/types.ts` | `internal/types/types.go` | 161 |
| `src/api.ts` | `internal/api/` | 787 |
| `src/gateway.ts` | `internal/gateway/` | ~3400 |
| `src/outbound.ts` | `internal/outbound/` | ~1200 |
| `src/config.ts` | `internal/config/config.go` | 187 |
| `src/channel.ts` | 集成到 `cmd/qqbot/main.go` | 369 |
| `src/image-server.ts` | `internal/image/server.go` | 478 |
| `src/known-users.ts` | `internal/store/known_users.go` | 354 |
| `src/proactive.ts` | `internal/proactive/proactive.go` | 531 |
| `src/ref-index-store.ts` | `internal/store/ref_index.go` | 359 |
| `src/session-store.ts` | `internal/store/session.go` | 304 |
| `src/onboarding.ts` | CLI 子命令 | 274 |
| `src/utils/audio-convert.ts` | `internal/audio/` | ~800 |
| `src/utils/media-tags.ts` | `internal/utils/mediatags.go` | ~200 |
| `src/utils/payload.ts` | `internal/utils/payload.go` | ~150 |
| `src/utils/file-utils.ts` | `internal/utils/fileutil.go` | 123 |
| `src/utils/image-size.ts` | `internal/image/size.go` | ~250 |
| `src/utils/platform.ts` | `internal/utils/platform.go` | ~400 |
| `src/utils/upload-cache.ts` | `internal/store/upload_cache.go` | ~100 |
