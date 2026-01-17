// Package models contains data structures used throughout the application
package models

import "time"

// GlucoseEntry represents a single glucose reading from Nightscout
type GlucoseEntry struct {
	ID        string `json:"_id"`
	SGV       int    `json:"sgv"`  // Sensor glucose value in mg/dL
	Date      int64  `json:"date"` // Unix timestamp in milliseconds
	DateStr   string `json:"dateString"`
	Trend     int    `json:"trend"`     // Trend direction (1-7)
	Direction string `json:"direction"` // Trend direction as string
	Device    string `json:"device"`
	Type      string `json:"type"`
	Mills     int64  `json:"mills"`
}

// Time returns the time of the glucose entry
func (g *GlucoseEntry) Time() time.Time {
	return time.UnixMilli(g.Date)
}

// ValueMgDL returns the glucose value in mg/dL
func (g *GlucoseEntry) ValueMgDL() int {
	return g.SGV
}

// ValueMmolL returns the glucose value in mmol/L
func (g *GlucoseEntry) ValueMmolL() float64 {
	return float64(g.SGV) / 18.0182
}

// TrendArrow returns the Unicode arrow character for the trend
func (g *GlucoseEntry) TrendArrow() string {
	arrows := map[string]string{
		"DoubleUp":          "⇈",
		"SingleUp":          "↑",
		"FortyFiveUp":       "↗",
		"Flat":              "→",
		"FortyFiveDown":     "↘",
		"SingleDown":        "↓",
		"DoubleDown":        "⇊",
		"NOT COMPUTABLE":    "?",
		"RATE OUT OF RANGE": "⚠",
	}

	if g.Direction != "" {
		if arrow, ok := arrows[g.Direction]; ok {
			return arrow
		}
	}

	// Fallback to numeric trend
	numericArrows := map[int]string{
		1: "⇈",
		2: "↑",
		3: "↗",
		4: "→",
		5: "↘",
		6: "↓",
		7: "⇊",
	}

	if arrow, ok := numericArrows[g.Trend]; ok {
		return arrow
	}

	return "-"
}

// GlucoseStatus represents the current glucose status for display
type GlucoseStatus struct {
	Value        int       `json:"value"`        // mg/dL
	ValueMmol    float64   `json:"valueMmol"`    // mmol/L
	Trend        string    `json:"trend"`        // Arrow character
	Direction    string    `json:"direction"`    // Direction string
	Time         time.Time `json:"time"`         // Reading time
	Delta        int       `json:"delta"`        // Change from previous reading
	Status       string    `json:"status"`       // "normal", "high", "low", "urgent_high", "urgent_low"
	StaleMinutes int       `json:"staleMinutes"` // Minutes since last reading
	IsStale      bool      `json:"isStale"`      // True if data is stale (>15 min)
	IOB          float64   `json:"iob"`          // Insulin on Board (units)
	COB          float64   `json:"cob"`          // Carbs on Board (grams)
}

// ChartData represents data for the glucose chart
type ChartData struct {
	Entries    []ChartEntry `json:"entries"`
	TargetLow  int          `json:"targetLow"`
	TargetHigh int          `json:"targetHigh"`
	UrgentLow  int          `json:"urgentLow"`
	UrgentHigh int          `json:"urgentHigh"`
	TimeRangeH int          `json:"timeRangeHours"`
	Unit       string       `json:"unit"` // "mg/dL" or "mmol/L"
}

// ChartEntry represents a single point on the chart
type ChartEntry struct {
	Time    int64   `json:"time"`    // Unix timestamp in milliseconds
	Value   float64 `json:"value"`   // Value in selected unit
	ValueMg int     `json:"valueMg"` // Original mg/dL value
	Status  string  `json:"status"`  // Status for coloring
}

// ServerStatus represents the Nightscout server status
type ServerStatus struct {
	Status            string         `json:"status"`
	Name              string         `json:"name"`
	Version           string         `json:"version"`
	ServerTime        string         `json:"serverTime"`
	APIEnabled        bool           `json:"apiEnabled"`
	CareportalEnabled bool           `json:"careportalEnabled"`
	Head              string         `json:"head"`
	Settings          ServerSettings `json:"settings,omitempty"`
}

// ServerSettings contains Nightscout server settings
type ServerSettings struct {
	Units           string     `json:"units"`
	TimeFormat      int        `json:"timeFormat"`
	NightMode       bool       `json:"nightMode"`
	Theme           string     `json:"theme"`
	Language        string     `json:"language"`
	ShowPlugins     string     `json:"showPlugins"`
	AlarmHigh       bool       `json:"alarmHigh"`
	AlarmLow        bool       `json:"alarmLow"`
	AlarmUrgentHigh bool       `json:"alarmUrgentHigh"`
	AlarmUrgentLow  bool       `json:"alarmUrgentLow"`
	Thresholds      Thresholds `json:"thresholds,omitempty"`
}

// Thresholds contains glucose threshold settings
type Thresholds struct {
	BGHigh         int `json:"bgHigh"`
	BGLow          int `json:"bgLow"`
	BGTargetTop    int `json:"bgTargetTop"`
	BGTargetBottom int `json:"bgTargetBottom"`
}
