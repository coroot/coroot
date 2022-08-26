package model

import (
	"github.com/coroot/coroot-focus/timeseries"
)

type Redis struct {
	Up        timeseries.TimeSeries
	Version   LabelLastValue
	Role      LabelLastValue
	Calls     map[string]timeseries.TimeSeries
	CallsTime map[string]timeseries.TimeSeries
}

func NewRedis() *Redis {
	return &Redis{
		Calls:     map[string]timeseries.TimeSeries{},
		CallsTime: map[string]timeseries.TimeSeries{},
	}
}
