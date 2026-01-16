// Package prediction provides glucose prediction and diabetes parameter calculation
package prediction

import (
	"math"
	"sort"
	"time"

	"github.com/mrcode/nightscout-tray/internal/models"
)

// MLPredictor implements sequence-based glucose prediction inspired by research
// on LSTM/GRU neural networks for blood glucose forecasting.
//
// Based on research: "Blood Glucose Prediction Using Deep Learning and Reinforcement Learning"
// Key insights:
// - Use 30 minutes of input data (6 points at 5-min intervals)
// - Predict 30 minutes ahead (6 points)
// - Normalize values between -1 and 1
// - Ensemble multiple prediction methods (voting)
type MLPredictor struct {
	params     *models.DiabetesParameters
	history    *SequenceHistory
	patterns   *PatternLibrary
}

// SequenceHistory stores recent glucose sequences for pattern matching
type SequenceHistory struct {
	sequences []GlucoseSequence
	maxAge    time.Duration
}

// GlucoseSequence represents a sequence of glucose readings with context
type GlucoseSequence struct {
	InputValues    [6]float64    // 6 readings at 5-min intervals (30 min of history)
	OutputValues   [6]float64    // Next 6 readings (30 min ahead) - for training
	Timestamp      time.Time     // When this sequence was recorded
	IOBAtTime      float64       // Insulin on board at sequence start
	COBAtTime      float64       // Carbs on board at sequence start
	TimeOfDay      float64       // Normalized time of day (0-1)
	RecentInsulin  float64       // Insulin given in last 2 hours
	RecentCarbs    float64       // Carbs eaten in last 2 hours
	TrendVelocity  float64       // Rate of change (mg/dL per 5 min)
	TrendAccel     float64       // Acceleration of change
}

// PatternLibrary stores learned patterns from historical data
type PatternLibrary struct {
	patterns     []LearnedPattern
	clusterCount int
}

// LearnedPattern represents a learned glucose response pattern
type LearnedPattern struct {
	InputPattern    [6]float64 // Normalized input sequence
	OutputPattern   [6]float64 // Resulting output sequence
	ContextIOB      float64    // Typical IOB for this pattern
	ContextCOB      float64    // Typical COB for this pattern
	Count           int        // How many times this pattern was seen
	MeanError       float64    // Average prediction error for this pattern
	Weight          float64    // Pattern weight based on recency and accuracy
}

// Constants for normalization (from research paper)
const (
	glucoseMin = 20.0  // Minimum glucose for normalization
	glucoseMax = 420.0 // Maximum glucose for normalization
	
	// Sequence parameters
	sequenceInputLen  = 6  // 30 minutes at 5-min intervals
	sequenceOutputLen = 6  // Predict 30 minutes ahead
	intervalMinutes   = 5  // 5 minutes between readings
	
	// Pattern matching parameters
	maxPatterns       = 1000 // Maximum patterns to store
	similarityThreshold = 0.85 // Minimum similarity for pattern match
)

// NewMLPredictor creates a new ML-based predictor
func NewMLPredictor(params *models.DiabetesParameters) *MLPredictor {
	if params == nil {
		params = models.NewDiabetesParameters()
	}
	return &MLPredictor{
		params: params,
		history: &SequenceHistory{
			sequences: make([]GlucoseSequence, 0, 10000),
			maxAge:    30 * 24 * time.Hour, // Keep 30 days of sequences
		},
		patterns: &PatternLibrary{
			patterns:     make([]LearnedPattern, 0, maxPatterns),
			clusterCount: 50, // Number of pattern clusters
		},
	}
}

// SetParameters updates the prediction parameters
func (m *MLPredictor) SetParameters(params *models.DiabetesParameters) {
	m.params = params
}

// LearnFromHistory builds the pattern library from historical data
func (m *MLPredictor) LearnFromHistory(entries []models.GlucoseEntry, treatments []models.Treatment) {
	if len(entries) < 20 {
		return
	}

	// Sort entries by time
	sorted := make([]models.GlucoseEntry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Date < sorted[j].Date
	})

	// Build sequences from historical data
	m.history.sequences = m.buildSequences(sorted, treatments)
	
	// Cluster sequences into patterns
	m.clusterPatterns()
}

