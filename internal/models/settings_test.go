package models

import (
	"testing"
)

func TestDefaultSettings(t *testing.T) {
	settings := DefaultSettings()

	if settings.Unit != "mg/dL" {
		t.Errorf("Default unit = %s, want mg/dL", settings.Unit)
	}
	if settings.RefreshInterval != 60 {
		t.Errorf("Default refresh interval = %d, want 60", settings.RefreshInterval)
	}
	if settings.TargetLow != 70 {
		t.Errorf("Default target low = %d, want 70", settings.TargetLow)
	}
	if settings.TargetHigh != 180 {
		t.Errorf("Default target high = %d, want 180", settings.TargetHigh)
	}
	if settings.UrgentLow != 55 {
		t.Errorf("Default urgent low = %d, want 55", settings.UrgentLow)
	}
	if settings.UrgentHigh != 250 {
		t.Errorf("Default urgent high = %d, want 250", settings.UrgentHigh)
	}
}

func TestSettings_GetGlucoseStatus(t *testing.T) {
	settings := DefaultSettings()

	tests := []struct {
		name     string
		mgdl     int
		expected string
	}{
		{"Urgent low", 50, "urgent_low"},
		{"Low", 60, "low"},
		{"Normal low boundary", 70, "low"},
		{"Normal", 120, "normal"},
		{"Normal high boundary", 180, "high"},
		{"High", 200, "high"},
		{"Urgent high", 260, "urgent_high"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := settings.GetGlucoseStatus(tt.mgdl)
			if result != tt.expected {
				t.Errorf("GetGlucoseStatus(%d) = %s, want %s", tt.mgdl, result, tt.expected)
			}
		})
	}
}

func TestSettings_Clone(t *testing.T) {
	original := DefaultSettings()
	original.NightscoutURL = "https://test.example.com"

	clone := original.Clone()

	if clone.NightscoutURL != original.NightscoutURL {
		t.Error("Clone did not copy NightscoutURL")
	}

	clone.NightscoutURL = "https://modified.example.com"
	if original.NightscoutURL == clone.NightscoutURL {
		t.Error("Modifying clone affected original")
	}
}

func TestSettings_IsConfigured(t *testing.T) {
	settings := DefaultSettings()

	if settings.IsConfigured() {
		t.Error("Empty settings should not be configured")
	}

	settings.NightscoutURL = "https://test.example.com"
	if !settings.IsConfigured() {
		t.Error("Settings with URL should be configured")
	}
}
