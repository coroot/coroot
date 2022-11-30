package model

import (
	"github.com/coroot/coroot/timeseries"
	"math"
	"sort"
	"strings"
)

type Protocol string

type Connection struct {
	ActualRemotePort string
	ActualRemoteIP   string
	Instance         *Instance
	RemoteInstance   *Instance

	Container string

	Rtt timeseries.TimeSeries

	Connects timeseries.TimeSeries
	Active   timeseries.TimeSeries

	RequestsCount     map[Protocol]map[string]timeseries.TimeSeries // by status
	RequestsLatency   map[Protocol]timeseries.TimeSeries
	RequestsHistogram map[Protocol]map[float64]timeseries.TimeSeries // by le

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

func IsRequestStatusFailed(status string) bool {
	return status == "failed" || strings.HasPrefix(status, "5")
}

func GetConnectionsRequestsSum(connections []*Connection) timeseries.TimeSeries {
	var sum timeseries.TimeSeries
	for _, c := range connections {
		for _, byStatus := range c.RequestsCount {
			for _, ts := range byStatus {
				sum = timeseries.Merge(sum, ts, timeseries.NanSum)
			}
		}
	}
	return sum
}

func GetConnectionsErrorsSum(connections []*Connection) timeseries.TimeSeries {
	var sum timeseries.TimeSeries
	for _, c := range connections {
		for _, byStatus := range c.RequestsCount {
			for status, ts := range byStatus {
				if !IsRequestStatusFailed(status) {
					continue
				}
				sum = timeseries.Merge(sum, ts, timeseries.NanSum)
			}
		}
	}
	return sum
}

func GetConnectionsRequestsLatency(connections []*Connection) timeseries.TimeSeries {
	var time, count timeseries.TimeSeries
	for _, c := range connections {
		for protocol, latency := range c.RequestsLatency {
			if len(c.RequestsCount[protocol]) == 0 {
				continue
			}
			var requests timeseries.TimeSeries
			for _, ts := range c.RequestsCount[protocol] {
				requests = timeseries.Merge(requests, ts, timeseries.NanSum)
			}
			time = timeseries.Merge(time, timeseries.Aggregate(timeseries.Mul, latency, requests), timeseries.NanSum)
			count = timeseries.Merge(count, requests, timeseries.NanSum)
		}
	}
	return timeseries.Aggregate(timeseries.Div, time, count)
}

func GetConnectionsRequestsHistogram(connections []*Connection) []HistogramBucket {
	sum := map[float64]timeseries.TimeSeries{}
	for _, c := range connections {
		for _, histogram := range c.RequestsHistogram {
			for le, ts := range histogram {
				sum[le] = timeseries.Merge(sum[le], ts, timeseries.NanSum)
			}
		}
	}
	buckets := make([]HistogramBucket, 0, len(sum))
	for le, ts := range sum {
		buckets = append(buckets, HistogramBucket{Le: le, TimeSeries: ts})
	}
	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].Le < buckets[j].Le
	})
	return buckets
}
