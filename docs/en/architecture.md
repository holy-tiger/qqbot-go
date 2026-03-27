# Architecture & Design Patterns

## Overview

openclaw-qqbot is a multi-account QQ Bot service built in Go. The architecture centers on **BotManager** as the top-level orchestrator, with each account getting fully isolated dependencies. The system exposes messaging capabilities via a RESTful HTTP API and receives messages through persistent WebSocket connections to the QQ Bot gateway. A separate **Channel Server** binary bridges QQ events to CodeBuddy Code via MCP protocol.

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
      └──────┬───────┘ └─────────────┘ └─────────────┘
             │ webhook events
             ▼
      ┌──────────────┐       ┌─────────────────────┐
      │   Webhook     │──────▶│  Channel Server      │ (separate binary)
      │  Dispatcher   │  HTTP │  (internal/channel/)  │
      └──────────────┘       └──────────┬──────────┘
                                         │ MCP stdio
                                         ▼
                              ┌─────────────────────┐
                              │  CodeBuddy Code      │
                              │  (MCP client)        │
                              └─────────────────────┘
```

## Component Architecture

### BotManager (`internal/qqbot/botmanager.go`)

The central orchestrator. Manages multiple accounts concurrently with per-account isolation.

**Responsibilities:**
- Creates and manages all accounts with isolated dependencies
- Provides the `BotAPI` interface for the HTTP API server
- Handles graceful shutdown (schedulers -> gateways -> store flush -> DB close)
- Coordinates webhook dispatcher for event forwarding

**Account lifecycle:**

```
AddAccount() → Start() → [gateway connects] → Stop()
                   │
                   ├─ Client.Init()        (start token refresh)
                   ├─ Scheduler.Start()    (load persisted reminders)
                   └─ Gateway.Connect()    (background goroutine per account)
```

### Account Isolation

Each account gets its own complete set of dependencies. No state is shared between accounts except the underlying SQLite database (which uses per-account partitioning via `account_id` columns).

| Component | Package | Purpose |
|-----------|---------|---------|
| `APIClient` | `internal/api` | QQ Bot REST API client with token caching |
| `Gateway` | `internal/gateway` | WebSocket connection, heartbeat, reconnect |
| `OutboundHandler` | `internal/outbound` | Message sending with rate limiting and media tag parsing |
| `ProactiveManager` | `internal/proactive` | Proactive messaging and broadcast |
| `Scheduler` | `internal/proactive` | Reminder job execution (100ms tick loop) |
| `KnownUsersStore` | `internal/store` | User tracking with upsert |
| `RefIndexStore` | `internal/store` | Message reference index for context |
| `SessionStore` | `internal/store` | WebSocket session persistence |
| `TTSProvider` | `internal/audio` | Text-to-speech synthesis |

### Interface Adapter Pattern

The HTTP API server (`internal/httpapi/`) communicates with `BotManager` through the `BotAPI` interface, implemented by `botAPIAdapter` in `internal/qqbot/run.go`. This avoids circular imports since `httpapi` only imports the interface, not the concrete `BotManager`.

```
httpapi.APIServer → BotAPI interface ← botAPIAdapter wraps *BotManager
```

### Channel Server (`internal/channel/`)

A separate binary (`cmd/qqbot-channel`) that bridges QQ Bot events to CodeBuddy Code via the MCP (Model Context Protocol) stdio transport. It runs as two concurrent components:

1. **MCP stdio server** — communicates with CodeBuddy Code using JSON-RPC over stdin/stdout
2. **HTTP webhook server** — receives forwarded QQ events from qqbot's webhook dispatcher

**Key design decisions:**
- Separate binary to avoid coupling the MCP server lifecycle with the main qqbot process
- `claude/channel` experimental capability signals CodeBuddy Code that this is a messaging channel
- `reply` MCP tool sends messages back to QQ via the qqbot HTTP API
- Notifications flow server→client: webhook event → MCP notification → CodeBuddy Code processes → calls reply tool → HTTP API → QQ

**Message routing:**
- Currently uses a single `-account` flag for all replies
- Future: route based on `account_id` from the webhook event payload

---

## Data Flow

### Incoming Message Flow

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

### Outgoing Message Flow

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

### Channel Server Message Flow (MCP)

```
QQ Bot Gateway (WebSocket)
    │
    ▼
