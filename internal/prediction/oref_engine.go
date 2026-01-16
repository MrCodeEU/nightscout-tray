// Package prediction provides glucose prediction and diabetes parameter calculation
// This file implements an oref1-inspired prediction engine based on research from
// OpenAPS, Loop, AndroidAPS, and academic literature on glucose prediction.
package prediction

import (
	"math"
	"sort"
	"time"

	"github.com/mrcode/nightscout-tray/internal/models"
)

// OrefEngine implements a comprehensive glucose prediction engine
// based on oref1 (OpenAPS Reference Implementation) principles combined
// with modern ML techniques for improved accuracy.
//
// Key features:
// - Multiple prediction curves (IOB, COB, ZT, UAM) with conservative selection
// - Exponential insulin activity curves (peak at 75 min, DIA 5 hours)
// - UVA/Padova-inspired carbohydrate absorption model
// - Autosens-style sensitivity detection
// - Circadian rhythm adjustments for dawn phenomenon
// - Hybrid physiological + ML residual learning
type OrefEngine struct {
	params *models.DiabetesParameters

	// Autosens state
	sensitivityRatio float64 // Current sensitivity ratio (0.7-1.2)
	deviationHistory []DeviationRecord

	// Pattern learning
	patterns *PatternDatabase

	// Configuration
	config OrefConfig
}

// OrefConfig contains configuration for the prediction engine
type OrefConfig struct {
	// Insulin parameters
	InsulinPeakMinutes float64 // Peak activity time (default 75 for rapid-acting)
	DIAMinutes         float64 // Duration of Insulin Action (default 300 = 5 hours)

	// Carb absorption parameters
	CarbAbsorptionDefault float64 // Default absorption time in minutes (180)
	Min5mCarbImpact       float64 // Minimum carb impact per 5 min (8 mg/dL with SMB)

	// Safety limits
	AutosensMax float64 // Maximum sensitivity adjustment (1.2 = +20%)
	AutosensMin float64 // Minimum sensitivity adjustment (0.7 = -30%)

	// Circadian adjustments (relative to baseline)
	DawnPhenomenonFactor   float64 // ISF reduction during dawn (0.6 = -40%)
	NightSensitivityFactor float64 // ISF increase at night (1.4 = +40%)

	// Prediction settings
	PredictionHorizonMinutes int  // How far to predict (default 360 = 6 hours)
	UseSMB                   bool // Whether Super Micro Bolus is enabled
}

// DeviationRecord tracks glucose deviations for Autosens
type DeviationRecord struct {
	Time          time.Time
	Deviation     float64 // Actual - Expected glucose change
	ExpectedDelta float64 // Expected change from insulin/carbs
	ActualDelta   float64 // Actual observed change
}

// PatternDatabase stores learned patterns for ML enhancement
type PatternDatabase struct {
	MealPatterns       []MealPattern
	CorrectionPatterns []CorrectionPattern
	CircadianProfile   CircadianProfile
}

// MealPattern represents learned meal response patterns
type MealPattern struct {
	TimeOfDay       float64    // 0-1 normalized time
	CarbAmount      float64    // Grams
	InsulinGiven    float64    // Units
	PreMealBG       float64    // Starting glucose
	PeakBGRise      float64    // Maximum rise
	TimeToReturn    float64    // Minutes to return near baseline
	ActualICR       float64    // Effective ICR from this meal
	GlucoseCurve    []float64  // 5-min samples of glucose response
	Count           int        // How many times seen
	LastSeen        time.Time
}

// CorrectionPattern represents learned correction response patterns
type CorrectionPattern struct {
	TimeOfDay    float64
	StartingBG   float64
	InsulinGiven float64
	BGDrop       float64
	TimeToNadir  float64 // Minutes to lowest point
	ActualISF    float64 // Effective ISF from this correction
	Count        int
	LastSeen     time.Time
}

// CircadianProfile represents learned time-of-day sensitivity patterns
type CircadianProfile struct {
	// Hourly sensitivity factors (0-23), 1.0 = baseline
	HourlySensitivity [24]float64
	// Hourly ICR factors
	HourlyICR [24]float64
	// Data counts per hour
	HourlyCounts [24]int
}

// PredictionCurves contains multiple prediction strategies
type PredictionCurves struct {
	IOBPrediction []PredictionPoint // Insulin-only projection
	COBPrediction []PredictionPoint // With carb effects
	ZTPrediction  []PredictionPoint // Zero-temp (what if insulin stops)
	UAMPrediction []PredictionPoint // Unannounced meal handling
	MLPrediction  []PredictionPoint // ML-enhanced prediction
	Final         []PredictionPoint // Conservative combination
}

// PredictionPoint represents a single prediction
type PredictionPoint struct {
	Time           time.Time
	Value          float64 // Predicted glucose mg/dL
	Confidence     float64 // 0-100
	InsulinEffect  float64 // Contribution from insulin
	CarbEffect     float64 // Contribution from carbs
	MomentumEffect float64 // Contribution from trend momentum
	SensAdjustment float64 // Sensitivity adjustment applied
}

