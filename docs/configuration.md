# Configuration Reference

## CLI Flags

| Flag | Default | Required | Description |
|------|---------|----------|-------------|
| `-config` | (auto-detect `configs/config.yaml`) | No* | Path to YAML configuration file |
| `-health` | `:8080` | No | Health check HTTP listen address (empty string to disable) |
| `-api` | `:9090` | No | HTTP API server listen address (empty string to disable) |

\* When `-config` is not specified, the program checks if `configs/config.yaml` exists in the current directory and uses it automatically. If neither is available, startup fails.

**Usage:**

```bash
qqbot -config configs/config.yaml -health :8080 -api :9090
```

---

## Config File Structure

The YAML config file has a single top-level `qqbot` key:

```yaml
qqbot:
  # Default account fields (inline)
  appId: "..."
  # ...

  # Multi-account overrides
  accounts:
    account-name:
      appId: "..."
      # ...
```

All top-level fields under `qqbot:` define the **default account** (ID: `"default"`). Named accounts are defined under `qqbot.accounts:` and inherit no fields from the default account -- each is fully independent.

---

## Default Account Fields

These fields are set at the top level under `qqbot:` and apply to the `"default"` account.

### Authentication

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `appId` | string | (none) | QQ Bot application ID. **Required.** |
| `clientSecret` | string | (none) | Client secret for token authentication. |
| `clientSecretFile` | string | (none) | Path to a file containing the client secret. Takes priority over `clientSecret` if both are set. |

### Identity

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Whether the account is active. Disabled accounts are skipped during startup. |
| `name` | string | (none) | Human-readable bot name. |

### Messaging

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `markdownSupport` | bool | `true` | Enable markdown format in outbound messages. |
| `systemPrompt` | string | (none) | AI system prompt (for integrations that use LLM). |

### Image Hosting

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `imageServerBaseUrl` | string | (none) | Base URL for the local image hosting server. Non-HTTP image paths in messages are prefixed with this URL. |

### Access Control

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `dmPolicy` | string | (none) | Private message policy: `"open"` (allow all) or `"allowlist"` (restrict to `allowFrom`). |
| `allowFrom` | []string | (none) | List of user IDs allowed to DM the bot when `dmPolicy` is `"allowlist"`. Use `"*"` to allow all. |

### Audio Processing

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `voiceDirectUploadFormats` | []string | `["silk"]` | Audio formats that skip SILK encoding and are uploaded directly. |
| `audioFormatPolicy.sttDirectFormats` | []string | `["wav", "mp3"]` | Audio formats for which speech-to-text (STT) skips transcoding. |
| `audioFormatPolicy.uploadDirectFormats` | []string | `["silk"]` | Audio formats that skip SILK encoding for upload to QQ Bot. |

### Voice

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `ttsVoice` | string | (none) | TTS voice selection (used by the TTS provider for voice synthesis). |

### Webhook

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `webhookUrl` | string | (none) | Per-account webhook URL for event forwarding. For the default account, this takes priority over `defaultWebhookUrl`. |
| `defaultWebhookUrl` | string | (none) | Global default webhook URL for all accounts that don't have a per-account `webhookUrl` set. |

---

## Multi-Account Configuration

Named accounts are defined under `qqbot.accounts:`. Each account is fully independent and does **not** inherit fields from the default account.

```yaml
qqbot:
  # Default account
  appId: "app-id-1"
  clientSecret: "secret-1"
  defaultWebhookUrl: "http://localhost:3000/webhook"

  accounts:
    second-bot:
      appId: "app-id-2"
      clientSecret: "secret-2"
      enabled: true
      name: "Second Bot"
      webhookUrl: "http://localhost:3000/webhook/second-bot"

    third-bot:
      appId: "app-id-3"
      clientSecretFile: "/secrets/third-bot-secret.txt"
      enabled: false
      # This account will be skipped during startup
```

Named accounts support the same fields as the default account (all fields except `defaultWebhookUrl`, which is a global-only setting).

### Account ID Resolution

| Scenario | Account ID |
|----------|-----------|
| Top-level fields under `qqbot:` | `"default"` |
| Named entry under `qqbot.accounts.second-bot:` | `"second-bot"` |

