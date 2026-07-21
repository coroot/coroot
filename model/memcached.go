package model

import (
	"github.com/coroot/coroot/timeseries"
)

type Memcached struct {
	Up           *timeseries.TimeSeries
	Version      LabelLastValue
	Calls        map[string]*timeseries.TimeSeries
	Hits         *timeseries.TimeSeries
	Misses       *timeseries.TimeSeries
	LimitBytes   *timeseries.TimeSeries
	EvictedItems *timeseries.TimeSeries
}

func NewMemcached() *Memcached {
	return &Memcached{
		Calls: map[string]*timeseries.TimeSeries{},
	}
}

func (r *Memcached) IsUp() bool {
	return r.Up.Last() > 0
}
