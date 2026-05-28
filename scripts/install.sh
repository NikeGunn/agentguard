#!/usr/bin/env sh
# AgentGuard installer — macOS and Linux.
#
# Usage:
#   curl -fsSL https://agentguard.dev/install | sh
# Or pin to a version:
#   curl -fsSL https://agentguard.dev/install | sh -s -- --version v1.0.0
#
# What this does:
#   1. Detect OS + arch
#   2. Download the matching release tarball from GitHub
#   3. Verify SHA256 against checksums.txt
#   4. Install the binary to ~/.agentguard/bin/agentguard
#   5. Print PATH instructions

set -eu

REPO="agentguard/agentguard"
VERSION="latest"
INSTALL_DIR="${AGENTGUARD_HOME:-$HOME/.agentguard}/bin"

while [ $# -gt 0 ]; do
  case "$1" in
    --version) VERSION="$2"; shift 2;;
    --dir)     INSTALL_DIR="$2"; shift 2;;
    *) echo "unknown flag: $1" >&2; exit 1;;
  esac
done

uname_s=$(uname -s)
uname_m=$(uname -m)

case "$uname_s" in
  Darwin) OS="darwin";;
  Linux)  OS="linux";;
  *) echo "unsupported OS: $uname_s" >&2; exit 1;;
esac

case "$uname_m" in
  x86_64|amd64)  ARCH="amd64";;
  arm64|aarch64) ARCH="arm64";;
  *) echo "unsupported arch: $uname_m" >&2; exit 1;;
esac

if [ "$VERSION" = "latest" ]; then
  VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
            | grep '"tag_name":' | head -1 | sed 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/')
fi

TARBALL="agentguard_${VERSION#v}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/$VERSION/$TARBALL"
CHECKSUMS_URL="https://github.com/$REPO/releases/download/$VERSION/checksums.txt"

TMP=$(mktemp -d)
trap "rm -rf $TMP" EXIT

echo "==> AgentGuard $VERSION  ($OS/$ARCH)"
echo "==> Downloading $TARBALL"
curl -fsSL "$URL" -o "$TMP/$TARBALL"
curl -fsSL "$CHECKSUMS_URL" -o "$TMP/checksums.txt"

echo "==> Verifying SHA-256"
expected=$(grep " $TARBALL\$" "$TMP/checksums.txt" | awk '{print $1}')
if [ -z "$expected" ]; then
  echo "no checksum for $TARBALL in checksums.txt" >&2
  exit 1
fi
actual=$(sha256sum "$TMP/$TARBALL" 2>/dev/null | awk '{print $1}' || shasum -a 256 "$TMP/$TARBALL" | awk '{print $1}')
if [ "$expected" != "$actual" ]; then
  echo "SHA-256 mismatch! expected $expected, got $actual" >&2
  exit 1
fi
echo "    OK  $actual"

mkdir -p "$INSTALL_DIR"
tar -xzf "$TMP/$TARBALL" -C "$TMP"
mv "$TMP/agentguard" "$INSTALL_DIR/agentguard"
chmod +x "$INSTALL_DIR/agentguard"

echo "==> Installed to $INSTALL_DIR/agentguard"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    echo ""
    echo "Add this to your shell startup file (~/.bashrc, ~/.zshrc, etc.):"
    echo "    export PATH=\"$INSTALL_DIR:\$PATH\""
    ;;
esac

echo ""
echo "Next: run 'agentguard init' to protect your installed AI agents."
