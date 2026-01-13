package tray

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/mrcode/nightscout-tray/internal/models"
)

func TestGenerateMultiLineSparkline(t *testing.T) {
	// Create a dummy Icon with history
	icon := &Icon{
		history: []float64{100, 110, 120, 130, 140, 150, 140, 130, 120, 110, 100},
	}

	// Call the private method
	chart := icon.generateMultiLineSparkline()

	if chart == "" {
		t.Error("Expected chart to be generated, got empty string")
	}

	// Check for Braille characters
	brailleChars := "⠀⣀⣤⣶⣿"
	containsBraille := false
	for _, r := range chart {
		if strings.ContainsRune(brailleChars, r) {
			containsBraille = true
			break
		}
	}

	if !containsBraille {
		t.Error("Expected chart to contain Braille characters")
	}

	// Print chart for manual inspection in logs
	t.Logf("Generated Chart:\n%s", chart)
}

func TestGenerateMultiLineSparkline_Rounding(t *testing.T) {
    // Test specific values to verify rounding logic
    // We reduced buffer to 10.
    // minVal will be min(history) - 10
    // maxVal will be max(history) + 10
    
    // Case 1: Value exactly in the middle of a block range
    // If we have history [100, 100], buffer 10.
    // Min = 90, Max = 110. Range = 20.
    // normalized = (100 - 90) / 20 = 0.5
    // Height = 10. SubBlocks = 4. Total = 10 * 4 = 40.
    // totalSubBlocks = 0.5 * 40 = 20.
    // Line 0 (bottom): range 0-4. 20 >= 4 -> Full block '⣿'
    // Line 1: range 4-8. 20 >= 8 -> Full block '⣿'
    // ...
    // Line 4: range 16-20. 20 >= 20 -> Full block '⣿'
    // Line 5: range 20-24. 20 is start. Remainder = 20 - 20 = 0. Empty '⠀' ?
    // Wait, let's trace the loop.
    // y=0 (bottom). lineIdx=9. lineStart=0. lineEnd=4. totalSubBlocks=20. 20>=4 -> Full.
    // ...
    // y=4. lineIdx=5. lineStart=16. lineEnd=20. 20>=20 -> Full.
    // y=5. lineIdx=4. lineStart=20. lineEnd=24. 20>20 is false? No, 20 > 20 is false.
    // It says `if totalSubBlocks >= lineEnd { ... } else if totalSubBlocks > lineStart { ... }`
    // So for y=5, 20 >= 24 (False). 20 > 20 (False). So it stays Empty. Correct.
    
    icon := &Icon{
        history: []float64{100, 100},
    }
    
    chart := icon.generateMultiLineSparkline()
    if chart == "" {
        t.Fatal("Chart empty")
    }
    
    // We expect the chart to correspond to the logic. 
    // This test primarily ensures no panic and basic valid output with the new rounding logic.
    t.Logf("Rounding Test Chart:\n%s", chart)
}

func TestGenerateMultiLineSparkline_SineWave(t *testing.T) {
	// Generate a sine wave
	var history []float64
	for i := 0; i < 24; i++ {
		// one full cycle over 24 points
		val := 100 + 50*math.Sin(float64(i)*2*math.Pi/24)
		history = append(history, val)
	}

	icon := &Icon{
		history: history,
	}

	chart := icon.generateMultiLineSparkline()
	if chart == "" {
		t.Fatal("Chart empty")
	}

	t.Logf("Sine Wave Chart:\n%s", chart)
}

func TestGenerateCompactSparkline(t *testing.T) {
	// Create an Icon with varying history
	icon := &Icon{
		history: []float64{100, 110, 120, 130, 140, 150, 140, 130, 120, 110, 100},
	}

	sparkline := icon.generateCompactSparkline()
	if sparkline == "" {
		t.Error("Expected sparkline to be generated, got empty string")
	}

	// Should have 2 lines (top + bottom) separated by newline
	lines := strings.Split(sparkline, "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines in sparkline, got %d", len(lines))
	}

	// Both lines should have same length as history
	for i, line := range lines {
		if len([]rune(line)) != len(icon.history) {
			t.Errorf("Line %d: expected length %d, got %d", i, len(icon.history), len([]rune(line)))
		}
	}

	// Check for sparkline block characters
	blockChars := "▁▄▀█ "
	for _, line := range lines {
		for _, r := range line {
			if !strings.ContainsRune(blockChars, r) {
				t.Errorf("Unexpected character in sparkline: %c", r)
			}
		}
	}

	t.Logf("Compact Sparkline:\n%s", sparkline)
}

func TestGenerateCompactSparkline_Empty(t *testing.T) {
	icon := &Icon{
		history: []float64{},
	}

	sparkline := icon.generateCompactSparkline()
	if sparkline != "" {
		t.Error("Expected empty sparkline for empty history")
	}
}

