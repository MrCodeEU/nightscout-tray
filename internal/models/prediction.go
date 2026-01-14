// Package models contains data structures used throughout the application
package models

import (
	"encoding/json"
	"time"
)

// DiabetesParameters represents calculated diabetes management parameters
type DiabetesParameters struct {
	// Insulin Sensitivity Factor (ISF) - how much 1 unit of insulin lowers BG (in mg/dL)
	ISF float64 `json:"isf"`
	// ISF by time of day (morning, midday, evening, night)
	ISFByTimeOfDay map[string]float64 `json:"isfByTimeOfDay"`

	// Insulin-to-Carb Ratio (ICR) - grams of carbs per 1 unit of insulin
	ICR float64 `json:"icr"`
	// ICR by time of day
	ICRByTimeOfDay map[string]float64 `json:"icrByTimeOfDay"`

	// Duration of Insulin Action (DIA) in hours
	DIA float64 `json:"dia"`

	// Carb absorption rate (grams per hour)
	CarbAbsorptionRate float64 `json:"carbAbsorptionRate"`

	// Basal rate estimates (units per hour by time of day)
	BasalRateByTimeOfDay map[string]float64 `json:"basalRateByTimeOfDay"`

	// Average daily insulin usage
	TotalDailyInsulin float64 `json:"totalDailyInsulin"`
	BasalInsulin      float64 `json:"basalInsulin"`
	BolusInsulin      float64 `json:"bolusInsulin"`

	// Average daily carbs
	TotalDailyCarbs float64 `json:"totalDailyCarbs"`

	// Glucose statistics
	AverageGlucose  float64 `json:"averageGlucose"`
	GlucoseStdDev   float64 `json:"glucoseStdDev"`
	TimeInRange     float64 `json:"timeInRange"`     // Percentage 70-180 mg/dL
	TimeBelowRange  float64 `json:"timeBelowRange"`  // Percentage <70 mg/dL
	TimeAboveRange  float64 `json:"timeAboveRange"`  // Percentage >180 mg/dL
	GMI             float64 `json:"gmi"`             // Glucose Management Indicator (estimated HbA1c)
	CoefficientOfVariation float64 `json:"coefficientOfVariation"` // CV%

	// Confidence scores (0-100)
	ISFConfidence float64 `json:"isfConfidence"`
	ICRConfidence float64 `json:"icrConfidence"`
	DIAConfidence float64 `json:"diaConfidence"`

	// Data coverage
	DataDays        int       `json:"dataDays"`
	EntriesAnalyzed int       `json:"entriesAnalyzed"`
	TreatmentsAnalyzed int    `json:"treatmentsAnalyzed"`
	CalculatedAt    time.Time `json:"calculatedAt"`
}

// TimeOfDayPeriod represents different times of day
type TimeOfDayPeriod string

const (
	Morning TimeOfDayPeriod = "morning"  // 6:00 - 11:00
	Midday  TimeOfDayPeriod = "midday"   // 11:00 - 17:00
	Evening TimeOfDayPeriod = "evening"  // 17:00 - 22:00
	Night   TimeOfDayPeriod = "night"    // 22:00 - 6:00
)

// GetTimeOfDayPeriod returns the time of day period for a given time
func GetTimeOfDayPeriod(t time.Time) TimeOfDayPeriod {
	hour := t.Hour()
	switch {
	case hour >= 6 && hour < 11:
		return Morning
	case hour >= 11 && hour < 17:
		return Midday
	case hour >= 17 && hour < 22:
		return Evening
	default:
		return Night
	}
}

// PredictionResult contains glucose predictions
type PredictionResult struct {
	// Short-term predictions (more accurate, 1-2 hours)
	ShortTerm []PredictedPoint `json:"shortTerm"`
	
	// Long-term predictions (less accurate, 3-6 hours)
	LongTerm []PredictedPoint `json:"longTerm"`

	// Active insulin on board
	IOB float64 `json:"iob"`
	
	// Active carbs on board
	COB float64 `json:"cob"`

	// Predicted time until IOB is depleted
	IOBDuration float64 `json:"iobDuration"` // minutes

	// Predicted time until COB is absorbed
	COBDuration float64 `json:"cobDuration"` // minutes

	// Time until predicted high threshold crossing (0 if not predicted or already high)
	HighInMinutes float64 `json:"highInMinutes"`
	
	// Time until predicted low threshold crossing (0 if not predicted or already low)
	LowInMinutes float64 `json:"lowInMinutes"`
	
	// Target thresholds used for high/low prediction
	HighThreshold float64 `json:"highThreshold"` // mg/dL
	LowThreshold  float64 `json:"lowThreshold"`  // mg/dL

	// Prediction metadata
	PredictedAt     time.Time `json:"predictedAt"`
	BasedOnGlucose  float64   `json:"basedOnGlucose"` // Current glucose value used
	BasedOnTrend    float64   `json:"basedOnTrend"`   // Current trend (mg/dL per 5 min)
}

