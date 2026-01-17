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
	client      *nightscout.Client
	analyzer    *Analyzer
	predictor   *Predictor
	mlPredictor *MLPredictor
	orefEngine  *OrefEngine // New oref1-inspired prediction engine

	mu                sync.RWMutex
	params            *models.DiabetesParameters
	lastPrediction    *models.PredictionResult
	isCalculating     bool
	calculationCancel chan struct{}
	useMLPrediction   bool // Whether to use ML-based prediction

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
		mlPredictor:   NewMLPredictor(nil),
		orefEngine:    NewOrefEngine(nil), // Initialize oref engine
		params:        models.NewDiabetesParameters(),
		cacheDuration: 5 * time.Minute,
	}

	// Try to load saved parameters
	if err := s.loadParams(); err != nil {
		fmt.Printf("Could not load saved parameters: %v\n", err)
	} else {
		s.predictor.SetParameters(s.params)
		s.mlPredictor.SetParameters(s.params)
		s.orefEngine.SetParameters(s.params)
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
	fmt.Printf("StartCalculation called: days=%d, mode=%s\n", days, mode)
	
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
		s.calculationCancel = nil
		s.isCalculating = false
		s.analyzer.SetProgress("Cancelled", 0)
	}
}

// isCancelled checks if calculation was cancelled
func (s *Service) isCancelled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.calculationCancel == nil {
		return true
	}
	
	select {
	case <-s.calculationCancel:
		return true
	default:
		return false
	}
}

func (s *Service) runCalculation(days int, mode string) {
	defer func() {
		s.mu.Lock()
		s.isCalculating = false
		s.calculationCancel = nil
		s.mu.Unlock()
	}()

	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()

	if client == nil {
		s.analyzer.SetProgress("Error: No client configured", 0)
		return
	}

	// Stage 1: Fetch entries
	s.analyzer.SetProgress("Fetching glucose entries...", 5)
	if s.isCancelled() {
		return
	}
	
	entries, err := client.GetEntriesDays(days)
	if err != nil {
		fmt.Printf("Error fetching entries: %v\n", err)
		s.analyzer.SetProgress("Error fetching entries", 0)
		return
	}

	fmt.Printf("Fetched %d glucose entries for %d days\n", len(entries), days)
	s.analyzer.SetProgress("Fetching treatments...", 15)
	
	if s.isCancelled() {
		return
	}

	// Stage 2: Fetch treatments
	treatments, err := client.GetTreatmentsDays(days)
	if err != nil {
		fmt.Printf("Error fetching treatments: %v\n", err)
		s.analyzer.SetProgress("Error fetching treatments", 0)
		return
	}

	fmt.Printf("Fetched %d treatments for %d days\n", len(treatments), days)
	
	if s.isCancelled() {
		return
	}

	// Run analysis based on mode
	var params *models.DiabetesParameters
	if mode == "ml" {
		// Stage 3: ML-based analysis
		s.analyzer.SetProgress("Running ML analysis...", 25)
		params, err = s.analyzer.AnalyzeDataML(entries, treatments)
		
		if s.isCancelled() {
			return
		}

		// Stage 4: Train the oref engine with historical patterns
		if err == nil {
			s.analyzer.SetProgress("Training pattern recognition engine...", 50)
			fmt.Println("Training oref prediction engine with historical patterns...")
			s.orefEngine.LearnFromHistory(entries, treatments)

			mealPatterns, correctionPatterns := s.orefEngine.GetPatternStats()
			fmt.Printf("Oref engine learned %d meal patterns, %d correction patterns\n",
				mealPatterns, correctionPatterns)

			autosens := s.orefEngine.GetAutosensRatio()
			fmt.Printf("Current Autosens ratio: %.2f\n", autosens)

			// Also update time-of-day parameters from learned circadian profile
			profile := s.orefEngine.GetCircadianProfile()
			s.updateTimeOfDayParams(params, profile)
			
			if s.isCancelled() {
				return
			}

			// Stage 5: Train LSTM neural network
			s.analyzer.SetProgress("Training LSTM neural network...", 70)
			fmt.Println("Training LSTM neural network for glucose prediction...")
			err := s.mlPredictor.TrainLSTM(entries)
			if err != nil {
				fmt.Printf("LSTM training error (non-fatal): %v\n", err)
			} else {
				fmt.Println("LSTM neural network trained successfully!")
			}
			
			// Also train pattern library
			s.analyzer.SetProgress("Building pattern library...", 85)
			s.mlPredictor.LearnFromHistory(entries, treatments)
			fmt.Printf("ML predictor learned %d patterns\n", len(s.mlPredictor.patterns.patterns))
		}

		s.mu.Lock()
		s.useMLPrediction = true
		s.mu.Unlock()
	} else {
		// Statistical analysis (default, faster)
		s.analyzer.SetProgress("Running statistical analysis...", 30)
		params, err = s.analyzer.AnalyzeData(entries, treatments)

		s.mu.Lock()
		s.useMLPrediction = false
		s.mu.Unlock()
	}
	
	if s.isCancelled() {
		return
	}

	if err != nil {
		fmt.Printf("Error analyzing data: %v\n", err)
		s.analyzer.SetProgress("Error analyzing data", 0)
		return
	}

	// Stage 6: Update parameters
	s.analyzer.SetProgress("Saving parameters...", 95)
	s.mu.Lock()
	s.params = params
	s.predictor.SetParameters(params)
	s.mlPredictor.SetParameters(params)
	s.orefEngine.SetParameters(params)
	s.mu.Unlock()

	// Save parameters
	if err := s.saveParams(); err != nil {
		fmt.Printf("Error saving parameters: %v\n", err)
	}
	
	s.analyzer.SetProgress("Complete!", 100)
	fmt.Println("Parameter calculation complete!")
}