// DefaultOrefConfig returns recommended default configuration
func DefaultOrefConfig() OrefConfig {
	return OrefConfig{
		InsulinPeakMinutes:       75,  // Research-backed for Novolog/Humalog
		DIAMinutes:               300, // 5 hours - research shows 3-4h is too short
		CarbAbsorptionDefault:    180, // 3 hours default
		Min5mCarbImpact:          8,   // 8 mg/dL per 5 min with SMB
		AutosensMax:              1.2, // Max +20% sensitivity
		AutosensMin:              0.7, // Min -30% sensitivity
		DawnPhenomenonFactor:     0.6, // -40% sensitivity during dawn
		NightSensitivityFactor:   1.4, // +40% sensitivity at night
		PredictionHorizonMinutes: 360, // 6 hours
		UseSMB:                   true,
	}
}

// NewOrefEngine creates a new prediction engine
func NewOrefEngine(params *models.DiabetesParameters) *OrefEngine {
	if params == nil {
		params = models.NewDiabetesParameters()
	}

	return &OrefEngine{
		params:           params,
		sensitivityRatio: 1.0,
		deviationHistory: make([]DeviationRecord, 0, 1000),
		patterns: &PatternDatabase{
			CircadianProfile: CircadianProfile{
				HourlySensitivity: [24]float64{
					1.4, 1.4, 1.2, 0.8, 0.7, 0.6, // 00:00-05:59 (night → dawn)
					0.6, 0.7, 0.8, 0.9, 1.0, 1.0, // 06:00-11:59 (morning)
					1.0, 1.0, 1.0, 1.0, 1.0, 1.0, // 12:00-17:59 (afternoon)
					1.0, 1.0, 1.1, 1.2, 1.3, 1.4, // 18:00-23:59 (evening → night)
				},
				HourlyICR: [24]float64{
					1.0, 1.0, 1.0, 1.0, 1.0, 1.0, // 00:00-05:59
					0.7, 0.75, 0.8, 0.9, 1.0, 1.0, // 06:00-11:59 (breakfast needs more insulin)
					1.0, 1.0, 1.0, 1.0, 1.0, 1.0, // 12:00-17:59
					1.1, 1.15, 1.1, 1.0, 1.0, 1.0, // 18:00-23:59 (dinner needs less)
				},
			},
		},
		config: DefaultOrefConfig(),
	}
}

// SetParameters updates the engine parameters
func (e *OrefEngine) SetParameters(params *models.DiabetesParameters) {
	e.params = params
}

// SetConfig updates the engine configuration
func (e *OrefEngine) SetConfig(config OrefConfig) {
	e.config = config
}

// LearnFromHistory trains the engine on historical data
func (e *OrefEngine) LearnFromHistory(entries []models.GlucoseEntry, treatments []models.Treatment) {
	if len(entries) < 100 || len(treatments) < 10 {
		return
	}

	// Sort data chronologically
	sortedEntries := make([]models.GlucoseEntry, len(entries))
	copy(sortedEntries, entries)
	sort.Slice(sortedEntries, func(i, j int) bool {
		return sortedEntries[i].Date < sortedEntries[j].Date
	})

	// Learn circadian patterns
	e.learnCircadianPatterns(sortedEntries, treatments)

	// Learn meal patterns
	e.learnMealPatterns(sortedEntries, treatments)

	// Learn correction patterns
	e.learnCorrectionPatterns(sortedEntries, treatments)

	// Calculate Autosens from recent history
	e.calculateAutosens(sortedEntries, treatments)
}

// learnCircadianPatterns learns time-of-day sensitivity variations
func (e *OrefEngine) learnCircadianPatterns(entries []models.GlucoseEntry, treatments []models.Treatment) {
	// Group correction events by hour
	hourlyISF := make(map[int][]float64)
	hourlyICR := make(map[int][]float64)

	// Find correction events (insulin without carbs)
	for _, t := range treatments {
		if !t.HasInsulin() || t.HasCarbs() {
			continue
		}
		if t.Insulin < 0.5 {
			continue
		}

		treatTime := t.Time()
		hour := treatTime.Hour()

		// Find BG before and after
		bgBefore := e.findBGAt(entries, treatTime)
		bgAfter := e.findBGAt(entries, treatTime.Add(2*time.Hour))

		if bgBefore > 150 && bgAfter > 0 && bgAfter < bgBefore {
			isf := (bgBefore - bgAfter) / t.Insulin
			if isf >= 10 && isf <= 200 {
				hourlyISF[hour] = append(hourlyISF[hour], isf)
			}
		}
	}

	// Find meal events
	for _, t := range treatments {
		if !t.HasInsulin() || !t.HasCarbs() {
			continue
		}

		treatTime := t.Time()
		hour := treatTime.Hour()

		// Find BG before and after
		bgBefore := e.findBGAt(entries, treatTime)
		bgAfter := e.findBGAt(entries, treatTime.Add(3*time.Hour))

		if bgBefore > 0 && bgAfter > 0 {
			bgChange := math.Abs(bgAfter - bgBefore)
			// Good meal coverage if BG stays relatively stable
			if bgChange < 50 {
				icr := t.Carbs / t.Insulin
				if icr >= 3 && icr <= 40 {
					hourlyICR[hour] = append(hourlyICR[hour], icr)
				}
			}
		}
	}

	// Calculate hourly sensitivities relative to median
	totalISF := 0
	for _, values := range hourlyISF {
		totalISF += len(values)
	}
	allISF := make([]float64, 0, totalISF)
	for _, values := range hourlyISF {
		allISF = append(allISF, values...)
	}

	if len(allISF) >= 10 {
		medianISF := median(allISF)
		for hour, values := range hourlyISF {
			if len(values) >= 2 {
				hourMedian := median(values)
				e.patterns.CircadianProfile.HourlySensitivity[hour] = hourMedian / medianISF
				e.patterns.CircadianProfile.HourlyCounts[hour] = len(values)
			}
		}
	}

	totalICR := 0
	for _, values := range hourlyICR {
		totalICR += len(values)
	}
	allICR := make([]float64, 0, totalICR)
	for _, values := range hourlyICR {
		allICR = append(allICR, values...)
	}

	if len(allICR) >= 10 {
		medianICR := median(allICR)
		for hour, values := range hourlyICR {
			if len(values) >= 2 {
				hourMedian := median(values)
				// ICR ratio is inverse (lower ICR = more insulin needed)
				e.patterns.CircadianProfile.HourlyICR[hour] = medianICR / hourMedian
			}
		}
	}
}

