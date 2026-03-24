# CODEBUDDY.md — openclaw-qqbot

## Project Overview

QQ Bot Go bridge — a Go rewrite of a TypeScript QQ Bot channel plugin. Bridges an AI Agent framework (OpenClaw) with the QQ messaging platform. Supports C2C/group/guild messaging, rich media, voice (SILK codec + STT/TTS), proactive scheduling, multi-account, and image hosting.

- **Module:** `github.com/openclaw/qqbot`
- **Go version:** 1.25 (CI also tests 1.24)
- **Entry point:** `cmd/qqbot/main.go`

## Build & Run

```bash
# Build
go build -o qqbot ./cmd/qqbot

# Run (requires config)
./qqbot -config configs/config.yaml -health :8080

# Run tests
go test -race -count=1 ./...

# Static analysis
go vet ./...
```

## Project Structure

```
cmd/qqbot/          CLI entry point
configs/            YAML configuration (config.example.yaml)
internal/
  api/              QQ Bot REST API client (token, media, messages)
  audio/            Audio processing (SILK encode/decode, STT, format conversion)
  config/           Configuration loading and multi-account resolution
  gateway/          WebSocket gateway (heartbeat, reconnect, message queue)
  image/            Image hosting HTTP server and dimension parsing
  outbound/         Outbound message handling, rate limiting, media tag parsing
  proactive/        Proactive messaging and cron-like scheduler
  qqbot/            Top-level orchestration (BotManager, health check, validation)
  store/            File-based persistent storage (known users, ref index, sessions)
  types/            Core domain types (events, configs, payloads)
  utils/            Utilities (file validation, media tag normalization, payload parsing)
```

## Key Architecture Patterns

- **Multi-account isolation:** Each account gets independent APIClient, Gateway, OutboundHandler, ProactiveManager, and all stores.
- **Concurrency:** Per-user message queue with cross-user parallelism, singleflight for token dedup, mutex-protected stores with throttled disk writes.
- **Reconnection:** Exponential backoff [1s→2s→5s→10s→30s→60s], max 100 attempts, intent fallback on INVALID_SESSION.
- **Rate limiting:** 4 passive replies/message_id/hour, auto-fallback to proactive API.
- **Storage:** File-based (JSON/JSONL) with in-memory caches and TTL eviction.

## Dependencies

Direct: `golang.org/x/sync` (singleflight), `gopkg.in/yaml.v3`, `github.com/gorilla/websocket`
Runtime: `ffmpeg`/`ffprobe` for audio processing

## Coding Conventions

- **Language:** Go 1.25, idiomatic Go style
- **Package naming:** Lowercase, single word (`api`, `config`, `types`)
- **File naming:** `snake_case`
- **Error handling:** Return `error`, use `fmt.Errorf` with `%w` wrapping
- **Logging:** Prefixed with `[gateway:accountID]`, `[qqbot]`, `[scheduler]`
- **All code under `internal/`:** Not importable by external modules
- **Tests:** Standard `testing` package, one `*_test.go` per package, run with `-race`
- **Configuration:** YAML with Chinese comments
- **Documentation:** GoDoc comments on all exported types and functions
- **No external linting configs:** Follow Go defaults

## Configuration

See `configs/config.example.yaml` for all options. Secrets can be set via config, env vars (`QQBOT_APP_ID`, `QQBOT_CLIENT_SECRET`), or file (`clientSecretFile`).

## CI

GitHub Actions: `go vet` → `go test -race` → `go build` on Go 1.24 and 1.25, triggered on push/PR to `main`/`master`.

## Important Notes

- Do NOT add code under `pkg/` — all application code belongs in `internal/`
- Data directory (`data/`) is created at runtime for persistent storage
- Health check endpoints: `/health` and `/healthz`
- No CGO required — SILK encoding uses ffmpeg via `os/exec`
