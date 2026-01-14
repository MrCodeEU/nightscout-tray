// Package prediction provides glucose prediction and diabetes parameter calculation
package prediction

import (
	"math"
	"sort"
	"sync"
	"time"

	"github.com/mrcode/nightscout-tray/internal/models"
)

// Analyzer calculates diabetes parameters from historical data
type Analyzer struct {
	mu       sync.RWMutex
	progress *models.CalculationProgress
	params   *models.DiabetesParameters
}

// NewAnalyzer creates a new Analyzer instance
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		progress: &models.CalculationProgress{},
		params:   models.NewDiabetesParameters(),
	}
}

// GetProgress returns the current calculation progress
func (a *Analyzer) GetProgress() *models.CalculationProgress {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	// Return a copy
	p := *a.progress
	return &p
}

// GetParameters returns the calculated parameters
func (a *Analyzer) GetParameters() *models.DiabetesParameters {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	// Return a copy
	p := *a.params
	return &p
}

// AnalyzeData performs full analysis on historical data
func (a *Analyzer) AnalyzeData(entries []models.GlucoseEntry, treatments []models.Treatment) (*models.DiabetesParameters, error) {
	a.mu.Lock()
	a.progress = &models.CalculationProgress{
		Stage:           "Initializing",
		Progress:        0,
		TotalEntries:    len(entries),
		TotalTreatments: len(treatments),
		StartedAt:       time.Now(),
	}
	a.mu.Unlock()

	params := models.NewDiabetesParameters()

	// Sort data by time
	sortedEntries := make([]models.GlucoseEntry, len(entries))
	copy(sortedEntries, entries)
	sort.Slice(sortedEntries, func(i, j int) bool {
		return sortedEntries[i].Date < sortedEntries[j].Date
	})

	sortedTreatments := make([]models.Treatment, len(treatments))
	copy(sortedTreatments, treatments)
	sort.Slice(sortedTreatments, func(i, j int) bool {
		return sortedTreatments[i].Time().Before(sortedTreatments[j].Time())
	})

	// Stage 1: Calculate glucose statistics
	a.updateProgress("Calculating glucose statistics", 10)
	a.calculateGlucoseStats(sortedEntries, params)

	// Stage 2: Calculate daily averages
	a.updateProgress("Calculating daily averages", 25)
	a.calculateDailyAverages(sortedTreatments, params)

	// Stage 3: Calculate ISF (Insulin Sensitivity Factor)
	a.updateProgress("Calculating insulin sensitivity", 40)
	a.calculateISF(sortedEntries, sortedTreatments, params)

	// Stage 4: Calculate ICR (Insulin-to-Carb Ratio)
	a.updateProgress("Calculating insulin-to-carb ratio", 60)
	a.calculateICR(sortedEntries, sortedTreatments, params)

	// Stage 5: Estimate DIA (Duration of Insulin Action)
	a.updateProgress("Estimating insulin duration", 75)
	a.calculateDIA(sortedEntries, sortedTreatments, params)

	// Stage 6: Calculate carb absorption rate
	a.updateProgress("Calculating carb absorption rate", 85)
	a.calculateCarbAbsorption(sortedEntries, sortedTreatments, params)

	// Stage 7: Calculate time-of-day variations
	a.updateProgress("Calculating time-of-day variations", 95)
	a.calculateTimeOfDayVariations(sortedEntries, sortedTreatments, params)

	// Finalize
	params.EntriesAnalyzed = len(entries)
	params.TreatmentsAnalyzed = len(treatments)
	params.CalculatedAt = time.Now()

	if len(entries) > 0 {
		firstEntry := sortedEntries[0].Time()
		lastEntry := sortedEntries[len(sortedEntries)-1].Time()
		params.DataDays = int(lastEntry.Sub(firstEntry).Hours() / 24)
	}

	a.updateProgress("Complete", 100)

	a.mu.Lock()
	a.params = params
	a.mu.Unlock()

	return params, nil
}

func (a *Analyzer) updateProgress(stage string, progress float64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.progress.Stage = stage
	a.progress.Progress = progress

	elapsed := time.Since(a.progress.StartedAt).Seconds()
	if progress > 0 {
		a.progress.EstimatedTimeRemaining = (elapsed / progress) * (100 - progress)
	}
}

