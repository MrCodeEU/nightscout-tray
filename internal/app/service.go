package app

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/mrcode/nightscout-tray/internal/autostart"
	"github.com/mrcode/nightscout-tray/internal/models"
	"github.com/mrcode/nightscout-tray/internal/nightscout"
	"github.com/mrcode/nightscout-tray/internal/notifications"
	"github.com/mrcode/nightscout-tray/internal/prediction"
	"github.com/mrcode/nightscout-tray/internal/tray"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const unitMmolL = "mmol/L"

type NightscoutService struct {
	settings      *models.Settings
	client        *nightscout.Client
	notifyManager *notifications.Manager
	predService   *prediction.Service

	mu                sync.RWMutex
	lastStatus        *models.GlucoseStatus
	lastSuccessTime   time.Time
	consecutiveErrors int
	ticker            *time.Ticker
	stopChan          chan struct{}
	isRunning         bool
	
	app *application.App
	tray *application.SystemTray
	iconGen *tray.IconGenerator
}

func NewNightscoutService() *NightscoutService {
	settings := models.DefaultSettings()
	if err := settings.Load(); err != nil {
		fmt.Printf("Error loading settings: %v\n", err)
	}

	return &NightscoutService{
		settings:      settings,
		notifyManager: notifications.NewManager(settings),
		stopChan:      make(chan struct{}),
		iconGen:       tray.NewIconGenerator(),
		predService:   nil, // Initialized when client is ready
	}
}


func (s *NightscoutService) initClient() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.client = nightscout.NewClient(
		s.settings.NightscoutURL,
		s.settings.APISecret,
		s.settings.APIToken,
		s.settings.UseToken,
	)

	// Initialize prediction service with the new client
	if s.predService == nil {
		s.predService = prediction.NewService(s.client)
	} else {
		s.predService.SetClient(s.client)
	}
}

func (s *NightscoutService) startUpdateLoop() {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return
	}
	s.isRunning = true

	interval := time.Duration(s.settings.RefreshInterval) * time.Second
	s.ticker = time.NewTicker(interval)
	s.mu.Unlock()

	// Initial fetch
	s.fetchAndUpdate()

	for {
		select {
		case <-s.ticker.C:
			s.fetchAndUpdate()
		case <-s.stopChan:
			s.ticker.Stop()
			return
		}
	}
}

func (s *NightscoutService) fetchAndUpdate() {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()

	if client == nil {
		return
	}

	entry, err := client.GetCurrentEntry()
	if err != nil {
		s.mu.Lock()
		s.consecutiveErrors++
		errorCount := s.consecutiveErrors
		lastStatus := s.lastStatus
		lastSuccess := s.lastSuccessTime
		a := s.app
		s.mu.Unlock()

		fmt.Printf("Error fetching glucose data (attempt %d): %v\n", errorCount, err)

		if lastStatus != nil && !lastSuccess.IsZero() {
			timeSinceSuccess := time.Since(lastSuccess)
			lastStatus.StaleMinutes = int(timeSinceSuccess.Minutes())
			lastStatus.IsStale = lastStatus.StaleMinutes > 7

			s.updateTray(lastStatus)
			if a != nil {
				a.Event.Emit("glucose:update", lastStatus)
			}
		} else {
			s.updateTrayError(err)
		}

		if a != nil {
			a.Event.Emit("glucose:error", err.Error())
		}
		return
	}

	s.mu.Lock()
	s.consecutiveErrors = 0
	s.lastSuccessTime = time.Now()
	s.mu.Unlock()

	status := s.createStatus(entry)

	s.mu.Lock()
	s.lastStatus = status
	s.mu.Unlock()

	s.updateTray(status)

	if err := s.notifyManager.CheckAndNotify(status); err != nil {
		fmt.Printf("Notification error: %v\n", err)
	}

	if s.app != nil {
		s.app.Event.Emit("glucose:update", status)
	}
}

func (s *NightscoutService) createStatus(entry *models.GlucoseEntry) *models.GlucoseStatus {
	s.mu.RLock()
	settings := s.settings
	predSvc := s.predService
	s.mu.RUnlock()

	staleMinutes := int(time.Since(entry.Time()).Minutes())

	status := &models.GlucoseStatus{
		Value:        entry.SGV,
		ValueMmol:    entry.ValueMmolL(),
		Trend:        entry.TrendArrow(),
		Direction:    entry.Direction,
		Time:         entry.Time(),
		Delta:        0,
		Status:       settings.GetGlucoseStatus(entry.SGV),
		StaleMinutes: staleMinutes,
		IsStale:      staleMinutes > 15,
	}

	// Add IoB/CoB if prediction service is available
	if predSvc != nil {
		iob, cob, err := predSvc.GetIOBCOB()
		if err == nil {
			status.IOB = iob
			status.COB = cob
		}
	}

	return status
}

