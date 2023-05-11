package model

import "github.com/coroot/coroot/timeseries"

type ContainerStatus string

const (
	ContainerStatusWaiting    ContainerStatus = "waiting"
	ContainerStatusRunning    ContainerStatus = "running"
	ContainerStatusTerminated ContainerStatus = "terminated"
)

type Container struct {
	Id   string
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
	CpuRequest    *timeseries.TimeSeries
	CpuUsage      *timeseries.TimeSeries
	CpuDelay      *timeseries.TimeSeries
	ThrottledTime *timeseries.TimeSeries

	MemoryRss     *timeseries.TimeSeries
	MemoryCache   *timeseries.TimeSeries
	MemoryLimit   *timeseries.TimeSeries
	MemoryRequest *timeseries.TimeSeries

	OOMKills *timeseries.TimeSeries
}

func NewContainer(id, name string) *Container {
	return &Container{
		Id:               id,
		Name:             name,
		ApplicationTypes: map[ApplicationType]bool{},
	}
}
