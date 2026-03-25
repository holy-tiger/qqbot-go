# CODEBUDDY.md — openclaw-qqbot

## Project Overview

QQ Bot HTTP API Service — a standalone Go service that exposes QQ Bot messaging capabilities via RESTful HTTP API. Supports C2C/group/guild messaging, rich media (image/voice/video/file), proactive scheduling, broadcast, webhook event forwarding, multi-account isolation, and image hosting.

- **Module:** `github.com/openclaw/qqbot`
- **Go version:** 1.25 (CI also tests 1.24)
- **Entry point:** `cmd/qqbot/main.go`

## Build & Run

```bash
go build -o qqbot ./cmd/qqbot
./qqbot -config configs/config.yaml -health :8080 -api :9090
go test -race -count=1 ./...
go vet ./...
```

## Detailed Docs

| Doc | Description |
|-----|-------------|
| [docs/api.md](docs/api.md) | HTTP API endpoints, response format, webhook forwarding |
| [docs/architecture.md](docs/architecture.md) | Architecture patterns, concurrency, dependencies |
| [docs/configuration.md](docs/configuration.md) | CLI flags, config file options, webhook config |

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

## CI

GitHub Actions: `go vet` -> `go test -race` -> `go build` on Go 1.24 and 1.25, triggered on push/PR to `main`/`master`.

## Important Notes

- Do NOT add code under `pkg/` -- all application code belongs in `internal/`
- Data directory (`data/`) is created at runtime for persistent storage
- Health check endpoints: `/health` and `/healthz`
- API server endpoints: `/api/v1/` (disabled when `-api` is empty)
- No CGO required -- SILK encoding uses ffmpeg via `os/exec`
