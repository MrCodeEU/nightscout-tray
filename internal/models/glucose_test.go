package models

import (
	"testing"
)

func TestGlucoseEntry_TrendArrow(t *testing.T) {
	tests := []struct {
		name      string
		direction string
		trend     int
		expected  string
	}{
		{"DoubleUp direction", "DoubleUp", 0, "⇈"},
		{"SingleUp direction", "SingleUp", 0, "↑"},
		{"FortyFiveUp direction", "FortyFiveUp", 0, "↗"},
		{"Flat direction", "Flat", 0, "→"},
		{"FortyFiveDown direction", "FortyFiveDown", 0, "↘"},
		{"SingleDown direction", "SingleDown", 0, "↓"},
		{"DoubleDown direction", "DoubleDown", 0, "⇊"},
		{"Empty direction with trend 1", "", 1, "⇈"},
		{"Empty direction with trend 4", "", 4, "→"},
		{"Empty direction with trend 7", "", 7, "⇊"},
		{"Unknown direction", "Unknown", 0, "-"},
		{"NOT COMPUTABLE", "NOT COMPUTABLE", 0, "?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := &GlucoseEntry{
				Direction: tt.direction,
				Trend:     tt.trend,
			}
			result := entry.TrendArrow()
			if result != tt.expected {
				t.Errorf("TrendArrow() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGlucoseEntry_ValueMmolL(t *testing.T) {
	tests := []struct {
		name     string
		sgv      int
		expected float64
	}{
		{"100 mg/dL", 100, 5.55},
		{"180 mg/dL", 180, 9.99},
		{"70 mg/dL", 70, 3.89},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := &GlucoseEntry{SGV: tt.sgv}
			result := entry.ValueMmolL()
			if result < tt.expected-0.1 || result > tt.expected+0.1 {
				t.Errorf("ValueMmolL() = %f, want approximately %f", result, tt.expected)
			}
		})
	}
}

func TestGlucoseEntry_ValueMgDL(t *testing.T) {
	entry := &GlucoseEntry{SGV: 120}
	if entry.ValueMgDL() != 120 {
		t.Errorf("ValueMgDL() = %d, want 120", entry.ValueMgDL())
	}
}
