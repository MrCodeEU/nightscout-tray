package prediction

import (
	"math"
	"math/rand"
)

// LSTM represents a simple Long Short-Term Memory network
type LSTM struct {
	InputSize  int
	HiddenSize int
	OutputSize int

	// Weights (Matrices)
	Wf, Uf, bf [][]float64
	Wi, Ui, bi [][]float64
	Wc, Uc, bc [][]float64
	Wo, Uo, bo [][]float64
	Why, by    [][]float64

	LearningRate float64
}

// NewLSTM creates a new LSTM network
func NewLSTM(inputSize, hiddenSize, outputSize int) *LSTM {
	lstm := &LSTM{
		InputSize:    inputSize,
		HiddenSize:   hiddenSize,
		OutputSize:   outputSize,
		LearningRate: 0.01,
	}
	lstm.initWeights()
	return lstm
}

func (l *LSTM) initWeights() {
	scale := math.Sqrt(2.0 / float64(l.InputSize+l.HiddenSize))

	// Helper to make matrix
	mkMat := func(r, c int) [][]float64 { return newMatrix(r, c, scale) }
	mkVec := func(r int) [][]float64 { return newMatrix(r, 1, 0) } // bias as col vector

	l.Wf = mkMat(l.HiddenSize, l.InputSize)
	l.Uf = mkMat(l.HiddenSize, l.HiddenSize)
	l.bf = mkVec(l.HiddenSize)
	l.Wi = mkMat(l.HiddenSize, l.InputSize)
	l.Ui = mkMat(l.HiddenSize, l.HiddenSize)
	l.bi = mkVec(l.HiddenSize)
	l.Wc = mkMat(l.HiddenSize, l.InputSize)
	l.Uc = mkMat(l.HiddenSize, l.HiddenSize)
	l.bc = mkVec(l.HiddenSize)
	l.Wo = mkMat(l.HiddenSize, l.InputSize)
	l.Uo = mkMat(l.HiddenSize, l.HiddenSize)
	l.bo = mkVec(l.HiddenSize)
	l.Why = newMatrix(l.OutputSize, l.HiddenSize, scale)
	l.by = mkVec(l.OutputSize)
}

// Forward performs the forward pass
// Returns: outputs, list of cache (for backprop), h states, c states
func (l *LSTM) Forward(inputs [][]float64) ([][]float64, [][][]float64, [][]float64, [][]float64) {
	T := len(inputs)
	h := make([][]float64, T+1)
	c := make([][]float64, T+1)
	outputs := make([][]float64, T)
	cache := make([][][]float64, T)

	h[0] = make([]float64, l.HiddenSize)
	c[0] = make([]float64, l.HiddenSize)

	for t := 0; t < T; t++ {
		x := inputs[t]
		hPrev := h[t]
		cPrev := c[t]

		// Gates
		f := sigmoidVec(addVec(addVec(matmulVal(l.Wf, x), matmulVal(l.Uf, hPrev)), colToVec(l.bf)))
		i := sigmoidVec(addVec(addVec(matmulVal(l.Wi, x), matmulVal(l.Ui, hPrev)), colToVec(l.bi)))
		cBar := tanhVec(addVec(addVec(matmulVal(l.Wc, x), matmulVal(l.Uc, hPrev)), colToVec(l.bc)))
		o := sigmoidVec(addVec(addVec(matmulVal(l.Wo, x), matmulVal(l.Uo, hPrev)), colToVec(l.bo)))

		cNext := addVec(hadamard(f, cPrev), hadamard(i, cBar))
		hNext := hadamard(o, tanhVec(cNext))
		y := addVec(matmulVal(l.Why, hNext), colToVec(l.by))

		h[t+1] = hNext
		c[t+1] = cNext
		outputs[t] = y

		cache[t] = [][]float64{f, i, cBar, o}
	}

	return outputs, cache, h, c
}

