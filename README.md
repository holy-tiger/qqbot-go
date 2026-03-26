# openclaw-qqbot

[中文](README_ZH.md) | English

QQ Bot HTTP API Service — a standalone Go service that exposes QQ Bot messaging capabilities via RESTful HTTP API.

## Features

- **C2C / Group / Channel Messaging** — text, image, voice, video, file
- **Proactive Messaging** — send messages to users/groups without requiring an incoming message trigger
- **Scheduled Reminders** — create, cancel, and query timed message tasks
- **Broadcast** — send messages to all C2C users or groups at once
- **Webhook Event Forwarding** — push user messages to external services via HTTP POST in real time
- **Multi-Account Isolation** — each account has independent connections, queues, and storage
- **Image Hosting** — built-in HTTP image server, supports local image to URL conversion
- **SQLite Persistence** — user records and session info are automatically stored

## Installation

### Download Pre-built Binaries

Download the archive for your platform from [GitHub Releases](https://github.com/holy-tiger/qqbot-go/releases) and extract it.

**Linux**

```bash
# x86_64
curl -sL https://github.com/holy-tiger/qqbot-go/releases/latest/download/qqbot_linux_x86_64.tar.gz | tar xz

# ARM64 (e.g. Raspberry Pi, Huawei Cloud Kunpeng)
curl -sL https://github.com/holy-tiger/qqbot-go/releases/latest/download/qqbot_linux_aarch64.tar.gz | tar xz
```

**macOS**

```bash
# Apple Silicon (M1/M2/M3/M4)
curl -sL https://github.com/holy-tiger/qqbot-go/releases/latest/download/qqbot_darwin_aarch64.tar.gz | tar xz

# Intel
curl -sL https://github.com/holy-tiger/qqbot-go/releases/latest/download/qqbot_darwin_x86_64.tar.gz | tar xz
```

**Windows**

```powershell
# x86_64
Invoke-WebRequest -Uri https://github.com/holy-tiger/qqbot-go/releases/latest/download/qqbot_windows_x86_64.zip -OutFile qqbot.zip
Expand-Archive qqbot.zip -DestinationPath .
```

> SHA256 checksums for all versions: [checksums.txt](https://github.com/holy-tiger/qqbot-go/releases/latest/download/checksums.txt).

### Running

```bash
# Configure
cp configs/config.example.yaml configs/config.yaml
# Edit config.yaml and fill in appId and clientSecret

# Start
./qqbot -config configs/config.yaml -health :8080 -api :9090
```

### Build from Source

Requires Go 1.24+ and ffmpeg/ffprobe (for audio processing):

```bash
git clone https://github.com/holy-tiger/qqbot-go.git
cd qqbot-go
go build -o qqbot ./cmd/qqbot
```

### CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-config` | `configs/config.yaml` | Path to configuration file |
| `-health` | `:8080` | Health check address (leave empty to disable) |
| `-api` | `:9090` | HTTP API address (leave empty to disable) |

## Configuration

See [`configs/config.example.yaml`](configs/config.example.yaml) for a full example configuration.

Secrets can be set in three ways (highest priority first):

1. Environment variables: `QQBOT_APP_ID`, `QQBOT_CLIENT_SECRET`, `QQBOT_IMAGE_SERVER_BASE_URL`
2. File: specify a secret file path via `clientSecretFile`
3. Config file: write directly in YAML

Multi-account configuration example:

```yaml
qqbot:
  appId: "default-app-id"
  clientSecret: "default-secret"
  accounts:
    second-bot:
      appId: "second-app-id"
      clientSecret: "second-secret"
      name: "Second Bot"
```

Full configuration reference: [`docs/en/configuration.md`](docs/en/configuration.md).

## HTTP API

All endpoints are prefixed with `/api/v1`. No authentication required (designed for internal network use).

### Message Sending

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/accounts/{id}/c2c/{openid}/messages` | Send text (C2C) |
| POST | `/api/v1/accounts/{id}/groups/{openid}/messages` | Send text (Group) |
| POST | `/api/v1/accounts/{id}/channels/{channel_id}/messages` | Send text (Channel) |
| POST | `/api/v1/accounts/{id}/c2c/{openid}/images` | Send image (C2C) |
| POST | `/api/v1/accounts/{id}/groups/{openid}/images` | Send image (Group) |
| POST | `/api/v1/accounts/{id}/c2c/{openid}/voice` | Send voice (C2C) |
| POST | `/api/v1/accounts/{id}/groups/{openid}/voice` | Send voice (Group) |
| POST | `/api/v1/accounts/{id}/c2c/{openid}/videos` | Send video (C2C) |
| POST | `/api/v1/accounts/{id}/groups/{openid}/videos` | Send video (Group) |
| POST | `/api/v1/accounts/{id}/c2c/{openid}/files` | Send file (C2C) |
| POST | `/api/v1/accounts/{id}/groups/{openid}/files` | Send file (Group) |

### Proactive Messaging & Broadcast

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/accounts/{id}/proactive/c2c/{openid}` | Proactive text (C2C) |
| POST | `/api/v1/accounts/{id}/proactive/groups/{openid}` | Proactive text (Group) |
| POST | `/api/v1/accounts/{id}/broadcast` | Broadcast to all C2C users |
| POST | `/api/v1/accounts/{id}/broadcast/groups` | Broadcast to all groups |

### Scheduled Reminders

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/accounts/{id}/reminders` | Create reminder |
| DELETE | `/api/v1/accounts/{id}/reminders/{rem_id}` | Cancel reminder |
| GET | `/api/v1/accounts/{id}/reminders` | List all reminders |

### Users & Status

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/accounts/{id}/users` | List known users |
| GET | `/api/v1/accounts/{id}/users/stats` | User statistics |
| DELETE | `/api/v1/accounts/{id}/users` | Clear all user records |
| GET | `/api/v1/accounts` | All account statuses |
| GET | `/api/v1/accounts/{id}` | Single account status |
| GET | `/health` | Health check |

### Response Format

```json
{"ok": true, "data": { ... }}
{"ok": false, "error": "error message"}
```

### Webhook Event Forwarding

When `defaultWebhookUrl` or `webhookUrl` is configured, user messages are forwarded via HTTP POST:

```json
{
  "account_id": "default",
  "event_type": "C2C_MESSAGE_CREATE",
  "timestamp": "2026-03-25T12:00:00Z",
  "data": { ... }
}
```

Async delivery, up to 3 retries with exponential backoff (1s, 2s, 4s).

Full API documentation: [`docs/en/api.md`](docs/en/api.md).

## Project Structure

```
cmd/qqbot/          CLI entry point
configs/            YAML configuration
internal/
  api/              QQ Bot REST API client
  audio/            Audio processing (SILK encode/decode, STT, format conversion)
  config/           Configuration loading and multi-account resolution
  gateway/          WebSocket gateway (heartbeat, reconnect, message queue)
  httpapi/          HTTP API server and webhook dispatcher
  image/            Image hosting server and dimension parsing
  outbound/         Outbound message handling, rate limiting, media tag parsing
  proactive/        Proactive messaging and cron-like scheduler
  qqbot/            Top-level orchestration (BotManager, health check)
  store/            SQLite persistent storage
  types/            Core domain types
  utils/            Utilities
```

Architecture documentation: [`docs/en/architecture.md`](docs/en/architecture.md).

## Development

```bash
# Test
go test -race -count=1 ./...

# Static analysis
go vet ./...

# Build
go build -o qqbot ./cmd/qqbot
```

## Dependencies

| Dependency | Purpose |
|------------|---------|
| `golang.org/x/sync` | singleflight (token deduplication) |
| `gopkg.in/yaml.v3` | YAML configuration parsing |
| `github.com/gorilla/websocket` | WebSocket connection |
| `modernc.org/sqlite` | SQLite (pure Go, no CGO required) |
| `ffmpeg` / `ffprobe` | Audio processing (runtime dependency) |

## License

MIT
