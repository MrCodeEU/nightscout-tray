package notifications

import (
	"strings"
	"testing"
	"time"

	"github.com/mrcode/nightscout-tray/internal/models"
)

// Test constants
const (
	testUrgentLow = "urgent_low"
	testMmolUnit  = "mmol/L"
)

func TestManager_shouldAlert(t *testing.T) {
	settings := models.DefaultSettings()
	manager := NewManager(settings)

	tests := []struct {
		name     string
		status   string
		expected string
	}{
		{"Urgent low enabled", "urgent_low", "urgent_low"},
		{"Low enabled", "low", "low"},
		{"High enabled", "high", "high"},
		{"Urgent high enabled", "urgent_high", "urgent_high"},
		{"Normal", "normal", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := &models.GlucoseStatus{Status: tt.status}
			result := manager.shouldAlert(status)
			if result != tt.expected {
				t.Errorf("shouldAlert() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestManager_shouldAlert_Disabled(t *testing.T) {
	settings := models.DefaultSettings()
	settings.EnableLowAlert = false
	settings.EnableHighAlert = false
	manager := NewManager(settings)

	status := &models.GlucoseStatus{Status: "low"}
	result := manager.shouldAlert(status)
	if result != "" {
		t.Errorf("shouldAlert() = %s, want empty (disabled)", result)
	}

	status = &models.GlucoseStatus{Status: "high"}
	result = manager.shouldAlert(status)
	if result != "" {
		t.Errorf("shouldAlert() = %s, want empty (disabled)", result)
	}

	status = &models.GlucoseStatus{Status: testUrgentLow}
	result = manager.shouldAlert(status)
	if result != testUrgentLow {
		t.Errorf("shouldAlert() = %s, want %s", result, testUrgentLow)
	}
}

func TestManager_formatNotification(t *testing.T) {
	settings := models.DefaultSettings()
	manager := NewManager(settings)

	tests := []struct {
		alertType     string
		expectedTitle string
	}{
		{"urgent_low", "⚠️ URGENT LOW GLUCOSE"},
		{"low", "⬇️ Low Glucose"},
		{"high", "⬆️ High Glucose"},
		{"urgent_high", "⚠️ URGENT HIGH GLUCOSE"},
	}

	status := &models.GlucoseStatus{
		Value:     100,
		ValueMmol: 5.5,
		Trend:     "→",
	}

	for _, tt := range tests {
		t.Run(tt.alertType, func(t *testing.T) {
			title, _ := manager.formatNotification(status, tt.alertType)
			if title != tt.expectedTitle {
				t.Errorf("title = %s, want %s", title, tt.expectedTitle)
			}
		})
	}
}

func TestManager_formatNotification_MmolL(t *testing.T) {
	settings := models.DefaultSettings()
	settings.Unit = testMmolUnit
	manager := NewManager(settings)

	status := &models.GlucoseStatus{
		Value:     100,
		ValueMmol: 5.5,
		Trend:     "→",
	}

	_, message := manager.formatNotification(status, "low")
	if message == "" {
		t.Error("Expected non-empty message")
	}
	if !strings.Contains(message, "5.5") {
		t.Errorf("Message should contain mmol/L value, got: %s", message)
	}
}

func TestManager_ClearAlertState(t *testing.T) {
	settings := models.DefaultSettings()
	manager := NewManager(settings)

	manager.lastAlertTime["low"] = time.Now()
	manager.lastAlertTime["high"] = time.Now()

	manager.ClearAlertState("low")
	if _, ok := manager.lastAlertTime["low"]; ok {
		t.Error("low alert should be cleared")
	}
	if _, ok := manager.lastAlertTime["high"]; !ok {
		t.Error("high alert should still exist")
	}

	manager.lastAlertTime["low"] = time.Now()
	manager.ClearAlertState("")
	if len(manager.lastAlertTime) != 0 {
		t.Error("All alerts should be cleared")
	}
}

func TestManager_UpdateSettings(t *testing.T) {
	settings := models.DefaultSettings()
	manager := NewManager(settings)

	newSettings := models.DefaultSettings()
	newSettings.Unit = testMmolUnit

	manager.UpdateSettings(newSettings)

	if manager.settings.Unit != testMmolUnit {
		t.Error("Settings were not updated")
	}
}
