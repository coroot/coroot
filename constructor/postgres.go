package constructor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func postgres(w *model.World, queryName string, m model.MetricValues) {
	instance := findInstance(w, m.Labels, model.ApplicationTypePostgres)
	if instance == nil {
		return
	}
	if instance.Postgres == nil {
		instance.Postgres = model.NewPostgres()
	}
	pg := instance.Postgres
	ls := m.Labels
	values := m.Values
	switch queryName {
	case "pg_up":
		pg.Up = timeseries.Merge(pg.Up, values, timeseries.Any)
	case "pg_info":
		pg.Version.Update(values, ls["server_version"])
	case "pg_connections":
		key := model.PgConnectionKey{
			Db:            ls["db"],
			User:          ls["user"],
			State:         ls["state"],
			Query:         ls["query"],
			WaitEventType: ls["wait_event_type"],
		}
		if key.State == "" {
			return
		}
		pg.Connections[key] = timeseries.Merge(pg.Connections[key], values, timeseries.Any)
	case "pg_setting":
		pg.Settings[ls["name"]] = model.PgSetting{
			Unit:    ls["unit"],
			Samples: timeseries.Merge(pg.Settings[ls["name"]].Samples, values, timeseries.Any),
		}
	case "pg_lock_awaiting_queries":
		key := model.QueryKey{
			Db:    ls["db"],
			User:  ls["user"],
			Query: ls["blocking_query"],
		}
		pg.AwaitingQueriesByLockingQuery[key] = timeseries.Merge(pg.AwaitingQueriesByLockingQuery[key], values, timeseries.Any)
	case "pg_db_queries_per_second":
		db := ls["db"]
		pg.QueriesByDB[db] = timeseries.Merge(pg.QueriesByDB[db], values, timeseries.Any)
	case "pg_top_query_calls_per_second", "pg_top_query_time_per_second", "pg_top_query_io_time_per_second":
		key := model.QueryKey{
			Db:    ls["db"],
			User:  ls["user"],
			Query: ls["query"],
		}
		qs, ok := pg.PerQuery[key]
		if !ok {
			qs = &model.QueryStat{}
			pg.PerQuery[key] = qs
		}
		switch queryName {
		case "pg_top_query_calls_per_second":
			qs.Calls = timeseries.Merge(qs.Calls, values, timeseries.Any)
		case "pg_top_query_time_per_second":
			qs.TotalTime = timeseries.Merge(qs.TotalTime, values, timeseries.Any)
		case "pg_top_query_io_time_per_second":
			qs.IoTime = timeseries.Merge(qs.IoTime, values, timeseries.Any)
		}
	case "pg_latency_seconds":
		switch ls["summary"] {
		case "avg":
			pg.Avg = timeseries.Merge(pg.Avg, values, timeseries.Any)
		case "p50":
			pg.P50 = timeseries.Merge(pg.P50, values, timeseries.Any)
		case "p95":
			pg.P95 = timeseries.Merge(pg.P95, values, timeseries.Any)
		case "p99":
			pg.P99 = timeseries.Merge(pg.P99, values, timeseries.Any)
		}
	case "pg_wal_current_lsn":
		pg.WalCurrentLsn = timeseries.Merge(pg.WalCurrentLsn, values, timeseries.Any)
	case "pg_wal_receive_lsn":
		pg.WalReceiveLsn = timeseries.Merge(pg.WalReceiveLsn, values, timeseries.Any)
	case "pg_wal_reply_lsn":
		pg.WalReplyLsn = timeseries.Merge(pg.WalReplyLsn, values, timeseries.Any)
	}
}
