#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-latest}"
INSTALL_DIR="${HOME}/.local/bin"
CONFIG_DIR="${HOME}/.qqbot"
REPO="holy-tiger/qqbot-go"

# Resolve version
if [ "$VERSION" = "latest" ]; then
  VERSION=$(curl -sfL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
  if [ -z "$VERSION" ]; then
    echo "Error: failed to resolve latest version" >&2
    exit 1
  fi
fi

echo "Installing qqbot ${VERSION}..."

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$OS" in
  linux)  ;;
  darwin) ;;
  *)      echo "Unsupported OS: ${OS}" >&2; exit 1 ;;
esac
case "$ARCH" in
  x86_64|amd64) ARCH_SUFFIX="x86_64" ;;
  aarch64|arm64) ARCH_SUFFIX="aarch64" ;;
  *)              echo "Unsupported arch: ${ARCH}" >&2; exit 1 ;;
esac

ARCHIVE="qqbot_${OS}_${ARCH_SUFFIX}"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}.tar.gz"
TMPDIR=$(mktemp -d)

echo "Downloading ${URL}..."
curl -fSL -o "${TMPDIR}/${ARCHIVE}.tar.gz" "$URL"
tar -xzf "${TMPDIR}/${ARCHIVE}.tar.gz" -C "${TMPDIR}"

# Install binaries
mkdir -p "$INSTALL_DIR"
cp "${TMPDIR}/qqbot" "${INSTALL_DIR}/qqbot"
cp "${TMPDIR}/qqbot-channel" "${INSTALL_DIR}/qqbot-channel"
chmod +x "${INSTALL_DIR}/qqbot" "${INSTALL_DIR}/qqbot-channel"

# Create default config
mkdir -p "$CONFIG_DIR"
if [ ! -f "${CONFIG_DIR}/config.yaml" ]; then
  cat > "${CONFIG_DIR}/config.yaml" <<YAML
# QQ Bot Configuration
# See https://github.com/holy-tiger/qqbot-go for details

app_id: "YOUR_APP_ID"
app_secret: "YOUR_APP_SECRET"

# Channel server
webhook_port: 8788

# Optional: health and API server
# health: :8080
# api: :9090
YAML
  echo "Created default config at ${CONFIG_DIR}/config.yaml"
  echo "Please edit it with your QQ Bot credentials."
fi

rm -rf "$TMPDIR"
echo ""
echo "Installed:"
echo "  qqbot         -> ${INSTALL_DIR}/qqbot"
echo "  qqbot-channel -> ${INSTALL_DIR}/qqbot-channel"
echo ""
echo "Make sure ${INSTALL_DIR} is in your PATH."
