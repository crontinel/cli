#!/usr/bin/env bash
set -e

VERSION="${VERSION:-latest}"
OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
  Darwin*) OS="darwin" ;;
  Linux*)  OS="linux" ;;
  *)       echo "Unsupported OS: $OS"; exit 1 ;;
esac

case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)       echo "Unsupported arch: $ARCH"; exit 1 ;;
esac

if [ "$VERSION" = "latest" ]; then
  VERSION=$(curl -s https://api.github.com/repos/crontinel/cli/releases/latest | grep '"tag_name"' | sed 's/.*"v\?\([^"]*\)".*/\1/')
fi

DEST="${DEST:-$HOME/.local/bin/crontinel}"
TMP=$(mktemp -d)
ARCHIVE="crontinel_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/crontinel/cli/releases/download/v${VERSION}/${ARCHIVE}"

echo "Installing Crontinel CLI v${VERSION} for ${OS}/${ARCH}..."

curl -fsSL "$URL" -o "${TMP}/${ARCHIVE}"
tar -xzf "${TMP}/${ARCHIVE}" -C "$TMP"
mv "${TMP}/crontinel" "$DEST"
chmod +x "$DEST"
rm -rf "$TMP"

echo "Installed to $DEST"
echo "Add $DEST to your PATH if needed"
