package model

import (
	"github.com/coroot/coroot/timeseries"
)

type Redis struct {
	InternalExporter bool

	Up    *timeseries.TimeSeries
	Error LabelLastValue

	Version   LabelLastValue
	Role      LabelLastValue
	Calls     map[string]*timeseries.TimeSeries
	CallsTime map[string]*timeseries.TimeSeries
}

func NewRedis(internalExporter bool) *Redis {
	return &Redis{
		InternalExporter: internalExporter,
		Calls:            map[string]*timeseries.TimeSeries{},
		CallsTime:        map[string]*timeseries.TimeSeries{},
	}
}

func (r *Redis) IsUp() bool {
	return r.Up.Last() > 0
}
