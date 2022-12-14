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
	Samples timeseries.TimeSeries
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
	Calls     timeseries.TimeSeries
	TotalTime timeseries.TimeSeries
	IoTime    timeseries.TimeSeries
}

type Postgres struct {
	Up timeseries.TimeSeries

	Version LabelLastValue

	Connections                   map[PgConnectionKey]timeseries.TimeSeries
	AwaitingQueriesByLockingQuery map[QueryKey]timeseries.TimeSeries

	Settings map[string]PgSetting

	PerQuery    map[QueryKey]*QueryStat
	QueriesByDB map[string]timeseries.TimeSeries

	Avg timeseries.TimeSeries
	P50 timeseries.TimeSeries
	P95 timeseries.TimeSeries
	P99 timeseries.TimeSeries

	WalCurrentLsn timeseries.TimeSeries
	WalReceiveLsn timeseries.TimeSeries
	WalReplyLsn   timeseries.TimeSeries
}

func NewPostgres() *Postgres {
	return &Postgres{
		Connections:                   map[PgConnectionKey]timeseries.TimeSeries{},
		AwaitingQueriesByLockingQuery: map[QueryKey]timeseries.TimeSeries{},
		Settings:                      map[string]PgSetting{},
		PerQuery:                      map[QueryKey]*QueryStat{},
		QueriesByDB:                   map[string]timeseries.TimeSeries{},
	}
}

func (p *Postgres) IsUp() bool {
	return timeseries.Last(p.Up) > 0
}

func (p *Postgres) Unavailability() timeseries.TimeSeries {
	if timeseries.IsEmpty(p.Up) {
		return nil
	}
	return timeseries.Map(func(t timeseries.Time, v float64) float64 {
		if v != 1 {
			return 1
		}
		return 0
	}, p.Up)
}
