// Package prediction provides glucose prediction and diabetes parameter calculation
package prediction

import (
	"math"
	"sort"
	"time"

	"github.com/mrcode/nightscout-tray/internal/models"
)

// Predictor predicts future glucose values based on current state and parameters
type Predictor struct {
	params *models.DiabetesParameters
}

// NewPredictor creates a new Predictor with the given parameters
func NewPredictor(params *models.DiabetesParameters) *Predictor {
	if params == nil {
		params = models.NewDiabetesParameters()
	}
	return &Predictor{params: params}
}

// SetParameters updates the prediction parameters
func (p *Predictor) SetParameters(params *models.DiabetesParameters) {
	p.params = params
}

// Predict generates glucose predictions based on current data
func (p *Predictor) Predict(
	currentGlucose float64,
	currentTrend float64, // mg/dL per 5 minutes
	recentEntries []models.GlucoseEntry,
	recentTreatments []models.Treatment,
) *models.PredictionResult {
	now := time.Now()

	result := &models.PredictionResult{
		PredictedAt:    now,
		BasedOnGlucose: currentGlucose,
		BasedOnTrend:   currentTrend,
	}

	// Calculate IOB (Insulin on Board)
	result.IOB = p.calculateIOB(recentTreatments, now)
	result.IOBDuration = p.calculateIOBDuration(recentTreatments, now)

	// Calculate COB (Carbs on Board)
	result.COB = p.calculateCOB(recentTreatments, now)
	result.COBDuration = p.calculateCOBDuration(result.COB)

	// Generate short-term predictions (every 5 minutes for 2 hours)
	result.ShortTerm = p.predictRange(
		currentGlucose,
		currentTrend,
		recentEntries,
		recentTreatments,
		now,
		2*time.Hour,
		5*time.Minute,
		true, // high confidence mode
	)

	// Generate long-term predictions (every 15 minutes for 6 hours)
	result.LongTerm = p.predictRange(
		currentGlucose,
		currentTrend,
		recentEntries,
		recentTreatments,
		now,
		6*time.Hour,
		15*time.Minute,
		false, // lower confidence mode
	)

	return result
}

// predictRange generates predictions for a given time range
func (p *Predictor) predictRange(
	currentGlucose float64,
	currentTrend float64,
	recentEntries []models.GlucoseEntry,
	recentTreatments []models.Treatment,
	startTime time.Time,
	duration time.Duration,
	interval time.Duration,
	highConfidence bool,
) []models.PredictedPoint {
	var predictions []models.PredictedPoint

	steps := int(duration / interval)
	prevGlucose := currentGlucose

	for i := 1; i <= steps; i++ {
		predTime := startTime.Add(time.Duration(i) * interval)
		minutesOut := float64(i) * interval.Minutes()

		// Calculate insulin effect at this time point
		insulinEffect := p.calculateInsulinEffect(recentTreatments, startTime, predTime)

		// Calculate carb effect at this time point
		carbEffect := p.calculateCarbEffect(recentTreatments, startTime, predTime)

		// Calculate trend effect (diminishing over time)
		trendEffect := p.calculateTrendEffect(currentTrend, minutesOut)

		// Base prediction starts from previous value for smoother curves
		baseGlucose := prevGlucose

		// Apply effects
		predictedValue := baseGlucose + insulinEffect + carbEffect + trendEffect

		// Apply physiological constraints
		predictedValue = p.applyConstraints(predictedValue, minutesOut)

		// Calculate confidence based on time and mode
		confidence := p.calculateConfidence(minutesOut, highConfidence, len(recentEntries))

		point := models.PredictedPoint{
			Time:          predTime.UnixMilli(),
			Value:         math.Round(predictedValue*10) / 10,
			ValueMmol:     models.ToMmol(predictedValue),
			Confidence:    confidence,
			InsulinEffect: insulinEffect,
			CarbEffect:    carbEffect,
			TrendEffect:   trendEffect,
		}

		predictions = append(predictions, point)
		prevGlucose = predictedValue
	}

	return predictions
}

// calculateIOB calculates current Insulin on Board using exponential decay
func (p *Predictor) calculateIOB(treatments []models.Treatment, now time.Time) float64 {
	diaMinutes := p.params.DIA * 60
	var totalIOB float64

	for _, t := range treatments {
		if !t.HasInsulin() || !t.IsBolus() {
			continue
		}

		treatTime := t.Time()
		if treatTime.After(now) {
			continue
		}

		minutesAgo := now.Sub(treatTime).Minutes()
		if minutesAgo > diaMinutes {
			continue
		}

		// Use exponential decay model for insulin activity
		// Peak activity at ~75 minutes, then decay
		remaining := p.insulinActivityRemaining(minutesAgo)
		totalIOB += t.Insulin * remaining
	}

	return math.Round(totalIOB*100) / 100
}