// learnMealPatterns learns individual meal response patterns
func (e *OrefEngine) learnMealPatterns(entries []models.GlucoseEntry, treatments []models.Treatment) {
	for _, t := range treatments {
		if !t.HasCarbs() {
			continue
		}

		treatTime := t.Time()
		timeOfDay := float64(treatTime.Hour()*60+treatTime.Minute()) / 1440.0

		// Get glucose curve for 3 hours after meal
		curve := make([]float64, 0, 36) // 3 hours at 5-min intervals
		preMealBG := e.findBGAt(entries, treatTime)
		if preMealBG <= 0 {
			continue
		}

		peakRise := 0.0
		for i := 0; i < 36; i++ {
			t := treatTime.Add(time.Duration(i*5) * time.Minute)
			bg := e.findBGAt(entries, t)
			if bg > 0 {
				curve = append(curve, bg)
				rise := bg - preMealBG
				if rise > peakRise {
					peakRise = rise
				}
			}
		}

		if len(curve) < 12 {
			continue
		}

		// Calculate effective ICR if we have insulin data
		actualICR := 0.0
		if t.HasInsulin() && t.Insulin > 0 {
			actualICR = t.Carbs / t.Insulin
		}

		pattern := MealPattern{
			TimeOfDay:    timeOfDay,
			CarbAmount:   t.Carbs,
			InsulinGiven: t.Insulin,
			PreMealBG:    preMealBG,
			PeakBGRise:   peakRise,
			ActualICR:    actualICR,
			GlucoseCurve: curve,
			Count:        1,
			LastSeen:     treatTime,
		}

		// Add or merge with existing pattern
		e.addMealPattern(pattern)
	}
}

// addMealPattern adds or merges a meal pattern
func (e *OrefEngine) addMealPattern(pattern MealPattern) {
	// Find similar existing pattern
	for i := range e.patterns.MealPatterns {
		p := &e.patterns.MealPatterns[i]
		if math.Abs(p.TimeOfDay-pattern.TimeOfDay) < 0.1 && // Within ~2.4 hours
			math.Abs(p.CarbAmount-pattern.CarbAmount) < 20 { // Similar carb amount
			// Merge patterns using exponential moving average
			alpha := 0.2
			p.PeakBGRise = (1-alpha)*p.PeakBGRise + alpha*pattern.PeakBGRise
			if pattern.ActualICR > 0 {
				p.ActualICR = (1-alpha)*p.ActualICR + alpha*pattern.ActualICR
			}
			p.Count++
			p.LastSeen = pattern.LastSeen
			return
		}
	}

	// Add new pattern
	if len(e.patterns.MealPatterns) < 100 {
		e.patterns.MealPatterns = append(e.patterns.MealPatterns, pattern)
	}
}

// learnCorrectionPatterns learns correction response patterns
func (e *OrefEngine) learnCorrectionPatterns(entries []models.GlucoseEntry, treatments []models.Treatment) {
	for _, t := range treatments {
		if !t.HasInsulin() || t.HasCarbs() {
			continue
		}
		if t.Insulin < 0.5 {
			continue
		}

		treatTime := t.Time()
		timeOfDay := float64(treatTime.Hour()*60+treatTime.Minute()) / 1440.0
		startingBG := e.findBGAt(entries, treatTime)

		if startingBG < 150 { // Not a correction
			continue
		}

		// Track BG over next 4 hours to find nadir
		nadirBG := startingBG
		nadirMinutes := 0.0
		for i := 1; i <= 48; i++ { // 4 hours at 5-min intervals
			checkTime := treatTime.Add(time.Duration(i*5) * time.Minute)
			bg := e.findBGAt(entries, checkTime)
			if bg > 0 && bg < nadirBG {
				nadirBG = bg
				nadirMinutes = float64(i * 5)
			}
		}

		bgDrop := startingBG - nadirBG
		if bgDrop < 20 {
			continue
		}

		actualISF := bgDrop / t.Insulin

		pattern := CorrectionPattern{
			TimeOfDay:    timeOfDay,
			StartingBG:   startingBG,
			InsulinGiven: t.Insulin,
			BGDrop:       bgDrop,
			TimeToNadir:  nadirMinutes,
			ActualISF:    actualISF,
			Count:        1,
			LastSeen:     treatTime,
		}

		// Add or merge
		e.addCorrectionPattern(pattern)
	}
}

