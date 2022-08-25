package model

import (
	"fmt"
	"github.com/coroot/coroot-focus/timeseries"
)

type PgConnectionKey struct {
	Db            string
	User          string
	State         string
	Query         string
	WaitEventType string
}

func (k PgConnectionKey) String() string {
	return fmt.Sprintf(`%s: %s`, k.Db, k.Query)
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
	return p.Up != nil && p.Up.Last() > 0
}

func (p *Postgres) Unavailability() timeseries.TimeSeries {
	if p.Up == nil {
		return nil
	}
	return timeseries.Map(func(v float64) float64 {
		if v != 1 {
			return 1
		}
		return 0
	}, p.Up)
}
