// Package app provides the main application logic
package app

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/mrcode/nightscout-tray/internal/autostart"
	"github.com/mrcode/nightscout-tray/internal/models"
	"github.com/mrcode/nightscout-tray/internal/nightscout"
	"github.com/mrcode/nightscout-tray/internal/notifications"
	"github.com/mrcode/nightscout-tray/internal/tray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct represents the main application
type App struct {
	ctx           context.Context
	settings      *models.Settings
	client        *nightscout.Client
	trayIcon      *tray.Icon
	notifyManager *notifications.Manager

	mu         sync.RWMutex
	lastStatus *models.GlucoseStatus
	ticker     *time.Ticker
	stopChan   chan struct{}
	isRunning  bool
}

// New creates a new App instance
func New() *App {
	settings := models.DefaultSettings()
	if err := settings.Load(); err != nil {
		// Log error but continue with defaults
		fmt.Printf("Error loading settings: %v\n", err)
	}

	app := &App{
		settings:      settings,
		notifyManager: notifications.NewManager(settings),
		stopChan:      make(chan struct{}),
	}

	return app
}

// Startup is called when the app starts
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	// Initialize Nightscout client if configured
	if a.settings.IsConfigured() {
		a.initClient()
		// Hydrate tray history
		go a.hydrateTrayHistory()
	}

	// Start the system tray in a goroutine
	go a.startTray()

	// Start the update loop
	go a.startUpdateLoop()
}

// hydrateTrayHistory fetches recent entries to populate the tray icon sparkline
func (a *App) hydrateTrayHistory() {
	a.mu.RLock()
	client := a.client
	a.mu.RUnlock()

	if client == nil {
		return
	}

	// Fetch last 24 entries (approx 2 hours)
	entries, err := client.GetRecentEntries(24)
	if err != nil {
		fmt.Printf("Error hydrating history: %v\n", err)
		return
	}

	// Sort old to new
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Date < entries[j].Date
	})

	// Wait for tray to be ready (hacky but simple)
	for a.trayIcon == nil {
		time.Sleep(100 * time.Millisecond)
	}

	// Feed entries to tray icon
	for i := range entries {
		// Create a status object for each entry
		status := a.createStatus(&entries[i])
		a.trayIcon.UpdateStatus(status)
	}
}

// initClient initializes the Nightscout client with current settings
func (a *App) initClient() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.client = nightscout.NewClient(
		a.settings.NightscoutURL,
		a.settings.APISecret,
		a.settings.APIToken,
		a.settings.UseToken,
	)
}

// startTray initializes and starts the system tray
func (a *App) startTray() {
	a.trayIcon = tray.NewIcon(
		a.settings,
		func() { a.ShowWindow() },
		func() { a.Quit() },
	)
	a.trayIcon.Run()
}

// startUpdateLoop starts the periodic glucose update loop
func (a *App) startUpdateLoop() {
	a.mu.Lock()
	if a.isRunning {
		a.mu.Unlock()
		return
	}
	a.isRunning = true

	interval := time.Duration(a.settings.RefreshInterval) * time.Second
	a.ticker = time.NewTicker(interval)
	a.mu.Unlock()

	// Initial fetch
	a.fetchAndUpdate()

	for {
		select {
		case <-a.ticker.C:
			a.fetchAndUpdate()
		case <-a.stopChan:
			a.ticker.Stop()
			return
		}
	}
}

// fetchAndUpdate fetches glucose data and updates the UI
func (a *App) fetchAndUpdate() {
	a.mu.RLock()
	client := a.client
	a.mu.RUnlock()

	if client == nil {
		return
	}

	// Fetch current entry
	entry, err := client.GetCurrentEntry()
	if err != nil {
		if a.trayIcon != nil {
			a.trayIcon.SetError(err)
		}
		runtime.EventsEmit(a.ctx, "glucose:error", err.Error())
		return
	}

	// Create status
	status := a.createStatus(entry)

	a.mu.Lock()
	a.lastStatus = status
	a.mu.Unlock()

	// Update tray
	if a.trayIcon != nil {
		a.trayIcon.UpdateStatus(status)
	}

	// Check for alerts
	if err := a.notifyManager.CheckAndNotify(status); err != nil {
		fmt.Printf("Notification error: %v\n", err)
	}

	// Emit event to frontend
	runtime.EventsEmit(a.ctx, "glucose:update", status)
}

// createStatus creates a GlucoseStatus from an entry
func (a *App) createStatus(entry *models.GlucoseEntry) *models.GlucoseStatus {
	a.mu.RLock()
	settings := a.settings
	a.mu.RUnlock()

	staleMinutes := int(time.Since(entry.Time()).Minutes())

	return &models.GlucoseStatus{
		Value:        entry.SGV,
		ValueMmol:    entry.ValueMmolL(),
		Trend:        entry.TrendArrow(),
		Direction:    entry.Direction,
		Time:         entry.Time(),
		Delta:        0, // TODO: Calculate from previous entry
		Status:       settings.GetGlucoseStatus(entry.SGV),
		StaleMinutes: staleMinutes,
		IsStale:      staleMinutes > 15,
	}
}

// Shutdown is called when the app is closing
func (a *App) Shutdown(_ context.Context) {
	a.mu.Lock()
	if a.ticker != nil {
		a.ticker.Stop()
	}
	close(a.stopChan)
	a.isRunning = false
	a.mu.Unlock()

	// Save settings
	if err := a.settings.Save(); err != nil {
		fmt.Printf("Error saving settings: %v\n", err)
	}
}