// buildSequences creates training sequences from historical data
func (m *MLPredictor) buildSequences(entries []models.GlucoseEntry, treatments []models.Treatment) []GlucoseSequence {
	var sequences []GlucoseSequence
	
	// Need at least 12 entries for input + output (30 min history + 30 min outcome)
	if len(entries) < 12 {
		return sequences
	}

	// Create index for fast treatment lookup
	treatmentsByTime := m.indexTreatments(treatments)

	// Slide through entries creating sequences
	for i := 0; i <= len(entries)-12; i++ {
		// Check if readings are roughly 5 minutes apart
		if !m.isValidSequence(entries[i:i+12]) {
			continue
		}

		seq := GlucoseSequence{
			Timestamp: entries[i].Time(),
		}

		// Fill input values (normalized)
		for j := 0; j < 6; j++ {
			seq.InputValues[j] = normalizeGlucose(float64(entries[i+j].SGV))
		}

		// Fill output values (normalized) - what actually happened next
		for j := 0; j < 6; j++ {
			seq.OutputValues[j] = normalizeGlucose(float64(entries[i+6+j].SGV))
		}

		// Calculate context at sequence start
		seqTime := entries[i].Time()
		seq.TimeOfDay = float64(seqTime.Hour()*60+seqTime.Minute()) / 1440.0
		
		// Calculate IOB and COB at this time
		seq.IOBAtTime = m.calculateIOBAt(treatmentsByTime, seqTime)
		seq.COBAtTime = m.calculateCOBAt(treatmentsByTime, seqTime)
		
		// Recent insulin and carbs (last 2 hours)
		seq.RecentInsulin = m.sumRecentInsulin(treatmentsByTime, seqTime, 2*time.Hour)
		seq.RecentCarbs = m.sumRecentCarbs(treatmentsByTime, seqTime, 2*time.Hour)

		// Calculate velocity (trend) and acceleration
		seq.TrendVelocity = (seq.InputValues[5] - seq.InputValues[4]) // Last 5-min change (normalized)
		seq.TrendAccel = (seq.InputValues[5] - seq.InputValues[4]) - (seq.InputValues[4] - seq.InputValues[3])

		sequences = append(sequences, seq)
	}

	return sequences
}

// isValidSequence checks if entries are roughly 5 minutes apart
func (m *MLPredictor) isValidSequence(entries []models.GlucoseEntry) bool {
	for i := 1; i < len(entries); i++ {
		gap := entries[i].Time().Sub(entries[i-1].Time()).Minutes()
		// Allow 3-7 minute gaps (some flexibility for real-world data)
		if gap < 3 || gap > 7 {
			return false
		}
	}
	return true
}

// clusterPatterns groups similar sequences into pattern clusters
func (m *MLPredictor) clusterPatterns() {
	if len(m.history.sequences) == 0 {
		return
	}

	// Use k-means-like clustering to find representative patterns
	// For simplicity, we'll use a greedy approach: add patterns that are distinct
	
	m.patterns.patterns = make([]LearnedPattern, 0, maxPatterns)

	for _, seq := range m.history.sequences {
		// Find most similar existing pattern
		bestMatch := -1
		bestSim := 0.0

		for i, p := range m.patterns.patterns {
			sim := m.sequenceSimilarity(seq.InputValues, p.InputPattern, seq.IOBAtTime, p.ContextIOB)
			if sim > bestSim {
				bestSim = sim
				bestMatch = i
			}
		}

		if bestSim >= similarityThreshold && bestMatch >= 0 {
			// Merge into existing pattern (running average)
			m.mergePattern(bestMatch, seq)
		} else if len(m.patterns.patterns) < maxPatterns {
			// Add as new pattern
			newPattern := LearnedPattern{
				InputPattern:  seq.InputValues,
				OutputPattern: seq.OutputValues,
				ContextIOB:    seq.IOBAtTime,
				ContextCOB:    seq.COBAtTime,
				Count:         1,
				Weight:        1.0,
			}
			m.patterns.patterns = append(m.patterns.patterns, newPattern)
		}
	}
}

