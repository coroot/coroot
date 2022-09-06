package constructor

import (
	"github.com/coroot/coroot/model"
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
		pg.Up = update(pg.Up, values)
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
		pg.Connections[key] = update(pg.Connections[key], values)
	case "pg_setting":
		pg.Settings[ls["name"]] = model.PgSetting{
			Unit:    ls["unit"],
			Samples: update(pg.Settings[ls["name"]].Samples, values),
		}
	case "pg_lock_awaiting_queries":
		key := model.QueryKey{
			Db:    ls["db"],
			User:  ls["user"],
			Query: ls["blocking_query"],
		}
		pg.AwaitingQueriesByLockingQuery[key] = update(pg.AwaitingQueriesByLockingQuery[key], values)
	case "pg_db_queries_per_second":
		db := ls["db"]
		pg.QueriesByDB[db] = update(pg.QueriesByDB[db], values)
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
			qs.Calls = update(qs.Calls, values)
		case "pg_top_query_time_per_second":
			qs.TotalTime = update(qs.TotalTime, values)
		case "pg_top_query_io_time_per_second":
			qs.IoTime = update(qs.IoTime, values)
		}
	case "pg_latency_seconds":
		switch ls["summary"] {
		case "avg":
			pg.Avg = update(pg.Avg, values)
		case "p50":
			pg.P50 = update(pg.P50, values)
		case "p95":
			pg.P95 = update(pg.P95, values)
		case "p99":
			pg.P99 = update(pg.P99, values)
		}
	case "pg_wal_current_lsn":
		pg.WalCurrentLsn = update(pg.WalCurrentLsn, values)
	case "pg_wal_receive_lsn":
		pg.WalReceiveLsn = update(pg.WalReceiveLsn, values)
	case "pg_wal_reply_lsn":
		pg.WalReplyLsn = update(pg.WalReplyLsn, values)
	}
}
