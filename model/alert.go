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
		//{LongWindow: 3 * timeseries.Day, ShortWindow: 6 * timeseries.Hour, BurnRateThreshold: 1, Severity: WARNING},
	}
	MaxAlertRuleWindow      timeseries.Duration
	MaxAlertRuleShortWindow timeseries.Duration
)

func init() {
	MaxAlertRuleWindow = timeseries.Hour
	MaxAlertRuleShortWindow = 5 * timeseries.Minute
	for _, r := range AlertRules {
		if r.ShortWindow > r.LongWindow {
			panic("invalid rule")
		}
		if r.LongWindow > MaxAlertRuleWindow {
			MaxAlertRuleWindow = r.LongWindow
		}
		if r.ShortWindow > MaxAlertRuleShortWindow {
			MaxAlertRuleShortWindow = r.ShortWindow
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
