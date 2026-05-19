#!/bin/bash
# Forge installer - one-liner to install The Forge
# Usage: curl -fsSL https://raw.githubusercontent.com/yethikrishna/the-forge/main/install.sh | bash

set -e

OS=$(uname -s | tr "[:upper:]" "[:lower:]")
ARCH=$(uname -m | sed "s/x86_64/amd64/;s/aarch64/arm64/")

if [ "$OS" != "linux" ] && [ "$OS" != "darwin" ]; then
    echo "Error: Unsupported OS: $OS"
    exit 1
fi

if [ "$ARCH" != "amd64" ] && [ "$ARCH" != "arm64" ]; then
    echo "Error: Unsupported architecture: $ARCH"
    exit 1
fi

INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY="forge-${OS}-${ARCH}"

# Check for latest release
LATEST_URL="https://github.com/yethikrishna/the-forge/releases/latest/download/${BINARY}"

echo "Forge: Downloading ${BINARY}..."
curl -fsSL "${LATEST_URL}" -o "${INSTALL_DIR}/forge" 2>/dev/null || {
    echo "Forge: No pre-built binary found. Building from source..."
    
    # Check for Go
    if ! command -v go &> /dev/null; then
        echo "Error: Go is required to build from source. Install from https://go.dev/dl/"
        exit 1
    fi
    
    echo "Forge: Cloning repository..."
    TMPDIR=$(mktemp -d)
    git clone --depth 1 https://github.com/yethikrishna/the-forge.git "${TMPDIR}/the-forge"
    cd "${TMPDIR}/the-forge"
    
    echo "Forge: Building..."
    make build
    
    cp forge "${INSTALL_DIR}/forge"
    rm -rf "${TMPDIR}"
}

chmod +x "${INSTALL_DIR}/forge"

echo ""
echo "Forge: Installed successfully!"
echo "  Binary: ${INSTALL_DIR}/forge"
echo ""
echo "  Quick start:"
echo "    forge serve -- claude"
echo "    forge agents"
echo "    forge orchestrate --agents claude,codex"
echo ""
echo "  The wielder and the sword are one."