Gateway → EventHandler → WebhookDispatcher
                                │
                                ▼
                    HTTP POST /webhook (async)
                    to Channel Server (:8788)
                                │
                                ▼
                        Channel Server
                        ├─ parse event type
                        ├─ extract content
                        ├─ appendAttachmentInfo()
                        │  (image/voice/video/file metadata)
                        └─ SendNotificationToAllClients()
                                │ MCP notification
                                ▼
                        CodeBuddy Code (MCP client)
                                │ AI processes message
                                ▼
                        CallTool("reply", chat_id, text)
                                │ MCP tool invocation
                                ▼
                        Channel Server.handleReply()
                                │
                                ▼
                        HTTP POST to qqbot API
                        /api/v1/accounts/{acct}/{type}/{id}/messages
                                │
                                ▼
                        qqbot OutboundHandler
                                │
                                ▼
                        QQ Bot API → QQ Bot Gateway → User
```

---

## Concurrency Model

### Per-User Message Queue (`internal/gateway/queue.go`)

Messages from the WebSocket gateway are processed through a **per-user serialization with cross-user parallelism** model:

| Parameter | Default | Description |
|-----------|---------|-------------|
| `maxConcurrentUsers` | 10 | Max users processed in parallel (semaphore) |
| `perUserQueueSize` | 20 | Max pending messages per user |
| `globalQueueSize` | 1000 | Buffer size for the dispatch channel |

- Each user gets their own goroutine (`userWorker`) that processes messages **serially** -- this preserves message ordering per user.
- A semaphore limits the total number of concurrently-processing users to 10.
- Per-user queue overflow drops messages silently (returns `false` from `Enqueue`).
- Handler errors are logged but do not stop processing.

### Token Cache Deduplication (`internal/api/token.go`)

Uses `singleflight.Group` from `golang.org/x/sync` to deduplicate concurrent token requests for the same appId. When multiple goroutines need a token simultaneously, only one HTTP request is made and the result is shared.

**Token lifecycle:**
- Background refresh starts on `Init()` and runs until `Close()`.
- Token is considered expired 5 minutes before actual expiry (`earlyExpiry`).
- Background refresh triggers 5 minutes before expiry (`refreshAhead`).
- Minimum refresh interval: 60 seconds.
- Failed refresh retries after 5 seconds.
- Default QQ Bot token TTL: ~7200 seconds (2 hours).

### Store Thread Safety

All store implementations use `sync.Mutex` for thread safety. The SQLite database uses:
- **WAL mode** for better concurrent read performance.
- **Single connection** (`MaxOpenConns=1`) for write safety.
- **5-second busy timeout** for contention handling.

### Reply Rate Limiter (`internal/outbound/ratelimit.go`)

Enforces the QQ Bot platform limit of **4 passive replies per message_id per hour**:

- In-memory `map[string]*replyRecord` protected by `sync.Mutex`.
- Auto-cleanup when track count exceeds 10,000 entries (purges expired).
- On limit exceeded or expiry (>1 hour), transparently falls back to proactive messaging.

---

## Gateway & Reconnection

### WebSocket Lifecycle (`internal/gateway/gateway.go`)

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

### Intent Fallback

The QQ Bot API may reject certain intent combinations. The gateway implements a 3-level intent fallback:

| Level | Name | Intents | Covers |
|-------|------|---------|--------|
| 0 | `full` | Guilds + GuildMembers + DirectMessage + GroupAndC2C + PublicGuildMessages | All channels |
| 1 | `group_and_guild` | Guilds + PublicGuildMessages + GroupAndC2C | Groups + guild public |
| 2 | `guild_only` | Guilds + PublicGuildMessages | Guild public channels only |

When `INVALID_SESSION` is received with `canResume=false`, the gateway clears the session and increments `intentIndex` to try the next (lower) intent level. The current intent level is persisted in the session store.

### Reconnection Configuration (`internal/gateway/reconnect.go`)

| Parameter | Default | Description |
|-----------|---------|-------------|
| Backoff delays | `[1s, 2s, 5s, 10s, 30s, 60s]` | Exponential backoff between attempts |
| Rate limit delay | 60s | Extra delay when rate limited by server |
| Max attempts | 100 | Max reconnect attempts before giving up |
| Quick disconnect threshold | 5s | Time window for "quick" disconnect detection |
| Max quick disconnects | 3 | Consecutive quick disconnects to trigger stop |

The `GetDelay()` method uses the attempt index to pick from the delays array (clamped to the last value for high attempt counts). The `ShouldQuickStop()` method detects rapid reconnection failures that likely indicate a configuration or credential issue.

### Session Persistence

WebSocket session state is persisted to SQLite to enable resume after restart:

| Field | Description |
|-------|-------------|
| `session_id` | QQ Bot session ID from READY event |
| `last_seq` | Last received sequence number |
| `intent_level_index` | Current intent fallback level |
| `app_id` | Account AppID (for validation on load) |

Session data is saved on READY and seq is updated after each received message. On load, sessions older than 5 minutes are considered expired. If the persisted `app_id` doesn't match the current account, the session is invalidated.

---

## Storage

### Database (`internal/store/db.go`)

Uses **pure-Go SQLite** (`modernc.org/sqlite`) -- no CGO required. Database file: `{dataDir}/qqbot.db`.

**Schema** (4 tables):

| Table | Purpose | Key Fields | TTL/Eviction |
|-------|---------|------------|--------------|
| `known_users` | Track message senders | `account_id`, `open_id`, `type`, `group_open_id` | None (persistent) |
| `ref_index` | Message reference index for AI context | `ref_key`, `content`, `sender_id` | 7 days, max 50,000 entries |
| `sessions` | WebSocket session state | `account_id` (primary key), `session_id`, `last_seq` | 5 minutes, appID mismatch |
| `reminders` | Scheduled reminder jobs | `id` (primary key), `account_id`, `schedule`, `next_run` | Removed after one-time execution |

**Indexes:**
- `known_users(account_id)`, `known_users(account_id, type)`
- `reminders(account_id)`

### Legacy Migration

On first startup, the store automatically migrates data from legacy JSON files:
- `known-users.json` (JSON array) → `known_users` table
- `ref-index.jsonl` (JSONL format) → `ref_index` table
- `session-*.json` (per-account files) → `sessions` table

After successful migration, each source file is renamed to `.bak` to prevent re-migration.

### Data Directory

All persistent data is stored under the `data/` directory (created at runtime):

```
data/
├── qqbot.db              # SQLite database (all stores)
├── known-users.json.bak  # Migrated legacy data
├── ref-index.jsonl.bak
└── session-default.json.bak
```

---

## API Client (`internal/api`)

### Token Management

- **Endpoint:** `https://bots.qq.com/app/getAppAccessToken`
- **Cache:** In-memory `TokenCache` with per-appId entries
- **Deduplication:** `singleflight.Group` prevents concurrent token requests
- **Background refresh:** Automatically refreshes 5 minutes before expiry
- **Early expiry:** Tokens are considered expired 5 minutes before actual TTL

