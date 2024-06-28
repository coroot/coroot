package model

import (
	"fmt"

	"github.com/coroot/coroot/timeseries"
)

type MysqlQueryKey struct {
	Schema string
	Query  string
}

func (k MysqlQueryKey) String() string {
	if k.Schema == "" {
		return k.Query
	}
	return fmt.Sprintf("%s: %s", k.Schema, k.Query)
}

type MysqlQueryStat struct {
	Calls     *timeseries.TimeSeries
	TotalTime *timeseries.TimeSeries
	LockTime  *timeseries.TimeSeries
}

type MysqlReplicationStatus struct {
	LastError LabelLastValue
	LastState LabelLastValue
	Status    *timeseries.TimeSeries
}

type MysqlTable struct {
	Schema string
	Table  string
}

func (k MysqlTable) String() string {
	return fmt.Sprintf("%s:%s", k.Schema, k.Table)
}

type MysqlTableIOStats struct {
	ReadTimePerSecond  *timeseries.TimeSeries
	WriteTimePerSecond *timeseries.TimeSeries
}

type Mysql struct {
	InternalExporter bool

	Up         *timeseries.TimeSeries
	ServerUUID LabelLastValue
	Error      LabelLastValue
	Warning    LabelLastValue
	Version    LabelLastValue
	PerQuery   map[MysqlQueryKey]*MysqlQueryStat

	ReplicationSourceUUID LabelLastValue
	ReplicationIOStatus   *MysqlReplicationStatus
	ReplicationSQLStatus  *MysqlReplicationStatus
	ReplicationLagSeconds *timeseries.TimeSeries

	ConnectionsMax     *timeseries.TimeSeries
	ConnectionsCurrent *timeseries.TimeSeries
	ConnectionsNew     *timeseries.TimeSeries
	ConnectionsAborted *timeseries.TimeSeries

	BytesSent     *timeseries.TimeSeries
	BytesReceived *timeseries.TimeSeries

	Queries     *timeseries.TimeSeries
	SlowQueries *timeseries.TimeSeries

	TablesIOTime map[MysqlTable]*MysqlTableIOStats
}

func NewMysql(internalExporter bool) *Mysql {
	return &Mysql{
		InternalExporter: internalExporter,
		PerQuery:         map[MysqlQueryKey]*MysqlQueryStat{},
		TablesIOTime:     map[MysqlTable]*MysqlTableIOStats{},
	}
}

func (r *Mysql) IsUp() bool {
	return r.Up.Last() > 0
}