func TestGenerateCompactSparkline_SingleValue(t *testing.T) {
	icon := &Icon{
		history: []float64{100},
	}

	sparkline := icon.generateCompactSparkline()
	if sparkline != "" {
		t.Error("Expected empty sparkline for single value history")
	}
}

func TestWindowsTooltipLength(t *testing.T) {
	// Test that Windows tooltip stays under 128 character limit
	settings := &models.Settings{
		Unit:       "mg/dL",
		TargetLow:  70,
		TargetHigh: 180,
		UrgentLow:  55,
		UrgentHigh: 250,
	}

	icon := &Icon{
		settings: settings,
		history:  []float64{100, 110, 120, 130, 140, 150, 140, 130, 120, 110, 100, 105},
	}

	status := &models.GlucoseStatus{
		Value:        125,
		ValueMmol:    6.9,
		Trend:        "↑",
		Direction:    "FortyFiveUp",
		Time:         time.Now().Add(-3 * time.Minute),
		Status:       "normal",
		StaleMinutes: 3,
		IsStale:      false,
	}

	icon.lastStatus = status

	// Generate compact sparkline
	sparkline := icon.generateCompactSparkline()

	// Build Windows-style tooltip (mimicking the actual code)
	var tooltip string
	if sparkline != "" {
		tooltip = "125mg/dL ↑\n" + sparkline + "\n✓OK 3m"
	} else {
		tooltip = "125mg/dL ↑\n✓OK 3m"
	}

	// Test with stale warning
	tooltipWithWarning := "125mg/dL ↑\n" + sparkline + "\n✓OK 3m ⚠"

	tooltipLen := len([]rune(tooltip))
	tooltipWithWarningLen := len([]rune(tooltipWithWarning))

	t.Logf("Tooltip length: %d chars", tooltipLen)
	t.Logf("Tooltip with warning length: %d chars", tooltipWithWarningLen)
	t.Logf("Tooltip:\n%s", tooltip)

	if tooltipLen > 128 {
		t.Errorf("Tooltip exceeds 128 character limit: %d chars", tooltipLen)
	}

	if tooltipWithWarningLen > 128 {
		t.Errorf("Tooltip with warning exceeds 128 character limit: %d chars", tooltipWithWarningLen)
	}
}

func TestWindowsTooltipLength_LongDuration(t *testing.T) {
	// Test with longer time durations
	settings := &models.Settings{
		Unit: "mmol/L",
	}

	icon := &Icon{
		settings: settings,
		history:  []float64{5.0, 5.5, 6.0, 6.5, 7.0, 7.5, 8.0, 7.5, 7.0, 6.5, 6.0},
	}

	status := &models.GlucoseStatus{
		Value:        125,
		ValueMmol:    6.9,
		Trend:        "→",
		Direction:    "Flat",
		Time:         time.Now().Add(-45 * time.Minute),
		Status:       "normal",
		StaleMinutes: 45,
		IsStale:      true,
	}

	icon.lastStatus = status
	sparkline := icon.generateCompactSparkline()

	tooltip := "6.9mmol/L →\n" + sparkline + "\n✓OK 45m ⚠"
	tooltipLen := len([]rune(tooltip))

	t.Logf("Tooltip length (long duration): %d chars", tooltipLen)
	t.Logf("Tooltip:\n%s", tooltip)

	if tooltipLen > 128 {
		t.Errorf("Tooltip with long duration exceeds 128 character limit: %d chars", tooltipLen)
	}
}

func TestCompactFormatting(t *testing.T) {
	icon := &Icon{}

	// Test compact status formatting
	tests := []struct {
		status   string
		expected string
	}{
		{"urgent_low", "🔻URGENT"},
		{"urgent_high", "🔺URGENT"},
		{"low", "↓Low"},
		{"high", "↑High"},
		{"normal", "✓OK"},
	}

	for _, tt := range tests {
		result := icon.formatCompactStatus(tt.status)
		if result != tt.expected {
			t.Errorf("formatCompactStatus(%s) = %s, want %s", tt.status, result, tt.expected)
		}
	}

	// Test compact duration formatting
	durationTests := []struct {
		minutes  int
		expected string
	}{
		{0, "now"},
		{1, "1m"},
		{30, "30m"},
		{59, "59m"},
		{60, "1h"},
		{120, "2h"},
		{180, "3h"},
	}

	for _, tt := range durationTests {
		result := icon.formatCompactDuration(tt.minutes)
		if result != tt.expected {
			t.Errorf("formatCompactDuration(%d) = %s, want %s", tt.minutes, result, tt.expected)
		}
	}
}

