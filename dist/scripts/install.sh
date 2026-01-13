#!/usr/bin/env bash
# Nightscout Tray Installer for Linux and macOS
# Usage: curl -sSL https://raw.githubusercontent.com/mrcode/nightscout-tray/main/dist/scripts/install.sh | bash
# Or: wget -qO- https://raw.githubusercontent.com/mrcode/nightscout-tray/main/dist/scripts/install.sh | bash

set -euo pipefail

# Configuration
REPO="mrcode/nightscout-tray"
APP_NAME="nightscout-tray"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
DESKTOP_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/applications"
ICON_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/icons/hicolor/256x256/apps"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
info() { echo -e "${BLUE}[INFO]${NC} $1"; }
success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1" >&2; }

# Detect OS and architecture
detect_platform() {
    local os arch

    case "$(uname -s)" in
        Linux*)  os="linux" ;;
        Darwin*) os="darwin" ;;
        *)       error "Unsupported OS: $(uname -s)"; exit 1 ;;
    esac

    case "$(uname -m)" in
        x86_64|amd64)   arch="amd64" ;;
        aarch64|arm64)  arch="arm64" ;;
        *)              error "Unsupported architecture: $(uname -m)"; exit 1 ;;
    esac

    echo "${os}-${arch}"
}

# Get the latest release version from GitHub
get_latest_version() {
    local version
    version=$(curl -sSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    
    if [[ -z "$version" ]]; then
        error "Failed to get latest version from GitHub"
        exit 1
    fi
    
    echo "$version"
}

# Download and extract the binary
download_and_install() {
    local version="$1"
    local platform="$2"
    local download_url="https://github.com/${REPO}/releases/download/${version}/${APP_NAME}-${platform}.tar.gz"
    local tmp_dir
    
    info "Downloading ${APP_NAME} ${version} for ${platform}..."
    
    tmp_dir=$(mktemp -d)
    trap "rm -rf ${tmp_dir}" EXIT
    
    if command -v curl &> /dev/null; then
        curl -sSL "$download_url" | tar -xzf - -C "$tmp_dir"
    elif command -v wget &> /dev/null; then
        wget -qO- "$download_url" | tar -xzf - -C "$tmp_dir"
    else
        error "Neither curl nor wget found. Please install one of them."
        exit 1
    fi
    
    # Create install directories
    mkdir -p "$INSTALL_DIR"
    
    # Install binary
    info "Installing binary to ${INSTALL_DIR}..."
    install -m 755 "${tmp_dir}/${APP_NAME}" "${INSTALL_DIR}/${APP_NAME}"
    
    # Linux-specific: Install desktop file and icon
    if [[ "$platform" == linux-* ]]; then
        mkdir -p "$DESKTOP_DIR" "$ICON_DIR"
        
        if [[ -f "${tmp_dir}/${APP_NAME}.desktop" ]]; then
            info "Installing desktop file..."
            install -m 644 "${tmp_dir}/${APP_NAME}.desktop" "${DESKTOP_DIR}/"
            # Update Exec path in desktop file
            sed -i "s|Exec=.*|Exec=${INSTALL_DIR}/${APP_NAME}|" "${DESKTOP_DIR}/${APP_NAME}.desktop"
        fi
        
        if [[ -f "${tmp_dir}/${APP_NAME}.png" ]]; then
            info "Installing icon..."
            install -m 644 "${tmp_dir}/${APP_NAME}.png" "${ICON_DIR}/"
        fi
        
        # Update desktop database if available
        if command -v update-desktop-database &> /dev/null; then
            update-desktop-database "$DESKTOP_DIR" 2>/dev/null || true
        fi
    fi
    
    success "Installation complete!"
}

# Verify PATH includes install directory
check_path() {
    if [[ ":$PATH:" != *":${INSTALL_DIR}:"* ]]; then
        warn "${INSTALL_DIR} is not in your PATH"
        echo ""
        echo "Add the following to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
        echo ""
        echo "  export PATH=\"\$PATH:${INSTALL_DIR}\""
        echo ""
    fi
}

# Main installation flow
main() {
    echo ""
    echo "╔══════════════════════════════════════════╗"
    echo "║     Nightscout Tray Installer            ║"
    echo "╚══════════════════════════════════════════╝"
    echo ""
    
    local platform version
    
    platform=$(detect_platform)
    info "Detected platform: ${platform}"
    
    version=$(get_latest_version)
    info "Latest version: ${version}"
    
    download_and_install "$version" "$platform"
    
    check_path
    
    echo ""
    success "Nightscout Tray has been installed to ${INSTALL_DIR}/${APP_NAME}"
    echo ""
    echo "To start the application, run:"
    echo "  ${APP_NAME}"
    echo ""
    echo "On first run, configure your Nightscout URL in the Settings."
    echo ""
}

# Run with optional version argument
if [[ "${1:-}" != "" ]]; then
    VERSION="$1"
else
    VERSION=""
fi

main
