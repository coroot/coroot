package model

import (
	"github.com/coroot/coroot-focus/timeseries"
	"github.com/coroot/coroot-focus/utils"
)

type Labels map[string]string

func (l Labels) Set() *utils.StringSet {
	s := utils.NewStringSet()
	for k, v := range l {
		s.Add(k + ":" + v)
	}
	return s
}

type MetricValues struct {
	Labels     Labels
	LabelsHash uint64
	Values     timeseries.TimeSeries
}
