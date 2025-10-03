package config

import "github.com/coroot/coroot/timeseries"

type CollectorConfig struct {
	TracesTTL   timeseries.Duration
	LogsTTL     timeseries.Duration
	ProfilesTTL timeseries.Duration
	MetricsTTL  timeseries.Duration
}
