#!/usr/bin/env bash
# ==============================================================================
#  jet CLI Installer
#  Usage:
#    curl -sSL https://raw.githubusercontent.com/tekibo/jet/main/install.sh | bash
#    curl -sSL https://raw.githubusercontent.com/tekibo/jet/main/install.sh | bash -s -- v1.0.0
# ==============================================================================

set -euo pipefail

REPO="tekibo/jet"
BINARY_NAME="jet"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

# 1. Parse Version Argument
VERSION="${1:-latest}"

echo "==> Installing ${BINARY_NAME} (${VERSION})..."

# 2. Detect Operating System
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  linux*)  OS="linux" ;;
  darwin*) OS="darwin" ;;
  msys*|mingw*|cygwin*) OS="windows" ;;
  *)
    echo "[-] Unsupported operating system: $OS"
    exit 1
    ;;
esac

# 3. Detect Architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)
    echo "[-] Unsupported CPU architecture: $ARCH"
    exit 1
    ;;
esac

# 4. Resolve latest release tag if version is 'latest'
if [ "$VERSION" = "latest" ]; then
  RELEASE_URL="https://api.github.com/repos/${REPO}/releases/latest"
  echo "==> Fetching latest release info from GitHub..."
  TAG="$(curl -sSL -H "Accept: application/vnd.github.v3+json" "$RELEASE_URL" | grep '"tag_name":' | head -n 1 | cut -d '"' -f 4 || true)"
  if [ -z "$TAG" ]; then
    echo "[!] No published release tag found on GitHub. Checking for local Go toolchain..."
    if command -v go >/dev/null 2>&1; then
      echo "==> Building directly from source via Go..."
      TMP_DIR="$(mktemp -d)"
      git clone --depth 1 "https://github.com/${REPO}.git" "$TMP_DIR"
      (cd "$TMP_DIR" && go build -ldflags="-s -w" -o "$INSTALL_DIR/$BINARY_NAME" .)
      rm -rf "$TMP_DIR"
      echo "==> Installed via Go to $INSTALL_DIR/$BINARY_NAME"
      exit 0
    else
      echo "[-] Error: No releases published yet on GitHub and Go compiler is not installed."
      exit 1
    fi
  fi
  VERSION="$TAG"
fi

# 5. Download Release Archive
EXT="tar.gz"
if [ "$OS" = "windows" ]; then
  EXT="zip"
fi

ARCHIVE_NAME="${BINARY_NAME}-${OS}-${ARCH}.${EXT}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE_NAME}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "==> Downloading from: $DOWNLOAD_URL"
if ! curl -sSL -f "$DOWNLOAD_URL" -o "$TMP_DIR/$ARCHIVE_NAME"; then
  echo "[-] Release asset ${ARCHIVE_NAME} not found on GitHub."
  echo "    Attempting to build from source via Go..."
  if command -v go >/dev/null 2>&1; then
    TMP_SRC="$(mktemp -d)"
    git clone --depth 1 "https://github.com/${REPO}.git" "$TMP_SRC"
    (cd "$TMP_SRC" && go build -ldflags="-s -w" -o "$INSTALL_DIR/$BINARY_NAME" .)
    rm -rf "$TMP_SRC"
  else
    echo "[-] Installation failed. Please check https://github.com/${REPO}/releases for available versions."
    exit 1
  fi
else
  # Unpack
  mkdir -p "$TMP_DIR/extracted"
  if [ "$EXT" = "zip" ]; then
    unzip -q "$TMP_DIR/$ARCHIVE_NAME" -d "$TMP_DIR/extracted"
  else
    tar -xzf "$TMP_DIR/$ARCHIVE_NAME" -C "$TMP_DIR/extracted"
  fi

  # Install binary
  mkdir -p "$INSTALL_DIR"
  mv "$TMP_DIR/extracted/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
  chmod +x "$INSTALL_DIR/$BINARY_NAME"
fi

echo "==> Successfully installed ${BINARY_NAME} to: ${INSTALL_DIR}/${BINARY_NAME}"

# 6. Verify PATH
if ! echo "$PATH" | grep -q "$INSTALL_DIR"; then
  echo ""
  echo "====================================================================="
  echo " NOTE: '${INSTALL_DIR}' is not currently in your PATH."
  echo " Add it to your shell configuration file (~/.bashrc or ~/.zshrc):"
  echo "   export PATH=\"\$PATH:${INSTALL_DIR}\""
  echo "====================================================================="
  echo ""
fi

# 7. Quick verification
if command -v "$BINARY_NAME" >/dev/null 2>&1 || [ -x "$INSTALL_DIR/$BINARY_NAME" ]; then
  "$INSTALL_DIR/$BINARY_NAME" --help | head -n 8
fi

echo ""
echo "✓ Installation complete! Run 'jet doctor' to verify your Android SDK."
