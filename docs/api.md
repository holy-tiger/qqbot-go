# HTTP API Reference

All API endpoints are prefixed with `/api/v1`. No authentication is required (designed for internal network use).

## Message Sending

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

## Proactive & Broadcast

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/accounts/{id}/proactive/c2c/{openid}` | Proactive text to C2C user |
| POST | `/api/v1/accounts/{id}/proactive/groups/{openid}` | Proactive text to group |
| POST | `/api/v1/accounts/{id}/broadcast` | Broadcast to all C2C users |
| POST | `/api/v1/accounts/{id}/broadcast/groups` | Broadcast to all groups |

## Scheduler / Reminders

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/accounts/{id}/reminders` | Create reminder |
| DELETE | `/api/v1/accounts/{id}/reminders/{rem_id}` | Cancel reminder |
| GET | `/api/v1/accounts/{id}/reminders` | List all reminders |

## User Management

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/accounts/{id}/users` | List known users |
| GET | `/api/v1/accounts/{id}/users/stats` | User statistics |
| DELETE | `/api/v1/accounts/{id}/users` | Clear all users for account |

## Account Status

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health`, `/healthz` | Health check |
| GET | `/api/v1/accounts` | List all accounts with status |
| GET | `/api/v1/accounts/{id}` | Single account status |

## Response Format

All endpoints return JSON:

```json
{"ok": true, "data": { ... }}
{"ok": false, "error": "error message"}
```

## Webhook Event Forwarding

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