// calculateGlucoseStats calculates basic glucose statistics
func (a *Analyzer) calculateGlucoseStats(entries []models.GlucoseEntry, params *models.DiabetesParameters) {
	if len(entries) == 0 {
		return
	}

	var sum float64
	var inRange, belowRange, aboveRange int

	for _, e := range entries {
		sum += float64(e.SGV)

		switch {
		case e.SGV < 70:
			belowRange++
		case e.SGV > 180:
			aboveRange++
		default:
			inRange++
		}

		a.mu.Lock()
		a.progress.EntriesProcessed++
		a.mu.Unlock()
	}

	n := float64(len(entries))
	params.AverageGlucose = sum / n

	// Calculate standard deviation
	var sumSq float64
	for _, e := range entries {
		diff := float64(e.SGV) - params.AverageGlucose
		sumSq += diff * diff
	}
	params.GlucoseStdDev = math.Sqrt(sumSq / n)

	// Calculate percentages
	params.TimeInRange = float64(inRange) / n * 100
	params.TimeBelowRange = float64(belowRange) / n * 100
	params.TimeAboveRange = float64(aboveRange) / n * 100

	// Calculate GMI (Glucose Management Indicator) - estimated A1C
	// Formula: GMI = 3.31 + 0.02392 × mean glucose (mg/dL)
	params.GMI = 3.31 + 0.02392*params.AverageGlucose

	// Coefficient of Variation
	if params.AverageGlucose > 0 {
		params.CoefficientOfVariation = (params.GlucoseStdDev / params.AverageGlucose) * 100
	}
}

// calculateDailyAverages calculates average daily insulin and carb intake
func (a *Analyzer) calculateDailyAverages(treatments []models.Treatment, params *models.DiabetesParameters) {
	if len(treatments) == 0 {
		return
	}

	// Group by day
	dailyInsulin := make(map[string]float64)
	dailyCarbs := make(map[string]float64)
	dailyBolus := make(map[string]float64)

	for _, t := range treatments {
		day := t.Time().Format("2006-01-02")

		if t.HasInsulin() {
			dailyInsulin[day] += t.Insulin
			if t.IsBolus() {
				dailyBolus[day] += t.Insulin
			}
		}

		if t.HasCarbs() {
			dailyCarbs[day] += t.Carbs
		}

		a.mu.Lock()
		a.progress.TreatmentsProcessed++
		a.mu.Unlock()
	}

	// Calculate averages
	if len(dailyInsulin) > 0 {
		var totalInsulin, totalBolus float64
		for _, v := range dailyInsulin {
			totalInsulin += v
		}
		for _, v := range dailyBolus {
			totalBolus += v
		}
		params.TotalDailyInsulin = totalInsulin / float64(len(dailyInsulin))
		params.BolusInsulin = totalBolus / float64(len(dailyBolus))
		params.BasalInsulin = params.TotalDailyInsulin - params.BolusInsulin
	}

	if len(dailyCarbs) > 0 {
		var totalCarbs float64
		for _, v := range dailyCarbs {
			totalCarbs += v
		}
		params.TotalDailyCarbs = totalCarbs / float64(len(dailyCarbs))
	}
}

// calculateISF calculates Insulin Sensitivity Factor
func (a *Analyzer) calculateISF(entries []models.GlucoseEntry, treatments []models.Treatment, params *models.DiabetesParameters) {
	// Find correction boluses (insulin without carbs) and measure BG drop
	correctionEvents := a.findCorrectionEvents(entries, treatments)

	if len(correctionEvents) == 0 {
		// Use the 1800 rule as fallback: ISF = 1800 / TDD
		if params.TotalDailyInsulin > 0 {
			params.ISF = 1800 / params.TotalDailyInsulin
		}
		params.ISFConfidence = 30
		return
	}

	var isfValues []float64
	for _, event := range correctionEvents {
		if event.InsulinUnits > 0 && event.BGDrop != 0 {
			isf := math.Abs(event.BGDrop) / event.InsulinUnits
			// Filter out unrealistic values (ISF typically 10-100 mg/dL per unit)
			if isf >= 10 && isf <= 150 {
				isfValues = append(isfValues, isf)
			}
		}
	}

	if len(isfValues) > 0 {
		params.ISF = median(isfValues)
		params.ISFConfidence = math.Min(100, float64(len(isfValues))*5)
	} else if params.TotalDailyInsulin > 0 {
		params.ISF = 1800 / params.TotalDailyInsulin
		params.ISFConfidence = 30
	}
}

