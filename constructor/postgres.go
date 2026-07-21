package constructor

import (
	"strconv"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func postgres(instance *model.Instance, queryName string, m *model.MetricValues, pjs promJobStatuses) {
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
	case "pg_wal_throughput":
		pg.WalThroughput = merge(pg.WalThroughput, values, timeseries.Any)
	case "pg_wal_receive_lsn":
		pg.WalReceiveLsn = merge(pg.WalReceiveLsn, values, timeseries.Any)
	case "pg_wal_reply_lsn":
		pg.WalReplayLsn = merge(pg.WalReplayLsn, values, timeseries.Any)
	case "pg_wal_replay_paused":
		pg.WalReplayPaused = merge(pg.WalReplayPaused, values, timeseries.Any)
	case "pg_wal_receiver_status":
		pg.WalReceiverStatus = merge(pg.WalReceiverStatus, values, timeseries.Any)
	case "pg_wal_size_bytes":
		pg.WalSize = merge(pg.WalSize, values, timeseries.Any)
	case "pg_replication_slot_retained_wal_bytes":
		name := ls["slot"]
		slot := pg.ReplicationSlots[name]
		if slot == nil {
			slot = &model.PgReplicationSlot{}
			pg.ReplicationSlots[name] = slot
		}
		slot.RetainedWal = merge(slot.RetainedWal, values, timeseries.Any)
		slot.Active.Update(values, ls["active"])
		slot.WalStatus.Update(values, ls["wal_status"])
	case "pg_wal_archived_segments_total":
		pg.WalArchivedSegments = merge(pg.WalArchivedSegments, timeseries.Increase(values, pjs.get(ls)), timeseries.Any)
	case "pg_wal_archive_failures_total":
		pg.WalArchiveFailures = merge(pg.WalArchiveFailures, timeseries.Increase(values, pjs.get(ls)), timeseries.Any)
	case "pg_wal_archiving_status":
		pg.WalArchivingStatus = merge(pg.WalArchivingStatus, values, timeseries.Any)
	case "pg_xid_age":
		pg.XidAge[ls["db"]] = merge(pg.XidAge[ls["db"]], values, timeseries.Any)
	case "pg_multixact_age":
		pg.MultixactAge[ls["db"]] = merge(pg.MultixactAge[ls["db"]], values, timeseries.Any)
	case "pg_oldest_xmin_age":
		pg.OldestXminAge[ls["holder"]] = merge(pg.OldestXminAge[ls["holder"]], values, timeseries.Any)
	case "pg_database_size_bytes":
		db := ls["db"]
		pg.DatabaseSize[db] = merge(pg.DatabaseSize[db], values, timeseries.Any)
	case "pg_table_size_bytes":
		key := model.DbTableKey{Db: ls["db"], Table: ls["table"]}
		pg.TableSize[key] = merge(pg.TableSize[key], values, timeseries.Any)
	case "pg_db_table_bloat_bytes":
		pg.DatabaseTableBloat[ls["db"]] = merge(pg.DatabaseTableBloat[ls["db"]], values, timeseries.Any)
	case "pg_db_index_bloat_bytes":
		pg.DatabaseIndexBloat[ls["db"]] = merge(pg.DatabaseIndexBloat[ls["db"]], values, timeseries.Any)
	case "pg_table_bloat_bytes":
		key := model.DbTableKey{Db: ls["db"], Table: ls["table"]}
		pg.TableBloat[key] = merge(pg.TableBloat[key], values, timeseries.Any)
	case "pg_index_bloat_bytes":
		key := model.DbIndexKey{Db: ls["db"], Table: ls["table"], Index: ls["index"]}
		pg.IndexBloat[key] = merge(pg.IndexBloat[key], values, timeseries.Any)
	case "pg_table_dead_tuple_bytes":
		key := model.DbTableKey{Db: ls["db"], Table: ls["table"]}
		pg.TableDeadTupleBytes[key] = merge(pg.TableDeadTupleBytes[key], values, timeseries.Any)
	case "pg_table_dead_tuples":
		key := model.DbTableKey{Db: ls["db"], Table: ls["table"]}
		pg.TableDeadTuples[key] = merge(pg.TableDeadTuples[key], values, timeseries.Any)
	case "pg_table_live_tuples":
		key := model.DbTableKey{Db: ls["db"], Table: ls["table"]}
		pg.TableLiveTuples[key] = merge(pg.TableLiveTuples[key], values, timeseries.Any)
	case "pg_table_seconds_since_last_autovacuum":
		key := model.DbTableKey{Db: ls["db"], Table: ls["table"]}
		pg.TableSecondsSinceAutovacuum[key] = merge(pg.TableSecondsSinceAutovacuum[key], values, timeseries.Any)
	case "pg_table_setting":
		if values.Last() != 1 {
			return
		}
		key := model.DbTableKey{Db: ls["db"], Table: ls["table"]}
		m := pg.TableSettings[key]
		if m == nil {
			m = map[string]float32{}
			pg.TableSettings[key] = m
		}
		for _, name := range []string{"autovacuum_disabled", "autovacuum_vacuum_scale_factor", "autovacuum_vacuum_threshold", "autovacuum_vacuum_cost_delay", "autovacuum_vacuum_cost_limit", "autovacuum_analyze_scale_factor", "autovacuum_analyze_threshold"} {
			if s := ls[name]; s != "" {
				if f, err := strconv.ParseFloat(s, 32); err == nil {
					m[name] = float32(f)
				}
			}
		}
	case "pg_autovacuum_workers":
		pg.AutovacuumWorkers = merge(pg.AutovacuumWorkers, values, timeseries.Any)
	case "pg_table_vacuum_in_progress":
		key := model.DbTableKey{Db: ls["db"], Table: ls["table"]}
		pg.TableVacuumInProgress[key] = merge(pg.TableVacuumInProgress[key], values, timeseries.Any)
	case "pg_table_vacuum_throttled":
		key := model.DbTableKey{Db: ls["db"], Table: ls["table"]}
		pg.TableVacuumThrottled[key] = merge(pg.TableVacuumThrottled[key], values, timeseries.Any)
	case "pg_table_mods_since_analyze":
		key := model.DbTableKey{Db: ls["db"], Table: ls["table"]}
		pg.TableModsSinceAnalyze[key] = merge(pg.TableModsSinceAnalyze[key], values, timeseries.Any)
	case "pg_table_reltuples":
		key := model.DbTableKey{Db: ls["db"], Table: ls["table"]}
		pg.TableReltuples[key] = merge(pg.TableReltuples[key], values, timeseries.Any)
	case "pg_table_seconds_since_last_analyze":
		key := model.DbTableKey{Db: ls["db"], Table: ls["table"]}
		pg.TableSecondsSinceAnalyze[key] = merge(pg.TableSecondsSinceAnalyze[key], values, timeseries.Any)
	case "pg_checkpoints_scheduled_total":
		t := ls["type"]
		pg.CheckpointsScheduledByType[t] = merge(pg.CheckpointsScheduledByType[t], timeseries.Increase(values, pjs.get(ls)), timeseries.Any)
	case "pg_checkpoints_total":
		pg.Checkpoints = merge(pg.Checkpoints, timeseries.Increase(values, pjs.get(ls)), timeseries.Any)
	case "pg_restartpoints_total":
		pg.Restartpoints = merge(pg.Restartpoints, timeseries.Increase(values, pjs.get(ls)), timeseries.Any)
	case "pg_buffers_written_total":
		src := ls["source"]
		pg.BuffersWrittenBySource[src] = merge(pg.BuffersWrittenBySource[src], values, timeseries.Any)
	case "pg_time_since_last_checkpoint_seconds":
		pg.TimeSinceLastCheckpoint = merge(pg.TimeSinceLastCheckpoint, values, timeseries.Any)
	case "pg_wal_since_last_checkpoint_bytes":
		pg.WalSinceLastCheckpoint = merge(pg.WalSinceLastCheckpoint, values, timeseries.Any)
	}
}

