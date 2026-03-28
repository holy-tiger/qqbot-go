# 配置参考

## 命令行参数

| 参数 | 默认值 | 是否必填 | 描述 |
|------|--------|----------|------|
| `-config` | (自动检测 `configs/config.yaml`) | 否* | YAML 配置文件路径 |
| `-health` | `:8080` | 否 | 健康检查 HTTP 监听地址（空字符串表示禁用） |
| `-api` | `:9090` | 否 | HTTP API 服务器监听地址（空字符串表示禁用） |

\* 当未指定 `-config` 时，程序会检查当前目录下是否存在 `configs/config.yaml`，如果存在则自动使用。如果两者都不可用，则启动失败。

**使用示例：**

```bash
qqbot -config configs/config.yaml -health :8080 -api :9090
```

---

## 配置文件结构

YAML 配置文件有一个顶级 `qqbot` 键：

```yaml
qqbot:
  # 默认账号字段（内联）
  appId: "..."
  # ...

  # 多账号覆盖
  accounts:
    account-name:
      appId: "..."
      # ...
```

`qqbot:` 下的所有顶级字段定义的是**默认账号**（ID: `"default"`）。命名账号定义在 `qqbot.accounts:` 下，不继承默认账号的任何字段——每个账号完全独立。

---

## 默认账号字段

这些字段设置在 `qqbot:` 的顶级下，应用于 `"default"` 账号。

### 认证

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `appId` | string | (无) | QQ Bot 应用 ID。**必填。** |
| `clientSecret` | string | (无) | 用于令牌认证的客户端密钥。 |
| `clientSecretFile` | string | (无) | 包含客户端密钥的文件路径。如果同时设置了两者，优先级高于 `clientSecret`。 |

### 身份

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `enabled` | bool | `true` | 账号是否启用。禁用的账号在启动时会被跳过。 |
| `name` | string | (无) | 机器人可读名称。 |

### 消息

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `markdownSupport` | bool | `true` | 在外发消息中启用 Markdown 格式。 |
| `systemPrompt` | string | (无) | AI 系统提示词（用于使用 LLM 的集成）。 |

### 图床

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `imageServerBaseUrl` | string | (无) | 本地图床服务器的基础 URL。消息中非 HTTP 协议的图片路径会添加此前缀。 |

### 访问控制

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `dmPolicy` | string | (无) | 私聊策略：`"open"`（允许所有）或 `"allowlist"`（限制为 `allowFrom` 列表）。 |
| `allowFrom` | []string | (无) | 当 `dmPolicy` 为 `"allowlist"` 时，允许私聊机器人的用户 ID 列表。使用 `"*"` 允许所有用户。 |

### 音频处理

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `voiceDirectUploadFormats` | []string | `["silk"]` | 跳过 SILK 编码直接上传的音频格式。 |
| `audioFormatPolicy.sttDirectFormats` | []string | `["wav", "mp3"]` | 语音转文字（STT）跳过转码的音频格式。 |
| `audioFormatPolicy.uploadDirectFormats` | []string | `["silk"]` | 跳过 SILK 编码直接上传到 QQ Bot 的音频格式。 |

### 语音

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `ttsVoice` | string | (无) | TTS 语音选择（由 TTS 提供方用于语音合成）。 |

### Webhook

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `webhookUrl` | string | (无) | 每个账号的事件转发 Webhook URL。对于默认账号，优先级高于 `defaultWebhookUrl`。 |
| `defaultWebhookUrl` | string | (无) | 全局默认 Webhook URL，适用于所有未设置 `webhookUrl` 的账号。 |

---

## 多账号配置

命名账号定义在 `qqbot.accounts:` 下。每个账号完全独立，**不**继承默认账号的字段。

```yaml
qqbot:
  # 默认账号
  appId: "app-id-1"
  clientSecret: "secret-1"
  defaultWebhookUrl: "http://localhost:3000/webhook"

  accounts:
    second-bot:
      appId: "app-id-2"
      clientSecret: "secret-2"
      enabled: true
      name: "Second Bot"
      webhookUrl: "http://localhost:3000/webhook/second-bot"

    third-bot:
      appId: "app-id-3"
      clientSecretFile: "/secrets/third-bot-secret.txt"
      enabled: false
      # 此账号在启动时会被跳过
```

命名账号支持与默认账号相同的字段（除 `defaultWebhookUrl` 外的所有字段，该字段为全局专用设置）。

### 账号 ID 解析

| 场景 | 账号 ID |
|------|---------|
| `qqbot:` 下的顶级字段 | `"default"` |
| `qqbot.accounts.second-bot:` 下的命名条目 | `"second-bot"` |

账号 ID 用作 API 端点中的 `{id}` 路径参数。

---

## 环境变量

环境变量**仅作为默认账号的回退值**使用。`accounts:` 下的命名账号不使用环境变量回退。

