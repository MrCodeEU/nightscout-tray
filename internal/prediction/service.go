// Package prediction provides glucose prediction and diabetes parameter calculation
package prediction

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mrcode/nightscout-tray/internal/models"
	"github.com/mrcode/nightscout-tray/internal/nightscout"
)

// Service provides prediction functionality to the application
type Service struct {
	client    *nightscout.Client
	analyzer  *Analyzer
	predictor *Predictor

	mu                 sync.RWMutex
	params             *models.DiabetesParameters
	lastPrediction     *models.PredictionResult
	isCalculating      bool
	calculationCancel  chan struct{}

	// Cached data
	cachedEntries    []models.GlucoseEntry
	cachedTreatments []models.Treatment
	cacheTime        time.Time
	cacheDuration    time.Duration
}

// NewService creates a new prediction service
func NewService(client *nightscout.Client) *Service {
	s := &Service{
		client:        client,
		analyzer:      NewAnalyzer(),
		predictor:     NewPredictor(nil),
		params:        models.NewDiabetesParameters(),
		cacheDuration: 5 * time.Minute,
	}

	// Try to load saved parameters
	if err := s.loadParams(); err != nil {
		fmt.Printf("Could not load saved parameters: %v\n", err)
	} else {
		s.predictor.SetParameters(s.params)
	}

	return s
}

// SetClient updates the Nightscout client
func (s *Service) SetClient(client *nightscout.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.client = client
}

// GetParameters returns the current diabetes parameters
func (s *Service) GetParameters() *models.DiabetesParameters {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	p := *s.params
	return &p
}

// GetCalculationProgress returns the current calculation progress
func (s *Service) GetCalculationProgress() *models.CalculationProgress {
	return s.analyzer.GetProgress()
}

// IsCalculating returns true if a calculation is in progress
func (s *Service) IsCalculating() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isCalculating
}

// StartCalculation begins parameter calculation with the given timeframe
// mode can be "statistical" or "ml"
func (s *Service) StartCalculation(days int, mode string) error {
	s.mu.Lock()
	if s.isCalculating {
		s.mu.Unlock()
		return fmt.Errorf("calculation already in progress")
	}
	s.isCalculating = true
	s.calculationCancel = make(chan struct{})
	s.mu.Unlock()

	go s.runCalculation(days, mode)
	return nil
}

// CancelCalculation cancels an in-progress calculation
func (s *Service) CancelCalculation() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isCalculating && s.calculationCancel != nil {
		close(s.calculationCancel)
	}
}

func (s *Service) runCalculation(days int, mode string) {
	defer func() {
		s.mu.Lock()
		s.isCalculating = false
		s.mu.Unlock()
	}()

	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()

	if client == nil {
		return
	}

	// Fetch entries
	entries, err := client.GetEntriesDays(days)
	if err != nil {
		fmt.Printf("Error fetching entries: %v\n", err)
		return
	}

	// Fetch treatments
	treatments, err := client.GetTreatmentsDays(days)
	if err != nil {
		fmt.Printf("Error fetching treatments: %v\n", err)
		return
	}

	// Run analysis based on mode
	var params *models.DiabetesParameters
	if mode == "ml" {
		// ML-based analysis (more computationally expensive)
		params, err = s.analyzer.AnalyzeDataML(entries, treatments)
	} else {
		// Statistical analysis (default, faster)
		params, err = s.analyzer.AnalyzeData(entries, treatments)
	}
	
	if err != nil {
		fmt.Printf("Error analyzing data: %v\n", err)
		return
	}

	// Update parameters
	s.mu.Lock()
	s.params = params
	s.predictor.SetParameters(params)
	s.mu.Unlock()

	// Save parameters
	if err := s.saveParams(); err != nil {
		fmt.Printf("Error saving parameters: %v\n", err)
	}
}

// GetPrediction generates a new prediction based on current data
func (s *Service) GetPrediction() (*models.PredictionResult, error) {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("no client configured")
	}

	// Get recent data (use cache if fresh)
	entries, treatments, err := s.getRecentData()
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("no glucose data available")
	}

	// Get current glucose and trend
	currentGlucose := float64(entries[0].SGV)
	currentTrend := CalculateTrend(entries)

	// Generate prediction
	prediction := s.predictor.Predict(currentGlucose, currentTrend, entries, treatments)

	s.mu.Lock()
	s.lastPrediction = prediction
	s.mu.Unlock()

	return prediction, nil
}

// GetLastPrediction returns the most recent prediction without generating a new one
func (s *Service) GetLastPrediction() *models.PredictionResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastPrediction
}