// mergePattern updates an existing pattern with new data (exponential moving average)
func (m *MLPredictor) mergePattern(idx int, seq GlucoseSequence) {
	p := &m.patterns.patterns[idx]
	alpha := 0.1 // Learning rate

	for i := 0; i < 6; i++ {
		p.OutputPattern[i] = (1-alpha)*p.OutputPattern[i] + alpha*seq.OutputValues[i]
	}
	p.ContextIOB = (1-alpha)*p.ContextIOB + alpha*seq.IOBAtTime
	p.ContextCOB = (1-alpha)*p.ContextCOB + alpha*seq.COBAtTime
	p.Count++
}

// sequenceSimilarity calculates similarity between two input sequences
func (m *MLPredictor) sequenceSimilarity(a, b [6]float64, iobA, iobB float64) float64 {
	// Cosine similarity for sequence shape
	var dotProduct, normA, normB float64
	for i := 0; i < 6; i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	shapeSim := dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
	
	// IOB similarity (important context)
	iobDiff := math.Abs(iobA - iobB)
	iobSim := math.Exp(-iobDiff / 2.0) // Exponential decay for IOB difference

	// Combined similarity (weighted)
	return 0.8*shapeSim + 0.2*iobSim
}

// PredictML generates predictions using the ML approach
func (m *MLPredictor) PredictML(
	currentGlucose float64,
	recentEntries []models.GlucoseEntry,
	recentTreatments []models.Treatment,
	highThreshold float64,
	lowThreshold float64,
) *models.PredictionResult {
	now := time.Now()

	// Build current input sequence
	inputSeq := m.buildCurrentSequence(recentEntries)
	
	// Calculate current context
	iob := m.calculateIOB(recentTreatments, now)
	cob := m.calculateCOB(recentTreatments, now)

	// Get predictions from multiple methods (ensemble)
	predictions := m.ensemblePredict(inputSeq, iob, cob, recentEntries, recentTreatments)

	// Build result
	result := &models.PredictionResult{
		PredictedAt:    now,
		BasedOnGlucose: currentGlucose,
		HighThreshold:  highThreshold,
		LowThreshold:   lowThreshold,
		IOB:            iob,
		COB:            cob,
	}

	// Convert predictions to PredictedPoints
	result.ShortTerm = m.predictionsToPoints(predictions[:6], now, true)
	
	// For long-term, extrapolate with decay
	result.LongTerm = m.extendPredictions(predictions, now, recentTreatments)

	// Calculate time to thresholds
	result.HighInMinutes, result.LowInMinutes = m.calculateThresholdTimes(
		result.ShortTerm, result.LongTerm, highThreshold, lowThreshold,
	)

	return result
}

// buildCurrentSequence creates an input sequence from recent entries
func (m *MLPredictor) buildCurrentSequence(entries []models.GlucoseEntry) [6]float64 {
	var seq [6]float64
	
	if len(entries) == 0 {
		return seq
	}

	// Sort entries by time (newest first typically, so reverse for chronological)
	sorted := make([]models.GlucoseEntry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Date < sorted[j].Date
	})

	// Take the last 6 entries within 30-35 minutes
	now := time.Now()
	var recent []models.GlucoseEntry
	for i := len(sorted) - 1; i >= 0 && len(recent) < 12; i-- {
		age := now.Sub(sorted[i].Time()).Minutes()
		if age <= 35 {
			recent = append([]models.GlucoseEntry{sorted[i]}, recent...)
		}
	}

	// Fill the sequence (may have gaps)
	if len(recent) >= 6 {
		// Use the last 6
		for i := 0; i < 6; i++ {
			idx := len(recent) - 6 + i
			seq[i] = normalizeGlucose(float64(recent[idx].SGV))
		}
	} else if len(recent) > 0 {
		// Interpolate to fill gaps
		for i := 0; i < 6; i++ {
			progress := float64(i) / 5.0
			idx := int(progress * float64(len(recent)-1))
			seq[i] = normalizeGlucose(float64(recent[idx].SGV))
		}
	}

	return seq
}

