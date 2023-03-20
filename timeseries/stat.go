package timeseries

import (
	"gonum.org/v1/gonum/stat"
)

type LinearRegression struct {
	alpha, beta float64
}

func NewLinearRegression(ts *TimeSeries) *LinearRegression {
	if ts.IsEmpty() {
		return nil
	}
	lr := &LinearRegression{}
	var (
		x, y []float64
		t    Time
		v    float32
	)
	iter := ts.Iter()
	for iter.Next() {
		t, v = iter.Value()
		if IsNaN(v) {
			continue
		}
		x = append(x, float64(t))
		y = append(y, float64(v))
	}
	if len(x) == 0 {
		return nil
	}
	lr.alpha, lr.beta = stat.LinearRegression(x, y, nil, false)
	return lr
}

func (lr *LinearRegression) Calc(t Time) float32 {
	if lr == nil {
		return NaN
	}
	return float32(lr.alpha + lr.beta*float64(t))
}