The account ID is used in API endpoints as the `{id}` path parameter.

---

## Environment Variables

Environment variables are **only used as fallbacks for the default account**. Named accounts under `accounts:` do not get environment variable fallbacks.

| Variable | Maps To | Priority |
|----------|---------|----------|
| `QQBOT_APP_ID` | `appId` | Lowest (config > file > env) |
| `QQBOT_CLIENT_SECRET` | `clientSecret` | Lowest (config > file > env) |
| `QQBOT_IMAGE_SERVER_BASE_URL` | `imageServerBaseUrl` | Fallback if config value is empty |

### Secret Resolution Order

For the default account, `clientSecret` is resolved in this priority order:

```
1. qqbot.clientSecret (config value)
2. qqbot.clientSecretFile (read file contents)
3. QQBOT_CLIENT_SECRET (environment variable)
```

For named accounts, only options 1 and 2 apply (no env var fallback).

---

## Webhook Configuration

### URL Resolution

Each account's webhook URL is resolved in this priority order:

```
1. Per-account webhookUrl      (qqbot.accounts.<name>.webhookUrl)
2. Global defaultWebhookUrl    (qqbot.defaultWebhookUrl)
3. No webhook (events not forwarded)
```

### Behavior

- When a webhook URL is configured, all incoming message events are forwarded via async HTTP POST.
- Delivery has up to 3 retries with exponential backoff (1s, 2s).
- Request timeout: 10 seconds per attempt.
- Events are forwarded non-blocking -- gateway message processing is not delayed.
- No authentication headers are added to webhook requests.

---

## Validation

The configuration is validated at startup. The service will refuse to start if validation fails.

### Errors (startup blocked)

| Condition | Message |
|-----------|---------|
| `qqbot:` section missing | `"config: qqbot section is required"` |
| Top-level account has no `appId` | `"config: top-level account has no appId configured"` |
| Named account has no `appId` | `"config: account \"name\" has no appId configured"` |
| No accounts have a valid `appId` | `"config: at least one account must have a non-empty appId"` |
| Duplicate account ID | `"config: duplicate account ID \"name\""` |

### Warnings (startup continues)

| Condition | Message |
|-----------|---------|
| Account has no `clientSecret` | `"config: account \"name\" has no clientSecret configured"` |
| Top-level account has no `clientSecret` | `"config: top-level account has no clientSecret configured"` |

Secrets may be provided via environment variables at runtime, so missing `clientSecret` is a warning, not an error.

---

## Full Example

```yaml
qqbot:
  # ====== Default Account (id: "default") ======
  appId: "1234567890"
  clientSecret: "your-client-secret-here"

  enabled: true
  name: "My QQ Bot"
  markdownSupport: true

  # AI system prompt (for LLM integrations)
  systemPrompt: "You are a helpful assistant."

  # Health check address and API server address are set via CLI flags:
  # -health :8080 -api :9090

  # Global webhook (used by accounts without per-account webhookUrl)
  defaultWebhookUrl: "http://your-server:3000/webhook"

  # Image hosting server (for local image URL resolution)
  imageServerBaseUrl: "http://your-ip:18765"

  # Private message access control
  dmPolicy: "open"
  allowFrom:
    - "*"

  # Audio processing options
  voiceDirectUploadFormats:
    - "silk"
  audioFormatPolicy:
    sttDirectFormats:
      - "wav"
      - "mp3"
    uploadDirectFormats:
      - "silk"

  # TTS voice
  ttsVoice: "default"

  # ====== Multi-Account Configuration ======
  accounts:
    second-bot:
      appId: "0987654321"
      clientSecret: "second-bot-secret"
      enabled: true
      name: "Second Bot"
      systemPrompt: "You are another helpful assistant."
      # Override global webhook for this account
      webhookUrl: "http://your-server:3000/webhook/second-bot"

    disabled-bot:
      appId: "1111111111"
      enabled: false  # This account will be skipped
```

---

## File Locations

| Path | Description |
|------|-------------|
| `configs/config.yaml` | Default config file path (auto-detected) |
| `configs/config.example.yaml` | Example configuration with all options documented |
| `data/qqbot.db` | SQLite database (created at runtime) |
