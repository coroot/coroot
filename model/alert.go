package model

import (
	"fmt"
	"github.com/coroot/coroot/timeseries"
	"github.com/dustin/go-humanize/english"
)

type AlertRule struct {
	LongWindow        timeseries.Duration
	ShortWindow       timeseries.Duration
	BurnRateThreshold float32
	Severity          Status
}

var (
	AlertRules = []AlertRule{
		{LongWindow: timeseries.Hour, ShortWindow: 5 * timeseries.Minute, BurnRateThreshold: 14.4, Severity: CRITICAL},
		{LongWindow: 6 * timeseries.Hour, ShortWindow: 30 * timeseries.Minute, BurnRateThreshold: 6, Severity: CRITICAL},
		{LongWindow: timeseries.Day, ShortWindow: 2 * timeseries.Hour, BurnRateThreshold: 3, Severity: WARNING},
		{LongWindow: 3 * timeseries.Day, ShortWindow: 6 * timeseries.Hour, BurnRateThreshold: 1, Severity: WARNING},
	}
	MaxAlertRuleWindow timeseries.Duration
)

func init() {
	MaxAlertRuleWindow = timeseries.Hour
	for _, r := range AlertRules {
		if r.ShortWindow > r.LongWindow {
			panic("invalid rule")
		}
		if r.LongWindow > MaxAlertRuleWindow {
			MaxAlertRuleWindow = r.LongWindow
		}
	}
}

type BurnRate struct {
	Value    float32
	Window   timeseries.Duration
	Severity Status
}

func (br BurnRate) FormatSLOStatus() string {
	hours := int(br.Window / timeseries.Hour)
	return fmt.Sprintf("error budget burn rate is %.1fx within %s", br.Value, english.Plural(hours, "hour", ""))
}

func CheckBurnRates(now timeseries.Time, bad, total *timeseries.TimeSeries, objectivePercentage float32) BurnRate {
	if bad.IsEmpty() || total.IsEmpty() {
		return BurnRate{Severity: UNKNOWN}
	}

	objective := 1 - objectivePercentage/100

	sumFrom := func(ts *timeseries.TimeSeries, from timeseries.Time) float32 {
		return ts.Reduce(func(t timeseries.Time, accumulator, v float32) float32 {
			if t.Before(from) {
				return 0
			}
			return timeseries.NanSum(t, accumulator, v)
		})
	}

	first := BurnRate{}
	for _, r := range AlertRules {
		from := now.Add(-r.LongWindow)
		br := sumFrom(bad, from) / sumFrom(total, from) / objective
		if timeseries.IsNaN(br) {
			br = 0
		}
		if first.Window == 0 {
			first.Window = r.LongWindow
			first.Value = br
		}
		if br < r.BurnRateThreshold {
			continue
		}
		from = now.Add(-r.ShortWindow)
		br = sumFrom(bad, from) / sumFrom(total, from) / objective
		if timeseries.IsNaN(br) {
			br = 0
		}
		if br < r.BurnRateThreshold {
			continue
		}
		return BurnRate{Value: br, Window: r.LongWindow, Severity: r.Severity}
	}
	first.Severity = OK
	return first
}