// addCorrectionPattern adds or merges a correction pattern
func (e *OrefEngine) addCorrectionPattern(pattern CorrectionPattern) {
	for i := range e.patterns.CorrectionPatterns {
		p := &e.patterns.CorrectionPatterns[i]
		if math.Abs(p.TimeOfDay-pattern.TimeOfDay) < 0.1 &&
			math.Abs(p.StartingBG-pattern.StartingBG) < 30 {
			alpha := 0.2
			p.ActualISF = (1-alpha)*p.ActualISF + alpha*pattern.ActualISF
			p.TimeToNadir = (1-alpha)*p.TimeToNadir + alpha*pattern.TimeToNadir
			p.Count++
			p.LastSeen = pattern.LastSeen
			return
		}
	}

	if len(e.patterns.CorrectionPatterns) < 100 {
		e.patterns.CorrectionPatterns = append(e.patterns.CorrectionPatterns, pattern)
	}
}

// calculateAutosens calculates current sensitivity ratio from recent deviations
func (e *OrefEngine) calculateAutosens(entries []models.GlucoseEntry, treatments []models.Treatment) {
	// Look at last 8-24 hours of data
	now := time.Now()
	cutoff := now.Add(-24 * time.Hour)

	// Build a map of expected vs actual glucose changes
	deviations := make([]float64, 0)

	for i := 1; i < len(entries); i++ {
		entryTime := entries[i].Time()
		if entryTime.Before(cutoff) {
			continue
		}

		prevTime := entries[i-1].Time()
		gap := entryTime.Sub(prevTime).Minutes()
		if gap < 4 || gap > 6 {
			continue
		}

		actualDelta := float64(entries[i].SGV - entries[i-1].SGV)

		// Calculate expected delta from insulin and carb activity
		expectedDelta := e.calculateExpectedDelta(entries[i-1].Time(), entries[i].Time(), treatments)

		if math.Abs(expectedDelta) > 5 { // Only use periods with significant expected effects
			// Calculate ratio of actual to expected change
			ratio := (actualDelta + 100) / (expectedDelta + 100) // Add offset to avoid division issues
			deviations = append(deviations, ratio)
		}
	}

	if len(deviations) >= 10 {
		// Use median ratio as sensitivity
		sort.Float64s(deviations)
		medianRatio := deviations[len(deviations)/2]

		// Clamp to safety limits
		e.sensitivityRatio = math.Max(e.config.AutosensMin,
			math.Min(e.config.AutosensMax, medianRatio))
	}
}

// calculateExpectedDelta calculates expected glucose change between two times
func (e *OrefEngine) calculateExpectedDelta(from, to time.Time, treatments []models.Treatment) float64 {
	insulinEffect := 0.0
	carbEffect := 0.0

	for _, t := range treatments {
		if t.HasInsulin() {
			// Calculate insulin activity in this window
			activityFrom := e.insulinActivityRemaining(from.Sub(t.Time()).Minutes())
			activityTo := e.insulinActivityRemaining(to.Sub(t.Time()).Minutes())
			activityUsed := activityFrom - activityTo
			if activityUsed > 0 {
				insulinEffect -= t.Insulin * activityUsed * e.params.ISF
			}
		}

		if t.HasCarbs() {
			// Calculate carb absorption in this window
			absorbedFrom := e.carbsAbsorbed(t.Carbs, from.Sub(t.Time()).Minutes())
			absorbedTo := e.carbsAbsorbed(t.Carbs, to.Sub(t.Time()).Minutes())
			absorbedInWindow := absorbedTo - absorbedFrom
			if absorbedInWindow > 0 {
				csf := e.params.ISF / e.params.ICR // Carb sensitivity factor
				carbEffect += absorbedInWindow * csf
			}
		}
	}

	return insulinEffect + carbEffect
}

