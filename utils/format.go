package utils

import (
	"fmt"
	"github.com/coroot/coroot/timeseries"
	"github.com/dustin/go-humanize"
	"github.com/hako/durafmt"
	"math"
	"strings"
)

var (
	shortDurations, _ = durafmt.DefaultUnitsCoder.Decode("y:y,w:w,d:d,h:h,m:m,s:s,ms:ms,µs:µs")
)

func FormatFloat(v float64) string {
	switch {
	case math.IsNaN(v):
		return ""
	case v == 0:
		return "0"
	case v >= 1:
		return fmt.Sprintf("%.0f", v)
	case v >= 0.1:
		return fmt.Sprintf("%.1f", v)
	case v >= 0.01:
		return fmt.Sprintf("%.2f", v)
	}
	return fmt.Sprintf("%.3f", v)
}

func FormatDuration(d timeseries.Duration, limitFirstN int) string {
	return durafmt.Parse(d.ToStandard()).LimitFirstN(limitFirstN).String()
}
func FormatDurationShort(d timeseries.Duration, limitFirstN int) string {
	return strings.Replace(durafmt.Parse(d.ToStandard()).LimitFirstN(limitFirstN).Format(shortDurations), " ", "", -1)
}

func FormatBytes(b float64) (string, string) {
	s := humanize.Bytes(uint64(b))
	parts := strings.Fields(s)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func HumanBits(v float64) string {
	if math.IsNaN(v) {
		return ""
	}
	return strings.Replace(humanize.Bytes(uint64(v)), "B", "b", -1) + "ps"
}

func FormatLatency(v float64) string {
	if v < 1 {
		return FormatFloat(v*1000) + " ms"
	}
	return FormatFloat(v) + "s"
}

func FormatPercentage(v float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", v), "0"), ".") + "%"
}

func LastPart(s string, sep string) string {
	parts := strings.Split(s, sep)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
