package tray

import (
	"math"
	"strings"
	"testing"
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

