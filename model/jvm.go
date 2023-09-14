package model

import "github.com/coroot/coroot/timeseries"

type Jvm struct {
	JavaVersion LabelLastValue

	HeapSize *timeseries.TimeSeries
	HeapUsed *timeseries.TimeSeries

	SafepointTime     *timeseries.TimeSeries
	SafepointSyncTime *timeseries.TimeSeries

	GcTime map[string]*timeseries.TimeSeries
}

func (j *Jvm) IsUp() bool {
	return j.HeapSize.Last() > 0
}
