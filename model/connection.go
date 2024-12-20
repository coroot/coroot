package model

import (
	"strings"

	"github.com/coroot/coroot/timeseries"
)

type Protocol string

const (
	ProtocolHttp       Protocol = "http"
	ProtocolPostgres   Protocol = "postgres"
	ProtocolMongodb    Protocol = "mongo"
	ProtocolRedis      Protocol = "redis"
	ProtocolMysql      Protocol = "mysql"
	ProtocolMemcached  Protocol = "memcached"
	ProtocolKafka      Protocol = "kafka"
	ProtocolCassandra  Protocol = "cassandra"
	ProtocolRabbitmq   Protocol = "rabbitmq"
	ProtocolNats       Protocol = "nats"
	ProtocolClickhouse Protocol = "clickhouse"
	ProtocolZookeeper  Protocol = "zookeeper"
)

func (p Protocol) ToApplicationType() ApplicationType {
	switch p {
	case ProtocolPostgres:
		return ApplicationTypePostgres
	case ProtocolRedis:
		return ApplicationTypeRedis
	case ProtocolMongodb:
		return ApplicationTypeMongodb
	case ProtocolMysql:
		return ApplicationTypeMysql
	case ProtocolMemcached:
		return ApplicationTypeMemcached
	}
	return ApplicationTypeUnknown
}

type ConnectionKey struct {
	Destination       string
	ActualDestination string
}

type Connection struct {
	ActualRemotePort string
	ActualRemoteIP   string
	Instance         *Instance
	RemoteInstance   *Instance

	Container string

	Rtt *timeseries.TimeSeries

	SuccessfulConnections *timeseries.TimeSeries
	Active                *timeseries.TimeSeries
	FailedConnections     *timeseries.TimeSeries
	ConnectionTime        *timeseries.TimeSeries
	BytesSent             *timeseries.TimeSeries
	BytesReceived         *timeseries.TimeSeries

	Retransmissions *timeseries.TimeSeries

	RequestsCount     map[Protocol]map[string]*timeseries.TimeSeries // by status
	RequestsLatency   map[Protocol]*timeseries.TimeSeries
	RequestsHistogram map[Protocol]map[float32]*timeseries.TimeSeries // by le

	Service           *Service
	RemoteApplication *Application

	ServiceRemoteIP   string
	ServiceRemotePort string
}

func (c *Connection) IsActual() bool {
	if c.IsObsolete() {
		return false
	}
	return (c.SuccessfulConnections.Last() > 0) || (c.Active.Last() > 0) || c.FailedConnections.Last() > 0
}

func (c *Connection) IsEmpty() bool {
	switch {
	case c.Active.Reduce(timeseries.NanSum) > 0:
	case c.SuccessfulConnections.Reduce(timeseries.NanSum) > 0:
	case c.FailedConnections.Reduce(timeseries.NanSum) > 0:
	default:
		return true
	}
	return false
}

func (c *Connection) IsObsolete() bool {
	if c.Container != "" && c.Instance.Pod != nil && c.Instance.Pod.InitContainers[c.Container] != nil {
		return false
	}
	return (c.RemoteInstance != nil && c.RemoteInstance.IsObsolete()) || (c.Instance != nil && c.Instance.IsObsolete())
}

func (c *Connection) HasConnectivityIssues() bool {
	if !c.IsActual() {
		return false
	}
	return !c.Rtt.IsEmpty() && c.Rtt.TailIsEmpty()
}

func (c *Connection) HasFailedConnectionAttempts() bool {
	if !c.IsActual() {
		return false
	}
	return c.FailedConnections.Last() > 0
}

func (c *Connection) Status() (Status, string) {
	if !c.IsActual() {
		return UNKNOWN, ""
	}
	status := OK
	switch {
	case c.HasConnectivityIssues():
		return CRITICAL, "connectivity issues"
	case c.HasFailedConnectionAttempts():
		return CRITICAL, "failed connections"
	}
	return status, ""
}

func IsRequestStatusFailed(status string) bool {
	return status == "failed" || strings.HasPrefix(status, "5")
}

func GetConnectionsRequestsSum(connections []*Connection, protocolFilter func(protocol Protocol) bool) *timeseries.TimeSeries {
	sum := timeseries.NewAggregate(timeseries.NanSum)
	for _, c := range connections {
		for protocol, byStatus := range c.RequestsCount {
			if protocolFilter != nil && !protocolFilter(protocol) {
				continue
			}
			for _, ts := range byStatus {
				sum.Add(ts)
			}
		}
	}
	return sum.Get()
}

func GetConnectionsErrorsSum(connections []*Connection, protocolFilter func(protocol Protocol) bool) *timeseries.TimeSeries {
	sum := timeseries.NewAggregate(timeseries.NanSum)
	for _, c := range connections {
		for protocol, byStatus := range c.RequestsCount {
			if protocolFilter != nil && !protocolFilter(protocol) {
				continue
			}
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

func GetConnectionsRequestsLatency(connections []*Connection, protocolFilter func(protocol Protocol) bool) *timeseries.TimeSeries {
	time := timeseries.NewAggregate(timeseries.NanSum)
	count := timeseries.NewAggregate(timeseries.NanSum)
	for _, c := range connections {
		for protocol, latency := range c.RequestsLatency {
			if protocolFilter != nil && !protocolFilter(protocol) {
				continue
			}
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
