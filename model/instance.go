package model

import (
	"github.com/coroot/coroot-focus/timeseries"
	"k8s.io/klog"
	"net"
)

type InstanceId struct {
	Name    string
	OwnerId ApplicationId
}

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
	return "-"
}

type Instance struct {
	InstanceId

	Node *Node

	Pod *Pod

	Volumes []*Volume

	Upstreams   []*Connection
	Downstreams []*Connection

	TcpListens map[Listen]bool

	Containers map[string]*Container

	LogMessagesByLevel map[LogLevel]timeseries.TimeSeries
	LogPatterns        map[string]*LogPattern

	ClusterName LabelLastValue
	clusterRole timeseries.TimeSeries
}

func NewInstance(name string, owner ApplicationId) *Instance {
	return &Instance{
		InstanceId:         InstanceId{Name: name, OwnerId: owner},
		LogMessagesByLevel: map[LogLevel]timeseries.TimeSeries{},
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

func (instance *Instance) GetOrCreateContainer(name string) *Container {
	c := instance.Containers[name]
	if c == nil {
		c = NewContainer(name)
		instance.Containers[name] = c
	}
	return c
}

func (instance *Instance) GetOrCreateUpstreamConnection(ls Labels, container string) *Connection {
	actualDest := ls["actual_destination"]
	dest := ls["destination"]

	var serviceIP, servicePort, actualIP, actualPort string
	var err error

	serviceIP, servicePort, err = net.SplitHostPort(dest)
	if err != nil {
		klog.Warningf("failed to split %s to ip:port pair: %s", dest, err)
		return nil
	}
	if actualDest != "" {
		actualIP, actualPort, err = net.SplitHostPort(actualDest)
		if err != nil {
			klog.Warningf("failed to split %s to ip:port pair: %s", actualDest, err)
			return nil
		}
	}
	for _, c := range instance.Upstreams {
		if c.ActualRemoteIP == actualIP && c.ActualRemotePort == actualPort &&
			c.ServiceRemoteIP == serviceIP && c.ServiceRemotePort == servicePort {
			return c
		}
	}
	c := &Connection{
		Instance:          instance,
		ActualRemoteIP:    actualIP,
		ActualRemotePort:  actualPort,
		ServiceRemoteIP:   serviceIP,
		ServiceRemotePort: servicePort,
		Container:         container,
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

func (instance *Instance) UpdateClusterRole(role string, v timeseries.TimeSeries) {
	if instance.clusterRole == nil {
		instance.clusterRole = timeseries.Aggregate(timeseries.Any)
	}
	switch role {
	case "primary":
		instance.clusterRole.(*timeseries.AggregatedTimeseries).AddInput(
			timeseries.Map(func(v float64) float64 {
				if v == 1 {
					return float64(ClusterRolePrimary)
				}
				return timeseries.NaN
			}, v))
	case "replica":
		instance.clusterRole.(*timeseries.AggregatedTimeseries).AddInput(
			timeseries.Map(func(v float64) float64 {
				if v == 1 {
					return float64(ClusterRoleReplica)
				}
				return timeseries.NaN
			}, v))
	}
}

func (instance *Instance) ClusterRole() timeseries.TimeSeries {
	if instance.Pod == nil || instance.Pod.Ready == nil || instance.clusterRole.IsEmpty() {
		return instance.clusterRole
	}
	return timeseries.Aggregate(timeseries.Mul, instance.clusterRole, instance.Pod.Ready)
}

func (instance *Instance) ClusterRoleLast() ClusterRole {
	role := instance.ClusterRole()
	if role == nil || role.IsEmpty() {
		return ClusterRoleNone
	}
	return ClusterRole(role.Last())
}

func (instance *Instance) IsListenActive(ip, port string) bool {
	for l, active := range instance.TcpListens {
		if l.IP == ip && l.Port == port {
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