// calculateICR calculates Insulin-to-Carb Ratio
func (a *Analyzer) calculateICR(entries []models.GlucoseEntry, treatments []models.Treatment, params *models.DiabetesParameters) {
	// Find meal boluses and analyze BG response
	mealEvents := a.findMealEvents(entries, treatments)

	if len(mealEvents) == 0 {
		// Use the 500 rule as fallback: ICR = 500 / TDD
		if params.TotalDailyInsulin > 0 {
			params.ICR = 500 / params.TotalDailyInsulin
		}
		params.ICRConfidence = 30
		return
	}

	var icrValues []float64
	for _, event := range mealEvents {
		if event.InsulinUnits > 0 && event.Carbs > 0 {
			// Check if BG returned to near baseline
			bgChange := math.Abs(event.BGAfter - event.BGBefore)
			if bgChange < 50 { // Reasonable BG outcome
				icr := event.Carbs / event.InsulinUnits
				// Filter out unrealistic values (ICR typically 5-25)
				if icr >= 3 && icr <= 40 {
					icrValues = append(icrValues, icr)
				}
			}
		}
	}

	if len(icrValues) > 0 {
		params.ICR = median(icrValues)
		params.ICRConfidence = math.Min(100, float64(len(icrValues))*5)
	} else if params.TotalDailyInsulin > 0 {
		params.ICR = 500 / params.TotalDailyInsulin
		params.ICRConfidence = 30
	}
}

// calculateDIA estimates Duration of Insulin Action
func (a *Analyzer) calculateDIA(entries []models.GlucoseEntry, treatments []models.Treatment, params *models.DiabetesParameters) {
	// Analyze correction boluses to see how long until BG stabilizes
	correctionEvents := a.findCorrectionEvents(entries, treatments)

	if len(correctionEvents) < 5 {
		// Default DIA
		params.DIA = 4.0
		params.DIAConfidence = 20
		return
	}

	var diaValues []float64
	for _, event := range correctionEvents {
		if event.TimeToStable > 0 {
			diaHours := event.TimeToStable / 60.0 // Convert minutes to hours
			// Filter realistic values (2-6 hours)
			if diaHours >= 2 && diaHours <= 6 {
				diaValues = append(diaValues, diaHours)
			}
		}
	}

	if len(diaValues) > 0 {
		params.DIA = median(diaValues)
		params.DIAConfidence = math.Min(100, float64(len(diaValues))*10)
	} else {
		params.DIA = 4.0
		params.DIAConfidence = 20
	}
}

// calculateCarbAbsorption estimates carb absorption rate
func (a *Analyzer) calculateCarbAbsorption(entries []models.GlucoseEntry, treatments []models.Treatment, params *models.DiabetesParameters) {
	mealEvents := a.findMealEvents(entries, treatments)

	if len(mealEvents) < 3 {
		params.CarbAbsorptionRate = 30 // Default 30g/hour
		return
	}

	var absorptionRates []float64
	for _, event := range mealEvents {
		if event.TimeToPeak > 0 && event.Carbs > 0 {
			// Estimate how long to absorb carbs based on peak time
			// Assume ~60% of carbs absorbed by peak
			absorptionTime := event.TimeToPeak / 60.0 * 1.67 // hours
			if absorptionTime > 0 {
				rate := event.Carbs / absorptionTime
				if rate >= 10 && rate <= 100 {
					absorptionRates = append(absorptionRates, rate)
				}
			}
		}
	}

	if len(absorptionRates) > 0 {
		params.CarbAbsorptionRate = median(absorptionRates)
	}
}

