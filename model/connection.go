package model

import (
	"strings"

	"github.com/coroot/coroot/timeseries"
)

type Protocol string

const (
	ProtocolHttp         Protocol = "http"
	ProtocolPostgres     Protocol = "postgres"
	ProtocolMongodb      Protocol = "mongo"
	ProtocolRedis        Protocol = "redis"
	ProtocolMysql        Protocol = "mysql"
	ProtocolMemcached    Protocol = "memcached"
	ProtocolKafka        Protocol = "kafka"
	ProtocolCassandra    Protocol = "cassandra"
	ProtocolRabbitmq     Protocol = "rabbitmq"
	ProtocolNats         Protocol = "nats"
	ProtocolClickhouse   Protocol = "clickhouse"
	ProtocolZookeeper    Protocol = "zookeeper"
	ProtocolFoundationdb Protocol = "foundationdb"
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

type AppToAppConnection struct {
	RemoteApplication *Application
	Application       *Application

	Rtt *timeseries.TimeSeries

	SuccessfulConnections *timeseries.TimeSeries
	Active                *timeseries.TimeSeries
	FailedConnections     *timeseries.TimeSeries
	ConnectionTime        *timeseries.TimeSeries
	BytesSent             *timeseries.TimeSeries
	BytesReceived         *timeseries.TimeSeries

	Retransmissions *timeseries.TimeSeries

	RequestsCount   map[Protocol]map[string]*timeseries.TimeSeries // by status
	RequestsLatency map[Protocol]*timeseries.TimeSeries
}

type Connection struct {
	ActualRemotePort string
	ActualRemoteIP   string
	Instance         *Instance
	RemoteInstance   *Instance

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
	remoteApplication *Application

	ServiceRemoteIP   string
	ServiceRemotePort string
}

func (c *Connection) RemoteApplication() *Application {
	if c.RemoteInstance != nil {
		return c.RemoteInstance.Owner
	}
	return nil
}

func (c *AppToAppConnection) IsActual() bool {
	return (c.SuccessfulConnections.Last() > 0) || (c.Active.Last() > 0) || c.FailedConnections.Last() > 0
}

func (c *AppToAppConnection) HasConnectivityIssues() bool {
	if !c.IsActual() {
		return false
	}
	return !c.Rtt.IsEmpty() && c.Rtt.TailIsEmpty()
}

func (c *AppToAppConnection) HasFailedConnectionAttempts() bool {
	if !c.IsActual() {
		return false
	}
	return c.FailedConnections.Last() > 0
}

func (c *AppToAppConnection) Status() (Status, string) {
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

func (c *AppToAppConnection) GetConnectionsRequestsSum(protocolFilter func(protocol Protocol) bool) *timeseries.TimeSeries {
	sum := timeseries.NewAggregate(timeseries.NanSum)
	for protocol, byStatus := range c.RequestsCount {
		if protocolFilter != nil && !protocolFilter(protocol) {
			continue
		}
		for _, ts := range byStatus {
			sum.Add(ts)
		}
	}
	return sum.Get()
}

func (c *AppToAppConnection) GetConnectionsErrorsSum(protocolFilter func(protocol Protocol) bool) *timeseries.TimeSeries {
	sum := timeseries.NewAggregate(timeseries.NanSum)
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
	return sum.Get()
}

func (c *AppToAppConnection) GetConnectionsRequestsLatency(protocolFilter func(protocol Protocol) bool) *timeseries.TimeSeries {
	time := timeseries.NewAggregate(timeseries.NanSum)
	count := timeseries.NewAggregate(timeseries.NanSum)
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
		time.Add(latency)
		count.Add(req)
	}
	return timeseries.Div(time.Get(), count.Get())
}

func IsRequestStatusFailed(status string) bool {
	return status == "failed" || strings.HasPrefix(status, "5")
}
