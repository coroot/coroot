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
	Calls        *timeseries.TimeSeries
	TotalTime    *timeseries.TimeSeries
	LockTime     *timeseries.TimeSeries
	RowsExamined *timeseries.TimeSeries
	RowsSent     *timeseries.TimeSeries
}

type MysqlReplicationStatus struct {
	LastError LabelLastValue
	LastState LabelLastValue
	Status    *timeseries.TimeSeries
}

type MysqlTableIOStats struct {
	ReadTimePerSecond  *timeseries.TimeSeries
	WriteTimePerSecond *timeseries.TimeSeries
}

type Mysql struct {
	Up         *timeseries.TimeSeries
	ServerUUID LabelLastValue
	Error      LabelLastValue
	Warning    LabelLastValue
	Version    LabelLastValue
	PerQuery   map[MysqlQueryKey]*MysqlQueryStat

	LockedQueries                  map[MysqlQueryKey]*timeseries.TimeSeries
	AwaitingQueriesByBlockingQuery map[MysqlQueryKey]*timeseries.TimeSeries

	ReplicationSourceUUID LabelLastValue
	ReplicationIOStatus   *MysqlReplicationStatus
	ReplicationSQLStatus  *MysqlReplicationStatus
	ReplicationLagSeconds *timeseries.TimeSeries

	ConnectionsMax                 *timeseries.TimeSeries
	ConnectionsCurrent             *timeseries.TimeSeries
	ConnectionsNew                 *timeseries.TimeSeries
	ConnectionsAborted             *timeseries.TimeSeries
	ConnectionErrorsMaxConnections *timeseries.TimeSeries

	ThreadsRunning       *timeseries.TimeSeries
	CreatedTmpDiskTables *timeseries.TimeSeries
	TableLocksWaited     *timeseries.TimeSeries
	TableLocksImmediate  *timeseries.TimeSeries

	BinlogSize          *timeseries.TimeSeries
	BinlogFiles         *timeseries.TimeSeries
	BinlogExpireSeconds *timeseries.TimeSeries
	UndoSize            *timeseries.TimeSeries

	InnodbTransactions map[string]*timeseries.TimeSeries

	Galera           *MysqlGalera
	GroupReplication *MysqlGroupReplication
	Innodb           *MysqlInnodb

	BytesSent     *timeseries.TimeSeries
	BytesReceived *timeseries.TimeSeries

	Queries     *timeseries.TimeSeries
	SlowQueries *timeseries.TimeSeries

	TablesIOTime map[DbTableKey]*MysqlTableIOStats

	DatabaseSize    map[string]*timeseries.TimeSeries
	TableSize       map[DbTableKey]*timeseries.TimeSeries
	TableSizeGrowth map[DbTableKey]*timeseries.TimeSeries
}

type MysqlGalera struct {
	ClusterStatus     LabelLastValue
	ClusterSize       *timeseries.TimeSeries
	LocalStateComment LabelLastValue
	Ready             *timeseries.TimeSeries
	Connected         *timeseries.TimeSeries

	FlowControlPaused *timeseries.TimeSeries
	LocalRecvQueue    *timeseries.TimeSeries
	LocalSendQueue    *timeseries.TimeSeries
	CertFailures      *timeseries.TimeSeries
	BfAborts          *timeseries.TimeSeries
}

type MysqlGroupReplication struct {
	State       LabelLastValue
	ClusterSize *timeseries.TimeSeries
	Online      *timeseries.TimeSeries

	TransactionsInQueue    *timeseries.TimeSeries
	TransactionsApplyQueue *timeseries.TimeSeries
	ConflictsDetected      *timeseries.TimeSeries
}

type MysqlInnodb struct {
	BufferPoolReadRequests  *timeseries.TimeSeries
	BufferPoolReads         *timeseries.TimeSeries
	BufferPoolWriteRequests *timeseries.TimeSeries
	BufferPoolPagesTotal    *timeseries.TimeSeries
	BufferPoolPagesFree     *timeseries.TimeSeries
	BufferPoolPagesDirty    *timeseries.TimeSeries
	BufferPoolPagesData     *timeseries.TimeSeries
	BufferPoolWaitFree      *timeseries.TimeSeries
	BufferPoolPagesFlushed  *timeseries.TimeSeries
	PageSize                *timeseries.TimeSeries

	Deadlocks         *timeseries.TimeSeries
	LockWaitTimeouts  *timeseries.TimeSeries
	HistoryListLength *timeseries.TimeSeries

	RowsRead     *timeseries.TimeSeries
	RowsInserted *timeseries.TimeSeries
	RowsUpdated  *timeseries.TimeSeries
	RowsDeleted  *timeseries.TimeSeries

	RowLockWaits        *timeseries.TimeSeries
	RowLockTime         *timeseries.TimeSeries
	RowLockCurrentWaits *timeseries.TimeSeries

	DataReads   *timeseries.TimeSeries
	DataWrites  *timeseries.TimeSeries
	DataRead    *timeseries.TimeSeries
	DataWritten *timeseries.TimeSeries
	DataFsyncs  *timeseries.TimeSeries

	LogWaits     *timeseries.TimeSeries
	OsLogWritten *timeseries.TimeSeries

	SortMergePasses *timeseries.TimeSeries

	Commits   *timeseries.TimeSeries
	Rollbacks *timeseries.TimeSeries
}

func NewMysql() *Mysql {
	return &Mysql{
		PerQuery:                       map[MysqlQueryKey]*MysqlQueryStat{},
		LockedQueries:                  map[MysqlQueryKey]*timeseries.TimeSeries{},
		AwaitingQueriesByBlockingQuery: map[MysqlQueryKey]*timeseries.TimeSeries{},
		TablesIOTime:                   map[DbTableKey]*MysqlTableIOStats{},
		DatabaseSize:                   map[string]*timeseries.TimeSeries{},
		TableSize:                      map[DbTableKey]*timeseries.TimeSeries{},
		TableSizeGrowth:                map[DbTableKey]*timeseries.TimeSeries{},
		InnodbTransactions:             map[string]*timeseries.TimeSeries{},
	}
}

func (r *Mysql) IsUp() bool {
	return r.Up.Last() > 0
}
