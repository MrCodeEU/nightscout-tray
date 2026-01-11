// Package autostart handles auto-start functionality across platforms
package autostart

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	appName        = "nightscout-tray"
	appDisplayName = "Nightscout Tray"

	// OS constants
	osLinux   = "linux"
	osWindows = "windows"
	osDarwin  = "darwin"
)

// IsEnabled checks if auto-start is enabled
func IsEnabled() (bool, error) {
	switch runtime.GOOS {
	case osLinux:
		return isEnabledLinux()
	case osWindows:
		return isEnabledWindows()
	case osDarwin:
		return isEnabledMacOS()
	default:
		return false, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// Enable enables auto-start
func Enable() error {
	switch runtime.GOOS {
	case osLinux:
		return enableLinux()
	case osWindows:
		return enableWindows()
	case osDarwin:
		return enableMacOS()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// Disable disables auto-start
func Disable() error {
	switch runtime.GOOS {
	case osLinux:
		return disableLinux()
	case osWindows:
		return disableWindows()
	case osDarwin:
		return disableMacOS()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// Linux implementation using XDG autostart
func getLinuxAutostartPath() (string, error) {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "autostart", appName+".desktop"), nil
}

func isEnabledLinux() (bool, error) {
	path, err := getLinuxAutostartPath()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	return err == nil, nil
}

func enableLinux() error {
	path, err := getLinuxAutostartPath()
	if err != nil {
		return err
	}

	// Get executable path
	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	// Create autostart directory
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}

	// Create .desktop file
	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Exec=%s
Icon=%s
Comment=Nightscout glucose monitoring tray application
Categories=Utility;
Terminal=false
StartupNotify=false
X-GNOME-Autostart-enabled=true
`, appDisplayName, execPath, appName)

	return os.WriteFile(path, []byte(content), 0600)
}

func disableLinux() error {
	path, err := getLinuxAutostartPath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Windows implementation using registry
func isEnabledWindows() (bool, error) {
	cmd := exec.Command("reg", "query",
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`,
		"/v", appName)
	err := cmd.Run()
	return err == nil, nil
}

func enableWindows() error {
	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	//nolint:gosec // G204: execPath comes from os.Executable(), not user input
	cmd := exec.Command("reg", "add",
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`,
		"/v", appName,
		"/t", "REG_SZ",
		"/d", execPath,
		"/f")
	return cmd.Run()
}

func disableWindows() error {
	cmd := exec.Command("reg", "delete",
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`,
		"/v", appName,
		"/f")
	err := cmd.Run()
	if err != nil && strings.Contains(err.Error(), "not exist") {
		return nil
	}
	return err
}

// macOS implementation using LaunchAgents
func getMacOSLaunchAgentPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", "com."+appName+".plist"), nil
}

func isEnabledMacOS() (bool, error) {
	path, err := getMacOSLaunchAgentPath()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	return err == nil, nil
}

func enableMacOS() error {
	path, err := getMacOSLaunchAgentPath()
	if err != nil {
		return err
	}

	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	// Create LaunchAgents directory
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}

	// Create plist file
	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <false/>
</dict>
</plist>
`, appName, execPath)

	return os.WriteFile(path, []byte(content), 0600)
}

func disableMacOS() error {
	path, err := getMacOSLaunchAgentPath()
	if err != nil {
		return err
	}

	// Unload the agent first (ignore errors as the file may not be loaded)
	//nolint:gosec // G204: path comes from getMacOSLaunchAgentPath(), not user input
	_ = exec.Command("launchctl", "unload", path).Run()

	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
