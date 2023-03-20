package model

import (
	"github.com/coroot/coroot/timeseries"
)

type ClusterRole int

const (
	ClusterRoleNone ClusterRole = iota
	ClusterRolePrimary
	ClusterRoleReplica
)

func (r ClusterRole) String() string {
	switch r {
	case ClusterRolePrimary:
		return "primary"
	case ClusterRoleReplica:
		return "replica"
	}
	return ""
}

type Instance struct {
	Name    string
	OwnerId ApplicationId

	Node *Node

	Pod *Pod

	Rds *Rds

	Jvm *Jvm

	Volumes []*Volume

	Upstreams []*Connection

	TcpListens map[Listen]bool

	Containers map[string]*Container

	LogMessagesByLevel map[LogLevel]*timeseries.TimeSeries
	LogPatterns        map[string]*LogPattern

	ClusterName LabelLastValue
	clusterRole *timeseries.TimeSeries

	Postgres *Postgres
	Redis    *Redis
}

func NewInstance(name string, owner ApplicationId) *Instance {
	return &Instance{
		Name:               name,
		OwnerId:            owner,
		LogMessagesByLevel: map[LogLevel]*timeseries.TimeSeries{},
		LogPatterns:        map[string]*LogPattern{},
		Containers:         map[string]*Container{},
		TcpListens:         map[Listen]bool{},
	}
}

func (instance *Instance) ApplicationTypes() map[ApplicationType]bool {
	res := map[ApplicationType]bool{}
	for _, c := range instance.Containers {
		for t := range c.ApplicationTypes {
			res[t] = true
		}
	}
	return res
}

func (instance *Instance) InstrumentedType() ApplicationType {
	switch {
	case instance.Postgres != nil:
		return ApplicationTypePostgres
	case instance.Redis != nil:
		return ApplicationTypeRedis
	}
	return ApplicationTypeUnknown
}

func (instance *Instance) GetOrCreateContainer(name string) *Container {
	c := instance.Containers[name]
	if c == nil {
		c = NewContainer(name)
		instance.Containers[name] = c
	}
	return c
}

func (instance *Instance) AddUpstreamConnection(actualIP, actualPort, serviceIP, servicePort, container string) *Connection {
	c := &Connection{
		Instance:          instance,
		ActualRemoteIP:    actualIP,
		ActualRemotePort:  actualPort,
		ServiceRemoteIP:   serviceIP,
		ServiceRemotePort: servicePort,
		Container:         container,

		RequestsCount:     map[Protocol]map[string]*timeseries.TimeSeries{},
		RequestsLatency:   map[Protocol]*timeseries.TimeSeries{},
		RequestsHistogram: map[Protocol]map[float32]*timeseries.TimeSeries{},
	}
	instance.Upstreams = append(instance.Upstreams, c)
	return c
}

func (instance *Instance) NodeName() string {
	if instance.Node != nil {
		return instance.Node.Name.Value()
	}
	return ""
}

func (instance *Instance) UpdateClusterRole(role string, v *timeseries.TimeSeries) {
	switch role {
	case "primary":
		instance.clusterRole = v.Map(func(t timeseries.Time, v float32) float32 {
			if v == 1 {
				return float32(ClusterRolePrimary)
			}
			return timeseries.NaN
		})
	case "replica":
		instance.clusterRole = v.Map(func(t timeseries.Time, v float32) float32 {
			if v == 1 {
				return float32(ClusterRoleReplica)
			}
			return timeseries.NaN
		})
	}
}

func (instance *Instance) ClusterRole() *timeseries.TimeSeries {
	if instance.Pod == nil || instance.Pod.Ready.IsEmpty() || instance.clusterRole.IsEmpty() {
		return instance.clusterRole
	}
	return timeseries.Mul(instance.clusterRole, instance.Pod.Ready)
}

func (instance *Instance) ClusterRoleLast() ClusterRole {
	role := instance.ClusterRole()
	if role.IsEmpty() {
		return ClusterRoleNone
	}
	return ClusterRole(role.Last())
}

func (instance *Instance) LifeSpan() *timeseries.TimeSeries {
	if instance.Pod != nil {
		return instance.Pod.LifeSpan
	}
	for _, c := range instance.Containers {
		return c.MemoryRss.Map(timeseries.Defined)
	}
	return nil
}

func (instance *Instance) IsUp() bool {
	for _, c := range instance.Containers {
		if !DataIsMissing(c.MemoryRss) {
			return true
		}
	}
	return false
}

func (instance *Instance) IsObsolete() bool {
	return instance.Pod != nil && instance.Pod.IsObsolete()
}

func (instance *Instance) IsFailed() bool {
	return instance.Pod != nil && instance.Pod.IsFailed()
}

func (instance *Instance) UpAndRunning() *timeseries.TimeSeries {
	mem := timeseries.NewAggregate(timeseries.Any)
	for _, c := range instance.Containers {
		mem.Add(c.MemoryRss)
	}
	memTs := mem.Get()

	if memTs.IsEmpty() {
		return nil
	}
	up := memTs.Map(func(t timeseries.Time, v float32) float32 {
		if v > 0 {
			return 1
		}
		return 0
	})
	if instance.Pod == nil {
		return up
	}
	running := instance.Pod.Running.Map(timeseries.NanToZero)
	ready := instance.Pod.Ready.Map(timeseries.NanToZero)
	return timeseries.NewAggregate(timeseries.Min).Add(running, ready, up).Get()
}

func (instance *Instance) IsListenActive(ip, port string) bool {
	for l, active := range instance.TcpListens {
		if l.IP == ip && (l.Port == port || l.Port == "0") {
			return active
		}
	}
	return false
}

type Listen struct {
	IP      string
	Port    string
	Proxied bool
}
