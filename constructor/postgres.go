package constructor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func postgres(instance *model.Instance, queryName string, m *model.MetricValues) {
	if instance == nil {
		return
	}
	if instance.Postgres == nil {
		instance.Postgres = model.NewPostgres(false)
	}
	if instance.Postgres.InternalExporter != metricFromInternalExporter(m.Labels) {
		return
	}

	pg := instance.Postgres
	ls := m.Labels
	values := m.Values
	switch queryName {
	case "pg_up":
		pg.Up = merge(pg.Up, values, timeseries.Any)
	case "pg_scrape_error":
		pg.Error.Update(values, ls["error"])
		pg.Warning.Update(values, ls["warning"])
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
		pg.Connections[key] = merge(pg.Connections[key], values, timeseries.Any)
	case "pg_setting":
		pg.Settings[ls["name"]] = model.PgSetting{
			Unit:    ls["unit"],
			Samples: merge(pg.Settings[ls["name"]].Samples, values, timeseries.Any),
		}
	case "pg_lock_awaiting_queries":
		key := model.QueryKey{
			Db:    ls["db"],
			User:  ls["user"],
			Query: ls["blocking_query"],
		}
		pg.AwaitingQueriesByLockingQuery[key] = merge(pg.AwaitingQueriesByLockingQuery[key], values, timeseries.Any)
	case "pg_db_queries_per_second":
		db := ls["db"]
		pg.QueriesByDB[db] = merge(pg.QueriesByDB[db], values, timeseries.Any)
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
			qs.Calls = merge(qs.Calls, values, timeseries.Any)
		case "pg_top_query_time_per_second":
			qs.TotalTime = merge(qs.TotalTime, values, timeseries.Any)
		case "pg_top_query_io_time_per_second":
			qs.IoTime = merge(qs.IoTime, values, timeseries.Any)
		}
	case "pg_latency_seconds":
		switch ls["summary"] {
		case "avg":
			pg.Avg = merge(pg.Avg, values, timeseries.Any)
		case "p50":
			pg.P50 = merge(pg.P50, values, timeseries.Any)
		case "p95":
			pg.P95 = merge(pg.P95, values, timeseries.Any)
		case "p99":
			pg.P99 = merge(pg.P99, values, timeseries.Any)
		}
	case "pg_wal_current_lsn":
		pg.WalCurrentLsn = merge(pg.WalCurrentLsn, values, timeseries.Any)
	case "pg_wal_receive_lsn":
		pg.WalReceiveLsn = merge(pg.WalReceiveLsn, values, timeseries.Any)
	case "pg_wal_reply_lsn":
		pg.WalReplayLsn = merge(pg.WalReplayLsn, values, timeseries.Any)
	}
}
