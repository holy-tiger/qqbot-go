# HTTP API 参考文档

所有 API 端点以 `/api/v1` 为前缀。无需认证（设计用于内部网络）。所有端点接受并返回 `Content-Type: application/json`。

## 服务

| 服务 | CLI 标志 | 描述 |
|--------|----------|-------------|
| 健康检查 | `-health` | 存活状态和账号连接状态（独立端口） |
| API 服务器 | `-api` | 所有 `/api/v1/` 端点（标志为空时禁用） |

健康检查服务器运行在与 API 服务器**不同的** HTTP 服务器和端口上。

## 路径参数

| 参数 | 描述 |
|-----------|-------------|
| `{id}` | 账号 ID。顶层配置账号使用 `"default"`，或使用配置中 `qqbot.accounts` 下的任意命名账号 ID。 |
| `{openid}` | C2C 用户 OpenID 或群组 OpenID，取决于端点上下文。 |
| `{channelID}` | 频道频道 ID。 |
| `{remID}` | 提醒任务 ID，由创建提醒的响应返回（格式：`rem-{unix_nano}`）。 |

## 响应格式

### API 服务器 (`/api/v1/`)

所有端点使用统一的 JSON 信封格式：

```json
{"ok": true, "data": { ... }}
{"ok": false, "error": "error message"}
```

### 健康检查服务器 (`/health`, `/healthz`)

健康检查端点使用**不同的**响应格式（无 `ok`/`data` 信封）：

```json
{
  "status": "ok",
  "uptime": "2h30m15s",
  "version": "0.1.0",
  "accounts": [...],
  "timestamp": "2026-03-25T12:00:00Z"
}
```

### 错误码

| HTTP 状态码 | 使用场景 | 错误信息 |
|-------------|-----------|---------------|
| `400` | JSON 正文解码失败 | `"invalid request body"` |
| `400` | 创建提醒时 `target_type` 无效 | `"target_type must be 'c2c' or 'group'"` |
| `400` | 创建提醒时缺少 `target_address` | `"target_address is required"` |
| `404` | 账号 `{id}` 未找到 | `"account not found"` |
| `404` | 提醒 `{remID}` 未找到 | `"reminder not found"` |
| `405` | 健康检查端点上使用非 GET 方法 | （空正文） |
| `500` | 上游操作失败 | 底层服务返回的实际 Go 错误信息 |

---

## 健康检查

### GET `/health` / GET `/healthz`

返回服务健康状态和所有账号连接状态。运行在健康检查服务器上（与 API 服务器分离）。

**响应：**

```json
{
  "status": "ok",
  "uptime": "1h30m0s",
  "version": "0.1.0",
  "accounts": [
    {"id": "default", "connected": true},
    {"id": "bot2", "connected": false, "token_status": "...", "error": "..."}
  ],
  "timestamp": "2026-03-25T12:00:00Z"
}
```

| 字段 | 类型 | 描述 |
|-------|------|-------------|
| `status` | string | 始终为 `"ok"` |
| `uptime` | string | 服务启动后的运行时间（例如 `"2h30m15s"`） |
| `version` | string | 服务版本（当前为 `"0.1.0"`） |
| `accounts` | array | 每个账号的健康信息（未配置账号时省略） |
| `accounts[].id` | string | 账号 ID |
| `accounts[].connected` | bool | 网关 WebSocket 是否已连接 |
| `accounts[].token_status` | string | Token 刷新状态（已连接时省略） |
| `accounts[].error` | string | 连接失败时的错误信息（已连接时省略） |
| `timestamp` | string | 服务器时间，RFC3339 UTC 格式 |

---

## 账号状态

### GET `/api/v1/accounts`

列出所有已配置的账号及其连接状态。

**响应：**

```json
{
  "ok": true,
  "data": [
    {"id": "default", "connected": true},
    {"id": "bot2", "connected": false}
  ]
}
```

### GET `/api/v1/accounts/{id}`

获取单个账号的状态。

**响应（成功）：**

```json
{
  "ok": true,
  "data": {"id": "default", "connected": true}
}
```

**响应（账号未找到）：**

```json
{"ok": false, "error": "account not found"}
```

---

## 消息发送

### 文本消息

#### POST `/api/v1/accounts/{id}/c2c/{openid}/messages`

向 C2C（私聊）用户发送文本消息。

