package model

import (
	"fmt"

	"github.com/coroot/coroot/timeseries"
)

type PgConnectionKey struct {
	Db            string
	User          string
	State         string
	Query         string
	WaitEventType string
}

func (k PgConnectionKey) String() string {
	return fmt.Sprintf("%s@%s: %s", k.User, k.Db, k.Query)
}

type PgSetting struct {
	Samples *timeseries.TimeSeries
	Unit    string
}

type QueryKey struct {
	Db    string
	User  string
	Query string
}

func (k QueryKey) String() string {
	return fmt.Sprintf("%s@%s: %s", k.User, k.Db, k.Query)
}

type QueryStat struct {
	Calls     *timeseries.TimeSeries
	TotalTime *timeseries.TimeSeries
	IoTime    *timeseries.TimeSeries
}

type DbTableKey struct {
	Db    string
	Table string
}

func (k DbTableKey) String() string {
	return fmt.Sprintf("%s.%s", k.Db, k.Table)
}

type DbIndexKey struct {
	Db    string
	Table string
	Index string
}

func (k DbIndexKey) String() string {
	return fmt.Sprintf("%s.%s", k.Table, k.Index)
}

type PgReplicationSlot struct {
	Active      LabelLastValue
	WalStatus   LabelLastValue
	RetainedWal *timeseries.TimeSeries
}

type PgBackups struct {
	Schedule            string
	NextScheduledBackup timeseries.Time
	RetentionPolicy     string
	Methods             map[string]*PgBackupMethod
	LastFailedBackup    timeseries.Time
	Conditions          map[string]PgBackupCondition
	Runs                []*PgBackupRun
}

type PgBackupMethod struct {
	Destination              string
	Endpoint                 string
	Schedule                 string
	LastSuccessfulBackup     timeseries.Time
	FirstRecoverabilityPoint timeseries.Time
}

type PgBackupRun struct {
	Name        string
	Method      string
	Kind        string
	Destination string
	Status      string
	CompletedAt timeseries.Time
}

func (r *PgBackupRun) Succeeded() bool {
	switch r.Status {
	case "Succeeded", "completed":
		return true
	}
	return false
}

type PgBackupCondition struct {
	Status string
	Reason string
}

type Postgres struct {
	Up *timeseries.TimeSeries

	Error   LabelLastValue
	Warning LabelLastValue

	Version LabelLastValue

	Connections                   map[PgConnectionKey]*timeseries.TimeSeries
	AwaitingQueriesByLockingQuery map[QueryKey]*timeseries.TimeSeries

	Settings map[string]PgSetting

	PerQuery    map[QueryKey]*QueryStat
	QueriesByDB map[string]*timeseries.TimeSeries

	Avg *timeseries.TimeSeries
	P50 *timeseries.TimeSeries
	P95 *timeseries.TimeSeries
	P99 *timeseries.TimeSeries

	WalCurrentLsn       *timeseries.TimeSeries
	WalThroughput       *timeseries.TimeSeries
	WalReceiveLsn       *timeseries.TimeSeries
	WalReplayLsn        *timeseries.TimeSeries
	WalReplayPaused     *timeseries.TimeSeries
	WalReceiverStatus   *timeseries.TimeSeries
	WalSize             *timeseries.TimeSeries
	WalArchivedSegments *timeseries.TimeSeries
	WalArchiveFailures  *timeseries.TimeSeries
	WalArchivingStatus  *timeseries.TimeSeries

	ReplicationSlots map[string]*PgReplicationSlot

	XidAge        map[string]*timeseries.TimeSeries
	MultixactAge  map[string]*timeseries.TimeSeries
	OldestXminAge map[string]*timeseries.TimeSeries

	CheckpointsScheduledByType map[string]*timeseries.TimeSeries
	Checkpoints                *timeseries.TimeSeries
	Restartpoints              *timeseries.TimeSeries
	TimeSinceLastCheckpoint    *timeseries.TimeSeries
	WalSinceLastCheckpoint     *timeseries.TimeSeries
	BuffersWrittenBySource     map[string]*timeseries.TimeSeries

	DatabaseSize map[string]*timeseries.TimeSeries
	TableSize    map[DbTableKey]*timeseries.TimeSeries

	DatabaseTableBloat map[string]*timeseries.TimeSeries
	DatabaseIndexBloat map[string]*timeseries.TimeSeries
	TableBloat         map[DbTableKey]*timeseries.TimeSeries
	IndexBloat         map[DbIndexKey]*timeseries.TimeSeries

	TableDeadTupleBytes         map[DbTableKey]*timeseries.TimeSeries
	TableDeadTuples             map[DbTableKey]*timeseries.TimeSeries
	TableLiveTuples             map[DbTableKey]*timeseries.TimeSeries
	TableSecondsSinceAutovacuum map[DbTableKey]*timeseries.TimeSeries
	TableVacuumInProgress       map[DbTableKey]*timeseries.TimeSeries
	TableVacuumThrottled        map[DbTableKey]*timeseries.TimeSeries

	TableModsSinceAnalyze    map[DbTableKey]*timeseries.TimeSeries
	TableReltuples           map[DbTableKey]*timeseries.TimeSeries
	TableSecondsSinceAnalyze map[DbTableKey]*timeseries.TimeSeries

	TableSettings map[DbTableKey]map[string]float32

	AutovacuumWorkers *timeseries.TimeSeries
}

