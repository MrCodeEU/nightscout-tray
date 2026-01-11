# Nightscout Tray

A modern, cross-platform system tray application for monitoring glucose levels from your Nightscout instance.

![Nightscout Tray](docs/screenshot.png)

## Features

- 📊 **Real-time glucose monitoring** - Display current glucose value and trend arrow in system tray
- 📈 **Interactive chart** - View glucose history with customizable time ranges (4h, 8h, 12h, 24h, custom)
- ⚠️ **Configurable alerts** - System notifications for high/low glucose with optional sounds
- 🌍 **Multi-unit support** - Works with both mg/dL and mmol/L
- 🎨 **Customizable appearance** - Colors, chart styles (line, points, or both)
- 🔄 **Flexible refresh rate** - From 30 seconds to 10 minutes
- 🚀 **Auto-start** - Optional launch at system boot
- 🔐 **Secure** - Supports Nightscout API tokens and hashed secrets

## Supported Platforms

- **Linux**: Debian/Ubuntu (.deb), Fedora (.rpm), Arch (AUR), AppImage
- **Windows**: Installer (NSIS) and portable .exe
- **macOS**: .dmg and .app bundle

## Installation

### Package Managers (Recommended)

#### Homebrew (macOS/Linux)
```bash
brew tap mrcode/tap
brew install nightscout-tray
```

#### Scoop (Windows)
```powershell
scoop bucket add mrcode https://github.com/mrcode/scoop-bucket
scoop install nightscout-tray
```

#### AUR (Arch Linux)
```bash
yay -S nightscout-tray-bin
# or: paru -S nightscout-tray-bin
```

### Quick Install Scripts

#### Linux/macOS
```bash
curl -sSL https://raw.githubusercontent.com/mrcode/nightscout-tray/main/dist/scripts/install.sh | bash
```

#### Windows (PowerShell)
```powershell
irm https://raw.githubusercontent.com/mrcode/nightscout-tray/main/dist/scripts/install.ps1 | iex
```

### Manual Installation

#### Linux

**Debian/Ubuntu**
```bash
sudo dpkg -i nightscout-tray_1.0.0_amd64.deb
```

**Fedora/RHEL**
```bash
sudo dnf install nightscout-tray-1.0.0.x86_64.rpm
```

**AppImage**
```bash
chmod +x nightscout-tray-x86_64.AppImage
./nightscout-tray-x86_64.AppImage
```

#### Windows

Download and run `nightscout-tray-installer.exe` or extract the portable ZIP.

#### macOS

Mount the DMG and drag Nightscout Tray to Applications.

## Configuration

1. Launch the application
2. Click on the tray icon to open the main window
3. Navigate to Settings
4. Enter your Nightscout URL and API secret/token
5. Configure glucose ranges and alert thresholds
6. Customize chart appearance

## Building from Source

### Prerequisites

- Go 1.22+
- Node.js 18+
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

### Development

```bash
# Install dependencies
wails dev

# Run in development mode
wails dev
```

### Production Build

```bash
wails build
```

## License

MIT License - see [LICENSE](LICENSE) file.

## Contributing

Contributions are welcome! Please read our contributing guidelines before submitting PRs.

## Acknowledgments

- [Nightscout](https://nightscout.github.io/) - The amazing open-source CGM data platform
- [Wails](https://wails.io/) - Cross-platform desktop app framework for Go
