package model

import (
	"github.com/coroot/coroot/timeseries"
)

type Redis struct {
	Up    *timeseries.TimeSeries
	Error LabelLastValue

	Version      LabelLastValue
	Role         LabelLastValue
	Calls        map[string]*timeseries.TimeSeries
	CallsTime    map[string]*timeseries.TimeSeries
	Keys         map[string]*timeseries.TimeSeries
	KeysExpiring map[string]*timeseries.TimeSeries
}

func NewRedis() *Redis {
	return &Redis{
		Calls:        map[string]*timeseries.TimeSeries{},
		CallsTime:    map[string]*timeseries.TimeSeries{},
		Keys:         map[string]*timeseries.TimeSeries{},
		KeysExpiring: map[string]*timeseries.TimeSeries{},
	}
}

func (r *Redis) IsUp() bool {
	return r.Up.Last() > 0
}
