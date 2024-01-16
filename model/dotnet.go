package model

import "github.com/coroot/coroot/timeseries"

type DotNet struct {
	RuntimeVersion LabelLastValue
	Up             *timeseries.TimeSeries

	Exceptions *timeseries.TimeSeries

	MemoryAllocationRate     *timeseries.TimeSeries
	HeapSize                 map[string]*timeseries.TimeSeries
	HeapFragmentationPercent *timeseries.TimeSeries
	GcCount                  map[string]*timeseries.TimeSeries

	MonitorLockContentions   *timeseries.TimeSeries
	ThreadPoolCompletedItems *timeseries.TimeSeries
	ThreadPoolQueueSize      *timeseries.TimeSeries
	ThreadPoolSize           *timeseries.TimeSeries
}

func (dn *DotNet) IsUp() bool {
	return dn.Up.Last() > 0
}
