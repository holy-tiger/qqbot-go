# HTTP API Reference

All API endpoints are prefixed with `/api/v1`. No authentication is required (designed for internal network use). All endpoints accept and return `Content-Type: application/json`.

## Servers

| Server | CLI Flag | Description |
|--------|----------|-------------|
| Health Check | `-health` | Liveness and account status (separate port) |
| API Server | `-api` | All `/api/v1/` endpoints (disabled when flag is empty) |

The health check server runs on a **separate** HTTP server and port from the API server.

## Path Parameters

| Parameter | Description |
|-----------|-------------|
| `{id}` | Account ID. Use `"default"` for the top-level config account, or any named account ID from `qqbot.accounts` in the config. |
| `{openid}` | C2C user OpenID or Group OpenID, depending on the endpoint context. |
| `{channelID}` | Guild channel ID. |
| `{remID}` | Reminder job ID, returned by the create reminder response (format: `rem-{unix_nano}`). |

## Response Format

### API Server (`/api/v1/`)

All endpoints use a unified JSON envelope:

```json
{"ok": true, "data": { ... }}
{"ok": false, "error": "error message"}
```

### Health Check Server (`/health`, `/healthz`)

Health endpoints use a **different** response format (no `ok`/`data` envelope):

```json
{
  "status": "ok",
  "uptime": "2h30m15s",
  "version": "0.1.0",
  "accounts": [...],
  "timestamp": "2026-03-25T12:00:00Z"
}
```

### Error Codes

| HTTP Status | When Used | Error Message |
|-------------|-----------|---------------|
| `400` | JSON body decode fails | `"invalid request body"` |
| `400` | Invalid `target_type` on reminder creation | `"target_type must be 'c2c' or 'group'"` |
| `400` | Missing `target_address` on reminder creation | `"target_address is required"` |
| `404` | Account `{id}` not found | `"account not found"` |
| `404` | Reminder `{remID}` not found | `"reminder not found"` |
| `405` | Non-GET on health endpoints | (empty body) |
| `500` | Upstream operation failure | The actual Go error message from the underlying service |

---

## Health Check

### GET `/health` / GET `/healthz`

Returns service health and all account connection statuses. Runs on the health check server (separate from the API server).

**Response:**

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

| Field | Type | Description |
|-------|------|-------------|
| `status` | string | Always `"ok"` |
| `uptime` | string | Time since service start (e.g. `"2h30m15s"`) |
| `version` | string | Service version (currently `"0.1.0"`) |
| `accounts` | array | Per-account health info (omitted when no accounts configured) |
| `accounts[].id` | string | Account ID |
| `accounts[].connected` | bool | Whether the gateway WebSocket is connected |
| `accounts[].token_status` | string | Token refresh status (omitted when connected) |
| `accounts[].error` | string | Error message if connection failed (omitted when connected) |
| `timestamp` | string | Server time in RFC3339 UTC |

---

## Account Status

### GET `/api/v1/accounts`

List all configured accounts with their connection status.

**Response:**

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

Get the status of a single account.

**Response (success):**

```json
{
  "ok": true,
  "data": {"id": "default", "connected": true}
}
```

**Response (account not found):**

```json
{"ok": false, "error": "account not found"}
```

---

## Message Sending

### Text Messages

#### POST `/api/v1/accounts/{id}/c2c/{openid}/messages`

Send a text message to a C2C (private) user.

**Request:**