| 变量 | 映射到 | 优先级 |
|------|--------|--------|
| `QQBOT_APP_ID` | `appId` | 最低（配置 > 文件 > 环境变量） |
| `QQBOT_CLIENT_SECRET` | `clientSecret` | 最低（配置 > 文件 > 环境变量） |
| `QQBOT_IMAGE_SERVER_BASE_URL` | `imageServerBaseUrl` | 配置值为空时的回退值 |

### 密钥解析顺序

对于默认账号，`clientSecret` 按以下优先级解析：

```
1. qqbot.clientSecret (配置值)
2. qqbot.clientSecretFile (读取文件内容)
3. QQBOT_CLIENT_SECRET (环境变量)
```

对于命名账号，仅适用选项 1 和 2（无环境变量回退）。

---

## Webhook 配置

### URL 解析

每个账号的 Webhook URL 按以下优先级解析：

```
1. 每账号 webhookUrl      (qqbot.accounts.<name>.webhookUrl)
2. 全局 defaultWebhookUrl    (qqbot.defaultWebhookUrl)
3. 无 webhook（事件不转发）
```

### 行为

- 配置了 Webhook URL 时，所有收到的消息事件通过异步 HTTP POST 转发。
- 投递最多重试 3 次，采用指数退避策略（1s、2s）。
- 请求超时：每次尝试 10 秒。
- 事件转发为非阻塞操作——不会延迟网关消息处理。
- Webhook 请求不添加认证头。

### Webhook 载荷

所有消息事件使用以下 JSON 结构转发：

```json
{
  "account_id": "default",
  "event_type": "C2C_MESSAGE_CREATE",
  "timestamp": "2026-01-01T00:00:00Z",
  "data": { ... }
}
```

| 字段 | 描述 |
|------|------|
| `account_id` | 接收事件的账号 |
| `event_type` | 事件类型：`C2C_MESSAGE_CREATE`、`GROUP_AT_MESSAGE_CREATE`、`GUILD_MESSAGE_CREATE` 或 `DIRECT_MESSAGE_CREATE` |
| `timestamp` | 事件时间戳（ISO 8601） |
| `data` | 原始事件载荷（因事件类型而异） |

---

## Channel Server（MCP）

Channel Server 通过 MCP（Model Context Protocol）stdio 传输层将 QQ Bot 事件桥接到 CodeBuddy Code。它作为 MCP 服务器运行，在 `.mcp.json` 中配置，支持两种部署模式：

### 部署模式

**1. 内嵌模式**（推荐）

作为主 `qqbot` 二进制的子命令运行，共享同一进程和配置。

```json
{
  "mcpServers": {
    "qq-channel": {
      "command": "./qqbot",
      "args": ["channel", "-config", "/path/to/config.yaml"]
    }
  }
}
```

**2. 独立模式**

作为单独的二进制文件运行（`cmd/qqbot-channel`），通过 HTTP API 连接 qqbot。

| 参数 | 默认值 | 描述 |
|------|--------|------|
| `-webhook-port` | `8788` | 接收 qqbot webhook 事件的 HTTP 端口 |
| `-qqbot-api` | `http://127.0.0.1:9090` | qqbot HTTP API 地址（用于发送回复） |
| `-account` | `default` | 用于回复路由的默认 QQ Bot 账号 ID |

```json
{
  "mcpServers": {
    "qq-channel": {
      "command": "./qqbot-channel",
      "args": ["-qqbot-api", "http://127.0.0.1:9090", "-account", "default"]
    }
  }
}
```

### 工作原理

```
QQ Bot Gateway → qqbot webhook 转发 → HTTP POST 到 Channel Server
                                                  │
                                                  ▼
                                          Channel Server
                                          ├─ 解析事件
                                          ├─ 提取内容 + 附件
                                          └─ 发送 MCP 通知
                                                  │
                                                  ▼
                                        CodeBuddy Code（MCP 客户端）
                                                  │
                                                  ▼
                                          AI 处理消息
                                                  │
                                                  ▼
                                          调用 "reply" 工具
                                                  │
                                                  ▼
                                        Channel Server → qqbot HTTP API
                                                          │
                                                          ▼
                                                  QQ Bot Gateway
```

### MCP 能力

Channel Server 声明了两个实验性能力：

- `claude/channel` — 告知 CodeBuddy Code 此服务器提供消息通道
- `claude/channel/permission` — 启用权限中继，允许将工具调用审批请求转发到 QQ 进行远程审批

### 权限审批

当 CodeBuddy Code 需要用户审批某个工具调用时，权限请求会通过 MCP 通知推送到 QQ。用户可以通过回复 `"yes <id>"` 或 `"no <id>"` 进行审批，审批结果会回传给 CodeBuddy Code。

### MCP 工具