// Predict generates comprehensive glucose predictions
func (e *OrefEngine) Predict(
	currentGlucose float64,
	entries []models.GlucoseEntry,
	treatments []models.Treatment,
	thresholdHigh float64,
	thresholdLow float64,
) *models.PredictionResult {
	now := time.Now()

	// Generate all prediction curves
	curves := e.generatePredictionCurves(currentGlucose, entries, treatments, now)

	// Build result
	result := &models.PredictionResult{
		PredictedAt:    now,
		BasedOnGlucose: currentGlucose,
		BasedOnTrend:   e.calculateTrend(entries),
		HighThreshold:  thresholdHigh,
		LowThreshold:   thresholdLow,
		IOB:            e.calculateIOB(treatments, now),
		COB:            e.calculateCOB(treatments, now),
	}

	// Convert to model format
	result.ShortTerm = e.curvesToPredictedPoints(curves.Final[:24], true)  // First 2 hours
	result.LongTerm = e.curvesToPredictedPoints(curves.Final[24:], false) // 2-6 hours

	// Calculate time to thresholds
	result.HighInMinutes, result.LowInMinutes = e.calculateThresholdTimes(
		curves.Final, thresholdHigh, thresholdLow,
	)

	return result
}

// generatePredictionCurves creates all prediction strategies
func (e *OrefEngine) generatePredictionCurves(
	currentGlucose float64,
	entries []models.GlucoseEntry,
	treatments []models.Treatment,
	now time.Time,
) *PredictionCurves {
	curves := &PredictionCurves{}

	// Get current momentum
	momentum := e.calculateMomentum(entries)

	// Calculate predictions for each 5-minute interval
	steps := e.config.PredictionHorizonMinutes / 5
	for step := 1; step <= steps; step++ {
		predTime := now.Add(time.Duration(step*5) * time.Minute)
		minutesOut := float64(step * 5)

		// 1. IOB-only prediction
		iobPred := e.predictIOBOnly(currentGlucose, momentum, treatments, now, predTime)
		curves.IOBPrediction = append(curves.IOBPrediction, iobPred)

		// 2. COB prediction (includes carbs)
		cobPred := e.predictWithCOB(currentGlucose, momentum, treatments, now, predTime)
		curves.COBPrediction = append(curves.COBPrediction, cobPred)

		// 3. Zero-temp prediction (what if insulin delivery stops)
		ztPred := e.predictZeroTemp(currentGlucose, momentum, treatments, now, predTime)
		curves.ZTPrediction = append(curves.ZTPrediction, ztPred)

		// 4. UAM prediction (unannounced meal handling)
		uamPred := e.predictUAM(currentGlucose, entries, treatments, now, predTime)
		curves.UAMPrediction = append(curves.UAMPrediction, uamPred)

		// 5. ML-enhanced prediction
		mlPred := e.predictML(currentGlucose, entries, treatments, now, predTime)
		curves.MLPrediction = append(curves.MLPrediction, mlPred)

		// 6. Final: Conservative combination
		// For safety, use the prediction that results in highest insulin need
		// (usually the highest predicted glucose)
		final := e.selectConservativePrediction(
			iobPred, cobPred, ztPred, uamPred, mlPred, minutesOut,
		)
		curves.Final = append(curves.Final, final)
	}

	return curves
}

// predictIOBOnly predicts based only on insulin-on-board effects
func (e *OrefEngine) predictIOBOnly(
	currentBG float64,
	momentum float64,
	treatments []models.Treatment,
	startTime, predTime time.Time,
) PredictionPoint {
	minutesOut := predTime.Sub(startTime).Minutes()

	// Get hour-adjusted ISF
	hour := predTime.Hour()
	sensitivityFactor := e.patterns.CircadianProfile.HourlySensitivity[hour]
	adjustedISF := e.params.ISF * sensitivityFactor * e.sensitivityRatio

	// Calculate insulin effect
	insulinEffect := 0.0
	for _, t := range treatments {
		if !t.HasInsulin() {
			continue
		}

		activityAtStart := e.insulinActivityRemaining(startTime.Sub(t.Time()).Minutes())
		activityAtPred := e.insulinActivityRemaining(predTime.Sub(t.Time()).Minutes())
		activityUsed := activityAtStart - activityAtPred

		if activityUsed > 0 {
			insulinEffect -= t.Insulin * activityUsed * adjustedISF
		}
	}

	// Momentum effect with decay
	momentumDecay := math.Exp(-0.03 * minutesOut)
	momentumEffect := momentum * (minutesOut / 5) * momentumDecay

	predicted := currentBG + insulinEffect + momentumEffect
	predicted = math.Max(20, math.Min(500, predicted))

	confidence := 90 - minutesOut*0.15
	if confidence < 20 {
		confidence = 20
	}

	return PredictionPoint{
		Time:           predTime,
		Value:          predicted,
		Confidence:     confidence,
		InsulinEffect:  insulinEffect,
		MomentumEffect: momentumEffect,
		SensAdjustment: sensitivityFactor,
	}
}

// predictWithCOB includes carbohydrate effects
func (e *OrefEngine) predictWithCOB(
	currentBG float64,
	momentum float64,
	treatments []models.Treatment,
	startTime, predTime time.Time,
) PredictionPoint {
	pred := e.predictIOBOnly(currentBG, momentum, treatments, startTime, predTime)

	hour := predTime.Hour()
	icrFactor := e.patterns.CircadianProfile.HourlyICR[hour]
	adjustedICR := e.params.ICR / icrFactor // Lower ICR = more insulin per carb
	csf := (e.params.ISF * e.sensitivityRatio) / adjustedICR

	// Calculate carb effect
	carbEffect := 0.0
	for _, t := range treatments {
		if !t.HasCarbs() {
			continue
		}

		absorbedAtStart := e.carbsAbsorbed(t.Carbs, startTime.Sub(t.Time()).Minutes())
		absorbedAtPred := e.carbsAbsorbed(t.Carbs, predTime.Sub(t.Time()).Minutes())
		absorbedInWindow := absorbedAtPred - absorbedAtStart

		if absorbedInWindow > 0 {
			carbEffect += absorbedInWindow * csf
		}
	}

	pred.Value += carbEffect
	pred.Value = math.Max(20, math.Min(500, pred.Value))
	pred.CarbEffect = carbEffect

	return pred
}

