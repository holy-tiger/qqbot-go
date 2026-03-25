#!/usr/bin/env bash
# CodeBuddy Notification -> QQ Bot notification script
# Reads Notification event JSON from stdin and sends a QQ message.
#
# Environment variables (override defaults):
#   QQBOT_API_URL   - qqbot HTTP API base URL (default: http://localhost:9090)
#   QQBOT_ACCOUNT   - account ID (default: default)
#   QQBOT_OPENID    - C2C target openid (default: CD98C1EE7B7E043405044497FE6366BC)

API_URL="${QQBOT_API_URL:-http://localhost:9090}"
ACCOUNT="${QQBOT_ACCOUNT:-default}"
OPENID="${QQBOT_OPENID:-CD98C1EE7B7E043405044497FE6366BC}"

# Read the Notification event JSON from stdin
INPUT=$(cat)

# Extract notification_type and message
NOTIF_TYPE=$(echo "$INPUT" | jq -r '.notification_type // "unknown"')
MESSAGE=$(echo "$INPUT" | jq -r '.message // "No message"')

# Truncate message if too long (QQ text limit is ~2000 chars, keep it safe)
if [ ${#MESSAGE} -gt 500 ]; then
    MESSAGE="${MESSAGE:0:500}..."
fi

# Build notification content with type label
case "$NOTIF_TYPE" in
    permission_prompt)
        LABEL="[CodeBuddy] Permission Required"
        ;;
    idle_prompt)
        LABEL="[CodeBuddy] Idle Reminder"
        ;;
    auth_success)
        LABEL="[CodeBuddy] Auth Success"
        ;;
    *)
        LABEL="[CodeBuddy] $NOTIF_TYPE"
        ;;
esac

CONTENT="${LABEL}

${MESSAGE}"

# Send via qqbot C2C message API
BODY=$(jq -n --arg c "$CONTENT" '{content: $c}')
RESPONSE=$(curl -s --connect-timeout 5 --max-time 10 -X POST \
    "${API_URL}/api/v1/accounts/${ACCOUNT}/c2c/${OPENID}/messages" \
    -H "Content-Type: application/json" \
    -d "$BODY" 2>&1)

# Log result (only visible in CodeBuddy debug mode)
if [ -z "$RESPONSE" ]; then
    echo "QQ notification failed: cannot connect to ${API_URL}" >&2
    exit 1
fi

OK=$(echo "$RESPONSE" | jq -r '.ok // false' 2>/dev/null)
if [ "$OK" = "true" ]; then
    echo "QQ notification sent: ${NOTIF_TYPE}" >&2
else
    echo "QQ notification failed: ${RESPONSE}" >&2
fi
