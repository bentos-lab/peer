#!/usr/bin/env bash

set -euo pipefail

REPO="bentos-lab/peer"
APP_NAME="peer"
INSTALL_DIR="/usr/local/bin"

# Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"

case "$OS" in
  linux) OS="linux" ;;
  darwin) OS="darwin" ;;
  *)
    echo "❌ Unsupported OS: $OS"
    exit 1
    ;;
esac

# Detect ARCH
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "❌ Unsupported ARCH: $ARCH"
    exit 1
    ;;
esac

echo "🔍 Detect: $OS/$ARCH"

# Get latest version from GitHub
VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep tag_name | cut -d '"' -f4)

echo "📦 Latest version: $VERSION"

FILENAME="${APP_NAME}-${VERSION}-${OS}-${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}"

TMP_DIR=$(mktemp -d)

echo "⬇️ Downloading..."
curl -fsSL "$URL" -o "${TMP_DIR}/${FILENAME}"

echo "📂 Extracting..."
tar -xzf "${TMP_DIR}/${FILENAME}" -C "${TMP_DIR}"

echo "🚀 Installing to ${INSTALL_DIR} (may need sudo)..."

if [ -w "${INSTALL_DIR}" ]; then
  mv "${TMP_DIR}/${APP_NAME}" "${INSTALL_DIR}/${APP_NAME}"
else
  sudo mv "${TMP_DIR}/${APP_NAME}" "${INSTALL_DIR}/${APP_NAME}"
fi

chmod +x "${INSTALL_DIR}/${APP_NAME}"

rm -rf "${TMP_DIR}"

echo ""
echo "✅ Installed!"
echo "👉 Run: ${APP_NAME} --version"