func NewPostgres() *Postgres {
	return &Postgres{
		Connections:                   map[PgConnectionKey]*timeseries.TimeSeries{},
		AwaitingQueriesByLockingQuery: map[QueryKey]*timeseries.TimeSeries{},
		Settings:                      map[string]PgSetting{},
		PerQuery:                      map[QueryKey]*QueryStat{},
		QueriesByDB:                   map[string]*timeseries.TimeSeries{},
		CheckpointsScheduledByType:    map[string]*timeseries.TimeSeries{},
		ReplicationSlots:              map[string]*PgReplicationSlot{},
		XidAge:                        map[string]*timeseries.TimeSeries{},
		MultixactAge:                  map[string]*timeseries.TimeSeries{},
		OldestXminAge:                 map[string]*timeseries.TimeSeries{},
		BuffersWrittenBySource:        map[string]*timeseries.TimeSeries{},
		DatabaseSize:                  map[string]*timeseries.TimeSeries{},
		TableSize:                     map[DbTableKey]*timeseries.TimeSeries{},
		DatabaseTableBloat:            map[string]*timeseries.TimeSeries{},
		DatabaseIndexBloat:            map[string]*timeseries.TimeSeries{},
		TableBloat:                    map[DbTableKey]*timeseries.TimeSeries{},
		IndexBloat:                    map[DbIndexKey]*timeseries.TimeSeries{},
		TableDeadTupleBytes:           map[DbTableKey]*timeseries.TimeSeries{},
		TableDeadTuples:               map[DbTableKey]*timeseries.TimeSeries{},
		TableLiveTuples:               map[DbTableKey]*timeseries.TimeSeries{},
		TableSecondsSinceAutovacuum:   map[DbTableKey]*timeseries.TimeSeries{},
		TableVacuumInProgress:         map[DbTableKey]*timeseries.TimeSeries{},
		TableVacuumThrottled:          map[DbTableKey]*timeseries.TimeSeries{},
		TableModsSinceAnalyze:         map[DbTableKey]*timeseries.TimeSeries{},
		TableReltuples:                map[DbTableKey]*timeseries.TimeSeries{},
		TableSecondsSinceAnalyze:      map[DbTableKey]*timeseries.TimeSeries{},
		TableSettings:                 map[DbTableKey]map[string]float32{},
	}
}

func (p *Postgres) IsUp() bool {
	return p.Up.Last() > 0
}

func (p *Postgres) Unavailability() *timeseries.TimeSeries {
	if p.Up.IsEmpty() {
		return nil
	}
	return p.Up.Map(func(t timeseries.Time, v float32) float32 {
		if v != 1 {
			return 1
		}
		return 0
	})
}
