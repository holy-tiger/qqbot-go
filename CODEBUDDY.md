# CODEBUDDY.md — openclaw-qqbot

## Project Overview

QQ Bot HTTP API Service — a standalone Go service that exposes QQ Bot messaging capabilities via RESTful HTTP API. Supports C2C/group/guild messaging, rich media (image/voice/video/file), proactive scheduling, broadcast, webhook event forwarding, multi-account isolation, and image hosting.

- **Module:** `github.com/openclaw/qqbot`
- **Go version:** 1.25 (CI also tests 1.24)
- **Entry point:** `cmd/qqbot/main.go`

## Build & Run

```bash
# Build
go build -o qqbot ./cmd/qqbot

# Run (requires config)
./qqbot -config configs/config.yaml -health :8080 -api :9090

# Run tests
go test -race -count=1 ./...

# Static analysis
go vet ./...
```

### CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-config` | `configs/config.yaml` | YAML config file path |
| `-health` | `:8080` | Health check HTTP address (empty to disable) |
| `-api` | `:9090` | HTTP API server address (empty to disable) |

## HTTP API

All API endpoints are prefixed with `/api/v1`. No authentication is required (designed for internal network use).

### Message Sending

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/accounts/{id}/c2c/{openid}/messages` | Send text to C2C user |
| POST | `/api/v1/accounts/{id}/groups/{openid}/messages` | Send text to group |
| POST | `/api/v1/accounts/{id}/channels/{channel_id}/messages` | Send text to channel |
| POST | `/api/v1/accounts/{id}/c2c/{openid}/images` | Send image to C2C user |
| POST | `/api/v1/accounts/{id}/groups/{openid}/images` | Send image to group |
| POST | `/api/v1/accounts/{id}/c2c/{openid}/voice` | Send voice to C2C user |
| POST | `/api/v1/accounts/{id}/groups/{openid}/voice` | Send voice to group |
| POST | `/api/v1/accounts/{id}/c2c/{openid}/videos` | Send video to C2C user |
| POST | `/api/v1/accounts/{id}/groups/{openid}/videos` | Send video to group |
| POST | `/api/v1/accounts/{id}/c2c/{openid}/files` | Send file to C2C user |
| POST | `/api/v1/accounts/{id}/groups/{openid}/files` | Send file to group |

### Proactive & Broadcast

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/accounts/{id}/proactive/c2c/{openid}` | Proactive text to C2C user |
| POST | `/api/v1/accounts/{id}/proactive/groups/{openid}` | Proactive text to group |
| POST | `/api/v1/accounts/{id}/broadcast` | Broadcast to all C2C users |
| POST | `/api/v1/accounts/{id}/broadcast/groups` | Broadcast to all groups |

### Scheduler / Reminders

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/accounts/{id}/reminders` | Create reminder |
| DELETE | `/api/v1/accounts/{id}/reminders/{rem_id}` | Cancel reminder |
| GET | `/api/v1/accounts/{id}/reminders` | List all reminders |

### User Management

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/accounts/{id}/users` | List known users |
| GET | `/api/v1/accounts/{id}/users/stats` | User statistics |
| DELETE | `/api/v1/accounts/{id}/users` | Clear all users for account |

### Account Status

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health`, `/healthz` | Health check |
| GET | `/api/v1/accounts` | List all accounts with status |
| GET | `/api/v1/accounts/{id}` | Single account status |

### Response Format

All endpoints return JSON:

```json
{"ok": true, "data": { ... }}
{"ok": false, "error": "error message"}
```

### Webhook Event Forwarding

When configured with `defaultWebhookUrl` or per-account `webhookUrl`, incoming user messages are forwarded via HTTP POST:

```json
{
  "account_id": "default",
  "event_type": "C2C_MESSAGE_CREATE",
  "timestamp": "2026-03-25T12:00:00Z",
  "data": { ... }
}
```

Delivery is async (non-blocking to gateway), with up to 3 retries and exponential backoff (1s, 2s, 4s).

## Project Structure

```
cmd/qqbot/          CLI entry point
configs/            YAML configuration (config.example.yaml)
internal/
  api/              QQ Bot REST API client (token, media, messages)
  audio/            Audio processing (SILK encode/decode, STT, format conversion)
  config/           Configuration loading and multi-account resolution
  gateway/          WebSocket gateway (heartbeat, reconnect, message queue)
  httpapi/          HTTP API server and webhook dispatcher
  image/            Image hosting HTTP server and dimension parsing
  outbound/         Outbound message handling, rate limiting, media tag parsing
  proactive/        Proactive messaging and cron-like scheduler
  qqbot/            Top-level orchestration (BotManager, health check, validation)
  store/            SQLite persistent storage (known users, ref index, sessions)
  types/            Core domain types (events, configs, payloads)
  utils/            Utilities (file validation, media tag normalization, payload parsing)
```

## Key Architecture Patterns

- **Multi-account isolation:** Each account gets independent APIClient, Gateway, OutboundHandler, ProactiveManager, Scheduler, and all stores.
- **RESTful HTTP API:** `internal/httpapi/` exposes all bot capabilities via `/api/v1/` endpoints; uses interface-based adapter (`botAPIAdapter`) to avoid circular imports with `internal/qqbot/`.
- **Webhook forwarding:** Async fire-and-forget event delivery with bounded retry; per-account URL overrides global `defaultWebhookUrl`.
- **Concurrency:** Per-user message queue with cross-user parallelism, singleflight for token dedup, mutex-protected stores with throttled disk writes.
- **Reconnection:** Exponential backoff [1s->2s->5s->10s->30s->60s], max 100 attempts, intent fallback on INVALID_SESSION.
- **Rate limiting:** 4 passive replies/message_id/hour, auto-fallback to proactive API.
- **Storage:** SQLite with in-memory caches and TTL eviction.

## Dependencies

Direct: `golang.org/x/sync` (singleflight), `gopkg.in/yaml.v3`, `github.com/gorilla/websocket`, `modernc.org/sqlite`
Runtime: `ffmpeg`/`ffprobe` for audio processing

## Coding Conventions

- **Language:** Go 1.25, idiomatic Go style
- **Package naming:** Lowercase, single word (`api`, `config`, `types`)
- **File naming:** `snake_case`
- **Error handling:** Return `error`, use `fmt.Errorf` with `%w` wrapping
- **Logging:** Prefixed with `[gateway:accountID]`, `[qqbot]`, `[scheduler]`, `[webhook]`
- **All code under `internal/`:** Not importable by external modules
- **Tests:** Standard `testing` package, one `*_test.go` per package, run with `-race`
- **Configuration:** YAML with Chinese comments
- **Documentation:** GoDoc comments on all exported types and functions
- **No external linting configs:** Follow Go defaults

## Configuration

See `configs/config.example.yaml` for all options. Secrets can be set via config, env vars (`QQBOT_APP_ID`, `QQBOT_CLIENT_SECRET`), or file (`clientSecretFile`).

### Webhook Configuration

- `defaultWebhookUrl`: global webhook URL for all accounts
- Per-account `webhookUrl`: overrides the global setting

## CI

GitHub Actions: `go vet` -> `go test -race` -> `go build` on Go 1.24 and 1.25, triggered on push/PR to `main`/`master`.

## Important Notes

- Do NOT add code under `pkg/` -- all application code belongs in `internal/`
- Data directory (`data/`) is created at runtime for persistent storage
- Health check endpoints: `/health` and `/healthz`
- API server endpoints: `/api/v1/` (disabled when `-api` is empty)
- No CGO required -- SILK encoding uses ffmpeg via `os/exec`