// predictZeroTemp predicts what would happen if insulin delivery stopped
func (e *OrefEngine) predictZeroTemp(
	currentBG float64,
	momentum float64,
	treatments []models.Treatment,
	startTime, predTime time.Time,
) PredictionPoint {
	// Only count insulin already delivered, no new insulin
	// This is useful for safety - shows where BG is heading
	pred := e.predictWithCOB(currentBG, momentum, treatments, startTime, predTime)

	// Zero-temp is identical to COB prediction since we're not modeling
	// future insulin delivery in this implementation
	pred.Confidence *= 0.9 // Slightly lower confidence

	return pred
}

// predictUAM handles unannounced meals through deviation detection
func (e *OrefEngine) predictUAM(
	currentBG float64,
	entries []models.GlucoseEntry,
	treatments []models.Treatment,
	startTime, predTime time.Time,
) PredictionPoint {
	minutesOut := predTime.Sub(startTime).Minutes()

	// Detect if BG is rising faster than expected (possible unannounced meal)
	recentTrend := e.calculateTrend(entries)
	expectedTrend := e.calculateExpectedTrend(entries, treatments, startTime)
	deviation := recentTrend - expectedTrend

	// If BG is rising unexpectedly, assume some unannounced carbs
	uamEffect := 0.0
	if deviation > 0.5 { // Rising faster than expected by 0.5 mg/dL per 5 min
		// Calculate how this would affect future BG
		// Assume these "carbs" peak at 45 min and absorb over 2 hours
		peakMinutes := 45.0
		if minutesOut < peakMinutes {
			uamEffect = deviation * (minutesOut / 5) * (minutesOut / peakMinutes)
		} else {
			// Decay after peak
			decay := math.Exp(-0.02 * (minutesOut - peakMinutes))
			uamEffect = deviation * (peakMinutes / 5) * decay
		}
	}

	// Start with COB prediction
	pred := e.predictWithCOB(currentBG, recentTrend, treatments, startTime, predTime)
	pred.Value += uamEffect
	pred.Value = math.Max(20, math.Min(500, pred.Value))

	return pred
}

// predictML uses learned patterns for prediction
func (e *OrefEngine) predictML(
	currentBG float64,
	entries []models.GlucoseEntry,
	treatments []models.Treatment,
	startTime, predTime time.Time,
) PredictionPoint {
	// Start with physiological prediction as base
	momentum := e.calculateMomentum(entries)
	pred := e.predictWithCOB(currentBG, momentum, treatments, startTime, predTime)

	minutesOut := predTime.Sub(startTime).Minutes()

	// Apply pattern-based corrections
	timeOfDay := float64(startTime.Hour()*60+startTime.Minute()) / 1440.0

	// Find similar meal patterns if we recently had carbs
	for _, t := range treatments {
		if !t.HasCarbs() {
			continue
		}
		mealAge := startTime.Sub(t.Time()).Minutes()
		if mealAge < 0 || mealAge > 180 {
			continue
		}

		// Find similar meal pattern
		for _, pattern := range e.patterns.MealPatterns {
			if math.Abs(pattern.TimeOfDay-timeOfDay) < 0.1 &&
				math.Abs(pattern.CarbAmount-t.Carbs) < 20 &&
				pattern.Count >= 2 {
				// Apply learned pattern correction
				// If similar meals typically peaked higher, adjust prediction up
				predictedIdx := int(minutesOut / 5)
				if predictedIdx < len(pattern.GlucoseCurve) {
					patternBG := pattern.GlucoseCurve[predictedIdx]
					patternRise := patternBG - pattern.PreMealBG
					expectedRise := pred.Value - currentBG

					// Blend pattern-based prediction with physiological
					correction := (patternRise - expectedRise) * 0.3 // 30% weight to pattern
					pred.Value += correction
				}
			}
		}
	}

	// Apply correction pattern insights
	if currentBG > 180 {
		for _, pattern := range e.patterns.CorrectionPatterns {
			if math.Abs(pattern.StartingBG-currentBG) < 30 &&
				math.Abs(pattern.TimeOfDay-timeOfDay) < 0.1 &&
				pattern.Count >= 2 {
				// Adjust based on learned ISF
				isfRatio := pattern.ActualISF / e.params.ISF
				pred.InsulinEffect *= isfRatio
				pred.Value = currentBG + pred.InsulinEffect + pred.CarbEffect + pred.MomentumEffect
			}
		}
	}

	pred.Value = math.Max(20, math.Min(500, pred.Value))
	pred.Confidence *= 0.95 // Slightly higher confidence with ML enhancement

	return pred
}

