package model

import (
	"github.com/coroot/coroot/timeseries"
)

type AvailabilitySLI struct {
	Config CheckConfigSLOAvailability

	TotalRequests  *timeseries.TimeSeries
	FailedRequests *timeseries.TimeSeries

	TotalRequestsRaw  *timeseries.TimeSeries
	FailedRequestsRaw *timeseries.TimeSeries
}

type HistogramBucket struct {
	Le         float32
	TimeSeries *timeseries.TimeSeries
}

type LatencySLI struct {
	Config CheckConfigSLOLatency

	Histogram    []HistogramBucket
	HistogramRaw []HistogramBucket
}

func (sli *LatencySLI) GetTotalAndFast(raw bool) (*timeseries.TimeSeries, *timeseries.TimeSeries) {
	var total, fast *timeseries.TimeSeries
	histogram := sli.Histogram
	if raw {
		histogram = sli.HistogramRaw
	}
	for _, b := range histogram {
		if b.Le <= sli.Config.ObjectiveBucket {
			fast = b.TimeSeries
		}
		if timeseries.IsInf(b.Le, 1) {
			total = b.TimeSeries
		}
	}
	return total, fast
}