func (s *NightscoutService) SetTray(tray *application.SystemTray) {
	s.mu.Lock()
	s.tray = tray
	s.mu.Unlock()
}

func (s *NightscoutService) SetApp(app *application.App) {
	s.mu.Lock()
	s.app = app
	s.mu.Unlock()

	// Now that we have app reference, start background tasks
	if s.settings.IsConfigured() {
		s.initClient()
		go s.hydrateHistory()
	}
	go s.startUpdateLoop()
}

func (s *NightscoutService) updateTray(status *models.GlucoseStatus) {
	s.mu.RLock()
	t := s.tray
	s.mu.RUnlock()
	
	if t == nil {
		return
	}
	
	val := float64(status.Value)
	if s.settings.Unit == unitMmolL {
		val = status.ValueMmol
	}
	s.iconGen.AddHistory(val)

	valStr := fmt.Sprintf("%d", status.Value)
	if s.settings.Unit == unitMmolL {
		valStr = fmt.Sprintf("%.1f", status.ValueMmol)
	}
	
	t.SetLabel(valStr + " " + status.Trend)
	// No tooltip - we use the popup window instead

	iconData := s.iconGen.GenerateIcon(valStr, status.Direction, status)
	if iconData != nil {
		t.SetIcon(iconData)
	}
}

func (s *NightscoutService) updateTrayError(err error) {
	s.mu.RLock()
	t := s.tray
	s.mu.RUnlock()
	
	if t == nil {
		return
	}
	t.SetLabel("ERR")
	// No tooltip - we use the popup window instead
}

func (s *NightscoutService) hydrateHistory() {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()

	if client == nil {
		return
	}

	entries, err := client.GetRecentEntries(24)
	if err != nil {
		fmt.Printf("Error hydrating history: %v\n", err)
		return
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Date < entries[j].Date
	})

	s.mu.Lock()
	s.iconGen.ClearHistory()
	for i := range entries {
		val := float64(entries[i].SGV)
		if s.settings.Unit == unitMmolL {
			val = entries[i].ValueMmolL()
		}
		s.iconGen.AddHistory(val)
	}
	s.mu.Unlock()
	
	// Trigger a tray update if we have the latest status
	s.mu.RLock()
	lastStatus := s.lastStatus
	s.mu.RUnlock()
	if lastStatus != nil {
		s.updateTray(lastStatus)
	}
}

// Public methods for Binding

func (s *NightscoutService) GetSettings() *models.Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings.Clone()
}

func (s *NightscoutService) SaveSettings(settings *models.Settings) error {
	s.mu.Lock()
	s.settings.Update(settings)
	s.mu.Unlock()

	if err := s.settings.Save(); err != nil {
		return err
	}

	s.initClient()
	s.notifyManager.UpdateSettings(s.settings)
	s.restartUpdateLoop()

	if settings.AutoStart {
		_ = autostart.Enable()
	} else {
		_ = autostart.Disable()
	}

	return nil
}

func (s *NightscoutService) restartUpdateLoop() {
	s.mu.Lock()
	if s.ticker != nil {
		s.ticker.Stop()
	}
	interval := time.Duration(s.settings.RefreshInterval) * time.Second
	s.ticker = time.NewTicker(interval)
	s.mu.Unlock()
}

func (s *NightscoutService) GetCurrentStatus() *models.GlucoseStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastStatus
}

func (s *NightscoutService) GetChartData(hours int, offsetHours int) (*models.ChartData, error) {
	s.mu.RLock()
	client := s.client
	settings := s.settings
	s.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("not configured")
	}

	now := time.Now()
	to := now.Add(-time.Duration(offsetHours) * time.Hour)
	from := to.Add(-time.Duration(hours) * time.Hour)

	entries, err := client.GetEntries(from, to, 2000)
	if err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Date < entries[j].Date
	})

	chartEntries := make([]models.ChartEntry, len(entries))
	useMmol := settings.Unit == unitMmolL

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

	return &models.ChartData{
		Entries:    chartEntries,
		TargetLow:  settings.TargetLow,
		TargetHigh: settings.TargetHigh,
		UrgentLow:  settings.UrgentLow,
		UrgentHigh: settings.UrgentHigh,
		TimeRangeH: hours,
		Unit:       settings.Unit,
	}, nil
}

