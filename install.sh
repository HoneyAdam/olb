#!/usr/bin/env sh
# OpenLoadBalancer install script
# Usage: curl -sSL https://openloadbalancer.dev/install.sh | sh
#        or: curl -sSL https://raw.githubusercontent.com/openloadbalancer/olb/main/install.sh | sh
set -eu

REPO="openloadbalancer/olb"
BINARY="olb"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# --- Helpers ---
info()  { printf '\033[1;34m[info]\033[0m  %s\n' "$*"; }
ok()    { printf '\033[1;32m[ok]\033[0m    %s\n' "$*"; }
err()   { printf '\033[1;31m[error]\033[0m %s\n' "$*" >&2; exit 1; }

# --- Detect platform ---
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
    linux)  OS="linux" ;;
    darwin) OS="darwin" ;;
    freebsd) OS="freebsd" ;;
    *)      err "Unsupported OS: $OS" ;;
esac

case "$ARCH" in
    x86_64|amd64)        ARCH="amd64" ;;
    aarch64|arm64)       ARCH="arm64" ;;
    *)                   err "Unsupported architecture: $ARCH" ;;
esac

# --- Determine version ---
if [ -n "${VERSION:-}" ]; then
    TAG="$VERSION"
else
    TAG=$(curl -sfL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep '"tag_name":' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
    if [ -z "$TAG" ]; then
        err "Could not determine latest version. Set VERSION env var manually."
    fi
fi

info "Installing OpenLoadBalancer ${TAG} for ${OS}-${ARCH}"

# --- Download ---
FILENAME="${BINARY}-${OS}-${ARCH}"
if [ "$OS" = "windows" ]; then
    FILENAME="${FILENAME}.exe"
fi

URL="https://github.com/${REPO}/releases/download/${TAG}/${FILENAME}"

TMPDIR="$(mktemp -d)"
TARGET="${TMPDIR}/${BINARY}"

info "Downloading ${URL}..."
curl -sfL "$URL" -o "$TARGET" || err "Download failed. Check that the release exists at ${URL}"

chmod +x "$TARGET"

# --- Install ---
if [ -w "$INSTALL_DIR" ]; then
    mv "$TARGET" "${INSTALL_DIR}/${BINARY}"
else
    info "Requires sudo to install to ${INSTALL_DIR}"
    sudo mv "$TARGET" "${INSTALL_DIR}/${BINARY}"
fi

# --- Verify ---
INSTALLED="$(${INSTALL_DIR}/${BINARY} version 2>/dev/null | head -1 || echo "${TAG}")"
ok "Installed: ${INSTALLED}"

# --- Cleanup ---
rm -rf "$TMPDIR"

printf '\nRun `olb setup` to create an initial configuration, or:\n'
printf '  olb start --config olb.yaml\n\n'