// ensemblePredict combines multiple prediction methods
func (m *MLPredictor) ensemblePredict(
	inputSeq [6]float64,
	iob, cob float64,
	entries []models.GlucoseEntry,
	treatments []models.Treatment,
) [12]float64 {
	var predictions [12]float64
	
	// Method 1: Pattern matching (like k-NN)
	patternPred := m.patternMatchPredict(inputSeq, iob, cob)
	
	// Method 2: Trend extrapolation with physiological model
	trendPred := m.trendModelPredict(inputSeq, iob, cob, treatments)
	
	// Method 3: Momentum-based prediction
	momentumPred := m.momentumPredict(inputSeq)

	// Voting: weighted average of methods
	// Weights based on confidence in each method
	patternWeight := m.getPatternConfidence(inputSeq, iob)
	trendWeight := 0.3
	momentumWeight := 0.2

	totalWeight := patternWeight + trendWeight + momentumWeight

	for i := 0; i < 12; i++ {
		predictions[i] = (patternWeight*patternPred[i] + trendWeight*trendPred[i] + momentumWeight*momentumPred[i]) / totalWeight
	}

	return predictions
}

// patternMatchPredict finds similar historical patterns and uses their outcomes
func (m *MLPredictor) patternMatchPredict(inputSeq [6]float64, iob, cob float64) [12]float64 {
	var predictions [12]float64
	
	if len(m.patterns.patterns) == 0 {
		// No patterns - just return linear extrapolation
		return m.linearExtrapolate(inputSeq)
	}

	// Find top k most similar patterns
	type patternScore struct {
		pattern    *LearnedPattern
		similarity float64
	}
	
	var matches []patternScore
	for i := range m.patterns.patterns {
		sim := m.sequenceSimilarity(inputSeq, m.patterns.patterns[i].InputPattern, iob, m.patterns.patterns[i].ContextIOB)
		if sim > 0.5 { // Minimum threshold
			matches = append(matches, patternScore{&m.patterns.patterns[i], sim})
		}
	}

	if len(matches) == 0 {
		return m.linearExtrapolate(inputSeq)
	}

	// Sort by similarity
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].similarity > matches[j].similarity
	})

	// Take top 5 matches
	k := 5
	if len(matches) < k {
		k = len(matches)
	}

	// Weighted average of outputs
	var totalWeight float64
	for i := 0; i < k; i++ {
		weight := matches[i].similarity * float64(matches[i].pattern.Count)
		for j := 0; j < 6; j++ {
			predictions[j] += weight * matches[i].pattern.OutputPattern[j]
		}
		totalWeight += weight
	}

	for i := 0; i < 6; i++ {
		predictions[i] /= totalWeight
	}

	// Extend to 12 points with decay towards last known value
	for i := 6; i < 12; i++ {
		decay := 0.9
		predictions[i] = predictions[5] + (predictions[5]-predictions[4])*decay
	}

	return predictions
}

// trendModelPredict uses current trend with insulin/carb effects
func (m *MLPredictor) trendModelPredict(inputSeq [6]float64, iob, cob float64, treatments []models.Treatment) [12]float64 {
	var predictions [12]float64
	
	// Calculate current trend (normalized)
	trend := inputSeq[5] - inputSeq[4]
	
	// ISF and ICR effects
	isf := m.params.ISF
	icr := m.params.ICR
	if icr == 0 {
		icr = 10
	}
	csf := isf / icr // Carb sensitivity factor

	// Predict each point
	prev := inputSeq[5]
	for i := 0; i < 12; i++ {
		minutesAhead := float64((i + 1) * 5)
		
		// Trend effect with decay
		trendDecay := math.Exp(-0.02 * minutesAhead)
		trendEffect := (trend / (glucoseMax - glucoseMin) * 2) * trendDecay
		
		// Insulin effect (normalized)
		insulinEffect := 0.0
		if iob > 0 {
			// Insulin is most active around 75 minutes
			insulinActivity := m.insulinActivityCurve(minutesAhead)
			insulinEffect = -(iob * insulinActivity * isf) / (glucoseMax - glucoseMin) * 2
		}

		// Carb effect (normalized)
		carbEffect := 0.0
		if cob > 0 {
			// Carbs peak around 45 minutes
			carbActivity := m.carbActivityCurve(minutesAhead)
			carbEffect = (cob * carbActivity * csf) / (glucoseMax - glucoseMin) * 2
		}

		predictions[i] = prev + trendEffect + insulinEffect + carbEffect
		
		// Clamp to valid range
		if predictions[i] < -1 {
			predictions[i] = -1
		}
		if predictions[i] > 1 {
			predictions[i] = 1
		}

		prev = predictions[i]
	}

	return predictions
}

