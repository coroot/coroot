package model

import (
	"github.com/coroot/coroot/timeseries"
)

type EBS struct {
	AllocatedGibs   *timeseries.TimeSeries
	StorageType     LabelLastValue
	ProvisionedIOPS *timeseries.TimeSeries
	VolumeId        string
}

type Volume struct {
	Name       LabelLastValue
	Device     LabelLastValue
	MountPoint string

	EBS           *EBS
	CapacityBytes *timeseries.TimeSeries
	UsedBytes     *timeseries.TimeSeries
}
