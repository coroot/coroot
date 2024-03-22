package model

import (
	"github.com/coroot/coroot/timeseries"
	"github.com/prometheus/common/model"
)

type Labels map[string]string

func (ls Labels) Hash() uint64 {
	return model.LabelsToSignature(ls)
}

type MetricValues struct {
	Labels     Labels
	LabelsHash uint64
	Values     *timeseries.TimeSeries
}
