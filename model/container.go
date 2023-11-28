package model

import (
	"github.com/coroot/coroot/timeseries"
	"regexp"
	"strings"
)

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

	MemoryRss         *timeseries.TimeSeries
	MemoryRssForTrend *timeseries.TimeSeries

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

var (
	deploymentPodRegex  = regexp.MustCompile(`(/k8s/[a-z0-9-]+/[a-z0-9-]+)-[0-9a-f]{1,10}-[bcdfghjklmnpqrstvwxz2456789]{5}/.+`)
	daemonsetPodRegex   = regexp.MustCompile(`(/k8s/[a-z0-9-]+/[a-z0-9-]+)-[bcdfghjklmnpqrstvwxz2456789]{5}/.+`)
	statefulsetPodRegex = regexp.MustCompile(`(/k8s/[a-z0-9-]+/[a-z0-9-]+)-\d+/.+`)
)

func ContainerIdToServiceName(containerId string) string {
	if !strings.HasPrefix(containerId, "/k8s/") {
		return containerId
	}
	for _, r := range []*regexp.Regexp{deploymentPodRegex, daemonsetPodRegex, statefulsetPodRegex} {
		if g := r.FindStringSubmatch(containerId); len(g) == 2 {
			return g[1]
		}
	}
	return containerId
}