// selectConservativePrediction chooses the safest prediction
func (e *OrefEngine) selectConservativePrediction(
	iob, cob, zt, uam, ml PredictionPoint,
	minutesOut float64,
) PredictionPoint {
	// For short-term (< 30 min), use the highest prediction (safest for avoiding hypo)
	// For medium-term, blend predictions
	// For long-term, rely more on physiological model

	if minutesOut <= 30 {
		// Short-term: Use highest prediction
		highest := iob
		for _, p := range []PredictionPoint{cob, zt, uam, ml} {
			if p.Value > highest.Value {
				highest = p
			}
		}
		return highest
	} else if minutesOut <= 120 {
		// Medium-term: Weighted average with emphasis on COB and ML
		weights := []float64{0.1, 0.3, 0.15, 0.15, 0.3} // iob, cob, zt, uam, ml
		preds := []PredictionPoint{iob, cob, zt, uam, ml}

		result := PredictionPoint{
			Time:       iob.Time,
			Confidence: 0,
		}

		totalWeight := 0.0
		for i, p := range preds {
			result.Value += p.Value * weights[i]
			result.InsulinEffect += p.InsulinEffect * weights[i]
			result.CarbEffect += p.CarbEffect * weights[i]
			result.MomentumEffect += p.MomentumEffect * weights[i]
			result.Confidence += p.Confidence * weights[i]
			totalWeight += weights[i]
		}

		result.Value /= totalWeight
		result.InsulinEffect /= totalWeight
		result.CarbEffect /= totalWeight
		result.MomentumEffect /= totalWeight
		result.Confidence /= totalWeight

		return result
	} else {
		// Long-term: Rely on physiological COB prediction
		return cob
	}
}

// Helper functions

// insulinActivityRemaining returns fraction of insulin still active
// Uses exponential activity curve with peak at 75 minutes
func (e *OrefEngine) insulinActivityRemaining(minutesSince float64) float64 {
	if minutesSince <= 0 {
		return 1.0
	}
	if minutesSince >= e.config.DIAMinutes {
		return 0.0
	}

	// Exponential activity curve: Activity(t) = (t/τ²) × exp(-t/τ)
	// Integrated to get remaining insulin
	peak := e.config.InsulinPeakMinutes
	dia := e.config.DIAMinutes

	tau := peak * (1 - peak/dia)
	if tau <= 0 {
		tau = peak * 0.75
	}

	a := 2 * tau / dia
	S := 1 / (1 - a + (1+a)*math.Exp(-dia/tau))

	remaining := 1 - S*(1-(1+minutesSince/tau)*math.Exp(-minutesSince/tau))
	return math.Max(0, math.Min(1, remaining))
}

// carbsAbsorbed returns grams of carbs absorbed after given minutes
// Uses UVA/Padova-inspired nonlinear absorption model
func (e *OrefEngine) carbsAbsorbed(totalCarbs, minutesSince float64) float64 {
	if minutesSince <= 0 {
		return 0
	}

	// Calculate absorption time based on carb amount
	// More carbs = slower absorption
	absorptionTime := e.config.CarbAbsorptionDefault
	if totalCarbs > 60 {
		absorptionTime *= 1.3 // Large meals absorb 30% slower
	} else if totalCarbs < 20 {
		absorptionTime *= 0.7 // Small snacks absorb 30% faster
	}

	if minutesSince >= absorptionTime {
		return totalCarbs
	}

	// Nonlinear absorption: fast-then-slow profile
	// Uses modified logistic curve
	progress := minutesSince / absorptionTime

	// Parameters for absorption curve
	// Peak absorption rate around 30% into the meal
	k := 8.0  // Steepness
	c := 0.35 // Center point (35% through absorption time = peak rate)

	absorbed := totalCarbs / (1 + math.Exp(-k*(progress-c)))

	// Ensure minimum absorption rate (min_5m_carbimpact safety)
	minAbsorbed := (minutesSince / 5) * (e.config.Min5mCarbImpact / (e.params.ISF / e.params.ICR))
	absorbed = math.Max(absorbed, minAbsorbed)

	return math.Min(absorbed, totalCarbs)
}

// calculateIOB returns current insulin on board
func (e *OrefEngine) calculateIOB(treatments []models.Treatment, now time.Time) float64 {
	var iob float64

	for _, t := range treatments {
		if !t.HasInsulin() {
			continue
		}

		minutesSince := now.Sub(t.Time()).Minutes()
		if minutesSince < 0 || minutesSince > e.config.DIAMinutes {
			continue
		}

		remaining := e.insulinActivityRemaining(minutesSince)
		iob += t.Insulin * remaining
	}

	return math.Round(iob*100) / 100
}