// BeforeClose is called before the window closes
func (a *App) BeforeClose(ctx context.Context) bool {
	// Hide window instead of closing
	runtime.WindowHide(ctx)
	return true // Prevent close
}

// ShowWindow shows the main window
func (a *App) ShowWindow() {
	if a.ctx != nil {
		runtime.WindowShow(a.ctx)
		runtime.WindowSetAlwaysOnTop(a.ctx, true)
		runtime.WindowSetAlwaysOnTop(a.ctx, false)
	}
}

// HideWindow hides the main window
func (a *App) HideWindow() {
	if a.ctx != nil {
		runtime.WindowHide(a.ctx)
	}
}

// Quit exits the application
func (a *App) Quit() {
	if a.trayIcon != nil {
		a.trayIcon.Quit()
	}
	if a.ctx != nil {
		runtime.Quit(a.ctx)
	}
}

// GetSettings returns the current settings
func (a *App) GetSettings() *models.Settings {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.settings.Clone()
}

// SaveSettings saves the provided settings
func (a *App) SaveSettings(settings *models.Settings) error {
	a.mu.Lock()
	a.settings.Update(settings)
	a.mu.Unlock()

	// Save to disk
	if err := a.settings.Save(); err != nil {
		return fmt.Errorf("saving settings: %w", err)
	}

	// Reinitialize client
	a.initClient()

	// Update notification manager
	a.notifyManager.UpdateSettings(a.settings)

	// Update tray icon settings
	if a.trayIcon != nil {
		a.trayIcon.UpdateSettings(a.settings)
	}

	// Restart update loop with new interval
	a.restartUpdateLoop()

	// Handle auto-start
	if settings.AutoStart {
		if err := autostart.Enable(); err != nil {
			fmt.Printf("Error enabling autostart: %v\n", err)
		}
	} else {
		if err := autostart.Disable(); err != nil {
			fmt.Printf("Error disabling autostart: %v\n", err)
		}
	}

	return nil
}

// restartUpdateLoop restarts the update loop with new settings
func (a *App) restartUpdateLoop() {
	a.mu.Lock()
	if a.ticker != nil {
		a.ticker.Stop()
	}
	interval := time.Duration(a.settings.RefreshInterval) * time.Second
	a.ticker = time.NewTicker(interval)
	a.mu.Unlock()
}

// TestConnection tests the Nightscout connection
func (a *App) TestConnection(url, secret, token string, useToken bool) error {
	client := nightscout.NewClient(url, secret, token, useToken)
	return client.TestConnection()
}

// GetCurrentStatus returns the current glucose status
func (a *App) GetCurrentStatus() *models.GlucoseStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastStatus
}

// GetChartData returns glucose data for the chart
func (a *App) GetChartData(hours int, offsetHours int) (*models.ChartData, error) {
	a.mu.RLock()
	client := a.client
	settings := a.settings
	a.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("not configured")
	}

	// Calculate time range with offset for scrolling back
	now := time.Now()
	to := now.Add(-time.Duration(offsetHours) * time.Hour)
	from := to.Add(-time.Duration(hours) * time.Hour)

	// Fetch enough entries (assume 5 min intervals -> 12/hr. 24h = 288. 2000 is safe)
	entries, err := client.GetEntries(from, to, 2000)
	if err != nil {
		return nil, err
	}

	// Sort by time ascending
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Date < entries[j].Date
	})

	// Convert to chart entries
	chartEntries := make([]models.ChartEntry, len(entries))
	useMmol := settings.Unit == "mmol/L"

	for i, entry := range entries {
		value := float64(entry.SGV)
		if useMmol {
			value = entry.ValueMmolL()
		}

		chartEntries[i] = models.ChartEntry{
			Time:    entry.Date,
			Value:   value,
			ValueMg: entry.SGV,
			Status:  settings.GetGlucoseStatus(entry.SGV),
		}
	}

	// Convert thresholds for display
	targetLow := settings.TargetLow
	targetHigh := settings.TargetHigh
	urgentLow := settings.UrgentLow
	urgentHigh := settings.UrgentHigh

	// Note: Thresholds are stored as mg/dL integers
	// The frontend handles mmol/L conversion for display if needed
	_ = useMmol // Kept for potential future use

	return &models.ChartData{
		Entries:    chartEntries,
		TargetLow:  targetLow,
		TargetHigh: targetHigh,
		UrgentLow:  urgentLow,
		UrgentHigh: urgentHigh,
		TimeRangeH: hours,
		Unit:       settings.Unit,
	}, nil
}

// SendTestNotification sends a test notification
func (a *App) SendTestNotification() error {
	return a.notifyManager.SendTestNotification()
}

// GetAutoStartEnabled returns whether auto-start is enabled
func (a *App) GetAutoStartEnabled() bool {
	enabled, _ := autostart.IsEnabled()
	return enabled
}

// ForceRefresh forces an immediate data refresh
func (a *App) ForceRefresh() {
	go a.fetchAndUpdate()
}

// IsConfigured returns true if the app is configured
func (a *App) IsConfigured() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.settings.IsConfigured()
}

// GetVersion returns the application version
func (a *App) GetVersion() string {
	return "1.0.0"
}
