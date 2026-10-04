package model

import "github.com/coroot/coroot/timeseries"

// RUM metric names used when bridging ClickHouse aggregates into the metrics world / docs.
const (
	MetricRumWebVitalP75     = "rum_web_vital_p75"
	MetricRumErrorsPerSecond = "rum_errors_per_second"
	MetricRumFetchErrorPct   = "rum_fetch_error_percent"
)

// RumMetricLabels are low-cardinality labels for RUM recording-rule style series.
type RumMetricLabels struct {
	Service string
	Vital   string // lcp, inp, cls, ttfb
}

// RumStats holds pre-aggregated RUM timeseries for auditor checks and charts.
type RumStats struct {
	ServiceName   string
	LcpP75        *timeseries.TimeSeries
	InpP75        *timeseries.TimeSeries
	ClsP75        *timeseries.TimeSeries
	TtfbP75       *timeseries.TimeSeries
	PageViewsPs   *timeseries.TimeSeries
	JsErrorsPs    *timeseries.TimeSeries
	FetchErrorPct float32 // last window percentage 0-100
}

// RumSignal is a frontend UX symptom attached to a backend incident blast radius.
type RumSignal struct {
	Service   string  `json:"service"`
	Check     string  `json:"check"`
	Value     float32 `json:"value"`
	Threshold float32 `json:"threshold"`
	Message   string  `json:"message"`
}