// calculateTimeOfDayVariations calculates ISF and ICR variations by time of day
func (a *Analyzer) calculateTimeOfDayVariations(entries []models.GlucoseEntry, treatments []models.Treatment, params *models.DiabetesParameters) {
	// Initialize maps
	params.ISFByTimeOfDay = map[string]float64{
		string(models.Morning): params.ISF,
		string(models.Midday):  params.ISF,
		string(models.Evening): params.ISF,
		string(models.Night):   params.ISF,
	}

	params.ICRByTimeOfDay = map[string]float64{
		string(models.Morning): params.ICR,
		string(models.Midday):  params.ICR,
		string(models.Evening): params.ICR,
		string(models.Night):   params.ICR,
	}

	// Group correction events by time of day
	correctionEvents := a.findCorrectionEvents(entries, treatments)
	isfByPeriod := make(map[models.TimeOfDayPeriod][]float64)

	for _, event := range correctionEvents {
		period := models.GetTimeOfDayPeriod(event.Time)
		if event.InsulinUnits > 0 && event.BGDrop != 0 {
			isf := math.Abs(event.BGDrop) / event.InsulinUnits
			if isf >= 10 && isf <= 150 {
				isfByPeriod[period] = append(isfByPeriod[period], isf)
			}
		}
	}

	// Calculate period-specific ISF
	for period, values := range isfByPeriod {
		if len(values) >= 3 {
			params.ISFByTimeOfDay[string(period)] = median(values)
		}
	}

	// Group meal events by time of day
	mealEvents := a.findMealEvents(entries, treatments)
	icrByPeriod := make(map[models.TimeOfDayPeriod][]float64)

	for _, event := range mealEvents {
		period := models.GetTimeOfDayPeriod(event.Time)
		if event.InsulinUnits > 0 && event.Carbs > 0 {
			bgChange := math.Abs(event.BGAfter - event.BGBefore)
			if bgChange < 50 {
				icr := event.Carbs / event.InsulinUnits
				if icr >= 3 && icr <= 40 {
					icrByPeriod[period] = append(icrByPeriod[period], icr)
				}
			}
		}
	}

	// Calculate period-specific ICR
	for period, values := range icrByPeriod {
		if len(values) >= 3 {
			params.ICRByTimeOfDay[string(period)] = median(values)
		}
	}

	// Estimate basal rates by period
	params.BasalRateByTimeOfDay = map[string]float64{
		string(models.Morning): params.BasalInsulin / 24,
		string(models.Midday):  params.BasalInsulin / 24,
		string(models.Evening): params.BasalInsulin / 24,
		string(models.Night):   params.BasalInsulin / 24,
	}
}

// Helper types and functions

type correctionEvent struct {
	Time         time.Time
	InsulinUnits float64
	BGBefore     float64
	BGDrop       float64
	TimeToStable float64 // minutes
}

type mealEvent struct {
	Time         time.Time
	InsulinUnits float64
	Carbs        float64
	BGBefore     float64
	BGAfter      float64
	BGPeak       float64
	TimeToPeak   float64 // minutes
}

func (a *Analyzer) findCorrectionEvents(entries []models.GlucoseEntry, treatments []models.Treatment) []correctionEvent {
	var events []correctionEvent

	// Build a map for quick glucose lookups
	glucoseByTime := make(map[int64]float64)
	for _, e := range entries {
		glucoseByTime[e.Date] = float64(e.SGV)
	}

	for _, t := range treatments {
		// Look for correction boluses (insulin without carbs)
		if t.HasInsulin() && !t.HasCarbs() && t.IsBolus() {
			treatTime := t.Time()

			// Find BG before (within 30 minutes before)
			bgBefore := findNearestGlucose(entries, treatTime.Add(-15*time.Minute), 30*time.Minute)
			if bgBefore == 0 {
				continue
			}

			// Find BG after (2-4 hours later)
			bgAfter := findNearestGlucose(entries, treatTime.Add(3*time.Hour), 60*time.Minute)
			if bgAfter == 0 {
				continue
			}

			// Check if there were other treatments in the window that would affect results
			hasOtherTreatments := false
			for _, other := range treatments {
				otherTime := other.Time()
				if otherTime.After(treatTime) && otherTime.Before(treatTime.Add(3*time.Hour)) {
					if other.ID != t.ID && (other.HasInsulin() || other.HasCarbs()) {
						hasOtherTreatments = true
						break
					}
				}
			}

			if hasOtherTreatments {
				continue
			}

			// Calculate time to stable (when BG stops dropping significantly)
			timeToStable := findTimeToStable(entries, treatTime, 4*time.Hour)

			events = append(events, correctionEvent{
				Time:         treatTime,
				InsulinUnits: t.Insulin,
				BGBefore:     bgBefore,
				BGDrop:       bgBefore - bgAfter,
				TimeToStable: timeToStable,
			})
		}
	}

	return events
}

