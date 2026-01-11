// Package notifications handles system notifications and alerts
package notifications

import (
	"fmt"
	"sync"
	"time"

	"github.com/gen2brain/beeep"
	"github.com/mrcode/nightscout-tray/internal/models"
)

// Alert type constants
const (
	alertUrgentLow  = "urgent_low"
	alertLow        = "low"
	alertUrgentHigh = "urgent_high"
	alertHigh       = "high"
)

// Manager handles glucose alerts and notifications
type Manager struct {
	settings      *models.Settings
	lastAlertTime map[string]time.Time
	mu            sync.Mutex
}

// NewManager creates a new notification manager
func NewManager(settings *models.Settings) *Manager {
	return &Manager{
		settings:      settings,
		lastAlertTime: make(map[string]time.Time),
	}
}

// UpdateSettings updates the settings reference
func (m *Manager) UpdateSettings(settings *models.Settings) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.settings = settings
}

// CheckAndNotify checks glucose value and sends notification if needed
func (m *Manager) CheckAndNotify(status *models.GlucoseStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alertType := m.shouldAlert(status)
	if alertType == "" {
		return nil
	}

	// Check if we should repeat the alert
	if lastTime, ok := m.lastAlertTime[alertType]; ok {
		if m.settings.RepeatAlertMinutes > 0 {
			repeatDuration := time.Duration(m.settings.RepeatAlertMinutes) * time.Minute
			if time.Since(lastTime) < repeatDuration {
				return nil
			}
		} else {
			// No repeat, only alert once per status change
			return nil
		}
	}

	// Send notification
	title, message := m.formatNotification(status, alertType)
	err := m.sendNotification(title, message)
	if err != nil {
		return err
	}

	m.lastAlertTime[alertType] = time.Now()
	return nil
}

// shouldAlert determines if an alert should be sent
func (m *Manager) shouldAlert(status *models.GlucoseStatus) string {
	switch status.Status {
	case alertUrgentLow:
		if m.settings.EnableUrgentLowAlert {
			return alertUrgentLow
		}
	case alertLow:
		if m.settings.EnableLowAlert {
			return alertLow
		}
	case alertUrgentHigh:
		if m.settings.EnableUrgentHighAlert {
			return alertUrgentHigh
		}
	case alertHigh:
		if m.settings.EnableHighAlert {
			return alertHigh
		}
	}
	return ""
}

// formatNotification creates the notification title and message
func (m *Manager) formatNotification(status *models.GlucoseStatus, alertType string) (string, string) {
	var title, message string
	var valueStr string

	if m.settings.Unit == "mmol/L" {
		valueStr = fmt.Sprintf("%.1f mmol/L", status.ValueMmol)
	} else {
		valueStr = fmt.Sprintf("%d mg/dL", status.Value)
	}

	switch alertType {
	case alertUrgentLow:
		title = "⚠️ URGENT LOW GLUCOSE"
		message = fmt.Sprintf("Glucose is critically low: %s %s", valueStr, status.Trend)
	case alertLow:
		title = "⬇️ Low Glucose"
		message = fmt.Sprintf("Glucose is low: %s %s", valueStr, status.Trend)
	case alertUrgentHigh:
		title = "⚠️ URGENT HIGH GLUCOSE"
		message = fmt.Sprintf("Glucose is critically high: %s %s", valueStr, status.Trend)
	case alertHigh:
		title = "⬆️ High Glucose"
		message = fmt.Sprintf("Glucose is high: %s %s", valueStr, status.Trend)
	}

	return title, message
}

// sendNotification sends a system notification
func (m *Manager) sendNotification(title, message string) error {
	// Use beeep for cross-platform notifications
	return beeep.Notify(title, message, "")
}

// ClearAlertState clears the alert state for a specific type or all types
func (m *Manager) ClearAlertState(alertType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if alertType == "" {
		m.lastAlertTime = make(map[string]time.Time)
	} else {
		delete(m.lastAlertTime, alertType)
	}
}

// SendTestNotification sends a test notification
func (m *Manager) SendTestNotification() error {
	return beeep.Notify("Nightscout Tray", "Test notification - alerts are working!", "")
}
