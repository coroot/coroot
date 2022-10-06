package model

import "github.com/coroot/coroot/timeseries"

type LatencySLI struct {
	Config CheckConfigSLOLatency

	Histogram     map[string]timeseries.TimeSeries
	TotalRequests timeseries.TimeSeries
	FastRequests  timeseries.TimeSeries
}

type AvailabilitySLI struct {
	Config CheckConfigSLOAvailability

	TotalRequests  timeseries.TimeSeries
	FailedRequests timeseries.TimeSeries
}
