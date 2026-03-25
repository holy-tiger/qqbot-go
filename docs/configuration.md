# Configuration Reference

## CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-config` | `configs/config.yaml` | YAML config file path |
| `-health` | `:8080` | Health check HTTP address (empty to disable) |
| `-api` | `:9090` | HTTP API server address (empty to disable) |

## Config File

See `configs/config.example.yaml` for all options. Secrets can be set via config, env vars (`QQBOT_APP_ID`, `QQBOT_CLIENT_SECRET`), or file (`clientSecretFile`).

## Webhook Configuration

- `defaultWebhookUrl`: global webhook URL for all accounts
- Per-account `webhookUrl`: overrides the global setting
