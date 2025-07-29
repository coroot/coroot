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
		{LongWindow: 6 * timeseries.Hour, ShortWindow: 15 * timeseries.Minute, BurnRateThreshold: 6, Severity: CRITICAL},
	}
	MaxAlertRuleWindow      timeseries.Duration
	MaxAlertRuleShortWindow timeseries.Duration
	MinAlertRuleShortWindow timeseries.Duration

	IncidentTimeOffset = timeseries.Hour + 5*timeseries.Minute // long + short window of the first rule
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
		if MinAlertRuleShortWindow == 0 || r.ShortWindow < MinAlertRuleShortWindow {
			MinAlertRuleShortWindow = r.ShortWindow
		}
	}
}

type BurnRate struct {
	LongWindowPercentage  float32             `json:"long_window_percentage"`
	ShortWindowPercentage float32             `json:"short_window_percentage"`
	LongWindowBurnRate    float32             `json:"long_window_burn_rate"`
	ShortWindowBurnRate   float32             `json:"short_window_burn_rate"`
	LongWindow            timeseries.Duration `json:"long_window"`
	ShortWindow           timeseries.Duration `json:"short_window"`
	Severity              Status              `json:"severity"`
	Threshold             float32             `json:"threshold"`
}

func (br BurnRate) FormatSLOStatus() string {
	hours := int(br.LongWindow / timeseries.Hour)
	return fmt.Sprintf("error budget burn rate is %.1fx within %s", br.LongWindowBurnRate, english.Plural(hours, "hour", ""))
}
