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

type Postgres struct {
	InternalExporter bool

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

	WalCurrentLsn *timeseries.TimeSeries
	WalReceiveLsn *timeseries.TimeSeries
	WalReplayLsn  *timeseries.TimeSeries
}

func NewPostgres(internalExporter bool) *Postgres {
	return &Postgres{
		InternalExporter:              internalExporter,
		Connections:                   map[PgConnectionKey]*timeseries.TimeSeries{},
		AwaitingQueriesByLockingQuery: map[QueryKey]*timeseries.TimeSeries{},
		Settings:                      map[string]PgSetting{},
		PerQuery:                      map[QueryKey]*QueryStat{},
		QueriesByDB:                   map[string]*timeseries.TimeSeries{},
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