// calculateCOB returns current carbs on board
func (e *OrefEngine) calculateCOB(treatments []models.Treatment, now time.Time) float64 {
	var cob float64

	for _, t := range treatments {
		if !t.HasCarbs() {
			continue
		}

		minutesSince := now.Sub(t.Time()).Minutes()
		if minutesSince < 0 {
			continue
		}

		absorbed := e.carbsAbsorbed(t.Carbs, minutesSince)
		remaining := t.Carbs - absorbed
		if remaining > 0 {
			cob += remaining
		}
	}

	return math.Round(cob*10) / 10
}

// calculateTrend returns the current glucose trend (mg/dL per 5 min)
func (e *OrefEngine) calculateTrend(entries []models.GlucoseEntry) float64 {
	if len(entries) < 2 {
		return 0
	}

	// Sort by time (newest first)
	sorted := make([]models.GlucoseEntry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Date > sorted[j].Date
	})

	// Use last 15 minutes of data
	now := time.Now()
	var sumDelta, count float64

	for i := 0; i < len(sorted)-1 && i < 3; i++ {
		age := now.Sub(sorted[i].Time()).Minutes()
		if age > 20 {
			break
		}

		gap := sorted[i].Time().Sub(sorted[i+1].Time()).Minutes()
		if gap >= 4 && gap <= 6 {
			delta := float64(sorted[i].SGV - sorted[i+1].SGV)
			sumDelta += delta
			count++
		}
	}

	if count > 0 {
		return sumDelta / count
	}
	return 0
}

// calculateMomentum returns glucose momentum for prediction
func (e *OrefEngine) calculateMomentum(entries []models.GlucoseEntry) float64 {
	return e.calculateTrend(entries)
}

// calculateExpectedTrend calculates expected BG change based on insulin/carbs
func (e *OrefEngine) calculateExpectedTrend(entries []models.GlucoseEntry, treatments []models.Treatment, now time.Time) float64 {
	// Calculate expected 5-min change based on active insulin and carbs
	fiveMinAgo := now.Add(-5 * time.Minute)
	return e.calculateExpectedDelta(fiveMinAgo, now, treatments) / 5
}

// findBGAt finds the glucose value closest to a given time
func (e *OrefEngine) findBGAt(entries []models.GlucoseEntry, targetTime time.Time) float64 {
	var closest models.GlucoseEntry
	minDiff := time.Duration(1<<62 - 1)

	for _, entry := range entries {
		diff := entry.Time().Sub(targetTime)
		if diff < 0 {
			diff = -diff
		}
		if diff < minDiff {
			minDiff = diff
			closest = entry
		}
	}

	// Only return if within 10 minutes
	if minDiff <= 10*time.Minute {
		return float64(closest.SGV)
	}
	return 0
}

// curvesToPredictedPoints converts internal prediction to model format
func (e *OrefEngine) curvesToPredictedPoints(points []PredictionPoint, highConfidence bool) []models.PredictedPoint {
	result := make([]models.PredictedPoint, len(points))

	for i, p := range points {
		conf := p.Confidence
		if !highConfidence {
			conf *= 0.8
		}

		result[i] = models.PredictedPoint{
			Time:          p.Time.UnixMilli(),
			Value:         math.Round(p.Value*10) / 10,
			ValueMmol:     models.ToMmol(p.Value),
			Confidence:    conf,
			InsulinEffect: p.InsulinEffect,
			CarbEffect:    p.CarbEffect,
			TrendEffect:   p.MomentumEffect,
		}
	}

	return result
}

// calculateThresholdTimes calculates time until high/low thresholds
func (e *OrefEngine) calculateThresholdTimes(
	points []PredictionPoint,
	highThreshold, lowThreshold float64,
) (highIn, lowIn float64) {
	highIn = -1
	lowIn = -1

	for i, p := range points {
		minutes := time.Until(p.Time).Minutes()

		if highIn < 0 && p.Value >= highThreshold {
			if i > 0 && points[i-1].Value < highThreshold {
				// Interpolate
				ratio := (highThreshold - points[i-1].Value) / (p.Value - points[i-1].Value)
				prevMin := time.Until(points[i-1].Time).Minutes()
				highIn = prevMin + ratio*(minutes-prevMin)
			} else {
				highIn = minutes
			}
		}

		if lowIn < 0 && p.Value <= lowThreshold {
			if i > 0 && points[i-1].Value > lowThreshold {
				ratio := (points[i-1].Value - lowThreshold) / (points[i-1].Value - p.Value)
				prevMin := time.Until(points[i-1].Time).Minutes()
				lowIn = prevMin + ratio*(minutes-prevMin)
			} else {
				lowIn = minutes
			}
		}

		if highIn >= 0 && lowIn >= 0 {
			break
		}
	}

	return highIn, lowIn
}

// GetAutosensRatio returns the current autosens sensitivity ratio
func (e *OrefEngine) GetAutosensRatio() float64 {
	return e.sensitivityRatio
}

// GetCircadianProfile returns the learned circadian sensitivity profile
func (e *OrefEngine) GetCircadianProfile() CircadianProfile {
	return e.patterns.CircadianProfile
}

// GetPatternStats returns statistics about learned patterns
func (e *OrefEngine) GetPatternStats() (mealCount, correctionCount int) {
	return len(e.patterns.MealPatterns), len(e.patterns.CorrectionPatterns)
}