// Train performs one step of training (Backpropagation Through Time)
func (l *LSTM) Train(inputs [][]float64, targets [][]float64) float64 {
	T := len(inputs)
	if T == 0 {
		return 0
	}
	outputs, cache, h, c := l.Forward(inputs)

	// Initialize gradients
	dWf := zeroMatrix(l.HiddenSize, l.InputSize)
	dUf := zeroMatrix(l.HiddenSize, l.HiddenSize)
	dbf := zeroMatrix(l.HiddenSize, 1)
	dWi := zeroMatrix(l.HiddenSize, l.InputSize)
	dUi := zeroMatrix(l.HiddenSize, l.HiddenSize)
	dbi := zeroMatrix(l.HiddenSize, 1)
	dWc := zeroMatrix(l.HiddenSize, l.InputSize)
	dUc := zeroMatrix(l.HiddenSize, l.HiddenSize)
	dbc := zeroMatrix(l.HiddenSize, 1)
	dWo := zeroMatrix(l.HiddenSize, l.InputSize)
	dUo := zeroMatrix(l.HiddenSize, l.HiddenSize)
	dbo := zeroMatrix(l.HiddenSize, 1)
	dWhy := zeroMatrix(l.OutputSize, l.HiddenSize)
	dby := zeroMatrix(l.OutputSize, 1)

	dhNext := make([]float64, l.HiddenSize)
	dcNext := make([]float64, l.HiddenSize)

	loss := 0.0

	for t := T - 1; t >= 0; t-- {
		y := outputs[t]
		target := targets[t]

		dy := make([]float64, l.OutputSize)
		for k := 0; k < l.OutputSize; k++ {
			err := y[k] - target[k]
			dy[k] = err
			loss += err * err
		}

		// Backprop Output Layer
		for k := 0; k < l.OutputSize; k++ {
			dby[k][0] += dy[k]
			for j := 0; j < l.HiddenSize; j++ {
				dWhy[k][j] += dy[k] * h[t+1][j]
			}
		}

		// Backprop LSTM
		dh := make([]float64, l.HiddenSize)
		for j := 0; j < l.HiddenSize; j++ {
			sumWhy := 0.0
			for k := 0; k < l.OutputSize; k++ {
				sumWhy += dy[k] * l.Why[k][j]
			}
			dh[j] = sumWhy + dhNext[j]
		}

		f, i, cBar, o := cache[t][0], cache[t][1], cache[t][2], cache[t][3]
		cPrev := c[t]
		cCurr := c[t+1]
		tanhC := tanhVec(cCurr)

		do := make([]float64, l.HiddenSize)
		dc := make([]float64, l.HiddenSize)
		for j := 0; j < l.HiddenSize; j++ {
			do[j] = dh[j] * tanhC[j] * o[j] * (1 - o[j])
			dc[j] = dh[j]*o[j]*(1-tanhC[j]*tanhC[j]) + dcNext[j]
		}

		dcBar := make([]float64, l.HiddenSize)
		di := make([]float64, l.HiddenSize)
		df := make([]float64, l.HiddenSize)

		for j := 0; j < l.HiddenSize; j++ {
			dcBar[j] = dc[j] * i[j] * (1 - cBar[j]*cBar[j])
			di[j] = dc[j] * cBar[j] * i[j] * (1 - i[j])
			df[j] = dc[j] * cPrev[j] * f[j] * (1 - f[j])
		}

		x := inputs[t]
		hPrev := h[t]

		accumulateGrads(dWf, dUf, dbf, df, x, hPrev)
		accumulateGrads(dWi, dUi, dbi, di, x, hPrev)
		accumulateGrads(dWc, dUc, dbc, dcBar, x, hPrev)
		accumulateGrads(dWo, dUo, dbo, do, x, hPrev)

		// Next step gradients
		for j := 0; j < l.HiddenSize; j++ {
			dcNext[j] = dc[j] * f[j]
			sum := 0.0
			for k := 0; k < l.HiddenSize; k++ {
				sum += df[k]*l.Uf[k][j] + di[k]*l.Ui[k][j] + dcBar[k]*l.Uc[k][j] + do[k]*l.Uo[k][j]
			}
			dhNext[j] = sum
		}
	}

	lr := l.LearningRate
	clip := 1.0 // Gradient clipping

	applyGrads(l.Wf, dWf, lr, clip)
	applyGrads(l.Uf, dUf, lr, clip)
	applyGrads(l.bf, dbf, lr, clip)
	applyGrads(l.Wi, dWi, lr, clip)
	applyGrads(l.Ui, dUi, lr, clip)
	applyGrads(l.bi, dbi, lr, clip)
	applyGrads(l.Wc, dWc, lr, clip)
	applyGrads(l.Uc, dUc, lr, clip)
	applyGrads(l.bc, dbc, lr, clip)
	applyGrads(l.Wo, dWo, lr, clip)
	applyGrads(l.Uo, dUo, lr, clip)
	applyGrads(l.bo, dbo, lr, clip)
	applyGrads(l.Why, dWhy, lr, clip)
	applyGrads(l.by, dby, lr, clip)

	return loss / float64(T)
}

