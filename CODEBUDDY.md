# CODEBUDDY.md — openclaw-qqbot

## Project Overview

QQ Bot HTTP API Service — a standalone Go service that exposes QQ Bot messaging capabilities via RESTful HTTP API. Supports C2C/group/guild messaging, rich media (image/voice/video/file), proactive scheduling, broadcast, webhook event forwarding, multi-account isolation, image hosting, and MCP Channel Server integration with CodeBuddy Code.

- **Module:** `github.com/openclaw/qqbot`
- **Go version:** 1.25
- **Entry points:** `cmd/qqbot/main.go` (API server + `channel` subcommand), `cmd/qqbot-channel/main.go` (standalone MCP channel server)

## Build & Run

```bash
# Build both binaries
go build -o qqbot ./cmd/qqbot
go build -o qqbot-channel ./cmd/qqbot-channel

# Run the API server
./qqbot -config configs/config.yaml -health :8080 -api :9090

# Run the MCP channel server
./qqbot-channel -qqbot-api http://127.0.0.1:9090 -webhook-port 8788

go test -race -count=1 ./...
go vet ./...
```

## Detailed Docs

| Doc | Description |
|-----|-------------|
| [docs/en/api.md](docs/en/api.md) | HTTP API endpoints, response format, webhook forwarding |
| [docs/en/architecture.md](docs/en/architecture.md) | Architecture patterns, concurrency, dependencies |
| [docs/en/configuration.md](docs/en/configuration.md) | CLI flags, config file options, webhook config |

## Project Structure

```
cmd/
  qqbot/            CLI entry point (API server)
  qqbot-channel/     CLI entry point (MCP channel server)
configs/            YAML configuration (config.example.yaml)
internal/
  api/              QQ Bot REST API client (token, media, messages)
  audio/            Audio processing (SILK encode/decode, STT, TTS via edge-tts)
  channel/          MCP Channel Server (stdio MCP + webhook receiver)
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
plugin/
  scripts/         Setup and utility scripts (MCP setup, etc.)
```

## Coding Conventions

- **Language:** Go 1.25, idiomatic Go style
- **Package naming:** Lowercase, single word (`api`, `config`, `types`)
- **File naming:** `snake_case`
- **Error handling:** Return `error`, use `fmt.Errorf` with `%w` wrapping
- **Logging:** Prefixed with `[gateway:accountID]`, `[qqbot]`, `[scheduler]`, `[webhook]`, `[channel]`
- **All code under `internal/`:** Not importable by external modules
- **Tests:** Standard `testing` package, one `*_test.go` per package, run with `-race`
- **Configuration:** YAML with Chinese comments
- **Documentation:** GoDoc comments on all exported types and functions
- **No external linting configs:** Follow Go defaults

## CI

GitHub Actions: `go vet` -> `go test -race` -> `go build` on Go 1.25, triggered on push/PR to `main`/`master`.

## Important Notes
- 执行前，先检查是否有合适的subagent可以完成任务，指派给subagent去执行。
- Do NOT add code under `pkg/` -- all application code belongs in `internal/`
- Data directory (`data/`) is created at runtime for persistent storage
- Health check endpoints: `/health` and `/healthz`
- API server endpoints: `/api/v1/` (disabled when `-api` is empty)
- No CGO required -- SILK encoding uses ffmpeg via `os/exec`
- Channel server supports two modes: embedded (`qqbot channel` subcommand) and standalone (`qqbot-channel` binary)
- Channel server uses PID file lock (`data/channel.pid`) to prevent duplicate instances; new instances take over via SIGTERM
- Channel server receives QQ events via HTTP webhook and delivers them as MCP notifications
- Channel server uses `claude/channel` experimental capability for CodeBuddy Code integration
- MCP setup: `plugin/scripts/setup-mcp.sh` — one-click installation script for CodeBuddy Code and Codex
- `.mcp.json` is generated per-project by the setup script (not checked into the repo)
