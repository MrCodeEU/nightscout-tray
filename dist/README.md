# Distribution Files

This directory contains package definitions and installation scripts for various package managers.

## Package Managers

### Homebrew (macOS/Linux)

The Homebrew formula is automatically updated when a new release is tagged.

**Setup (one-time):**
```bash
brew tap mrcode/tap
```

**Install:**
```bash
brew install nightscout-tray
```

**Update:**
```bash
brew upgrade nightscout-tray
```

### Scoop (Windows)

The Scoop manifest is automatically updated when a new release is tagged.

**Setup (one-time):**
```powershell
scoop bucket add mrcode https://github.com/mrcode/scoop-bucket
```

**Install:**
```powershell
scoop install nightscout-tray
```

**Update:**
```powershell
scoop update nightscout-tray
```

### AUR (Arch Linux)

The PKGBUILD is generated for each release. You can find it in the release artifacts.

**Install with yay:**
```bash
yay -S nightscout-tray-bin
```

**Install with paru:**
```bash
paru -S nightscout-tray-bin
```

**Manual install:**
```bash
git clone https://aur.archlinux.org/nightscout-tray-bin.git
cd nightscout-tray-bin
makepkg -si
```

## Direct Installation Scripts

### Linux/macOS

```bash
# Install
curl -sSL https://raw.githubusercontent.com/mrcode/nightscout-tray/main/dist/scripts/install.sh | bash

# Or with wget
wget -qO- https://raw.githubusercontent.com/mrcode/nightscout-tray/main/dist/scripts/install.sh | bash

# Uninstall
curl -sSL https://raw.githubusercontent.com/mrcode/nightscout-tray/main/dist/scripts/uninstall.sh | bash
```

### Windows (PowerShell)

```powershell
# Install
irm https://raw.githubusercontent.com/mrcode/nightscout-tray/main/dist/scripts/install.ps1 | iex

# Or download and run
Invoke-WebRequest -Uri "https://raw.githubusercontent.com/mrcode/nightscout-tray/main/dist/scripts/install.ps1" -OutFile install.ps1
.\install.ps1
```

## Setting Up Package Manager Repositories

To enable automatic updates to package managers, you need to:

### 1. Create Homebrew Tap Repository

Create a new repository named `homebrew-tap` in your GitHub account.

Add the `HOMEBREW_TAP_TOKEN` secret to this repository with a GitHub Personal Access Token that has `repo` permissions.

### 2. Create Scoop Bucket Repository

Create a new repository named `scoop-bucket` in your GitHub account.

Create a `bucket` directory in that repository.

Add the `SCOOP_BUCKET_TOKEN` secret to this repository with a GitHub Personal Access Token that has `repo` permissions.

### 3. AUR Package

The AUR PKGBUILD is generated as a release artifact. To publish to AUR:

1. Create an AUR account
2. Create a new package named `nightscout-tray-bin`
3. Download the PKGBUILD and .SRCINFO from the release artifacts
4. Push to AUR using `git push`

## File Structure

```
dist/
├── homebrew/
│   └── nightscout-tray.rb      # Homebrew formula template
├── scoop/
│   └── nightscout-tray.json    # Scoop manifest template
├── aur/
│   ├── PKGBUILD                # AUR PKGBUILD (build from source)
│   └── PKGBUILD-bin            # AUR PKGBUILD (pre-built binary)
├── scripts/
│   ├── install.sh              # Linux/macOS install script
│   ├── install.ps1             # Windows install script
│   └── uninstall.sh            # Linux/macOS uninstall script
└── README.md                   # This file
```