// GetPredictionWithScenario generates a prediction with hypothetical treatment
func (s *Service) GetPredictionWithScenario(additionalInsulin, additionalCarbs float64) (*models.PredictionResult, error) {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("no client configured")
	}

	entries, treatments, err := s.getRecentData()
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("no glucose data available")
	}

	currentGlucose := float64(entries[0].SGV)
	currentTrend := CalculateTrend(entries)

	return s.predictor.PredictWithScenario(
		currentGlucose,
		currentTrend,
		entries,
		treatments,
		additionalInsulin,
		additionalCarbs,
	), nil
}

// GetIOBCOB returns current IOB and COB values
func (s *Service) GetIOBCOB() (iob, cob float64, err error) {
	entries, treatments, err := s.getRecentData()
	if err != nil {
		return 0, 0, err
	}

	if len(entries) == 0 {
		return 0, 0, nil
	}

	currentGlucose := float64(entries[0].SGV)
	currentTrend := CalculateTrend(entries)

	prediction := s.predictor.Predict(currentGlucose, currentTrend, entries, treatments)
	return prediction.IOB, prediction.COB, nil
}

func (s *Service) getRecentData() ([]models.GlucoseEntry, []models.Treatment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Use cache if still fresh
	if time.Since(s.cacheTime) < s.cacheDuration && len(s.cachedEntries) > 0 {
		return s.cachedEntries, s.cachedTreatments, nil
	}

	if s.client == nil {
		return nil, nil, fmt.Errorf("no client configured")
	}

	// Fetch recent entries (6 hours for predictions + DIA)
	entries, err := s.client.GetEntriesHours(8)
	if err != nil {
		return nil, nil, fmt.Errorf("fetching entries: %w", err)
	}

	// Fetch recent treatments
	treatments, err := s.client.GetTreatmentsHours(8)
	if err != nil {
		return nil, nil, fmt.Errorf("fetching treatments: %w", err)
	}

	// Update cache
	s.cachedEntries = entries
	s.cachedTreatments = treatments
	s.cacheTime = time.Now()

	return entries, treatments, nil
}

func (s *Service) getParamsPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	appDir := filepath.Join(configDir, "nightscout-tray")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", err
	}

	return filepath.Join(appDir, "prediction-params.json"), nil
}

func (s *Service) saveParams() error {
	path, err := s.getParamsPath()
	if err != nil {
		return err
	}

	s.mu.RLock()
	data, err := json.MarshalIndent(s.params, "", "  ")
	s.mu.RUnlock()

	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (s *Service) loadParams() error {
	path, err := s.getParamsPath()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	params := models.NewDiabetesParameters()
	if err := json.Unmarshal(data, params); err != nil {
		return err
	}

	s.mu.Lock()
	s.params = params
	s.mu.Unlock()

	return nil
}

// RefreshCache forces a cache refresh
func (s *Service) RefreshCache() error {
	s.mu.Lock()
	s.cacheTime = time.Time{} // Invalidate cache
	s.mu.Unlock()

	_, _, err := s.getRecentData()
	return err
}

// GetTreatments returns recent treatments for display
func (s *Service) GetTreatments(hours int) ([]models.Treatment, error) {
	if s.client == nil {
		return nil, fmt.Errorf("no client configured")
	}
	return s.client.GetTreatmentsHours(hours)
}

// GetChartPredictionData returns prediction data formatted for the chart
func (s *Service) GetChartPredictionData(showLongTerm bool) (*ChartPredictionData, error) {
	prediction, err := s.GetPrediction()
	if err != nil {
		return nil, err
	}

	data := &ChartPredictionData{
		ShortTerm:     make([]ChartPredictionPoint, len(prediction.ShortTerm)),
		IOB:           prediction.IOB,
		COB:           prediction.COB,
		HighInMinutes: prediction.HighInMinutes,
		LowInMinutes:  prediction.LowInMinutes,
	}

	for i, p := range prediction.ShortTerm {
		data.ShortTerm[i] = ChartPredictionPoint{
			Time:       p.Time,
			Value:      p.Value,
			Confidence: p.Confidence,
		}
	}

	if showLongTerm {
		data.LongTerm = make([]ChartPredictionPoint, len(prediction.LongTerm))
		for i, p := range prediction.LongTerm {
			data.LongTerm[i] = ChartPredictionPoint{
				Time:       p.Time,
				Value:      p.Value,
				Confidence: p.Confidence,
			}
		}
	}

	return data, nil
}

// ChartPredictionData contains prediction data for chart display
type ChartPredictionData struct {
	ShortTerm     []ChartPredictionPoint `json:"shortTerm"`
	LongTerm      []ChartPredictionPoint `json:"longTerm,omitempty"`
	IOB           float64                `json:"iob"`
	COB           float64                `json:"cob"`
	HighInMinutes float64                `json:"highInMinutes"`
	LowInMinutes  float64                `json:"lowInMinutes"`
}

// ChartPredictionPoint represents a prediction point for the chart
type ChartPredictionPoint struct {
	Time       int64   `json:"time"`
	Value      float64 `json:"value"`
	Confidence float64 `json:"confidence"`
}
