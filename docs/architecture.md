# Architecture & Design Patterns

## Key Patterns

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