// updateTimeOfDayParams updates ISF/ICR by time of day from learned circadian profile
func (s *Service) updateTimeOfDayParams(params *models.DiabetesParameters, profile CircadianProfile) {
	// Map hours to time-of-day periods
	// Morning: 6-11 (hours 6,7,8,9,10)
	// Midday: 11-17 (hours 11,12,13,14,15,16)
	// Evening: 17-22 (hours 17,18,19,20,21)
	// Night: 22-6 (hours 22,23,0,1,2,3,4,5)

	morningISF := (profile.HourlySensitivity[6] + profile.HourlySensitivity[7] +
		profile.HourlySensitivity[8] + profile.HourlySensitivity[9] + profile.HourlySensitivity[10]) / 5
	middayISF := (profile.HourlySensitivity[11] + profile.HourlySensitivity[12] +
		profile.HourlySensitivity[13] + profile.HourlySensitivity[14] + profile.HourlySensitivity[15] + profile.HourlySensitivity[16]) / 6
	eveningISF := (profile.HourlySensitivity[17] + profile.HourlySensitivity[18] +
		profile.HourlySensitivity[19] + profile.HourlySensitivity[20] + profile.HourlySensitivity[21]) / 5
	nightISF := (profile.HourlySensitivity[22] + profile.HourlySensitivity[23] +
		profile.HourlySensitivity[0] + profile.HourlySensitivity[1] + profile.HourlySensitivity[2] +
		profile.HourlySensitivity[3] + profile.HourlySensitivity[4] + profile.HourlySensitivity[5]) / 8

	// Apply sensitivity factors to base ISF
	params.ISFByTimeOfDay[string(models.Morning)] = params.ISF * morningISF
	params.ISFByTimeOfDay[string(models.Midday)] = params.ISF * middayISF
	params.ISFByTimeOfDay[string(models.Evening)] = params.ISF * eveningISF
	params.ISFByTimeOfDay[string(models.Night)] = params.ISF * nightISF

	// Same for ICR (but inverse - higher factor means lower ICR needed)
	morningICR := (profile.HourlyICR[6] + profile.HourlyICR[7] +
		profile.HourlyICR[8] + profile.HourlyICR[9] + profile.HourlyICR[10]) / 5
	middayICR := (profile.HourlyICR[11] + profile.HourlyICR[12] +
		profile.HourlyICR[13] + profile.HourlyICR[14] + profile.HourlyICR[15] + profile.HourlyICR[16]) / 6
	eveningICR := (profile.HourlyICR[17] + profile.HourlyICR[18] +
		profile.HourlyICR[19] + profile.HourlyICR[20] + profile.HourlyICR[21]) / 5
	nightICR := (profile.HourlyICR[22] + profile.HourlyICR[23] +
		profile.HourlyICR[0] + profile.HourlyICR[1] + profile.HourlyICR[2] +
		profile.HourlyICR[3] + profile.HourlyICR[4] + profile.HourlyICR[5]) / 8

	// Apply ICR factors (lower factor = need more insulin = lower ICR value)
	params.ICRByTimeOfDay[string(models.Morning)] = params.ICR * morningICR
	params.ICRByTimeOfDay[string(models.Midday)] = params.ICR * middayICR
	params.ICRByTimeOfDay[string(models.Evening)] = params.ICR * eveningICR
	params.ICRByTimeOfDay[string(models.Night)] = params.ICR * nightICR

	fmt.Printf("Time-of-day ISF: Morning=%.1f, Midday=%.1f, Evening=%.1f, Night=%.1f\n",
		params.ISFByTimeOfDay[string(models.Morning)],
		params.ISFByTimeOfDay[string(models.Midday)],
		params.ISFByTimeOfDay[string(models.Evening)],
		params.ISFByTimeOfDay[string(models.Night)])
}

// GetPrediction generates a new prediction based on current data
func (s *Service) GetPrediction() (*models.PredictionResult, error) {
	s.mu.RLock()
	client := s.client
	useML := s.useMLPrediction
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

	// Generate prediction using appropriate method
	var prediction *models.PredictionResult
	if useML {
		// Use the new oref1-inspired prediction engine
		// This uses multiple prediction curves and picks the most conservative
		prediction = s.orefEngine.Predict(
			currentGlucose,
			entries,
			treatments,
			180, // highThreshold - will be overridden by chart settings
			70,  // lowThreshold
		)
		prediction.BasedOnTrend = currentTrend
	} else {
		// Use traditional predictor
		prediction = s.predictor.Predict(currentGlucose, currentTrend, entries, treatments)
	}

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
	//nolint:gosec // 0755 is standard permission for application directories
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

	return os.WriteFile(path, data, 0600)
}

func (s *Service) loadParams() error {
	path, err := s.getParamsPath()
	if err != nil {
		return err
	}

	//nolint:gosec // Reading from trusted app data directory
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