func accumulateGrads(dW, dU, db [][]float64, dGate, x, hPrev []float64) {
	rows := len(dW)
	cols := len(dW[0])
	hSize := len(dU[0])

	for i := 0; i < rows; i++ {
		db[i][0] += dGate[i]
		for j := 0; j < cols; j++ {
			dW[i][j] += dGate[i] * x[j]
		}
		for j := 0; j < hSize; j++ {
			dU[i][j] += dGate[i] * hPrev[j]
		}
	}
}

func applyGrads(W, dW [][]float64, lr, clip float64) {
	for i := range W {
		for j := range W[i] {
			grad := dW[i][j]
			if grad > clip {
				grad = clip
			}
			if grad < -clip {
				grad = -clip
			}
			W[i][j] -= lr * grad
		}
	}
}

// Helpers

func newMatrix(rows, cols int, scale float64) [][]float64 {
	m := make([][]float64, rows)
	for i := range m {
		m[i] = make([]float64, cols)
		for j := range m[i] {
			if scale == 0 {
				m[i][j] = 0
			} else {
				//nolint:gosec // ML weights do not require crypto secure random
				m[i][j] = (rand.Float64()*2 - 1) * scale
			}
		}
	}
	return m
}

func zeroMatrix(rows, cols int) [][]float64 {
	return newMatrix(rows, cols, 0)
}

func colToVec(m [][]float64) []float64 {
	v := make([]float64, len(m))
	for i := range m {
		v[i] = m[i][0]
	}
	return v
}

func matmulVal(m [][]float64, v []float64) []float64 {
	rows := len(m)
	cols := len(m[0])
	res := make([]float64, rows)
	for i := 0; i < rows; i++ {
		sum := 0.0
		for j := 0; j < cols; j++ {
			sum += m[i][j] * v[j]
		}
		res[i] = sum
	}
	return res
}

func addVec(a, b []float64) []float64 {
	res := make([]float64, len(a))
	for i := range a {
		res[i] = a[i] + b[i]
	}
	return res
}

func hadamard(a, b []float64) []float64 {
	res := make([]float64, len(a))
	for i := range a {
		res[i] = a[i] * b[i]
	}
	return res
}

func sigmoid(x float64) float64 {
	return 1.0 / (1.0 + math.Exp(-x))
}

func sigmoidVec(v []float64) []float64 {
	res := make([]float64, len(v))
	for i := range v {
		res[i] = sigmoid(v[i])
	}
	return res
}

func tanhVec(v []float64) []float64 {
	res := make([]float64, len(v))
	for i := range v {
		res[i] = math.Tanh(v[i])
	}
	return res
}
