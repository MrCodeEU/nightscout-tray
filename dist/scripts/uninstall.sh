#!/usr/bin/env bash
# Nightscout Tray Uninstaller for Linux and macOS
# Usage: curl -sSL https://raw.githubusercontent.com/mrcode/nightscout-tray/main/dist/scripts/uninstall.sh | bash

set -euo pipefail

APP_NAME="nightscout-tray"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
DESKTOP_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/applications"
ICON_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/icons/hicolor/256x256/apps"
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/${APP_NAME}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info() { echo -e "${BLUE}[INFO]${NC} $1"; }
success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARNING]${NC} $1"; }

main() {
    echo ""
    echo "╔══════════════════════════════════════════╗"
    echo "║     Nightscout Tray Uninstaller          ║"
    echo "╚══════════════════════════════════════════╝"
    echo ""
    
    # Remove binary
    if [[ -f "${INSTALL_DIR}/${APP_NAME}" ]]; then
        info "Removing binary..."
        rm -f "${INSTALL_DIR}/${APP_NAME}"
    fi
    
    # Remove desktop file
    if [[ -f "${DESKTOP_DIR}/${APP_NAME}.desktop" ]]; then
        info "Removing desktop file..."
        rm -f "${DESKTOP_DIR}/${APP_NAME}.desktop"
    fi
    
    # Remove icon
    if [[ -f "${ICON_DIR}/${APP_NAME}.png" ]]; then
        info "Removing icon..."
        rm -f "${ICON_DIR}/${APP_NAME}.png"
    fi
    
    # Ask about config
    if [[ -d "$CONFIG_DIR" ]]; then
        echo ""
        read -p "Remove configuration files in ${CONFIG_DIR}? [y/N] " -n 1 -r
        echo ""
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            info "Removing configuration..."
            rm -rf "$CONFIG_DIR"
        else
            warn "Configuration files kept in ${CONFIG_DIR}"
        fi
    fi
    
    # Update desktop database
    if command -v update-desktop-database &> /dev/null; then
        update-desktop-database "$DESKTOP_DIR" 2>/dev/null || true
    fi
    
    echo ""
    success "Nightscout Tray has been uninstalled."
    echo ""
}

main
