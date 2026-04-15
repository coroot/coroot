package model

import "github.com/coroot/coroot/timeseries"

type GoRuntime struct {
	AllocBytes   *timeseries.TimeSeries
	AllocObjects *timeseries.TimeSeries
}
