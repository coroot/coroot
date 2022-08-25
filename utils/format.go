package utils

import (
	"github.com/dustin/go-humanize"
	"github.com/hako/durafmt"
	"math"
	"time"
)

func FormatFloat(v float64) string {
	switch {
	case math.IsNaN(v):
		return "-"
	case v == 0:
		return "0"
	case v > 10:
		return humanize.FtoaWithDigits(v, 0)
	case v >= 1:
		return humanize.FtoaWithDigits(v, 1)
	case v >= 0.1:
		return humanize.FtoaWithDigits(v, 2)
	}
	return humanize.FtoaWithDigits(v, 3)
}

func FormatDuration(d time.Duration, limitFirstN int) string {
	return durafmt.Parse(d).LimitFirstN(limitFirstN).String()
}