func (a *Analyzer) findMealEvents(entries []models.GlucoseEntry, treatments []models.Treatment) []mealEvent {
	var events []mealEvent

	for _, t := range treatments {
		// Look for meal boluses (insulin with carbs)
		if t.HasInsulin() && t.HasCarbs() {
			treatTime := t.Time()

			// Find BG before
			bgBefore := findNearestGlucose(entries, treatTime.Add(-15*time.Minute), 30*time.Minute)
			if bgBefore == 0 {
				continue
			}

			// Find BG after (3-4 hours later)
			bgAfter := findNearestGlucose(entries, treatTime.Add(3*time.Hour), 60*time.Minute)
			if bgAfter == 0 {
				continue
			}

			// Find peak BG and time to peak (within 2 hours)
			bgPeak, timeToPeak := findPeakGlucose(entries, treatTime, 2*time.Hour)

			events = append(events, mealEvent{
				Time:         treatTime,
				InsulinUnits: t.Insulin,
				Carbs:        t.Carbs,
				BGBefore:     bgBefore,
				BGAfter:      bgAfter,
				BGPeak:       bgPeak,
				TimeToPeak:   timeToPeak,
			})
		}
	}

	return events
}

func findNearestGlucose(entries []models.GlucoseEntry, targetTime time.Time, maxDiff time.Duration) float64 {
	var nearest float64
	minDiff := maxDiff

	for _, e := range entries {
		diff := e.Time().Sub(targetTime)
		if diff < 0 {
			diff = -diff
		}
		if diff < minDiff {
			minDiff = diff
			nearest = float64(e.SGV)
		}
	}

	return nearest
}

func findPeakGlucose(entries []models.GlucoseEntry, startTime time.Time, window time.Duration) (peak float64, timeToPeak float64) {
	endTime := startTime.Add(window)

	for _, e := range entries {
		t := e.Time()
		if t.After(startTime) && t.Before(endTime) {
			if float64(e.SGV) > peak {
				peak = float64(e.SGV)
				timeToPeak = t.Sub(startTime).Minutes()
			}
		}
	}

	return peak, timeToPeak
}

