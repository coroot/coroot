package model

import (
	"github.com/coroot/coroot-focus/timeseries"
)

type Volume struct {
	Name         LabelLastValue
	Phase        LabelLastValue
	StorageClass LabelLastValue
	Device       LabelLastValue
	MountPoint   string

	CapacityBytes timeseries.TimeSeries
	UsedBytes     timeseries.TimeSeries
}