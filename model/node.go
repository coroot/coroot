package model

import (
	"strings"

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

type NodeId struct {
	MachineID  string
	SystemUUID string
}

func NewNodeId(machineID, systemUUID string) NodeId {
	return NodeId{MachineID: machineID, SystemUUID: systemUUID}
}

func NewNodeIdFromLabels(mv *MetricValues) NodeId {
	machineID := mv.MachineID
	systemUUID := mv.SystemUUID
	if systemUUID == "" {
		systemUUID = machineID
	} else {
		systemUUID = strings.ReplaceAll(systemUUID, "-", "")
	}
	if machineID == "" {
		machineID = systemUUID
	}
	return NewNodeId(machineID, systemUUID)
}

type Node struct {
	AgentVersion  LabelLastValue
	KernelVersion LabelLastValue

	Name    LabelLastValue
	K8sName LabelLastValue
	Id      NodeId
	Uptime  *timeseries.TimeSeries

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

	Fargate           bool
	Price             *NodePrice
	DataTransferPrice *DataTransferPrice
}

type NodePrice struct {
	Total         float32
	PerCPUCore    float32
	PerMemoryByte float32
}

type InternetStartUsageAmountGB int64

type DataTransferPrice struct {
	InterZoneIngressPerGB float32
	InterZoneEgressPerGB  float32
	InternetPerGB         map[InternetStartUsageAmountGB]float32
}

func (dtp *DataTransferPrice) GetInternetEgressPrice() float32 {
	// so far it returns the price with minimum InternetStartUsageAmountGB
	var minThreshold InternetStartUsageAmountGB = -1
	var price float32

	for threshold, p := range dtp.InternetPerGB {
		if minThreshold == -1 || threshold < minThreshold {
			minThreshold = threshold
			price = p
		}
	}
	return price
}

func NewNode(id NodeId) *Node {
	return &Node{
		Id:             id,
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
	if len(n.Instances) == 1 && n.Instances[0].Owner.Id.Kind == ApplicationKindElasticacheCluster {
		return n.Instances[0].Elasticache.Status.Value() == "available"
	}

	return !n.MemoryTotalBytes.TailIsEmpty()
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
