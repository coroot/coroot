package model

import (
	"github.com/coroot/coroot/timeseries"
)

type ClusterRole int

const (
	ClusterRoleNone ClusterRole = iota
	ClusterRolePrimary
	ClusterRoleReplica
	ClusterRoleArbiter
)

func (r ClusterRole) String() string {
	switch r {
	case ClusterRolePrimary:
		return "primary"
	case ClusterRoleReplica:
		return "replica"
	case ClusterRoleArbiter:
		return "arbiter"
	}
	return ""
}

type Instance struct {
	Name  string
	Owner *Application

	Node *Node

	Pod *Pod

	Rds         *Rds
	Elasticache *Elasticache

	Jvms   map[string]*Jvm
	DotNet map[string]*DotNet
	Python *Python

	Volumes []*Volume

	Upstreams map[ConnectionKey]*Connection

	TcpListens map[Listen]bool

	Containers map[string]*Container

	ClusterName      LabelLastValue
	clusterRole      *timeseries.TimeSeries
	ClusterComponent *Application

	Postgres  *Postgres
	Redis     *Redis
	Mongodb   *Mongodb
	Memcached *Memcached
	Mysql     *Mysql
}

func NewInstance(name string, owner *Application) *Instance {
	return &Instance{
		Name:       name,
		Owner:      owner,
		Containers: map[string]*Container{},
		Upstreams:  map[ConnectionKey]*Connection{},
		TcpListens: map[Listen]bool{},
	}
}

func (instance *Instance) ApplicationTypes() map[ApplicationType]bool {
	res := map[ApplicationType]bool{}
	for _, c := range instance.Containers {
		for t := range c.ApplicationTypes {
			res[t] = true
		}
	}
	if t := instance.Rds.ApplicationType(); t != ApplicationTypeUnknown {
		res[t] = true
	}
	if t := instance.Elasticache.ApplicationType(); t != ApplicationTypeUnknown {
		res[t] = true
	}
	return res
}

func (instance *Instance) InstrumentedType() ApplicationType {
	switch {
	case instance.Postgres != nil:
		return ApplicationTypePostgres
	case instance.Mysql != nil:
		return ApplicationTypeMysql
	case instance.Redis != nil:
		return ApplicationTypeRedis
	case instance.Mongodb != nil:
		return ApplicationTypeMongodb
	case instance.Memcached != nil:
		return ApplicationTypeMemcached
	}
	return ApplicationTypeUnknown
}

func (instance *Instance) GetOrCreateContainer(id, name string) *Container {
	c := instance.Containers[name]
	if c == nil {
		c = NewContainer(id, name)
		instance.Containers[name] = c
	}
	return c
}

func (instance *Instance) NodeName() string {
	if instance.Node != nil {
		return instance.Node.GetName()
	}
	return ""
}

func (instance *Instance) NodeId() NodeId {
	if instance.Node != nil {
		return instance.Node.Id
	}
	return NodeId{}
}

func (instance *Instance) UpdateClusterRole(role string, v *timeseries.TimeSeries) {
	switch role {
	case "primary":
		v = v.Map(func(t timeseries.Time, v float32) float32 {
			if v == 1 {
				return float32(ClusterRolePrimary)
			}
			return timeseries.NaN
		})
	case "replica":
		v = v.Map(func(t timeseries.Time, v float32) float32 {
			if v == 1 {
				return float32(ClusterRoleReplica)
			}
			return timeseries.NaN
		})
	case "arbiter":
		v = v.Map(func(t timeseries.Time, v float32) float32 {
			if v == 1 {
				return float32(ClusterRoleArbiter)
			}
			return timeseries.NaN
		})
	default:
		return
	}
	if instance.clusterRole == nil {
		instance.clusterRole = v
	} else {
		instance.clusterRole = timeseries.NewAggregate(timeseries.Any).Add(instance.clusterRole, v).Get()
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
		if !c.MemoryRss.TailIsEmpty() {
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
