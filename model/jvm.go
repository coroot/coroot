package model

import "github.com/coroot/coroot/timeseries"

type Jvm struct {
	JavaVersion LabelLastValue

	HeapUsed    *timeseries.TimeSeries
	HeapMaxSize *timeseries.TimeSeries

	SafepointTime     *timeseries.TimeSeries
	SafepointSyncTime *timeseries.TimeSeries

	GcTime map[string]*timeseries.TimeSeries

	AllocBytes   *timeseries.TimeSeries
	AllocObjects *timeseries.TimeSeries

	LockContentions *timeseries.TimeSeries
	LockTime        *timeseries.TimeSeries

	ProfilingEnabled bool
}

func (j *Jvm) IsUp() bool {
	return j.HeapUsed.Last() > 0
}
