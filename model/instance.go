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
