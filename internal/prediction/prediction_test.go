package prediction

import (
	"testing"
	"time"

	"github.com/mrcode/nightscout-tray/internal/models"
)

func TestPredictor_Predict(t *testing.T) {
	params := models.NewDiabetesParameters()
	params.DIA = 4.0
	params.ISF = 50.0
	params.ICR = 10.0

	predictor := NewPredictor(params)

	// Mock data
	now := time.Now()
	entries := []models.GlucoseEntry{
		{Date: now.UnixMilli(), SGV: 120, Direction: "Flat"},
		{Date: now.Add(-5 * time.Minute).UnixMilli(), SGV: 120, Direction: "Flat"},
		{Date: now.Add(-10 * time.Minute).UnixMilli(), SGV: 120, Direction: "Flat"},
	}
	treatments := []models.Treatment{}

	currentGlucose := 120.0
	currentTrend := 0.0

	result := predictor.Predict(currentGlucose, currentTrend, entries, treatments)

	if result == nil {
		t.Fatal("Prediction result is nil")
	}

	if len(result.ShortTerm) != 24 { // 2 hours / 5 min
		t.Errorf("Expected 24 short term points, got %d", len(result.ShortTerm))
	}

	if len(result.LongTerm) != 24 { // 6 hours / 15 min
		t.Errorf("Expected 24 long term points, got %d", len(result.LongTerm))
	}
}

func TestMLPredictor_Predict(t *testing.T) {
	params := models.NewDiabetesParameters()
	mlPredictor := NewMLPredictor(params)

	// We need enough data to train/predict
	entries := make([]models.GlucoseEntry, 50)
	now := time.Now()
	for i := 0; i < 50; i++ {
		entries[i] = models.GlucoseEntry{
			Date: now.Add(time.Duration(-i*5) * time.Minute).UnixMilli(),
			SGV:  120,
		}
	}
	
	// Ensure entries are sorted for Train/Predict if needed, but MLPredictor sorts them internally usually.
	// MLPredictor expects sorted old -> new in Train, but the input here is New -> Old.
	// Let's reverse to match what we created or let MLPredictor handle it.
	// Reading MLPredictor code:
	// Train sorts: sort.Slice(sorted, func(i, j int) bool { return sorted[i].Date < sorted[j].Date })
	// Predict sorts: same.
	// So order doesn't matter for input slice.

	// Train first (implicit in Predict but let's be explicit if needed or just call Predict)
	// Predict calls Train if not trained.

	res, err := mlPredictor.Predict(entries)
	if err != nil {
		t.Fatalf("MLPredictor Predict failed: %v", err)
	}

	if res == nil {
		t.Fatal("Prediction result is nil")
	}

	if len(res.Points) != 6 { // PredHorizon is 6
		t.Errorf("Expected 6 points, got %d", len(res.Points))
	}
}

func TestAnalyzer_AnalyzeData(t *testing.T) {
	analyzer := NewAnalyzer()
	
	entries := []models.GlucoseEntry{
		{Date: 1000, SGV: 100},
		{Date: 2000, SGV: 110},
		{Date: 3000, SGV: 120},
	}
	treatments := []models.Treatment{
		{Date: 1000, Insulin: 1.0, Carbs: 10},
	}

	params, err := analyzer.AnalyzeData(entries, treatments)
	if err != nil {
		t.Fatalf("AnalyzeData failed: %v", err)
	}

	if params == nil {
		t.Fatal("Params is nil")
	}

	if params.EntriesAnalyzed != 3 {
		t.Errorf("Expected 3 entries analyzed, got %d", params.EntriesAnalyzed)
	}
}
