package model

import (
	"github.com/coroot/coroot/timeseries"
)

type Volume struct {
	Name       LabelLastValue
	Device     LabelLastValue
	MountPoint string

	CapacityBytes timeseries.TimeSeries
	UsedBytes     timeseries.TimeSeries
}