// calculateIOBDuration calculates minutes until IOB is depleted
func (p *Predictor) calculateIOBDuration(treatments []models.Treatment, now time.Time) float64 {
	var latestInsulinTime time.Time

	for _, t := range treatments {
		if !t.HasInsulin() || !t.IsBolus() {
			continue
		}

		treatTime := t.Time()
		if treatTime.Before(now) && treatTime.After(latestInsulinTime) {
			latestInsulinTime = treatTime
		}
	}

	if latestInsulinTime.IsZero() {
		return 0
	}

	diaMinutes := p.params.DIA * 60
	minutesAgo := now.Sub(latestInsulinTime).Minutes()
	remaining := diaMinutes - minutesAgo

	if remaining < 0 {
		return 0
	}

	return remaining
}

// calculateCOB calculates current Carbs on Board
func (p *Predictor) calculateCOB(treatments []models.Treatment, now time.Time) float64 {
	absorptionTime := p.params.TotalDailyCarbs / p.params.CarbAbsorptionRate * 60 // minutes for average meal
	if absorptionTime < 120 {
		absorptionTime = 120 // minimum 2 hours
	}
	if absorptionTime > 360 {
		absorptionTime = 360 // maximum 6 hours
	}

	var totalCOB float64

	for _, t := range treatments {
		if !t.HasCarbs() {
			continue
		}

		treatTime := t.Time()
		if treatTime.After(now) {
			continue
		}

		minutesAgo := now.Sub(treatTime).Minutes()
		if minutesAgo > absorptionTime {
			continue
		}

		// Linear absorption model
		absorbed := (minutesAgo / absorptionTime) * t.Carbs
		remaining := t.Carbs - absorbed
		if remaining > 0 {
			totalCOB += remaining
		}
	}

	return math.Round(totalCOB*10) / 10
}

// calculateCOBDuration calculates minutes until COB is absorbed
func (p *Predictor) calculateCOBDuration(cob float64) float64 {
	if cob <= 0 || p.params.CarbAbsorptionRate <= 0 {
		return 0
	}
	
	// Time in minutes to absorb remaining carbs
	hours := cob / p.params.CarbAbsorptionRate
	return hours * 60
}

// calculateInsulinEffect calculates the glucose-lowering effect of active insulin
func (p *Predictor) calculateInsulinEffect(treatments []models.Treatment, startTime, predTime time.Time) float64 {
	diaMinutes := p.params.DIA * 60
	var totalEffect float64

	period := models.GetTimeOfDayPeriod(predTime)
	isf := p.params.ISFByTimeOfDay[string(period)]
	if isf == 0 {
		isf = p.params.ISF
	}

	for _, t := range treatments {
		if !t.HasInsulin() || !t.IsBolus() {
			continue
		}

		treatTime := t.Time()
		if treatTime.After(startTime) {
			continue
		}

		minutesSinceTreatment := predTime.Sub(treatTime).Minutes()
		if minutesSinceTreatment > diaMinutes || minutesSinceTreatment < 0 {
			continue
		}

		// Calculate how much insulin effect occurs between start and pred time
		activityAtStart := p.insulinActivityRemaining(startTime.Sub(treatTime).Minutes())
		activityAtPred := p.insulinActivityRemaining(minutesSinceTreatment)

		// Effect is the difference in remaining activity times insulin dose times ISF
		activityUsed := activityAtStart - activityAtPred
		if activityUsed > 0 {
			totalEffect -= t.Insulin * activityUsed * isf
		}
	}

	return totalEffect
}

// calculateCarbEffect calculates the glucose-raising effect of active carbs
func (p *Predictor) calculateCarbEffect(treatments []models.Treatment, startTime, predTime time.Time) float64 {
	var totalEffect float64

	period := models.GetTimeOfDayPeriod(predTime)
	icr := p.params.ICRByTimeOfDay[string(period)]
	if icr == 0 {
		icr = p.params.ICR
	}
	isf := p.params.ISFByTimeOfDay[string(period)]
	if isf == 0 {
		isf = p.params.ISF
	}

	// Calculate carb sensitivity factor (how much 1g carbs raises BG)
	csf := isf / icr // mg/dL per gram of carbs

	absorptionTime := 180.0 // 3 hours default

	for _, t := range treatments {
		if !t.HasCarbs() {
			continue
		}

		treatTime := t.Time()
		minutesSinceTreatment := predTime.Sub(treatTime).Minutes()
		minutesSinceStart := startTime.Sub(treatTime).Minutes()

		if minutesSinceTreatment > absorptionTime || minutesSinceTreatment < 0 {
			continue
		}

		// Calculate carb absorption using a curve (peak at ~45 min)
		carbsAbsorbedByStart := p.carbsAbsorbed(t.Carbs, minutesSinceStart, absorptionTime)
		carbsAbsorbedByPred := p.carbsAbsorbed(t.Carbs, minutesSinceTreatment, absorptionTime)

		carbsAbsorbedInPeriod := carbsAbsorbedByPred - carbsAbsorbedByStart
		if carbsAbsorbedInPeriod > 0 {
			totalEffect += carbsAbsorbedInPeriod * csf
		}
	}

	return totalEffect
}

