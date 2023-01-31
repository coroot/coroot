package model

import "github.com/coroot/coroot/timeseries"

type ContainerStatus string

const (
	ContainerStatusWaiting    ContainerStatus = "waiting"
	ContainerStatusRunning    ContainerStatus = "running"
	ContainerStatusTerminated ContainerStatus = "terminated"
)

type Container struct {
	Name string

	InitContainer bool

	ApplicationTypes map[ApplicationType]bool

	Image string

	Status   ContainerStatus
	Reason   string
	Ready    bool
	Restarts *timeseries.TimeSeries

	LastTerminatedReason string

	CpuLimit      *timeseries.TimeSeries
	CpuUsage      *timeseries.TimeSeries
	CpuDelay      *timeseries.TimeSeries
	ThrottledTime *timeseries.TimeSeries

	MemoryRss     *timeseries.TimeSeries
	MemoryCache   *timeseries.TimeSeries
	MemoryRequest *timeseries.TimeSeries
	MemoryLimit   *timeseries.TimeSeries

	OOMKills *timeseries.TimeSeries
}

func NewContainer(name string) *Container {
	return &Container{
		Name:             name,
		ApplicationTypes: map[ApplicationType]bool{},
	}
}