| 工具 | 参数 | 描述 |
|------|------|------|
| `reply` | `chat_id`（必填）、`text`（必填）、`media_type`（可选）、`media_url`（可选） | 发送文本或富媒体回复到 QQ 会话 |
| `remind` | `chat_id`（必填）、`text`（必填）、`schedule`（可选） | 设置定时提醒，到时间后发送文本消息。仅支持 c2c 和 group |
| `cancel_reminder` | `job_id`（必填） | 取消已创建的定时提醒 |

| 参数 | 必填 | 描述 |
|------|------|------|
| `chat_id` | 是 | 会话 ID（见下方格式表） |
| `text` | 是 | 文本内容。语音消息时作为 TTS 输入文本。 |
| `media_type` | 否 | 媒体类型：`image`、`file`、`voice`、`video` |
| `media_url` | 否 | 媒体文件 URL（图片/文件/视频时必填；语音时可选） |

`chat_id` 格式由会话类型决定：

| 类型 | 格式 | 示例 |
|------|------|------|
| C2C（私聊） | `c2c:{user_openid}` | `c2c:o_abc123` |
| 群聊 | `group:{group_openid}` | `group:grp_abc123` |
| 频道消息 | `channel:{channel_id}` | `channel:12345` |
| 频道私信 | `dm:{channel_id}` | `dm:12345` |

### MCP 通知

当新消息到达时，Channel Server 向 MCP 客户端（CodeBuddy Code）发送通知：

- **方法：** `notifications/claude/channel`
- **Payload：** `{ "content": "...", "meta": { "source": "qq", "sender": "...", "chat_id": "..." } }`

### 设置步骤

**内嵌模式：**

1. 编译主程序：`go build -o qqbot ./cmd/qqbot`
2. 在 `.mcp.json` 中配置内嵌子命令（见上文）。
3. 内嵌模式读取与 qqbot 主进程相同的配置，包括 webhook 和 TTS 设置。

**独立模式：**

1. 编译 Channel Server：`go build -o qqbot-channel ./cmd/qqbot-channel`
2. 配置 qqbot webhook 转发指向 Channel Server 的 webhook 端口：

```yaml
qqbot:
  defaultWebhookUrl: "http://127.0.0.1:8788/webhook"
```

3. 项目根目录的 `.mcp.json` 文件将 Channel Server 注册到 CodeBuddy Code，无需额外配置。

---

## 校验

配置在启动时进行校验。如果校验失败，服务将拒绝启动。

### 错误（阻止启动）

| 条件 | 消息 |
|------|------|
| 缺少 `qqbot:` 部分 | `"config: qqbot section is required"` |
| 顶级账号无 `appId` | `"config: top-level account has no appId configured"` |
| 命名账号无 `appId` | `"config: account \"name\" has no appId configured"` |
| 没有账号有有效的 `appId` | `"config: at least one account must have a non-empty appId"` |
| 重复的账号 ID | `"config: duplicate account ID \"name\""` |

### 警告（启动继续）

| 条件 | 消息 |
|------|------|
| 账号无 `clientSecret` | `"config: account \"name\" has no clientSecret configured"` |
| 顶级账号无 `clientSecret` | `"config: top-level account has no clientSecret configured"` |

密钥可以在运行时通过环境变量提供，因此缺少 `clientSecret` 是警告而非错误。

---

## 完整示例

```yaml
qqbot:
  # ====== 默认账号 (id: "default") ======
  appId: "1234567890"
  clientSecret: "your-client-secret-here"

  enabled: true
  name: "My QQ Bot"
  markdownSupport: true

  # AI 系统提示词（用于 LLM 集成）
  systemPrompt: "You are a helpful assistant."

  # 健康检查地址和 API 服务器地址通过命令行参数设置：
  # -health :8080 -api :9090

  # 全局 webhook（由未设置每账号 webhookUrl 的账号使用）
  defaultWebhookUrl: "http://your-server:3000/webhook"

  # 图床服务器（用于本地图片 URL 解析）
  imageServerBaseUrl: "http://your-ip:18765"

  # 私聊访问控制
  dmPolicy: "open"
  allowFrom:
    - "*"

  # 音频处理选项
  voiceDirectUploadFormats:
    - "silk"
  audioFormatPolicy:
    sttDirectFormats:
      - "wav"
      - "mp3"
    uploadDirectFormats:
      - "silk"

  # TTS 语音
  ttsVoice: "default"

  # ====== 多账号配置 ======
  accounts:
    second-bot:
      appId: "0987654321"
      clientSecret: "second-bot-secret"
      enabled: true
      name: "Second Bot"
      systemPrompt: "You are another helpful assistant."
      # 为此账号覆盖全局 webhook
      webhookUrl: "http://your-server:3000/webhook/second-bot"

    disabled-bot:
      appId: "1111111111"
      enabled: false  # 此账号会被跳过
```

---

## 文件路径

| 路径 | 描述 |
|------|------|
| `configs/config.yaml` | 默认配置文件路径（自动检测） |
| `configs/config.example.yaml` | 包含所有选项文档的示例配置 |
| `data/qqbot.db` | SQLite 数据库（运行时创建） |
