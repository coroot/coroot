package timeseries

import (
	"math"
)

type F func(accumulator, v float64) float64

func Any(v1, v2 float64) float64 {
	if !math.IsNaN(v1) {
		return v1
	}
	return v2
}

func Last(prev, v float64) float64 {
	return v
}

func NanSum(sum, v float64) float64 {
	if math.IsNaN(sum) {
		sum = 0
	}
	if !math.IsNaN(v) {
		sum += v
	}
	return sum
}

func Sum(sum, v float64) float64 {
	return sum + v
}

func Max(max, v float64) float64 {
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

func Min(min, v float64) float64 {
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

func Div(div, v float64) float64 {
	return div / v
}

func Mul(mul, v float64) float64 {
	return mul / v
}

func Sub(sub, v float64) float64 {
	return sub - v
}

func Defined(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	return 1
}

func NanToZero(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	return v
}
