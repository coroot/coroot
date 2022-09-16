package model

import (
	"github.com/coroot/coroot/timeseries"
)

type Labels map[string]string

type MetricValues struct {
	Labels     Labels
	LabelsHash uint64
	Values     *timeseries.InMemoryTimeSeries
}
