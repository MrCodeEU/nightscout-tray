# Nightscout Tray Installer for Windows
# Usage: irm https://raw.githubusercontent.com/mrcode/nightscout-tray/main/dist/scripts/install.ps1 | iex
# Or: Invoke-WebRequest -Uri ... -OutFile install.ps1; .\install.ps1

param(
    [string]$Version = "",
    [string]$InstallDir = "$env:LOCALAPPDATA\Programs\nightscout-tray"
)

$ErrorActionPreference = "Stop"

# Configuration
$Repo = "mrcode/nightscout-tray"
$AppName = "nightscout-tray"

function Write-Info { param($Message) Write-Host "[INFO] " -ForegroundColor Blue -NoNewline; Write-Host $Message }
function Write-Success { param($Message) Write-Host "[SUCCESS] " -ForegroundColor Green -NoNewline; Write-Host $Message }
function Write-Warning { param($Message) Write-Host "[WARNING] " -ForegroundColor Yellow -NoNewline; Write-Host $Message }
function Write-Error { param($Message) Write-Host "[ERROR] " -ForegroundColor Red -NoNewline; Write-Host $Message }

function Get-LatestVersion {
    Write-Info "Fetching latest version from GitHub..."
    
    try {
        $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
        return $release.tag_name
    }
    catch {
        Write-Error "Failed to get latest version: $_"
        exit 1
    }
}

function Get-Architecture {
    if ([Environment]::Is64BitOperatingSystem) {
        return "amd64"
    }
    else {
        Write-Error "32-bit Windows is not supported"
        exit 1
    }
}

function Install-NightscoutTray {
    param(
        [string]$Version,
        [string]$InstallDir
    )
    
    $arch = Get-Architecture
    $platform = "windows-$arch"
    $downloadUrl = "https://github.com/$Repo/releases/download/$Version/$AppName-$platform.zip"
    
    Write-Info "Downloading $AppName $Version for $platform..."
    
    # Create temp directory
    $tempDir = Join-Path $env:TEMP "nightscout-tray-install"
    if (Test-Path $tempDir) {
        Remove-Item -Recurse -Force $tempDir
    }
    New-Item -ItemType Directory -Path $tempDir | Out-Null
    
    try {
        # Download
        $zipPath = Join-Path $tempDir "nightscout-tray.zip"
        Invoke-WebRequest -Uri $downloadUrl -OutFile $zipPath -UseBasicParsing
        
        # Extract
        Write-Info "Extracting..."
        Expand-Archive -Path $zipPath -DestinationPath $tempDir -Force
        
        # Create install directory
        if (-not (Test-Path $InstallDir)) {
            New-Item -ItemType Directory -Path $InstallDir | Out-Null
        }
        
        # Copy files
        Write-Info "Installing to $InstallDir..."
        Copy-Item -Path "$tempDir\*.exe" -Destination $InstallDir -Force
        
        # Create Start Menu shortcut
        $startMenuPath = [Environment]::GetFolderPath("StartMenu")
        $shortcutPath = Join-Path $startMenuPath "Programs\Nightscout Tray.lnk"
        
        Write-Info "Creating Start Menu shortcut..."
        $shell = New-Object -ComObject WScript.Shell
        $shortcut = $shell.CreateShortcut($shortcutPath)
        $shortcut.TargetPath = Join-Path $InstallDir "$AppName.exe"
        $shortcut.WorkingDirectory = $InstallDir
        $shortcut.Description = "Nightscout glucose monitoring tray application"
        $shortcut.Save()
        
        # Add to PATH (user level)
        $userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
        if ($userPath -notlike "*$InstallDir*") {
            Write-Info "Adding to PATH..."
            [Environment]::SetEnvironmentVariable(
                "PATH",
                "$userPath;$InstallDir",
                "User"
            )
        }
        
        Write-Success "Installation complete!"
    }
    finally {
        # Cleanup
        if (Test-Path $tempDir) {
            Remove-Item -Recurse -Force $tempDir
        }
    }
}

function Main {
    Write-Host ""
    Write-Host "╔══════════════════════════════════════════╗" -ForegroundColor Cyan
    Write-Host "║     Nightscout Tray Installer            ║" -ForegroundColor Cyan
    Write-Host "╚══════════════════════════════════════════╝" -ForegroundColor Cyan
    Write-Host ""
    
    # Get version
    if ([string]::IsNullOrEmpty($Version)) {
        $Version = Get-LatestVersion
    }
    Write-Info "Version: $Version"
    
    # Install
    Install-NightscoutTray -Version $Version -InstallDir $InstallDir
    
    Write-Host ""
    Write-Success "Nightscout Tray has been installed!"
    Write-Host ""
    Write-Host "You can now:"
    Write-Host "  - Search for 'Nightscout Tray' in the Start Menu"
    Write-Host "  - Run 'nightscout-tray' from the command line (restart terminal first)"
    Write-Host ""
    Write-Host "On first run, configure your Nightscout URL in the Settings."
    Write-Host ""
}

Main