**请求：**

```json
{
  "content": "Hello!",
  "msg_id": "optional_message_id_for_reply"
}
```

| 字段 | 类型 | 必填 | 描述 |
|-------|------|----------|-------------|
| `content` | string | 是 | 要发送的文本内容。支持媒体标签以发送混合内容（参见[媒体标签](#media-tags)）。 |
| `msg_id` | string | 否 | 用于被动回复的原始消息 ID。设置后，消息将作为回复发送（每个 msg_id 每小时限 4 次回复）。省略时使用主动发送。 |

**响应：**

```json
{"ok": true, "data": {"status": "sent"}}
```

#### POST `/api/v1/accounts/{id}/groups/{openid}/messages`

向群组发送文本消息。

请求/响应格式与 C2C 文本消息相同。

#### POST `/api/v1/accounts/{id}/channels/{channelID}/messages`

向频道频道发送文本消息。

请求/响应格式与 C2C 文本消息相同。

### 图片消息

#### POST `/api/v1/accounts/{id}/c2c/{openid}/images`

向 C2C 用户发送图片。

**请求：**

```json
{
  "image_url": "https://example.com/photo.jpg",
  "content": "Check this out!",
  "msg_id": "optional_message_id"
}
```

| 字段 | 类型 | 必填 | 描述 |
|-------|------|----------|-------------|
| `image_url` | string | 是 | 要发送的图片 URL。相对路径（非 HTTP）将基于配置的图片服务器基础 URL 进行解析。 |
| `content` | string | 否 | 附带的图片说明文字。 |
| `msg_id` | string | 否 | 用于被动回复的原始消息 ID。 |

**响应：**

```json
{"ok": true, "data": {"status": "sent"}}
```

#### POST `/api/v1/accounts/{id}/groups/{openid}/images`

向群组发送图片。请求/响应格式与 C2C 图片消息相同。

### 语音消息

#### POST `/api/v1/accounts/{id}/c2c/{openid}/voice`

向 C2C 用户发送语音消息。

**请求：**

```json
{
  "voice_base64": "SGVsbG8gV29ybGQ=",
  "tts_text": "optional TTS text",
  "msg_id": "optional_message_id"
}
```

| 字段 | 类型 | 必填 | 描述 |
|-------|------|----------|-------------|
| `voice_base64` | string | 是 | Base64 编码的语音数据（SILK 格式）。 |
| `tts_text` | string | 否 | 用于 TTS（文字转语音）合成的文本。提供后，系统将从此文本合成语音，而非使用 `voice_base64`。 |
| `msg_id` | string | 否 | 用于被动回复的原始消息 ID。 |

**响应：**

```json
{"ok": true, "data": {"status": "sent"}}
```

#### POST `/api/v1/accounts/{id}/groups/{openid}/voice`

向群组发送语音消息。请求/响应格式相同，但 `tts_text` 不会被转发（群组语音仅使用 `voice_base64`）。

### 视频消息

#### POST `/api/v1/accounts/{id}/c2c/{openid}/videos`

向 C2C 用户发送视频。

**请求：**

```json
{
  "video_url": "https://example.com/video.mp4",
  "video_base64": "optional_base64_data",
  "content": "Watch this!",
  "msg_id": "optional_message_id"
}
```

| 字段 | 类型 | 必填 | 描述 |
|-------|------|----------|-------------|
| `video_url` | string | 是* | 视频文件 URL。`video_url` 和 `video_base64` 必须提供其中之一。 |
| `video_base64` | string | 是* | Base64 编码的视频数据。`video_url` 的替代方案。 |
| `content` | string | 否 | 附带的视频说明文字。 |
| `msg_id` | string | 否 | 用于被动回复的原始消息 ID。 |

**响应：**

```json
{"ok": true, "data": {"status": "sent"}}
```

#### POST `/api/v1/accounts/{id}/groups/{openid}/videos`

向群组发送视频。请求/响应格式与 C2C 视频消息相同。

### 文件消息

#### POST `/api/v1/accounts/{id}/c2c/{openid}/files`

向 C2C 用户发送文件。

**请求：**

```json
{
  "file_url": "https://example.com/document.pdf",
  "file_base64": "optional_base64_data",
  "file_name": "report.pdf",
  "msg_id": "optional_message_id"
}
```

| 字段 | 类型 | 必填 | 描述 |
|-------|------|----------|-------------|
| `file_url` | string | 是* | 文件 URL。`file_url` 和 `file_base64` 必须提供其中之一。 |
| `file_base64` | string | 是* | Base64 编码的文件数据。`file_url` 的替代方案。 |
| `file_name` | string | 是 | 文件的显示名称。 |
| `msg_id` | string | 否 | 用于被动回复的原始消息 ID。 |

**响应：**

```json
{"ok": true, "data": {"status": "sent"}}
```

#### POST `/api/v1/accounts/{id}/groups/{openid}/files`

向群组发送文件。请求/响应格式与 C2C 文件消息相同。

### 回复频率限制

当提供 `msg_id` 时，消息将作为对原始用户消息的**被动回复**发送。QQ Bot 平台对被动回复有以下限制：

- 每个 `msg_id` 在首次回复后的 **1 小时窗口**内最多回复 **4 次**。
- 超过限制或 1 小时窗口过期时，系统将**自动回退**到主动发送（`msg_id` 被清除，改用主动发送方式）。
- 此回退对 API 调用方是透明的——请求仍然会成功。

### 媒体标签

消息端点中的文本内容支持嵌入媒体标签，可以在一条消息中发送混合的文本和媒体内容。标签从文本中解析，并按顺序作为独立的消息段发送。

**支持的标签格式：**

| 标签 | 媒体类型 | 描述 |
|-----|-----------|-------------|
| `<qqimg>url_or_path</qqimg>` | 图片 | 发送内联图片 |
| `<qqvoice>file_path</qqvoice>` | 语音 | 发送内联语音片段 |
| `<qqvideo>url_or_path</qqvideo>` | 视频 | 发送内联视频 |
| `<qqfile>file_path</qqfile>` | 文件 | 发送内联文件 |

**混合内容示例：**

```json
{"content": "Here is the chart:\n<qqimg>https://example.com/chart.png</qqimg>\nAnd the report:\n<qqfile>/data/report.pdf</qqfile>"}
```

还支持标签别名（例如 `<image>`、`<pic>`、`<voice>`、`<audio>`、`<doc>`、`<document>`）。标准化器还会处理全角括号、多行标签和反引号包裹的标签。

---

## 主动发送与广播

主动消息独立于任何传入的用户消息发送（不需要 `msg_id` 或回复上下文）。

### POST `/api/v1/accounts/{id}/proactive/c2c/{openid}`

向 C2C 用户发送主动文本消息。

**请求：**

```json
{"content": "Reminder: meeting at 3pm!"}
```

**响应：**

```json
{"ok": true, "data": {"status": "sent"}}
```

### POST `/api/v1/accounts/{id}/proactive/groups/{openid}`

向群组发送主动文本消息。

请求/响应格式相同。

### POST `/api/v1/accounts/{id}/broadcast`

向该账号的**所有已知 C2C 用户**广播文本消息。当用户向机器人发送消息时，用户会被自动记录。

**请求：**

```json
{"content": "Important announcement to all users!"}
```

**响应：**

```json
{
  "ok": true,
  "data": {
    "sent": 42,
    "errors": ["failed to send to user abc123: ..."]
  }
}
```

> **注意：** 即使个别发送失败，广播也始终返回 HTTP 200。`errors` 数组包含所有失败接收者的错误信息。调用方应检查 `sent` 和 `errors` 来判断广播结果。

### POST `/api/v1/accounts/{id}/broadcast/groups`

向该账号的**所有已知群组**广播文本消息。群组按 Group OpenID 自动去重。

**请求和响应格式**与 C2C 广播端点相同。

---

## 定时任务 / 提醒

提醒是持久化的定时任务，会在指定时间发送主动消息。它们存储在 SQLite 中，服务重启后仍然保留。调度器每 100ms 检查一次到期任务。

### POST `/api/v1/accounts/{id}/reminders`

创建新提醒。

**请求：**

```json
{
  "content": "Time for your daily standup!",
  "target_type": "c2c",
  "target_address": "user_openid_here",
  "schedule": "@every 1h"
}
```

| 字段 | 类型 | 必填 | 描述 |
|-------|------|----------|-------------|
| `content` | string | 否 | 提醒触发时要发送的文本消息。 |
| `target_type` | string | 是 | `"c2c"` 或 `"group"`。 |
| `target_address` | string | 是 | 接收者 OpenID（C2C 用户 OpenID 或群组 OpenID）。 |
| `schedule` | string | 否 | 调度表达式。省略或为空时，提醒将立即触发（一次性）。参见[调度语法](#schedule-syntax)。 |

**响应：**

```json
{
  "ok": true,
  "data": {
    "job_id": "rem-1709123456789123456",
    "next_run": "2026-03-25T13:00:00Z",
    "schedule": "@every 1h"
  }
}
```

### 调度语法

支持两种调度格式：

**1. 间隔 (`@every`)**

```
@every 30s    // every 30 seconds
@every 5m     // every 5 minutes
@every 1h     // every 1 hour
```

使用 Go `time.ParseDuration` 格式。

**2. Cron 表达式（5 个字段）**

```
┌───────────── minute   (0-59)
│ ┌───────────── hour     (0-23)
│ │ ┌───────────── day     (1-31)
│ │ │ ┌───────────── month   (1-12)
│ │ │ │ ┌───────────── weekday (0-6, 0=Sunday)
│ │ │ │ │
* * * * *
```

支持的语法：`*`（任意值）、具体值（`30`）、范围（`1-5`）、步长（`*/15`、`1-30/5`）。日和星期使用 OR 逻辑（标准 POSIX cron 行为）。

**示例：**

```
0 9 * * *       // every day at 9:00 AM
*/30 * * * *    // every 30 minutes
0 9 * * 1-5     // weekdays at 9:00 AM
0 0 1,15 * *    // 1st and 15th of each month at midnight
```

### DELETE `/api/v1/accounts/{id}/reminders/{remID}`

取消并删除提醒。

**响应（成功）：**

```json
{"ok": true, "data": {"status": "cancelled"}}
```

**响应（未找到）：**

```json
{"ok": false, "error": "reminder not found"}
```

HTTP 状态码：`404`。

### GET `/api/v1/accounts/{id}/reminders`

列出该账号的所有提醒。

**响应：**

```json
{
  "ok": true,
  "data": [
    {
      "ID": "rem-1709123456789123456",
      "Content": "Daily standup reminder",
      "TargetType": "c2c",
      "TargetAddress": "user_openid",
      "AccountID": "default",
      "Schedule": "@every 1h",
      "NextRun": "2026-03-25T14:00:00Z",
      "CreatedAt": "2026-03-25T12:00:00Z"
    }
  ]
}
```

| 字段 | 类型 | 描述 |
|-------|------|-------------|
| `ID` | string | 唯一任务 ID（格式：`rem-{unix_nano}`） |
| `Content` | string | 要发送的消息内容 |
| `TargetType` | string | `"c2c"` 或 `"group"` |
| `TargetAddress` | string | 接收者 OpenID |
| `AccountID` | string | 此提醒所属的账号 |
| `Schedule` | string | 原始调度表达式（一次性提醒为空） |
| `NextRun` | string | 下次执行时间（RFC3339 格式） |
| `CreatedAt` | string | 创建时间（RFC3339 格式） |

一次性提醒（空调度）在执行后自动删除。周期性提醒在每次执行后会重新调度。

---

## 用户管理

当用户向机器人发送消息时，用户会被**自动记录**。C2C 消息将发送者记录为类型 `"c2c"`，群组 @提及消息将发送者记录为类型 `"group"` 并附带其 `GroupOpenID`。每条消息会使 `interaction_count` 递增并更新 `last_seen_at`。

### GET `/api/v1/accounts/{id}/users`

列出已知用户，支持过滤、排序和分页。

**查询参数：**

| 参数 | 类型 | 默认值 | 描述 |
|-----------|------|---------|-------------|
| `type` | string | （全部） | 按用户类型过滤：`"c2c"` 或 `"group"` |
| `active_within` | int64 | （全部） | 仅包含在此毫秒数内活跃的用户 |
| `limit` | int | （全部） | 返回结果的最大数量 |
| `sort_by` | string | `"lastSeenAt"` | 排序字段：`"lastSeenAt"`、`"firstSeenAt"` 或 `"interactionCount"` |
| `sort_order` | string | `"desc"` | 排序方向：`"asc"` 或 `"desc"` |

**示例：**

```
GET /api/v1/accounts/default/users?type=c2c&active_within=86400000&limit=50&sort_by=interaction_count&sort_order=desc
```

**响应：**

```json
{
  "ok": true,
  "data": [
    {
      "openid": "user_openid_abc",
      "type": "c2c",
      "nickname": "Alice",
      "group_openid": "",
      "account_id": "default",
      "first_seen_at": 1700000000000,
      "last_seen_at": 1700086400000,
      "interaction_count": 15
    }
  ]
}
```

| 字段 | 类型 | 描述 |
|-------|------|-------------|
| `openid` | string | 用户 OpenID（C2C）或成员 OpenID（群组） |
| `type` | string | `"c2c"` 或 `"group"` |
| `nickname` | string | 用户昵称（可能为空） |
| `group_openid` | string | 群组 OpenID（仅群组用户） |
| `account_id` | string | 此用户所属的账号 |
| `first_seen_at` | int64 | 首次出现的时间戳（Unix 毫秒） |
| `last_seen_at` | int64 | 最后出现的时间戳（Unix 毫秒） |
| `interaction_count` | int | 此用户的消息总数 |

### GET `/api/v1/accounts/{id}/users/stats`

获取用户汇总统计信息。

**响应：**

```json
{
  "ok": true,
  "data": {
    "total_users": 128,
    "c2c_users": 95,
    "group_users": 33,
    "active_in_24h": 42,
    "active_in_7d": 87
  }
}
```

| 字段 | 类型 | 描述 |
|-------|------|-------------|
| `total_users` | int | 已知用户总数 |
| `c2c_users` | int | C2C 用户数量 |
| `group_users` | int | 群组用户数量 |
| `active_in_24h` | int | 最近 24 小时内活跃的用户 |
| `active_in_7d` | int | 最近 7 天内活跃的用户 |

### DELETE `/api/v1/accounts/{id}/users`

清空该账号的所有已知用户。此操作不可逆。

**响应：**

```json
{"ok": true, "data": {"removed": 128}}
```

---

## Webhook 事件转发

当配置了 `defaultWebhookUrl`（顶层配置）或 `webhookUrl`（每个账号的配置）时，传入的用户消息将通过 HTTP POST 转发到配置的 URL。

### URL 解析优先级

每个账号的 Webhook URL 按以下优先级解析：

1. 配置中每个账号的 `webhookUrl`
2. 顶层的 `defaultWebhookUrl`
3. 未配置 Webhook（事件不会被转发）

### 转发的事件类型

| 事件类型 | 描述 |
|------------|-------------|
| `C2C_MESSAGE_CREATE` | 用户发送的私聊（C2C）消息 |
| `GROUP_AT_MESSAGE_CREATE` | 机器人被 @提及 的群组消息 |
| `GUILD_MESSAGE_CREATE` | 机器人被 @提及 的频道频道消息 |
| `DIRECT_MESSAGE_CREATE` | 用户发送的频道私信 |

网关生命周期事件（`READY`、`RESUMED`）**不会**被转发。

### Webhook 载荷

```json
{
  "account_id": "default",
  "event_type": "C2C_MESSAGE_CREATE",
  "timestamp": "2026-03-25T12:00:00Z",
  "data": {
    "id": "message_id",
    "author": {
      "id": "user_id",
      "user_openid": "user_openid_value"
    },
    "content": "Hello bot!",
    "timestamp": "2026-03-25T12:00:00+08:00",
    "attachments": []
  }
}
```

| 字段 | 类型 | 描述 |
|-------|------|-------------|
| `account_id` | string | 接收事件的账号 ID |
| `event_type` | string | 上述转发事件类型之一 |
| `timestamp` | string | 转发时间，RFC3339 UTC 格式 |
| `data` | object | 来自 QQ Bot 网关的原始事件载荷（结构因事件类型而异） |

### 投递行为

- **异步**：Webhook 投递对网关是非阻塞的。事件在后台 goroutine 中分发。
- **HTTP 方法**：始终使用 `POST`，`Content-Type: application/json`。
- **超时**：每次请求尝试 10 秒超时。
- **重试**：最多 3 次尝试，使用指数退避策略：
  - 第 1 次：立即重试
  - 第 2 次：1 秒后
  - 第 3 次：2 秒后
- **成功**：HTTP 状态码 < 400。
- **失败**：所有重试耗尽后，事件将被丢弃并记录错误日志。无死信队列。
