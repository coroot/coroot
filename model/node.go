package model

import (
	"github.com/coroot/coroot/timeseries"
)

type DiskStats struct {
	IOUtilizationPercent *timeseries.TimeSeries
	ReadOps              *timeseries.TimeSeries
	WriteOps             *timeseries.TimeSeries
	WrittenBytes         *timeseries.TimeSeries
	ReadBytes            *timeseries.TimeSeries
	ReadTime             *timeseries.TimeSeries
	WriteTime            *timeseries.TimeSeries
	Wait                 *timeseries.TimeSeries
	Await                *timeseries.TimeSeries
}

type InterfaceStats struct {
	Name      string
	Addresses []string
	Up        *timeseries.TimeSeries
	RxBytes   *timeseries.TimeSeries
	TxBytes   *timeseries.TimeSeries
}

type Node struct {
	Name      LabelLastValue
	MachineID string
	Uptime    *timeseries.TimeSeries

	CpuCapacity     *timeseries.TimeSeries
	CpuUsagePercent *timeseries.TimeSeries
	CpuUsageByMode  map[string]*timeseries.TimeSeries

	MemoryTotalBytes     *timeseries.TimeSeries
	MemoryFreeBytes      *timeseries.TimeSeries
	MemoryAvailableBytes *timeseries.TimeSeries
	MemoryCachedBytes    *timeseries.TimeSeries

	Disks         map[string]*DiskStats
	NetInterfaces []*InterfaceStats

	Instances []*Instance `json:"-"`

	CloudProvider    LabelLastValue
	Region           LabelLastValue
	AvailabilityZone LabelLastValue
	InstanceType     LabelLastValue
}

func NewNode(machineId string) *Node {
	return &Node{
		MachineID:      machineId,
		Disks:          map[string]*DiskStats{},
		CpuUsageByMode: map[string]*timeseries.TimeSeries{},
	}
}

func (node *Node) IsUp() bool {
	return !DataIsMissing(node.CpuUsagePercent)
}