func findTimeToStable(entries []models.GlucoseEntry, startTime time.Time, maxWindow time.Duration) float64 {
	endTime := startTime.Add(maxWindow)

	var prevBG float64
	var stableStart time.Time
	stableThreshold := 10.0 // mg/dL change threshold to consider stable

	for _, e := range entries {
		t := e.Time()
		if t.After(startTime) && t.Before(endTime) {
			if prevBG > 0 {
				change := math.Abs(float64(e.SGV) - prevBG)
				if change < stableThreshold {
					if stableStart.IsZero() {
						stableStart = t
					} else if t.Sub(stableStart).Minutes() >= 30 {
						// Stable for 30 minutes, consider this the stable point
						return stableStart.Sub(startTime).Minutes()
					}
				} else {
					stableStart = time.Time{}
				}
			}
			prevBG = float64(e.SGV)
		}
	}

	return maxWindow.Minutes()
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

// AnalyzeDataML performs ML-based analysis on historical data
// This uses more sophisticated pattern recognition algorithms
func (a *Analyzer) AnalyzeDataML(entries []models.GlucoseEntry, treatments []models.Treatment) (*models.DiabetesParameters, error) {
	a.mu.Lock()
	a.progress = &models.CalculationProgress{
		Stage:           "Initializing ML Analysis",
		Progress:        0,
		TotalEntries:    len(entries),
		TotalTreatments: len(treatments),
		StartedAt:       time.Now(),
	}
	a.mu.Unlock()

	// Start with statistical analysis as base
	params := models.NewDiabetesParameters()

	// Sort data by time
	sortedEntries := make([]models.GlucoseEntry, len(entries))
	copy(sortedEntries, entries)
	sort.Slice(sortedEntries, func(i, j int) bool {
		return sortedEntries[i].Date < sortedEntries[j].Date
	})

	sortedTreatments := make([]models.Treatment, len(treatments))
	copy(sortedTreatments, treatments)
	sort.Slice(sortedTreatments, func(i, j int) bool {
		return sortedTreatments[i].Time().Before(sortedTreatments[j].Time())
	})

	// Stage 1: Calculate glucose statistics
	a.updateProgress("ML: Calculating glucose statistics", 5)
	a.calculateGlucoseStats(sortedEntries, params)

	// Stage 2: Build feature vectors for pattern recognition
	a.updateProgress("ML: Building feature vectors", 15)
	featureVectors := a.buildFeatureVectors(sortedEntries, sortedTreatments)

	// Stage 3: Pattern clustering for ISF
	a.updateProgress("ML: Analyzing insulin sensitivity patterns", 30)
	a.calculateISFML(featureVectors, params)

	// Stage 4: Pattern clustering for ICR
	a.updateProgress("ML: Analyzing carb response patterns", 50)
	a.calculateICRML(featureVectors, params)

	// Stage 5: Time series analysis for DIA
	a.updateProgress("ML: Estimating insulin duration", 65)
	a.calculateDIAML(sortedEntries, sortedTreatments, params)

	// Stage 6: Calculate carb absorption with curve fitting
	a.updateProgress("ML: Fitting carb absorption curves", 80)
	a.calculateCarbAbsorptionML(sortedEntries, sortedTreatments, params)

	// Stage 7: Calculate time-of-day variations with clustering
	a.updateProgress("ML: Detecting time-of-day patterns", 90)
	a.calculateTimeOfDayVariationsML(featureVectors, params)

	// Stage 8: Daily averages
	a.updateProgress("ML: Calculating daily averages", 95)
	a.calculateDailyAverages(sortedTreatments, params)

	// Finalize
	params.EntriesAnalyzed = len(entries)
	params.TreatmentsAnalyzed = len(treatments)
	params.CalculatedAt = time.Now()

	if len(entries) > 0 {
		firstEntry := sortedEntries[0].Time()
		lastEntry := sortedEntries[len(sortedEntries)-1].Time()
		params.DataDays = int(lastEntry.Sub(firstEntry).Hours() / 24)
	}

	a.updateProgress("Complete", 100)

	a.mu.Lock()
	a.params = params
	a.mu.Unlock()

	return params, nil
}

// FeatureVector represents a treatment event with surrounding glucose context
type FeatureVector struct {
	Time           time.Time
	TimeOfDay      models.TimeOfDayPeriod
	InsulinDose    float64
	CarbIntake     float64
	GlucoseBefore  float64
	GlucoseAfter1h float64
	GlucoseAfter2h float64
	GlucoseAfter3h float64
	GlucoseChange1h float64
	GlucoseChange2h float64
	TrendBefore    float64
}

func (a *Analyzer) buildFeatureVectors(entries []models.GlucoseEntry, treatments []models.Treatment) []FeatureVector {
	var vectors []FeatureVector

	for _, t := range treatments {
		if !t.HasInsulin() && !t.HasCarbs() {
			continue
		}

		treatTime := t.Time()
		vec := FeatureVector{
			Time:        treatTime,
			TimeOfDay:   models.GetTimeOfDayPeriod(treatTime),
			InsulinDose: t.Insulin,
			CarbIntake:  t.Carbs,
		}

		// Find glucose before and after treatment
		var glucoseBefore, glucose1h, glucose2h, glucose3h float64
		var trendSum float64
		var trendCount int

		for _, e := range entries {
			entryTime := e.Time()
			minutesDiff := entryTime.Sub(treatTime).Minutes()

			// Before treatment (within 30 min)
			if minutesDiff >= -30 && minutesDiff <= 0 {
				glucoseBefore = float64(e.SGV)
			}

			// Trend calculation (30 min before)
			if minutesDiff >= -35 && minutesDiff <= -5 {
				trendSum += float64(e.SGV)
				trendCount++
			}

			// After 1 hour (50-70 min)
			if minutesDiff >= 50 && minutesDiff <= 70 && glucose1h == 0 {
				glucose1h = float64(e.SGV)
			}

			// After 2 hours (110-130 min)
			if minutesDiff >= 110 && minutesDiff <= 130 && glucose2h == 0 {
				glucose2h = float64(e.SGV)
			}

			// After 3 hours (170-190 min)
			if minutesDiff >= 170 && minutesDiff <= 190 && glucose3h == 0 {
				glucose3h = float64(e.SGV)
			}
		}

		vec.GlucoseBefore = glucoseBefore
		vec.GlucoseAfter1h = glucose1h
		vec.GlucoseAfter2h = glucose2h
		vec.GlucoseAfter3h = glucose3h

		if glucoseBefore > 0 && glucose1h > 0 {
			vec.GlucoseChange1h = glucose1h - glucoseBefore
		}
		if glucoseBefore > 0 && glucose2h > 0 {
			vec.GlucoseChange2h = glucose2h - glucoseBefore
		}
		if trendCount > 0 {
			vec.TrendBefore = (trendSum / float64(trendCount)) - glucoseBefore
		}

		// Only include vectors with sufficient data
		if glucoseBefore > 0 && (glucose1h > 0 || glucose2h > 0) {
			vectors = append(vectors, vec)
		}
	}

	return vectors
}

func (a *Analyzer) calculateISFML(vectors []FeatureVector, params *models.DiabetesParameters) {
	// Find correction events (insulin without carbs, high glucose before)
	var isfValues []float64
	isfByTimeOfDay := make(map[models.TimeOfDayPeriod][]float64)

	for _, v := range vectors {
		if v.InsulinDose <= 0 || v.CarbIntake > 0 {
			continue
		}
		if v.GlucoseBefore < 150 {
			continue // Only use high glucose corrections
		}

		// Use 2h glucose change for ISF calculation
		glucoseChange := v.GlucoseChange2h
		if glucoseChange == 0 {
			glucoseChange = v.GlucoseChange1h
		}

		if glucoseChange < 0 { // Glucose dropped
			isf := math.Abs(glucoseChange) / v.InsulinDose
			if isf > 10 && isf < 200 {
				isfValues = append(isfValues, isf)
				isfByTimeOfDay[v.TimeOfDay] = append(isfByTimeOfDay[v.TimeOfDay], isf)
			}
		}
	}

	if len(isfValues) > 0 {
		// Use median for robustness
		params.ISF = median(isfValues)
		params.ISFConfidence = math.Min(100, float64(len(isfValues))*5)

		for period, values := range isfByTimeOfDay {
			if len(values) >= 3 {
				params.ISFByTimeOfDay[string(period)] = median(values)
			} else {
				params.ISFByTimeOfDay[string(period)] = params.ISF
			}
		}
	}
}

func (a *Analyzer) calculateICRML(vectors []FeatureVector, params *models.DiabetesParameters) {
	// Find meal events (carbs with proper insulin coverage)
	var icrValues []float64
	icrByTimeOfDay := make(map[models.TimeOfDayPeriod][]float64)

	for _, v := range vectors {
		if v.CarbIntake <= 0 || v.InsulinDose <= 0 {
			continue
		}
		if v.GlucoseBefore < 70 || v.GlucoseBefore > 180 {
			continue // Only use in-range starting glucose
		}

		// For well-covered meals, glucose should return near starting point
		glucoseChange2h := v.GlucoseChange2h
		if glucoseChange2h == 0 {
			glucoseChange2h = v.GlucoseChange1h
		}

		// If glucose stayed relatively stable, this was well-covered
		if math.Abs(glucoseChange2h) < 50 {
			icr := v.CarbIntake / v.InsulinDose
			if icr > 3 && icr < 30 {
				icrValues = append(icrValues, icr)
				icrByTimeOfDay[v.TimeOfDay] = append(icrByTimeOfDay[v.TimeOfDay], icr)
			}
		}
	}

	if len(icrValues) > 0 {
		params.ICR = median(icrValues)
		params.ICRConfidence = math.Min(100, float64(len(icrValues))*5)

		for period, values := range icrByTimeOfDay {
			if len(values) >= 3 {
				params.ICRByTimeOfDay[string(period)] = median(values)
			} else {
				params.ICRByTimeOfDay[string(period)] = params.ICR
			}
		}
	}
}

func (a *Analyzer) calculateDIAML(entries []models.GlucoseEntry, treatments []models.Treatment, params *models.DiabetesParameters) {
	// Analyze insulin-only corrections to determine how long effects last
	var diaValues []float64

	for _, t := range treatments {
		if !t.IsBolus() || t.Carbs > 0 {
			continue
		}

		treatTime := t.Time()
		
		// Track glucose over time to find when it stabilizes
		var glucoseTimeSeries []struct {
			minutes float64
			glucose float64
		}

		for _, e := range entries {
			minutesDiff := e.Time().Sub(treatTime).Minutes()
			if minutesDiff > 0 && minutesDiff < 360 { // 6 hours max
				glucoseTimeSeries = append(glucoseTimeSeries, struct {
					minutes float64
					glucose float64
				}{minutesDiff, float64(e.SGV)})
			}
		}

		if len(glucoseTimeSeries) < 10 {
			continue
		}

		// Find when glucose stabilizes (change < 5 mg/dL per 30 min)
		for i := 5; i < len(glucoseTimeSeries); i++ {
			recent := glucoseTimeSeries[i]
			earlier := glucoseTimeSeries[i-5]
			
			timeDiff := recent.minutes - earlier.minutes
			glucoseChange := math.Abs(recent.glucose - earlier.glucose)
			
			if timeDiff > 20 && glucoseChange/timeDiff*30 < 5 {
				// Glucose is stable, this is approximately when insulin action ended
				dia := recent.minutes / 60 // Convert to hours
				if dia > 2 && dia < 7 {
					diaValues = append(diaValues, dia)
				}
				break
			}
		}
	}

	if len(diaValues) > 3 {
		params.DIA = median(diaValues)
		params.DIAConfidence = math.Min(100, float64(len(diaValues))*8)
	}
}

func (a *Analyzer) calculateCarbAbsorptionML(entries []models.GlucoseEntry, treatments []models.Treatment, params *models.DiabetesParameters) {
	// Analyze carb-only events to determine absorption rate
	var absorptionRates []float64

	for _, t := range treatments {
		if t.Carbs <= 0 || t.Insulin > 0 {
			continue
		}

		treatTime := t.Time()
		var peakGlucose float64
		var peakTime time.Duration
		var baseGlucose float64

		for _, e := range entries {
			entryTime := e.Time()
			minutesDiff := entryTime.Sub(treatTime).Minutes()

			// Base glucose (just before carbs)
			if minutesDiff >= -15 && minutesDiff <= 0 {
				baseGlucose = float64(e.SGV)
			}

			// Track peak glucose within 3 hours
			if minutesDiff > 0 && minutesDiff < 180 {
				if float64(e.SGV) > peakGlucose {
					peakGlucose = float64(e.SGV)
					peakTime = time.Duration(minutesDiff) * time.Minute
				}
			}
		}

		if baseGlucose > 0 && peakGlucose > baseGlucose+20 {
			// Estimate absorption rate based on time to peak
			// Peak typically occurs when ~50% of carbs are absorbed
			peakHours := peakTime.Hours()
			if peakHours > 0.25 && peakHours < 3 {
				// Carbs absorbed at peak ≈ 50% of total
				rate := (t.Carbs * 0.5) / peakHours
				if rate > 10 && rate < 100 {
					absorptionRates = append(absorptionRates, rate)
				}
			}
		}
	}

	if len(absorptionRates) > 3 {
		params.CarbAbsorptionRate = median(absorptionRates)
	}
}

func (a *Analyzer) calculateTimeOfDayVariationsML(vectors []FeatureVector, params *models.DiabetesParameters) {
	// Group vectors by time of day and analyze patterns
	periods := []models.TimeOfDayPeriod{models.Morning, models.Midday, models.Evening, models.Night}

	for _, period := range periods {
		var periodVectors []FeatureVector
		for _, v := range vectors {
			if v.TimeOfDay == period {
				periodVectors = append(periodVectors, v)
			}
		}

		if len(periodVectors) < 3 {
			continue
		}

		// Calculate period-specific ISF
		var isfValues []float64
		for _, v := range periodVectors {
			if v.InsulinDose > 0 && v.CarbIntake == 0 && v.GlucoseChange2h < 0 {
				isf := math.Abs(v.GlucoseChange2h) / v.InsulinDose
				if isf > 10 && isf < 200 {
					isfValues = append(isfValues, isf)
				}
			}
		}
		if len(isfValues) >= 2 {
			params.ISFByTimeOfDay[string(period)] = median(isfValues)
		}

		// Calculate period-specific ICR
		var icrValues []float64
		for _, v := range periodVectors {
			if v.CarbIntake > 0 && v.InsulinDose > 0 {
				if math.Abs(v.GlucoseChange2h) < 50 {
					icr := v.CarbIntake / v.InsulinDose
					if icr > 3 && icr < 30 {
						icrValues = append(icrValues, icr)
					}
				}
			}
		}
		if len(icrValues) >= 2 {
			params.ICRByTimeOfDay[string(period)] = median(icrValues)
		}
	}

	// Fill in missing time-of-day values with overall values
	for _, period := range periods {
		if params.ISFByTimeOfDay[string(period)] == 0 {
			params.ISFByTimeOfDay[string(period)] = params.ISF
		}
		if params.ICRByTimeOfDay[string(period)] == 0 {
			params.ICRByTimeOfDay[string(period)] = params.ICR
		}
	}
}
