package model

import (
	"github.com/coroot/coroot/timeseries"
	"strings"
)

type Protocol string

type Connection struct {
	ActualRemotePort string
	ActualRemoteIP   string
	Instance         *Instance
	RemoteInstance   *Instance

	Container string

	Rtt *timeseries.TimeSeries

	Connects *timeseries.TimeSeries
	Active   *timeseries.TimeSeries

	Retransmissions *timeseries.TimeSeries

	RequestsCount     map[Protocol]map[string]*timeseries.TimeSeries // by status
	RequestsLatency   map[Protocol]*timeseries.TimeSeries
	RequestsHistogram map[Protocol]map[float32]*timeseries.TimeSeries // by le

	ServiceRemoteIP   string
	ServiceRemotePort string
}

func (c *Connection) IsActual() bool {
	if c.RemoteInstance == nil {
		return false
	}
	if !c.RemoteInstance.IsListenActive(c.ActualRemoteIP, c.ActualRemotePort) {
		return false
	}
	return (c.Connects.Last() > 0) || (c.Active.Last() > 0)
}

func (c *Connection) IsObsolete() bool {
	if c.Container != "" && c.Instance.Pod != nil && c.Instance.Pod.InitContainers[c.Container] != nil {
		return false
	}
	return c.RemoteInstance != nil && c.RemoteInstance.IsObsolete()
}

func (c *Connection) Status() Status {
	status := UNKNOWN
	if c.IsActual() && !c.Rtt.IsEmpty() {
		status = OK
		if DataIsMissing(c.Rtt) {
			status = WARNING
		}
	}
	return status
}

func IsRequestStatusFailed(status string) bool {
	return status == "failed" || strings.HasPrefix(status, "5")
}

func GetConnectionsRequestsSum(connections []*Connection) *timeseries.TimeSeries {
	sum := timeseries.NewAggregate(timeseries.NanSum)
	for _, c := range connections {
		for _, byStatus := range c.RequestsCount {
			for _, ts := range byStatus {
				sum.Add(ts)
			}
		}
	}
	return sum.Get()
}

func GetConnectionsErrorsSum(connections []*Connection) *timeseries.TimeSeries {
	sum := timeseries.NewAggregate(timeseries.NanSum)
	for _, c := range connections {
		for _, byStatus := range c.RequestsCount {
			for status, ts := range byStatus {
				if !IsRequestStatusFailed(status) {
					continue
				}
				sum.Add(ts)
			}
		}
	}
	return sum.Get()
}

func GetConnectionsRequestsLatency(connections []*Connection) *timeseries.TimeSeries {
	time := timeseries.NewAggregate(timeseries.NanSum)
	count := timeseries.NewAggregate(timeseries.NanSum)
	for _, c := range connections {
		for protocol, latency := range c.RequestsLatency {
			if len(c.RequestsCount[protocol]) == 0 {
				continue
			}
			requests := timeseries.NewAggregate(timeseries.NanSum)
			for _, ts := range c.RequestsCount[protocol] {
				requests.Add(ts)
			}
			req := requests.Get()
			time.Add(timeseries.Mul(latency, req))
			count.Add(req)
		}
	}
	return timeseries.Div(time.Get(), count.Get())
}
