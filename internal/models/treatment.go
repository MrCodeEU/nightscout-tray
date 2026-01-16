// Package models contains data structures used throughout the application
package models

import "time"

// Treatment represents a treatment entry from Nightscout (insulin, carbs, etc.)
type Treatment struct {
	ID          string  `json:"_id"`
	EventType   string  `json:"eventType"`
	Date        int64   `json:"date"`        // Unix timestamp in milliseconds
	DateStr     string  `json:"dateString"`
	CreatedAt   string  `json:"created_at"`
	Insulin     float64 `json:"insulin"`     // Units of insulin
	Carbs       float64 `json:"carbs"`       // Grams of carbohydrates
	Protein     float64 `json:"protein"`     // Grams of protein
	Fat         float64 `json:"fat"`         // Grams of fat
	Duration    float64 `json:"duration"`    // Duration in minutes (for temp basals, etc.)
	Glucose     float64 `json:"glucose"`     // Blood glucose value if recorded
	GlucoseType string  `json:"glucoseType"` // "Sensor", "Finger", "Manual"
	Units       string  `json:"units"`       // "mg/dl" or "mmol/l"
	Notes       string  `json:"notes"`
	EnteredBy   string  `json:"enteredBy"`
	Device      string  `json:"device"`
	
	// For basal changes
	Percent    float64 `json:"percent"`    // Basal change in percent
	Absolute   float64 `json:"absolute"`   // Basal change in absolute value
	
	// For temp targets
	TargetTop    float64 `json:"targetTop"`
	TargetBottom float64 `json:"targetBottom"`
	
	// For profile switches
	Profile string `json:"profile"`
	Reason  string `json:"reason"`
}

// Time returns the time of the treatment
func (t *Treatment) Time() time.Time {
	if t.Date > 0 {
		return time.UnixMilli(t.Date)
	}
	// Fallback to created_at
	parsed, err := time.Parse(time.RFC3339, t.CreatedAt)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

// HasInsulin returns true if this treatment includes insulin
func (t *Treatment) HasInsulin() bool {
	return t.Insulin > 0
}

// HasCarbs returns true if this treatment includes carbohydrates
func (t *Treatment) HasCarbs() bool {
	return t.Carbs > 0
}

// IsBolus returns true if this is a bolus treatment
func (t *Treatment) IsBolus() bool {
	bolusTypes := map[string]bool{
		"Bolus":            true,
		"Snack Bolus":      true,
		"Meal Bolus":       true,
		"Correction Bolus": true,
		"Combo Bolus":      true,
		"Bolus Wizard":     true,
	}
	return bolusTypes[t.EventType] || (t.HasInsulin() && t.EventType != "Temp Basal")
}

// IsMealBolus returns true if this appears to be a meal-related bolus
func (t *Treatment) IsMealBolus() bool {
	mealTypes := map[string]bool{
		"Meal Bolus":  true,
		"Snack Bolus": true,
	}
	return mealTypes[t.EventType] || (t.HasInsulin() && t.HasCarbs())
}

// TreatmentEventTypes contains common Nightscout event types
var TreatmentEventTypes = struct {
	BGCheck            string
	SnackBolus         string
	MealBolus          string
	CorrectionBolus    string
	CarbCorrection     string
	ComboBolus         string
	Announcement       string
	Note               string
	Question           string
	Exercise           string
	SiteChange         string
	SensorStart        string
	SensorChange       string
	PumpBatteryChange  string
	InsulinChange      string
	TempBasal          string
	ProfileSwitch      string
	DADAlert           string
	TemporaryTarget    string
	OpenAPSOffline     string
	BolusWizard        string
}{
	BGCheck:            "BG Check",
	SnackBolus:         "Snack Bolus",
	MealBolus:          "Meal Bolus",
	CorrectionBolus:    "Correction Bolus",
	CarbCorrection:     "Carb Correction",
	ComboBolus:         "Combo Bolus",
	Announcement:       "Announcement",
	Note:               "Note",
	Question:           "Question",
	Exercise:           "Exercise",
	SiteChange:         "Site Change",
	SensorStart:        "Sensor Start",
	SensorChange:       "Sensor Change",
	PumpBatteryChange:  "Pump Battery Change",
	InsulinChange:      "Insulin Change",
	TempBasal:          "Temp Basal",
	ProfileSwitch:      "Profile Switch",
	DADAlert:           "D.A.D. Alert",
	TemporaryTarget:    "Temporary Target",
	OpenAPSOffline:     "OpenAPS Offline",
	BolusWizard:        "Bolus Wizard",
}
