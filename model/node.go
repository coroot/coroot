package model

import (
	"github.com/coroot/coroot/timeseries"
)

const (
	CloudProviderAWS   = "aws"
	CloudProviderAzure = "azure"
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
	AgentVersion LabelLastValue

	Name      LabelLastValue
	K8sName   LabelLastValue
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

	CloudProvider     LabelLastValue
	Region            LabelLastValue
	AvailabilityZone  LabelLastValue
	InstanceType      LabelLastValue
	InstanceLifeCycle LabelLastValue

	Price *NodePrice
}

type NodePrice struct {
	Total         float32
	PerCPUCore    float32
	PerMemoryByte float32
}

func NewNode(machineId string) *Node {
	return &Node{
		MachineID:      machineId,
		Disks:          map[string]*DiskStats{},
		CpuUsageByMode: map[string]*timeseries.TimeSeries{},
	}
}

func (n *Node) GetName() string {
	if n.Name.Value() != "" {
		return n.Name.Value()
	}
	return n.K8sName.Value()
}

func (n *Node) IsAgentInstalled() bool {
	return n != nil && n.Name.Value() != ""
}

func (n *Node) IsUp() bool {
	if n == nil {
		return false
	}
	// currently, we don't collect OS metrics for Elasticache nodes
	if len(n.Instances) == 1 && n.Instances[0].OwnerId.Kind == ApplicationKindElasticacheCluster {
		return n.Instances[0].Elasticache.Status.Value() == "available"
	}

	return !DataIsMissing(n.CpuUsagePercent)
}

func (n *Node) IsDown() bool {
	return n != nil && n.IsAgentInstalled() && !n.IsUp()
}

func (n *Node) Status() Status {
	switch {
	case n == nil:
		return UNKNOWN
	case !n.IsAgentInstalled():
		return UNKNOWN
	case n.IsDown():
		return WARNING
	}
	return OK
}
