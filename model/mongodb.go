package model

import (
	"fmt"

	"github.com/coroot/coroot/timeseries"
)

type MongoQueryKey struct {
	DB         string
	Collection string
	Query      string
}

func (k MongoQueryKey) String() string {
	return fmt.Sprintf("%s.%s: %s", k.DB, k.Collection, k.Query)
}

type MongoQueryStat struct {
	Calls        *timeseries.TimeSeries
	TotalTime    *timeseries.TimeSeries
	DocsReturned *timeseries.TimeSeries
	DocsExamined *timeseries.TimeSeries
	KeysExamined *timeseries.TimeSeries
}

type MongoLongRunningOpKey struct {
	DB         string
	Collection string
	Query      string
	Plan       string
}

type MongoRsMemberConfig struct {
	Arbiter bool
	Votes   int64
}

type Mongodb struct {
	Up          *timeseries.TimeSeries
	Error       LabelLastValue
	Warning     LabelLastValue
	ReplicaSet  LabelLastValue
	State       LabelLastValue
	Version     LabelLastValue
	Flavor      LabelLastValue
	LastApplied *timeseries.TimeSeries

	MemberConfigs                      map[string]*MongoRsMemberConfig
	WriteConcernMajorityJournalDefault LabelLastValue

	OplogWindow   *timeseries.TimeSeries
	OplogMaxSize  *timeseries.TimeSeries
	OplogUsedSize *timeseries.TimeSeries

	ConnectionsCurrent  *timeseries.TimeSeries
	ConnectionsActive   *timeseries.TimeSeries
	ConnectionsMax      *timeseries.TimeSeries
	ConnectionsCreated  *timeseries.TimeSeries
	ConnectionsRejected *timeseries.TimeSeries
	ConnectionsByApp    map[string]*timeseries.TimeSeries

	Opcounters        map[string]*timeseries.TimeSeries
	DocumentsReturned *timeseries.TimeSeries
	OpLatencyTime     map[string]*timeseries.TimeSeries
	OpLatencyOps      map[string]*timeseries.TimeSeries
	QueuedOperations  map[string]*timeseries.TimeSeries
	TicketsAvailable  map[string]*timeseries.TimeSeries

	CacheUsed                   *timeseries.TimeSeries
	CacheDirty                  *timeseries.TimeSeries
	CacheMax                    *timeseries.TimeSeries
	CacheEvictedPagesByApp      *timeseries.TimeSeries
	CacheAppEvictingTime        *timeseries.TimeSeries
	CacheBytesReadInto          *timeseries.TimeSeries
	Checkpoints                 *timeseries.TimeSeries
	CheckpointTime              *timeseries.TimeSeries
	JournalBytesWritten         *timeseries.TimeSeries
	JournalBytesSinceCheckpoint *timeseries.TimeSeries
	TimeSinceCheckpoint         *timeseries.TimeSeries

	ScannedKeys      *timeseries.TimeSeries
	ScannedDocuments *timeseries.TimeSeries
	CollectionScans  *timeseries.TimeSeries
	ScanAndOrder     *timeseries.TimeSeries
	TtlDeleted       *timeseries.TimeSeries
	WriteConflicts   *timeseries.TimeSeries

	ReplApplyOps    *timeseries.TimeSeries
	ReplBufferCount *timeseries.TimeSeries
	ReplBufferBytes *timeseries.TimeSeries

	FsyncLocked           *timeseries.TimeSeries
	PreparedTransactions  *timeseries.TimeSeries
	OpenTransactionsByApp map[string]*timeseries.TimeSeries

	CursorsOpen          *timeseries.TimeSeries
	CursorsOpenNoTimeout *timeseries.TimeSeries
	CursorsTimedOut      *timeseries.TimeSeries

	FlowControlTime *timeseries.TimeSeries

	ProfilingLevel map[string]*timeseries.TimeSeries

	PerQuery                 map[MongoQueryKey]*MongoQueryStat
	OperationsWaitingForLock map[string]*timeseries.TimeSeries
	LongRunningOperations    map[MongoLongRunningOpKey]*timeseries.TimeSeries

	DatabaseSize          map[string]*timeseries.TimeSeries
	CollectionSize        map[DbTableKey]*timeseries.TimeSeries
	CollectionStorageSize map[DbTableKey]*timeseries.TimeSeries
	CollectionFreeStorage map[DbTableKey]*timeseries.TimeSeries
	CollectionDocuments   map[DbTableKey]*timeseries.TimeSeries
}

func NewMongodb() *Mongodb {
	return &Mongodb{
		ProfilingLevel:           map[string]*timeseries.TimeSeries{},
		MemberConfigs:            map[string]*MongoRsMemberConfig{},
		ConnectionsByApp:         map[string]*timeseries.TimeSeries{},
		OpenTransactionsByApp:    map[string]*timeseries.TimeSeries{},
		Opcounters:               map[string]*timeseries.TimeSeries{},
		OpLatencyTime:            map[string]*timeseries.TimeSeries{},
		OpLatencyOps:             map[string]*timeseries.TimeSeries{},
		QueuedOperations:         map[string]*timeseries.TimeSeries{},
		TicketsAvailable:         map[string]*timeseries.TimeSeries{},
		PerQuery:                 map[MongoQueryKey]*MongoQueryStat{},
		OperationsWaitingForLock: map[string]*timeseries.TimeSeries{},
		LongRunningOperations:    map[MongoLongRunningOpKey]*timeseries.TimeSeries{},
		DatabaseSize:             map[string]*timeseries.TimeSeries{},
		CollectionSize:           map[DbTableKey]*timeseries.TimeSeries{},
		CollectionStorageSize:    map[DbTableKey]*timeseries.TimeSeries{},
		CollectionFreeStorage:    map[DbTableKey]*timeseries.TimeSeries{},
		CollectionDocuments:      map[DbTableKey]*timeseries.TimeSeries{},
	}
}

func (m *Mongodb) IsUp() bool {
	return m.Up.Last() > 0
}