### Upload Caching

Media uploads are cached using `UploadCache` to avoid redundant uploads:

- **Cache key:** `md5(data):scope:targetId:fileType`
- **Max size:** 500 entries
- **TTL:** Based on server response TTL minus 60s safety margin (minimum 10s)
- **Eviction:** Expired entries first, then oldest half if at capacity

### Request Retry

File upload requests use exponential backoff retry:
- **Max retries:** 2
- **Base delay:** 1s (doubles each retry: 1s, 2s)
- **Non-retryable:** 400/401/Invalid/Timeout errors fail immediately
- **Timeouts:** 30s for standard API, 120s for file uploads

---

## Scheduler (`internal/proactive/scheduler.go`)

Reminder jobs are managed by an in-memory scheduler with SQLite persistence.

**Execution model:**
- Ticker-based: checks for due jobs every **100ms**.
- Due jobs are collected under lock, then executed sequentially.
- After execution, `NextRun` is recalculated from the schedule.
- One-time jobs (empty schedule or no next run) are deleted after execution.
- Thread-safe with `sync.Mutex`.

**Job lifecycle:**

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

**Schedule syntax:**
- `@every Xs/Xm/Xh` -- Go `time.ParseDuration` intervals
- 5-field cron -- `minute hour day month weekday` with ranges and steps

---

## Image Hosting (`internal/image`)

A local HTTP server for serving images uploaded by the bot:

- Serves images at `/images/{uuid}.{ext}` where UUID is 32 hex characters.
- Auto-detects format via magic bytes (PNG, JPEG, GIF, WebP).
- CORS enabled (`Access-Control-Allow-Origin: *`).
- TTL-based expiration (returns HTTP 410 Gone for expired images).
- Path traversal protection (validates resolved path is within image directory).

The image dimension parser (`internal/image/size.go`) reads binary headers directly (no image decoding) for PNG, JPEG, GIF, and WebP formats.

---

## Dependencies

### Go Modules

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/gorilla/websocket` | v1.5.3 | WebSocket client for gateway connection |
| `github.com/mark3labs/mcp-go` | v0.46.0 | MCP (Model Context Protocol) server for Channel Server |
| `golang.org/x/sync` | v0.20.0 | `singleflight` for token request deduplication |
| `gopkg.in/yaml.v3` | v3.0.1 | YAML configuration parsing |
| `modernc.org/sqlite` | v1.47.0 | Pure-Go SQLite driver (no CGO) |

### Runtime Dependencies

| Tool | Purpose |
|------|---------|
| `ffmpeg` | Audio processing (SILK encode/decode, format conversion) |
| `ffprobe` | Audio format detection |

No other external runtime dependencies. The binary is fully self-contained except for ffmpeg/ffprobe.

## Startup & Shutdown

### Startup Sequence

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

### Shutdown Sequence

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
