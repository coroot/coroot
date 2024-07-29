package utils

import (
	"fmt"
	"math"
	"strings"

	"github.com/coroot/coroot/timeseries"
	"github.com/dustin/go-humanize"
	"github.com/hako/durafmt"
)

var (
	shortDurations, _ = durafmt.DefaultUnitsCoder.Decode("y:y,w:w,d:d,h:h,m:m,s:s,ms:ms,µs:µs")
)

func FormatFloat(v float32) string {
	switch {
	case timeseries.IsNaN(v):
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

func FormatBytes(v float32) (string, string) {
	base := 1000.0
	sizes := []string{"B", "kB", "MB", "GB", "TB", "PB", "EB"}
	e := math.Floor(math.Log(float64(v)) / math.Log(base))
	if int(e) < 0 || int(e) > len(sizes)-1 {
		return fmt.Sprintf("%.f", v), ""
	}
	unit := sizes[int(e)]
	val := fmt.Sprintf("%.0f", math.Round(float64(v)/math.Pow(base, e)))
	return val, unit
}

func HumanBits(v float32) string {
	if timeseries.IsNaN(v) {
		return ""
	}
	return strings.Replace(humanize.Bytes(uint64(v)), "B", "b", -1) + "ps"
}

func FormatLatency(v float32) string {
	if v < 0.0001 {
		return "<0.1ms"
	}
	if v < 1 {
		return FormatFloat(v*1000) + "ms"
	}
	return FormatFloat(v) + "s"
}

func FormatPercentage(v float32) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", v), "0"), ".") + "%"
}

func FormatMoney(v float32) string {
	s := ""
	switch {
	case v > 0:
		s = "+"
	case v < 0:
		s = "-"
	}
	return fmt.Sprintf("%s$%.2f", s, math.Abs(float64(v)))
}

func LastPart(s string, sep string) string {
	parts := strings.Split(s, sep)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func FormatLinkStats(requests, latency, bytesSent, bytesReceived float32, issue string) []string {
	if issue != "" {
		return []string{"⚠️ " + issue}
	}
	var res []string
	line := ""
	if !timeseries.IsNaN(requests) {
		line += "📈 " + FormatFloat(requests) + " rps"
	}
	if !timeseries.IsNaN(latency) {
		if len(line) > 0 {
			line += " "
		}
		line += "⏱️ " + FormatLatency(latency)
	}
	if len(line) > 0 {
		res = append(res, line)
	}
	if bytesSent > 0 || bytesReceived > 0 {
		line = ""
		v, u := FormatBytes(bytesSent)
		line += "↑" + v + u + "/s"
		v, u = FormatBytes(bytesReceived)
		line += " ↓" + v + u + "/s"
		res = append(res, line)
	}
	return res
}
