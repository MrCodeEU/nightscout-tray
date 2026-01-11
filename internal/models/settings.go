// Package models contains data structures used throughout the application
package models

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// Settings contains all application settings
type Settings struct {
	mu sync.RWMutex `json:"-"`

	// Connection settings
	NightscoutURL string `json:"nightscoutUrl"`
	APISecret     string `json:"apiSecret"` // Plain API secret (will be hashed)
	APIToken      string `json:"apiToken"`  // Token-based auth
	UseToken      bool   `json:"useToken"`  // Use token instead of secret

	// Display settings
	Unit            string `json:"unit"`            // "mg/dL" or "mmol/L"
	RefreshInterval int    `json:"refreshInterval"` // Seconds (30-600)

	// Glucose thresholds (in mg/dL, converted for display)
	TargetLow  int `json:"targetLow"`
	TargetHigh int `json:"targetHigh"`
	UrgentLow  int `json:"urgentLow"`
	UrgentHigh int `json:"urgentHigh"`

	// Alert settings
	EnableHighAlert       bool `json:"enableHighAlert"`
	EnableLowAlert        bool `json:"enableLowAlert"`
	EnableUrgentHighAlert bool `json:"enableUrgentHighAlert"`
	EnableUrgentLowAlert  bool `json:"enableUrgentLowAlert"`
	EnableSoundAlerts     bool `json:"enableSoundAlerts"`
	RepeatAlertMinutes    int  `json:"repeatAlertMinutes"` // 0 = no repeat

	// Chart settings
	ChartTimeRange    int    `json:"chartTimeRange"`    // Hours (default 4)
	ChartMaxHistory   int    `json:"chartMaxHistory"`   // Days (default 7)
	ChartStyle        string `json:"chartStyle"`        // "line", "points", "both"
	ChartColorInRange string `json:"chartColorInRange"` // Hex color
	ChartColorHigh    string `json:"chartColorHigh"`
	ChartColorLow     string `json:"chartColorLow"`
	ChartColorUrgent  string `json:"chartColorUrgent"`
	ChartShowTarget   bool   `json:"chartShowTarget"` // Show target range band
	ChartShowNow      bool   `json:"chartShowNow"`    // Show current time marker

	// System settings
	StartMinimized bool `json:"startMinimized"`
	AutoStart      bool `json:"autoStart"`
	ShowInTaskbar  bool `json:"showInTaskbar"` // Windows only

	// Window state (not user-configurable)
	WindowWidth  int `json:"windowWidth"`
	WindowHeight int `json:"windowHeight"`
	WindowX      int `json:"windowX"`
	WindowY      int `json:"windowY"`
}

// DefaultSettings returns settings with default values
func DefaultSettings() *Settings {
	return &Settings{
		NightscoutURL:   "",
		APISecret:       "",
		APIToken:        "",
		UseToken:        false,
		Unit:            "mg/dL",
		RefreshInterval: 60, // 1 minute default

		TargetLow:  70,
		TargetHigh: 180,
		UrgentLow:  55,
		UrgentHigh: 250,

		EnableHighAlert:       true,
		EnableLowAlert:        true,
		EnableUrgentHighAlert: true,
		EnableUrgentLowAlert:  true,
		EnableSoundAlerts:     true,
		RepeatAlertMinutes:    15,

		ChartTimeRange:    4,
		ChartMaxHistory:   7,
		ChartStyle:        "both",
		ChartColorInRange: "#4ade80", // Green
		ChartColorHigh:    "#facc15", // Yellow
		ChartColorLow:     "#f97316", // Orange
		ChartColorUrgent:  "#ef4444", // Red
		ChartShowTarget:   true,
		ChartShowNow:      true,

		StartMinimized: true,
		AutoStart:      false,
		ShowInTaskbar:  true,

		WindowWidth:  900,
		WindowHeight: 700,
		WindowX:      -1,
		WindowY:      -1,
	}
}