// momentumPredict uses momentum/acceleration for prediction
func (m *MLPredictor) momentumPredict(inputSeq [6]float64) [12]float64 {
	var predictions [12]float64
	
	// Calculate velocity and acceleration
	v1 := inputSeq[5] - inputSeq[4]
	v0 := inputSeq[4] - inputSeq[3]
	accel := v1 - v0

	// Use damped harmonic motion model
	velocity := v1
	pos := inputSeq[5]
	damping := 0.85 // Velocity damping factor

	for i := 0; i < 12; i++ {
		velocity = velocity*damping + accel*0.1
		pos = pos + velocity
		
		// Apply constraints
		if pos < -1 {
			pos = -1
			velocity = 0
		}
		if pos > 1 {
			pos = 1
			velocity = 0
		}

		predictions[i] = pos
	}

	return predictions
}

// linearExtrapolate does simple linear extrapolation
func (m *MLPredictor) linearExtrapolate(inputSeq [6]float64) [12]float64 {
	var predictions [12]float64
	
	trend := inputSeq[5] - inputSeq[4]
	
	for i := 0; i < 12; i++ {
		predictions[i] = inputSeq[5] + trend*float64(i+1)*0.5 // Damped trend
		if predictions[i] < -1 {
			predictions[i] = -1
		}
		if predictions[i] > 1 {
			predictions[i] = 1
		}
	}

	return predictions
}

// getPatternConfidence returns confidence in pattern matching
func (m *MLPredictor) getPatternConfidence(inputSeq [6]float64, iob float64) float64 {
	if len(m.patterns.patterns) < 10 {
		return 0.2 // Low confidence with few patterns
	}

	// Find best match similarity
	bestSim := 0.0
	for i := range m.patterns.patterns {
		sim := m.sequenceSimilarity(inputSeq, m.patterns.patterns[i].InputPattern, iob, m.patterns.patterns[i].ContextIOB)
		if sim > bestSim {
			bestSim = sim
		}
	}

	// Scale confidence based on match quality and pattern count
	patternBonus := math.Min(1.0, float64(len(m.patterns.patterns))/500)
	return 0.3 + 0.4*bestSim + 0.1*patternBonus
}

// predictionsToPoints converts normalized predictions to PredictedPoints
func (m *MLPredictor) predictionsToPoints(predictions []float64, startTime time.Time, highConfidence bool) []models.PredictedPoint {
	points := make([]models.PredictedPoint, len(predictions))

	for i, p := range predictions {
		predTime := startTime.Add(time.Duration(i+1) * 5 * time.Minute)
		glucose := denormalizeGlucose(p)

		confidence := 90.0 - float64(i)*5
		if !highConfidence {
			confidence = 70.0 - float64(i)*5
		}
		if confidence < 20 {
			confidence = 20
		}

		points[i] = models.PredictedPoint{
			Time:       predTime.UnixMilli(),
			Value:      math.Round(glucose*10) / 10,
			ValueMmol:  models.ToMmol(glucose),
			Confidence: confidence,
		}
	}

	return points
}

