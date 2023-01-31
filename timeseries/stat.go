package timeseries

import (
	"gonum.org/v1/gonum/stat"
	"math"
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
		v    float64
	)
	iter := ts.Iter()
	for iter.Next() {
		t, v = iter.Value()
		if math.IsNaN(v) {
			continue
		}
		x = append(x, float64(t))
		y = append(y, v)
	}
	if len(x) == 0 {
		return nil
	}
	lr.alpha, lr.beta = stat.LinearRegression(x, y, nil, false)
	return lr
}

func (lr *LinearRegression) Calc(t Time) float64 {
	if lr == nil {
		return NaN
	}
	return lr.alpha + lr.beta*float64(t)
}