// GetConfigDir returns the configuration directory path
func GetConfigDir() (string, error) {
	var configDir string

	switch runtime.GOOS {
	case "windows":
		configDir = os.Getenv("APPDATA")
		if configDir == "" {
			configDir = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configDir = filepath.Join(home, "Library", "Application Support")
	default: // Linux and others
		configDir = os.Getenv("XDG_CONFIG_HOME")
		if configDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			configDir = filepath.Join(home, ".config")
		}
	}

	appDir := filepath.Join(configDir, "nightscout-tray")
	if err := os.MkdirAll(appDir, 0750); err != nil {
		return "", err
	}

	return appDir, nil
}

// GetConfigPath returns the full path to the config file
func GetConfigPath() (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

// Load loads settings from disk
func (s *Settings) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := GetConfigPath()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(path) //nolint:gosec // Config path is controlled by the app, not user input
	if err != nil {
		if os.IsNotExist(err) {
			// Use defaults if file doesn't exist
			s.copySettingsFields(DefaultSettings())
			return nil
		}
		return err
	}

	if err := json.Unmarshal(data, s); err != nil {
		return err
	}

	return nil
}

// Save saves settings to disk
func (s *Settings) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path, err := GetConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// Clone creates a copy of the settings
func (s *Settings) Clone() *Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Create a new Settings struct with copied values (not the mutex)
	clone := &Settings{}
	clone.copySettingsFields(s)
	return clone
}

// Update updates settings from another Settings object
func (s *Settings) Update(other *Settings) {
	s.mu.Lock()
	defer s.mu.Unlock()
	other.mu.RLock()
	defer other.mu.RUnlock()

	s.copySettingsFields(other)
}

// copySettingsFields copies all fields from other to s, excluding the mutex
// The caller must hold the necessary locks on s and other (if other is shared)
func (s *Settings) copySettingsFields(other *Settings) {
	s.NightscoutURL = other.NightscoutURL
	s.APISecret = other.APISecret
	s.APIToken = other.APIToken
	s.UseToken = other.UseToken
	s.Unit = other.Unit
	s.RefreshInterval = other.RefreshInterval
	s.TargetLow = other.TargetLow
	s.TargetHigh = other.TargetHigh
	s.UrgentLow = other.UrgentLow
	s.UrgentHigh = other.UrgentHigh
	s.EnableHighAlert = other.EnableHighAlert
	s.EnableLowAlert = other.EnableLowAlert
	s.EnableUrgentHighAlert = other.EnableUrgentHighAlert
	s.EnableUrgentLowAlert = other.EnableUrgentLowAlert
	s.EnableSoundAlerts = other.EnableSoundAlerts
	s.RepeatAlertMinutes = other.RepeatAlertMinutes
	s.ChartTimeRange = other.ChartTimeRange
	s.ChartMaxHistory = other.ChartMaxHistory
	s.ChartStyle = other.ChartStyle
	s.ChartColorInRange = other.ChartColorInRange
	s.ChartColorHigh = other.ChartColorHigh
	s.ChartColorLow = other.ChartColorLow
	s.ChartColorUrgent = other.ChartColorUrgent
	s.ChartShowTarget = other.ChartShowTarget
	s.ChartShowNow = other.ChartShowNow
	s.StartMinimized = other.StartMinimized
	s.AutoStart = other.AutoStart
	s.ShowInTaskbar = other.ShowInTaskbar
	s.WindowWidth = other.WindowWidth
	s.WindowHeight = other.WindowHeight
	s.WindowX = other.WindowX
	s.WindowY = other.WindowY
}

// IsConfigured returns true if minimum required settings are set
func (s *Settings) IsConfigured() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.NightscoutURL != ""
}

// GetGlucoseStatus returns the status string for a glucose value
func (s *Settings) GetGlucoseStatus(mgdl int) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	switch {
	case mgdl <= s.UrgentLow:
		return "urgent_low"
	case mgdl <= s.TargetLow:
		return "low"
	case mgdl >= s.UrgentHigh:
		return "urgent_high"
	case mgdl >= s.TargetHigh:
		return "high"
	default:
		return "normal"
	}
}