// Prediction-related methods

// GetPrediction returns glucose predictions based on current data
func (s *NightscoutService) GetPrediction() (*models.PredictionResult, error) {
	s.mu.RLock()
	predSvc := s.predService
	s.mu.RUnlock()

	if predSvc == nil {
		return nil, fmt.Errorf("prediction service not initialized")
	}

	return predSvc.GetPrediction()
}

// GetPredictionParameters returns the calculated diabetes parameters
func (s *NightscoutService) GetPredictionParameters() *models.DiabetesParameters {
	s.mu.RLock()
	predSvc := s.predService
	s.mu.RUnlock()

	if predSvc == nil {
		return models.NewDiabetesParameters()
	}

	return predSvc.GetParameters()
}

// StartParameterCalculation begins calculating diabetes parameters from historical data
// mode can be "statistical" or "ml"
func (s *NightscoutService) StartParameterCalculation(days int, mode string) error {
	s.mu.RLock()
	predSvc := s.predService
	s.mu.RUnlock()

	if predSvc == nil {
		return fmt.Errorf("prediction service not initialized")
	}

	return predSvc.StartCalculation(days, mode)
}

// GetCalculationProgress returns the progress of the current parameter calculation
func (s *NightscoutService) GetCalculationProgress() *models.CalculationProgress {
	s.mu.RLock()
	predSvc := s.predService
	s.mu.RUnlock()

	if predSvc == nil {
		return &models.CalculationProgress{}
	}

	return predSvc.GetCalculationProgress()
}

// IsCalculating returns true if parameter calculation is in progress
func (s *NightscoutService) IsCalculating() bool {
	s.mu.RLock()
	predSvc := s.predService
	s.mu.RUnlock()

	if predSvc == nil {
		return false
	}

	return predSvc.IsCalculating()
}

// CancelCalculation cancels an in-progress parameter calculation
func (s *NightscoutService) CancelCalculation() {
	s.mu.RLock()
	predSvc := s.predService
	s.mu.RUnlock()

	if predSvc != nil {
		predSvc.CancelCalculation()
	}
}

// GetPredictionWithScenario returns predictions with a hypothetical treatment
func (s *NightscoutService) GetPredictionWithScenario(additionalInsulin, additionalCarbs float64) (*models.PredictionResult, error) {
	s.mu.RLock()
	predSvc := s.predService
	s.mu.RUnlock()

	if predSvc == nil {
		return nil, fmt.Errorf("prediction service not initialized")
	}

	return predSvc.GetPredictionWithScenario(additionalInsulin, additionalCarbs)
}

// GetIOBCOB returns current Insulin on Board and Carbs on Board
func (s *NightscoutService) GetIOBCOB() (*IOBCOBResult, error) {
	s.mu.RLock()
	predSvc := s.predService
	s.mu.RUnlock()

	if predSvc == nil {
		return nil, fmt.Errorf("prediction service not initialized")
	}

	iob, cob, err := predSvc.GetIOBCOB()
	if err != nil {
		return nil, err
	}

	return &IOBCOBResult{
		IOB: iob,
		COB: cob,
	}, nil
}

// IOBCOBResult contains IOB and COB values
type IOBCOBResult struct {
	IOB float64 `json:"iob"`
	COB float64 `json:"cob"`
}

// GetChartPredictionData returns prediction data formatted for the chart
func (s *NightscoutService) GetChartPredictionData(showLongTerm bool) (*prediction.ChartPredictionData, error) {
	s.mu.RLock()
	predSvc := s.predService
	s.mu.RUnlock()

	if predSvc == nil {
		return nil, fmt.Errorf("prediction service not initialized")
	}

	return predSvc.GetChartPredictionData(showLongTerm)
}

// GetTreatments returns recent treatments
func (s *NightscoutService) GetTreatments(hours int) ([]models.Treatment, error) {
	s.mu.RLock()
	predSvc := s.predService
	s.mu.RUnlock()

	if predSvc == nil {
		return nil, fmt.Errorf("prediction service not initialized")
	}

	return predSvc.GetTreatments(hours)
}
