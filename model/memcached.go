package model

import (
	"github.com/coroot/coroot/timeseries"
)

type Memcached struct {
	InternalExporter bool

	Up           *timeseries.TimeSeries
	Version      LabelLastValue
	Calls        map[string]*timeseries.TimeSeries
	Hits         *timeseries.TimeSeries
	Misses       *timeseries.TimeSeries
	LimitBytes   *timeseries.TimeSeries
	EvictedItems *timeseries.TimeSeries
}

func NewMemcached(internalExporter bool) *Memcached {
	return &Memcached{
		InternalExporter: internalExporter,
		Calls:            map[string]*timeseries.TimeSeries{},
	}
}

func (r *Memcached) IsUp() bool {
	return r.Up.Last() > 0
}