// extendPredictions extends short-term predictions to long-term
func (m *MLPredictor) extendPredictions(predictions [12]float64, startTime time.Time, treatments []models.Treatment) []models.PredictedPoint {
	// Long-term: every 15 minutes for 6 hours (24 points)
	var longTerm []models.PredictedPoint

	lastNorm := predictions[11]
	trend := predictions[11] - predictions[10]

	for i := 0; i < 24; i++ {
		minutesAhead := (i + 1) * 15
		predTime := startTime.Add(time.Duration(minutesAhead) * time.Minute)

		// Damped extrapolation
		decay := math.Exp(-0.01 * float64(minutesAhead))
		predicted := lastNorm + trend*float64(i+1)*0.3*decay

		if predicted < -1 {
			predicted = -1
		}
		if predicted > 1 {
			predicted = 1
		}

		glucose := denormalizeGlucose(predicted)
		confidence := 50.0 - float64(i)*1.5
		if confidence < 10 {
			confidence = 10
		}

		longTerm = append(longTerm, models.PredictedPoint{
			Time:       predTime.UnixMilli(),
			Value:      math.Round(glucose*10) / 10,
			ValueMmol:  models.ToMmol(glucose),
			Confidence: confidence,
		})

		lastNorm = predicted
	}

	return longTerm
}