```json
{
  "content": "Hello!",
  "msg_id": "optional_message_id_for_reply"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `content` | string | Yes | Text content to send. Supports media tags for mixed content (see [Media Tags](#media-tags)). |
| `msg_id` | string | No | Original message ID for passive reply. When set, the message is sent as a reply (limited to 4 replies per msg_id per hour). When omitted, uses proactive messaging. |

**Response:**

```json
{"ok": true, "data": {"status": "sent"}}
```

#### POST `/api/v1/accounts/{id}/groups/{openid}/messages`

Send a text message to a group.

Same request/response format as C2C text messages.

#### POST `/api/v1/accounts/{id}/channels/{channelID}/messages`

Send a text message to a guild channel.

Same request/response format as C2C text messages.

### Image Messages

#### POST `/api/v1/accounts/{id}/c2c/{openid}/images`

Send an image to a C2C user.

**Request:**

```json
{
  "image_url": "https://example.com/photo.jpg",
  "content": "Check this out!",
  "msg_id": "optional_message_id"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `image_url` | string | Yes | URL of the image to send. Relative paths (non-HTTP) are resolved against the configured image server base URL. |
| `content` | string | No | Caption text to accompany the image. |
| `msg_id` | string | No | Original message ID for passive reply. |

**Response:**

```json
{"ok": true, "data": {"status": "sent"}}
```

#### POST `/api/v1/accounts/{id}/groups/{openid}/images`

Send an image to a group. Same request/response format as C2C image messages.

### Voice Messages

#### POST `/api/v1/accounts/{id}/c2c/{openid}/voice`

Send a voice message to a C2C user. Supports two modes: **send voice data directly** (`voice_base64`) and **text-to-speech** (`tts_text`). Provide one of the two.

> **TTS Mode**: When `tts_text` is provided, the server automatically calls the built-in edge-tts (Microsoft Edge TTS) to synthesize speech from text and send it. No preprocessing is required by the caller. The default voice is `zh-CN-XiaoxiaoNeural`. To use TTS, ensure edge-tts is installed on the server (`pip install edge-tts`).

**Option 1: TTS text-to-speech (recommended, simplest)**

```json
{
  "tts_text": "Hello, this is a voice message"
}
```

**Option 2: Send voice data directly**

```json
{
  "voice_base64": "SGVsbG8gV29ybGQ="
}
```

**Option 3: Passive reply (with original message ID)**

```json
{
  "tts_text": "Got your message",
  "msg_id": "original_message_id"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `voice_base64` | string | Yes* | Base64-encoded voice data. Not required when `tts_text` is provided. |
| `tts_text` | string | No | Text for TTS (text-to-speech) synthesis. When provided, the system automatically calls edge-tts to synthesize speech from this text and sends it. No preprocessing needed by the caller. Only supported for C2C private messages. |
| `msg_id` | string | No | Original message ID for passive reply. |

> **Note**: At least one of `voice_base64` and `tts_text` must be provided. When both are provided, `tts_text` takes priority.

**Response:**

```json
{"ok": true, "data": {"status": "sent"}}
```

#### POST `/api/v1/accounts/{id}/groups/{openid}/voice`

Send a voice message to a group.

> **Note**: Group voice messages **do not support** the `tts_text` field. Only `voice_base64` can be used to send voice data.

**Request:**

```json
{
  "voice_base64": "SGVsbG8gV29ybGQ=",
  "msg_id": "optional_message_id"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `voice_base64` | string | Yes | Base64-encoded voice data. |
| `msg_id` | string | No | Original message ID for passive reply. |

**Response:**

```json
{"ok": true, "data": {"status": "sent"}}
```

### Video Messages

#### POST `/api/v1/accounts/{id}/c2c/{openid}/videos`

Send a video to a C2C user.

**Request:**

```json
{
  "video_url": "https://example.com/video.mp4",
  "video_base64": "optional_base64_data",
  "content": "Watch this!",
  "msg_id": "optional_message_id"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `video_url` | string | Yes* | URL of the video file. Either `video_url` or `video_base64` must be provided. |
| `video_base64` | string | Yes* | Base64-encoded video data. Alternative to `video_url`. |
| `content` | string | No | Caption text to accompany the video. |
| `msg_id` | string | No | Original message ID for passive reply. |

**Response:**

```json
{"ok": true, "data": {"status": "sent"}}
```

#### POST `/api/v1/accounts/{id}/groups/{openid}/videos`

Send a video to a group. Same request/response format as C2C video messages.

### File Messages

#### POST `/api/v1/accounts/{id}/c2c/{openid}/files`

Send a file to a C2C user.

**Request:**

```json
{
  "file_url": "https://example.com/document.pdf",
  "file_base64": "optional_base64_data",
  "file_name": "report.pdf",
  "msg_id": "optional_message_id"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `file_url` | string | Yes* | URL of the file. Either `file_url` or `file_base64` must be provided. |
| `file_base64` | string | Yes* | Base64-encoded file data. Alternative to `file_url`. |
| `file_name` | string | Yes | Display name of the file. |
| `msg_id` | string | No | Original message ID for passive reply. |

**Response:**

```json
{"ok": true, "data": {"status": "sent"}}
```

#### POST `/api/v1/accounts/{id}/groups/{openid}/files`

Send a file to a group. Same request/response format as C2C file messages.

### Reply Rate Limiting

When `msg_id` is provided, the message is sent as a **passive reply** to the original user message. The QQ Bot platform limits passive replies:

- **Maximum 4 replies** per `msg_id` within a **1-hour window** from the first reply.
- When the limit is exceeded or the 1-hour window expires, the system **automatically falls back** to proactive messaging (the `msg_id` is cleared and a proactive send is used instead).
- This fallback is transparent to the API caller -- the request still succeeds.

### Media Tags

Text content in message endpoints supports embedded media tags for sending mixed text+media in a single message. Tags are parsed from the text and sent as separate message segments in order.

**Supported tag formats:**

| Tag | Media Type | Description |
|-----|-----------|-------------|
| `<qqimg>url_or_path</qqimg>` | Image | Send an inline image |
| `<qqvoice>file_path</qqvoice>` | Voice | Send an inline voice clip |
| `<qqvideo>url_or_path</qqvideo>` | Video | Send an inline video |
| `<qqfile>file_path</qqfile>` | File | Send an inline file |

**Example mixed content:**

```json
{"content": "Here is the chart:\n<qqimg>https://example.com/chart.png</qqimg>\nAnd the report:\n<qqfile>/data/report.pdf</qqfile>"}
```

Tag aliases are also supported (e.g. `<image>`, `<pic>`, `<voice>`, `<audio>`, `<doc>`, `<document>`). The normalizer also handles full-width brackets, multiline tags, and backtick-wrapped tags.

---

## Proactive & Broadcast

Proactive messages are sent independently of any incoming user message (no `msg_id` or reply context required).

### POST `/api/v1/accounts/{id}/proactive/c2c/{openid}`

Send a proactive text message to a C2C user.

**Request:**

```json
{"content": "Reminder: meeting at 3pm!"}
```

**Response:**

```json
{"ok": true, "data": {"status": "sent"}}
```

### POST `/api/v1/accounts/{id}/proactive/groups/{openid}`

Send a proactive text message to a group.

Same request/response format.

### POST `/api/v1/accounts/{id}/broadcast`

Broadcast a text message to **all known C2C users** for the account. Users are recorded automatically when they send messages to the bot.

**Request:**

```json
{"content": "Important announcement to all users!"}
```

**Response:**

```json
{
  "ok": true,
  "data": {
    "sent": 42,
    "errors": ["failed to send to user abc123: ..."]
  }
}
```

> **Note:** Broadcast always returns HTTP 200 even when individual sends fail. The `errors` array contains error messages for any failed recipients. The caller should inspect `sent` and `errors` to determine the broadcast result.

### POST `/api/v1/accounts/{id}/broadcast/groups`

Broadcast a text message to **all known groups** for the account. Groups are automatically deduplicated by Group OpenID.

**Request and response format** are the same as the C2C broadcast endpoint.

---

## Scheduler / Reminders

Reminders are persistent scheduled jobs that send proactive messages at specified times. They are persisted to SQLite and survive service restarts. The scheduler checks for due jobs every 100ms.

### POST `/api/v1/accounts/{id}/reminders`

Create a new reminder.

**Request:**

```json
{
  "content": "Time for your daily standup!",
  "target_type": "c2c",
  "target_address": "user_openid_here",
  "schedule": "@every 1h"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `content` | string | No | Text message to send when the reminder fires. |
| `target_type` | string | Yes | `"c2c"` or `"group"`. |
| `target_address` | string | Yes | Recipient OpenID (C2C user OpenID or Group OpenID). |
| `schedule` | string | No | Schedule expression. When omitted or empty, the reminder fires immediately (one-time). See [Schedule Syntax](#schedule-syntax). |

**Response:**

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

### Schedule Syntax

Two schedule formats are supported:

**1. Interval (`@every`)**

```
@every 30s    // every 30 seconds
@every 5m     // every 5 minutes
@every 1h     // every 1 hour
```

Uses Go `time.ParseDuration` format.

**2. Cron Expression (5 fields)**

```
┌───────────── minute   (0-59)
│ ┌───────────── hour     (0-23)
│ │ ┌───────────── day     (1-31)
│ │ │ ┌───────────── month   (1-12)
│ │ │ │ ┌───────────── weekday (0-6, 0=Sunday)
│ │ │ │ │
* * * * *
```

Supported syntax: `*` (any), specific values (`30`), ranges (`1-5`), steps (`*/15`, `1-30/5`). Day-of-month and day-of-week use OR logic (standard POSIX cron behavior).

**Examples:**

```
0 9 * * *       // every day at 9:00 AM
*/30 * * * *    // every 30 minutes
0 9 * * 1-5     // weekdays at 9:00 AM
0 0 1,15 * *    // 1st and 15th of each month at midnight
```

### DELETE `/api/v1/accounts/{id}/reminders/{remID}`

Cancel and delete a reminder.

**Response (success):**

```json
{"ok": true, "data": {"status": "cancelled"}}
```

**Response (not found):**

```json
{"ok": false, "error": "reminder not found"}
```

HTTP status code: `404`.

### GET `/api/v1/accounts/{id}/reminders`

List all reminders for the account.

**Response:**

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

| Field | Type | Description |
|-------|------|-------------|
| `ID` | string | Unique job ID (format: `rem-{unix_nano}`) |
| `Content` | string | Message content to send |
| `TargetType` | string | `"c2c"` or `"group"` |
| `TargetAddress` | string | Recipient OpenID |
| `AccountID` | string | Account this reminder belongs to |
| `Schedule` | string | Original schedule expression (empty for one-time) |
| `NextRun` | string | Next execution time (RFC3339) |
| `CreatedAt` | string | Creation time (RFC3339) |

One-time reminders (empty schedule) are automatically deleted after execution. Recurring reminders reschedule themselves after each execution.

---

## User Management

Users are **automatically recorded** when they send messages to the bot. C2C messages record the sender as type `"c2c"`, and group @mention messages record the sender as type `"group"` with their `GroupOpenID`. The `interaction_count` is incremented and `last_seen_at` is updated on each message.

### GET `/api/v1/accounts/{id}/users`

List known users with filtering, sorting, and pagination.

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `type` | string | (all) | Filter by user type: `"c2c"` or `"group"` |
| `active_within` | int64 | (all) | Only include users active within this many milliseconds from now |
| `limit` | int | (all) | Maximum number of results to return |
| `sort_by` | string | `"lastSeenAt"` | Sort field: `"lastSeenAt"`, `"firstSeenAt"`, or `"interactionCount"` |
| `sort_order` | string | `"desc"` | Sort direction: `"asc"` or `"desc"` |

**Example:**

```
GET /api/v1/accounts/default/users?type=c2c&active_within=86400000&limit=50&sort_by=interaction_count&sort_order=desc
```

**Response:**

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

| Field | Type | Description |
|-------|------|-------------|
| `openid` | string | User OpenID (C2C) or Member OpenID (group) |
| `type` | string | `"c2c"` or `"group"` |
| `nickname` | string | User nickname (may be empty) |
| `group_openid` | string | Group OpenID (only for group users) |
| `account_id` | string | Account this user belongs to |
| `first_seen_at` | int64 | First seen timestamp in Unix milliseconds |
| `last_seen_at` | int64 | Last seen timestamp in Unix milliseconds |
| `interaction_count` | int | Total message count from this user |

### GET `/api/v1/accounts/{id}/users/stats`

Get aggregate user statistics.

**Response:**

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

| Field | Type | Description |
|-------|------|-------------|
| `total_users` | int | Total known users |
| `c2c_users` | int | C2C users count |
| `group_users` | int | Group users count |
| `active_in_24h` | int | Users active in the last 24 hours |
| `active_in_7d` | int | Users active in the last 7 days |

### DELETE `/api/v1/accounts/{id}/users`

Clear all known users for the account. This action is irreversible.

**Response:**

```json
{"ok": true, "data": {"removed": 128}}
```

---

## Webhook Event Forwarding

When configured with `defaultWebhookUrl` (top-level config) or `webhookUrl` (per-account config), incoming user messages are forwarded to the configured URL via HTTP POST.

### URL Resolution

The webhook URL for each account is resolved with this priority:

1. Per-account `webhookUrl` in config
2. Top-level `defaultWebhookUrl`
3. No webhook configured (events are not forwarded)

### Forwarded Event Types

| Event Type | Description |
|------------|-------------|
| `C2C_MESSAGE_CREATE` | Private (C2C) message from a user |
| `GROUP_AT_MESSAGE_CREATE` | Group message where the bot was @mentioned |
| `GUILD_MESSAGE_CREATE` | Guild channel message where the bot was @mentioned |
| `DIRECT_MESSAGE_CREATE` | Guild direct message from a user |

Gateway lifecycle events (`READY`, `RESUMED`) are **not** forwarded.

### Webhook Payload

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

| Field | Type | Description |
|-------|------|-------------|
| `account_id` | string | The account ID that received the event |
| `event_type` | string | One of the forwarded event types listed above |
| `timestamp` | string | Forwarding time in RFC3339 UTC |
| `data` | object | Raw event payload from the QQ Bot gateway (structure varies by event type) |

### Delivery Behavior

- **Async**: Webhook delivery is non-blocking to the gateway. Events are dispatched in a background goroutine.
- **HTTP method**: Always `POST` with `Content-Type: application/json`.
- **Timeout**: 10 seconds per request attempt.
- **Retries**: Up to 3 attempts with exponential backoff:
  - Attempt 1: immediate
  - Attempt 2: after 1 second
  - Attempt 3: after 2 seconds
- **Success**: HTTP status code < 400.
- **Failure**: After all retries exhausted, the event is dropped and an error is logged. No dead-letter queue.
