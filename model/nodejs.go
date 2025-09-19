package model

import "github.com/coroot/coroot/timeseries"

type Nodejs struct {
	EventLoopBlockedTime *timeseries.TimeSeries
}