// calculateTrendEffect calculates the effect of current trend on prediction
func (p *Predictor) calculateTrendEffect(trendPer5Min float64, minutesOut float64) float64 {
	// Trend effect diminishes exponentially over time
	// Full effect for first 30 min, then decays
	if minutesOut <= 30 {
		return trendPer5Min * (minutesOut / 5)
	}

	// After 30 min, trend contribution decays
	effect30 := trendPer5Min * 6 // Full effect at 30 min
	decayFactor := math.Exp(-0.02 * (minutesOut - 30))
	additionalMinutes := minutesOut - 30
	additionalEffect := trendPer5Min * (additionalMinutes / 5) * decayFactor

	return effect30 + additionalEffect
}

// insulinActivityRemaining returns the fraction of insulin still active after given minutes
func (p *Predictor) insulinActivityRemaining(minutes float64) float64 {
	if minutes <= 0 {
		return 1.0
	}

	diaMinutes := p.params.DIA * 60
	if minutes >= diaMinutes {
		return 0
	}

	// Use a biexponential model for insulin action
	// Peak activity around 75 minutes, then decay
	peakTime := 75.0
	
	if minutes < peakTime {
		// Rising phase - activity builds up
		return 1 - (minutes/peakTime)*0.1 // Small amount used in rising phase
	}

	// Decay phase after peak
	remainingTime := diaMinutes - minutes
	totalDecayTime := diaMinutes - peakTime
	
	return 0.9 * (remainingTime / totalDecayTime)
}

// carbsAbsorbed returns the grams of carbs absorbed after given minutes
func (p *Predictor) carbsAbsorbed(totalCarbs float64, minutes float64, absorptionTime float64) float64 {
	if minutes <= 0 {
		return 0
	}

	if minutes >= absorptionTime {
		return totalCarbs
	}

	// Use a sigmoid-like curve for carb absorption
	// Slow start, faster in middle, slow at end
	progress := minutes / absorptionTime
	
	// Modified logistic function centered at 0.5
	absorbed := totalCarbs / (1 + math.Exp(-10*(progress-0.5)))

	return absorbed
}

// applyConstraints applies physiological constraints to predictions
func (p *Predictor) applyConstraints(value float64, minutesOut float64) float64 {
	// Glucose can't go below a realistic minimum
	if value < 20 {
		value = 20
	}

	// Cap at realistic maximum
	if value > 500 {
		value = 500
	}

	return value
}

// calculateConfidence calculates prediction confidence based on various factors
func (p *Predictor) calculateConfidence(minutesOut float64, highConfidenceMode bool, dataPoints int) float64 {
	// Base confidence starts at 90% and decays with time
	baseConfidence := 90.0
	if !highConfidenceMode {
		baseConfidence = 70.0
	}

	// Time decay: confidence drops faster as we predict further out
	timeDecay := math.Exp(-0.005 * minutesOut)

	// Data quality factor
	dataFactor := math.Min(1.0, float64(dataPoints)/50)

	// Parameter confidence factor
	paramFactor := (p.params.ISFConfidence + p.params.ICRConfidence + p.params.DIAConfidence) / 300

	confidence := baseConfidence * timeDecay * (0.5 + 0.5*dataFactor) * (0.5 + 0.5*paramFactor)

	return math.Max(10, math.Min(100, confidence))
}

// PredictWithScenario generates predictions with a hypothetical treatment
func (p *Predictor) PredictWithScenario(
	currentGlucose float64,
	currentTrend float64,
	recentEntries []models.GlucoseEntry,
	recentTreatments []models.Treatment,
	additionalInsulin float64,
	additionalCarbs float64,
) *models.PredictionResult {
	// Create a copy of treatments and add hypothetical treatment
	treatments := make([]models.Treatment, len(recentTreatments)+1)
	copy(treatments, recentTreatments)

	if additionalInsulin > 0 || additionalCarbs > 0 {
		hypothetical := models.Treatment{
			Date:      time.Now().UnixMilli(),
			EventType: "Hypothetical",
			Insulin:   additionalInsulin,
			Carbs:     additionalCarbs,
		}
		treatments[len(treatments)-1] = hypothetical
	}

	return p.Predict(currentGlucose, currentTrend, recentEntries, treatments)
}

// CalculateTrend calculates the current glucose trend from recent entries
func CalculateTrend(entries []models.GlucoseEntry) float64 {
	if len(entries) < 2 {
		return 0
	}

	// Sort by time descending (most recent first)
	sorted := make([]models.GlucoseEntry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Date > sorted[j].Date
	})

	// Use the last 3-5 readings for trend calculation (15-25 minutes)
	n := min(5, len(sorted))
	if n < 2 {
		return 0
	}

	// Linear regression for trend
	var sumX, sumY, sumXY, sumX2 float64
	baseTime := sorted[0].Date

	for i := 0; i < n; i++ {
		x := float64(baseTime-sorted[i].Date) / 60000 // minutes ago
		y := float64(sorted[i].SGV)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	nf := float64(n)
	denominator := nf*sumX2 - sumX*sumX
	if denominator == 0 {
		return 0
	}

	// Slope is the trend (mg/dL per minute)
	slope := (nf*sumXY - sumX*sumY) / denominator

	// Convert to per 5 minutes and return negative (since we measured time ago)
	return -slope * 5
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
