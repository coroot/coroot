package model

import (
	"github.com/coroot/coroot/timeseries"
	"math"
)

type Connection struct {
	ActualRemotePort string
	ActualRemoteIP   string
	Instance         *Instance
	RemoteInstance   *Instance

	Container string

	Rtt timeseries.TimeSeries

	Connects timeseries.TimeSeries
	Active   timeseries.TimeSeries

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
	return (timeseries.Last(c.Connects) > 0) || (timeseries.Last(c.Active) > 0)
}

func (c *Connection) Obsolete() bool {
	if c.Container != "" && c.Instance.Pod != nil && c.Instance.Pod.InitContainers[c.Container] != nil {
		return false
	}
	return c.RemoteInstance.Pod != nil && c.RemoteInstance.Pod.IsObsolete()
}

func (c *Connection) Status() Status {
	status := UNKNOWN
	if c.IsActual() && !timeseries.IsEmpty(c.Rtt) {
		status = OK
		if math.IsNaN(timeseries.Last(c.Rtt)) {
			status = WARNING
		}
	}
	return status
}