// calculateThresholdTimes calculates time until high/low thresholds
func (m *MLPredictor) calculateThresholdTimes(
	shortTerm, longTerm []models.PredictedPoint,
	highThreshold, lowThreshold float64,
) (highIn, lowIn float64) {
	highIn = -1
	lowIn = -1

	now := time.Now().UnixMilli()
	allPoints := append(shortTerm, longTerm...)

	for i, p := range allPoints {
		minutes := float64(p.Time-now) / 60000

		// Check high threshold
		if highIn < 0 && p.Value >= highThreshold {
			if i > 0 {
				// Interpolate
				prev := allPoints[i-1]
				if prev.Value < highThreshold {
					ratio := (highThreshold - prev.Value) / (p.Value - prev.Value)
					prevMin := float64(prev.Time-now) / 60000
					highIn = prevMin + ratio*(minutes-prevMin)
				}
			} else {
				highIn = minutes
			}
		}

		// Check low threshold
		if lowIn < 0 && p.Value <= lowThreshold {
			if i > 0 {
				prev := allPoints[i-1]
				if prev.Value > lowThreshold {
					ratio := (prev.Value - lowThreshold) / (prev.Value - p.Value)
					prevMin := float64(prev.Time-now) / 60000
					lowIn = prevMin + ratio*(minutes-prevMin)
				}
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

// Helper functions

// normalizeGlucose normalizes glucose to [-1, 1] range
func normalizeGlucose(glucose float64) float64 {
	return (2*glucose - glucoseMin - glucoseMax) / (glucoseMax - glucoseMin)
}

// denormalizeGlucose converts normalized value back to mg/dL
func denormalizeGlucose(normalized float64) float64 {
	return (normalized*(glucoseMax-glucoseMin) + glucoseMin + glucoseMax) / 2
}

// insulinActivityCurve returns insulin activity at given minutes
func (m *MLPredictor) insulinActivityCurve(minutes float64) float64 {
	// Peak around 75 minutes, DIA default 4 hours
	diaMinutes := m.params.DIA * 60
	if minutes >= diaMinutes {
		return 0
	}
	
	peakTime := 75.0
	
	// Biexponential model
	if minutes < peakTime {
		return (minutes / peakTime) * 0.3 // Rising phase
	}
	
	// Decay phase
	t := (minutes - peakTime) / (diaMinutes - peakTime)
	return 0.3 + 0.7*(1-t) // Linear decay for simplicity
}

// carbActivityCurve returns carb absorption activity at given minutes
func (m *MLPredictor) carbActivityCurve(minutes float64) float64 {
	// Sigmoid curve, peak around 45 minutes, full absorption by 180 min
	absorptionTime := 180.0
	if minutes >= absorptionTime {
		return 0
	}
	
	// Return rate of absorption at this time
	t := minutes / absorptionTime
	return (1 / (1 + math.Exp(-10*(t-0.3)))) * (1 - t)
}

// Treatment context functions

func (m *MLPredictor) indexTreatments(treatments []models.Treatment) map[int64][]models.Treatment {
	index := make(map[int64][]models.Treatment)
	for _, t := range treatments {
		// Index by hour
		hourKey := t.Time().Unix() / 3600
		index[hourKey] = append(index[hourKey], t)
	}
	return index
}

func (m *MLPredictor) calculateIOBAt(treatmentIndex map[int64][]models.Treatment, at time.Time) float64 {
	diaMinutes := m.params.DIA * 60
	var totalIOB float64

	// Look back DIA hours
	for h := int64(0); h <= int64(m.params.DIA)+1; h++ {
		hourKey := at.Unix()/3600 - h
		for _, t := range treatmentIndex[hourKey] {
			if !t.HasInsulin() {
				continue
			}
			minutesAgo := at.Sub(t.Time()).Minutes()
			if minutesAgo < 0 || minutesAgo > diaMinutes {
				continue
			}
			remaining := 1.0 - (minutesAgo / diaMinutes)
			totalIOB += t.Insulin * remaining
		}
	}

	return totalIOB
}

func (m *MLPredictor) calculateCOBAt(treatmentIndex map[int64][]models.Treatment, at time.Time) float64 {
	absorptionHours := 4.0
	var totalCOB float64

	for h := int64(0); h <= int64(absorptionHours)+1; h++ {
		hourKey := at.Unix()/3600 - h
		for _, t := range treatmentIndex[hourKey] {
			if !t.HasCarbs() {
				continue
			}
			minutesAgo := at.Sub(t.Time()).Minutes()
			if minutesAgo < 0 || minutesAgo > absorptionHours*60 {
				continue
			}
			absorbed := minutesAgo / (absorptionHours * 60)
			totalCOB += t.Carbs * (1 - absorbed)
		}
	}

	return totalCOB
}

func (m *MLPredictor) sumRecentInsulin(treatmentIndex map[int64][]models.Treatment, at time.Time, window time.Duration) float64 {
	var total float64
	hours := int64(window.Hours()) + 1

	for h := int64(0); h <= hours; h++ {
		hourKey := at.Unix()/3600 - h
		for _, t := range treatmentIndex[hourKey] {
			if t.HasInsulin() && at.Sub(t.Time()) <= window {
				total += t.Insulin
			}
		}
	}

	return total
}

func (m *MLPredictor) sumRecentCarbs(treatmentIndex map[int64][]models.Treatment, at time.Time, window time.Duration) float64 {
	var total float64
	hours := int64(window.Hours()) + 1

	for h := int64(0); h <= hours; h++ {
		hourKey := at.Unix()/3600 - h
		for _, t := range treatmentIndex[hourKey] {
			if t.HasCarbs() && at.Sub(t.Time()) <= window {
				total += t.Carbs
			}
		}
	}

	return total
}

func (m *MLPredictor) calculateIOB(treatments []models.Treatment, now time.Time) float64 {
	diaMinutes := m.params.DIA * 60
	var totalIOB float64

	for _, t := range treatments {
		if !t.HasInsulin() {
			continue
		}

		minutesAgo := now.Sub(t.Time()).Minutes()
		if minutesAgo < 0 || minutesAgo > diaMinutes {
			continue
		}

		remaining := 1.0 - (minutesAgo / diaMinutes)
		totalIOB += t.Insulin * remaining
	}

	return math.Round(totalIOB*100) / 100
}

func (m *MLPredictor) calculateCOB(treatments []models.Treatment, now time.Time) float64 {
	absorptionMinutes := 180.0
	var totalCOB float64

	for _, t := range treatments {
		if !t.HasCarbs() {
			continue
		}

		minutesAgo := now.Sub(t.Time()).Minutes()
		if minutesAgo < 0 || minutesAgo > absorptionMinutes {
			continue
		}

		// Sigmoid absorption
		progress := minutesAgo / absorptionMinutes
		absorbed := 1 / (1 + math.Exp(-10*(progress-0.5)))
		totalCOB += t.Carbs * (1 - absorbed)
	}

	return math.Round(totalCOB*10) / 10
}