// PredictedPoint represents a single predicted glucose value
type PredictedPoint struct {
	Time       int64   `json:"time"`       // Unix timestamp in milliseconds
	Value      float64 `json:"value"`      // Predicted glucose in mg/dL
	ValueMmol  float64 `json:"valueMmol"`  // Predicted glucose in mmol/L
	Confidence float64 `json:"confidence"` // Confidence level (0-100)
	
	// Factors contributing to this prediction
	InsulinEffect float64 `json:"insulinEffect"` // Expected glucose change from insulin
	CarbEffect    float64 `json:"carbEffect"`    // Expected glucose change from carbs
	TrendEffect   float64 `json:"trendEffect"`   // Expected glucose change from current trend
}

// CalculationProgress represents the progress of parameter calculation
type CalculationProgress struct {
	Stage           string  `json:"stage"`           // Current stage name
	Progress        float64 `json:"progress"`        // 0-100 percentage
	EntriesProcessed int    `json:"entriesProcessed"`
	TotalEntries    int     `json:"totalEntries"`
	TreatmentsProcessed int `json:"treatmentsProcessed"`
	TotalTreatments int     `json:"totalTreatments"`
	EstimatedTimeRemaining float64 `json:"estimatedTimeRemaining"` // seconds
	StartedAt       time.Time `json:"startedAt"`
	Error           string  `json:"error,omitempty"`
}

// InsulinEvent represents an insulin dose with timing information
type InsulinEvent struct {
	Time     time.Time `json:"time"`
	Units    float64   `json:"units"`
	Type     string    `json:"type"` // "bolus" or "basal"
	EventID  string    `json:"eventId"`
}

// CarbEvent represents a carbohydrate intake with timing information
type CarbEvent struct {
	Time    time.Time `json:"time"`
	Grams   float64   `json:"grams"`
	EventID string    `json:"eventId"`
}

// GlucoseWithTreatments combines a glucose reading with nearby treatments
type GlucoseWithTreatments struct {
	Glucose         GlucoseEntry    `json:"glucose"`
	InsulinBefore   []InsulinEvent  `json:"insulinBefore"`  // Insulin in the previous DIA hours
	CarbsBefore     []CarbEvent     `json:"carbsBefore"`    // Carbs in the previous 4 hours
	GlucoseBefore   []GlucoseEntry  `json:"glucoseBefore"`  // Previous glucose readings
	GlucoseAfter    []GlucoseEntry  `json:"glucoseAfter"`   // Following glucose readings
}

// NewDiabetesParameters creates a new DiabetesParameters with default values
func NewDiabetesParameters() *DiabetesParameters {
	return &DiabetesParameters{
		ISF:                 50,    // Default: 1 unit lowers BG by 50 mg/dL
		ICR:                 10,    // Default: 1 unit covers 10g carbs
		DIA:                 4,     // Default: 4 hours insulin action
		CarbAbsorptionRate:  30,    // Default: 30g/hour
		ISFByTimeOfDay:      make(map[string]float64),
		ICRByTimeOfDay:      make(map[string]float64),
		BasalRateByTimeOfDay: make(map[string]float64),
	}
}

// MarshalJSON implements custom JSON marshaling
func (d *DiabetesParameters) MarshalJSON() ([]byte, error) {
	type Alias DiabetesParameters
	return json.Marshal(&struct {
		*Alias
		CalculatedAt string `json:"calculatedAt"`
	}{
		Alias:        (*Alias)(d),
		CalculatedAt: d.CalculatedAt.Format(time.RFC3339),
	})
}

// ToMmol converts a mg/dL value to mmol/L
func ToMmol(mgdl float64) float64 {
	return mgdl / 18.0182
}

// ToMgdl converts a mmol/L value to mg/dL
func ToMgdl(mmol float64) float64 {
	return mmol * 18.0182
}
