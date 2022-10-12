package timeseries

import (
	"math"
)

type F func(t Time, accumulator, v float64) float64

func Any(t Time, v1, v2 float64) float64 {
	if !math.IsNaN(v1) {
		return v1
	}
	return v2
}

func NanSum(t Time, sum, v float64) float64 {
	if math.IsNaN(sum) {
		sum = 0
	}
	if !math.IsNaN(v) {
		sum += v
	}
	return sum
}

func Sum(t Time, sum, v float64) float64 {
	return sum + v
}

func Max(t Time, max, v float64) float64 {
	if math.IsNaN(max) {
		return v
	}
	if math.IsNaN(v) {
		return max
	}
	if v > max {
		return v
	}
	return max
}

func Min(t Time, min, v float64) float64 {
	if math.IsNaN(min) {
		return v
	}
	if math.IsNaN(v) {
		return min
	}
	if v < min {
		return v
	}
	return min
}

func Div(t Time, div, v float64) float64 {
	return div / v
}

func Mul(t Time, mul, v float64) float64 {
	return mul / v
}

func Sub(t Time, sub, v float64) float64 {
	return sub - v
}

func Defined(t Time, v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	return 1
}

func NanToZero(t Time, v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	return v
}