func pgTs(ts *timeseries.TimeSeries) timeseries.Time {
	if _, v := ts.LastNotNull(); !timeseries.IsNaN(v) && v > 0 {
		return timeseries.Time(v)
	}
	return 0
}

func loadPostgresBackups(w *model.World, metrics map[string][]*model.MetricValues, project *db.Project) {
	clustersByKey := map[string]*model.Application{}
	for _, app := range w.Applications {
		switch app.Cluster.Manager {
		case model.ClusterManagerCNPG, model.ClusterManagerCrunchy:
			clustersByKey[app.Id.Namespace+"/"+app.Id.Name] = app
		}
	}
	if len(clustersByKey) == 0 {
		return
	}

	getOrCreateBackup := func(ns, name string) *model.PgBackups {
		app := clustersByKey[ns+"/"+name]
		if app == nil {
			return nil
		}
		if app.Cluster.Backups == nil {
			app.Cluster.Backups = &model.PgBackups{
				Methods:    map[string]*model.PgBackupMethod{},
				Conditions: map[string]model.PgBackupCondition{},
			}
		}
		return app.Cluster.Backups
	}
	method := func(b *model.PgBackups, name string) *model.PgBackupMethod {
		m := b.Methods[name]
		if m == nil {
			m = &model.PgBackupMethod{}
			b.Methods[name] = m
		}
		return m
	}

	for _, m := range metrics["pg_backup_target_info"] {
		b := getOrCreateBackup(m.Labels["namespace"], m.Labels["name"])
		if b == nil || m.Labels["method"] == "" {
			continue
		}
		t := method(b, m.Labels["method"])
		switch {
		case m.Labels["path"] != "":
			t.Destination = m.Labels["path"]
			t.Endpoint = m.Labels["endpoint"]
		case m.Labels["s3_bucket"] != "":
			t.Destination = "s3://" + m.Labels["s3_bucket"]
			t.Endpoint = m.Labels["s3_endpoint"]
		case m.Labels["gcs_bucket"] != "":
			t.Destination = "gs://" + m.Labels["gcs_bucket"]
		case m.Labels["azure_container"] != "":
			t.Destination = "azure://" + m.Labels["azure_container"]
		}
		if m.Labels["schedule"] != "" {
			t.Schedule = m.Labels["schedule"]
		}
		if m.Labels["retention_policy"] != "" {
			b.RetentionPolicy = m.Labels["retention_policy"]
		}
	}
	for _, m := range metrics["pg_cluster_status"] {
		b := getOrCreateBackup(m.Labels["namespace"], m.Labels["name"])
		if b == nil || m.Values.Last() != 1 {
			continue
		}
		b.Conditions[m.Labels["type"]] = model.PgBackupCondition{Status: m.Labels["status"], Reason: m.Labels["reason"]}
	}
	for _, m := range metrics["pg_backup_last_successful_timestamp_seconds"] {
		if m.Labels["method"] == "" {
			continue
		}
		if b := getOrCreateBackup(m.Labels["namespace"], m.Labels["name"]); b != nil {
			method(b, m.Labels["method"]).LastSuccessfulBackup = pgTs(m.Values)
		}
	}
	for _, m := range metrics["pg_backup_first_recoverability_point_timestamp_seconds"] {
		if m.Labels["method"] == "" {
			continue
		}
		if b := getOrCreateBackup(m.Labels["namespace"], m.Labels["name"]); b != nil {
			method(b, m.Labels["method"]).FirstRecoverabilityPoint = pgTs(m.Values)
		}
	}
	for _, m := range metrics["pg_backup_last_failed_timestamp_seconds"] {
		if b := getOrCreateBackup(m.Labels["namespace"], m.Labels["name"]); b != nil {
			b.LastFailedBackup = pgTs(m.Values)
		}
	}
	for _, m := range metrics["pg_backup_schedule_info"] {
		if b := getOrCreateBackup(m.Labels["namespace"], m.Labels["cluster"]); b != nil {
			b.Schedule = m.Labels["schedule"]
		}
	}
	for _, m := range metrics["pg_backup_next_scheduled_timestamp_seconds"] {
		if b := getOrCreateBackup(m.Labels["namespace"], m.Labels["cluster"]); b != nil {
			b.NextScheduledBackup = pgTs(m.Values)
		}
	}

	runsByKey := map[string]*model.PgBackupRun{}
	for _, m := range metrics["pg_backup_info"] {
		b := getOrCreateBackup(m.Labels["namespace"], m.Labels["cluster"])
		if b == nil || m.Values.Last() != 1 {
			continue
		}
		kind := m.Labels["kind"]
		if kind == "" {
			kind = m.Labels["method"]
		}
		r := &model.PgBackupRun{
			Name:        m.Labels["name"],
			Method:      m.Labels["method"],
			Kind:        kind,
			Destination: m.Labels["path"],
		}
		b.Runs = append(b.Runs, r)
		runsByKey[m.Labels["namespace"]+"/"+m.Labels["name"]] = r
	}
	for _, m := range metrics["pg_backup_status"] {
		if m.Values.Last() != 1 {
			continue
		}
		if r := runsByKey[m.Labels["namespace"]+"/"+m.Labels["name"]]; r != nil {
			r.Status = m.Labels["status"]
		}
	}
	for _, m := range metrics["pg_backup_completed_timestamp_seconds"] {
		if r := runsByKey[m.Labels["namespace"]+"/"+m.Labels["name"]]; r != nil {
			r.CompletedAt = pgTs(m.Values)
		}
	}
}
