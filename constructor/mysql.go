package constructor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func mysql(instance *model.Instance, queryName string, m *model.MetricValues) {
	if instance == nil {
		return
	}
	if instance.Mysql == nil {
		instance.Mysql = model.NewMysql()
	}
	switch queryName {
	case "mysql_up":
		instance.Mysql.Up = merge(instance.Mysql.Up, m.Values, timeseries.Any)
	case "mysql_scrape_error":
		instance.Mysql.Error.Update(m.Values, m.Labels["error"])
		instance.Mysql.Warning.Update(m.Values, m.Labels["warning"])
	case "mysql_info":
		instance.Mysql.ServerUUID.Update(m.Values, m.Labels["server_uuid"])
		instance.Mysql.Version.Update(m.Values, m.Labels["server_version"])
	case "mysql_top_query_calls_per_second", "mysql_top_query_time_per_second", "mysql_top_query_lock_time_per_second",
		"mysql_top_query_rows_examined_per_second", "mysql_top_query_rows_sent_per_second":
		k := model.MysqlQueryKey{Schema: m.Labels["schema"], Query: m.Labels["query"]}
		s := instance.Mysql.PerQuery[k]
		if s == nil {
			s = &model.MysqlQueryStat{}
			instance.Mysql.PerQuery[k] = s
		}
		switch queryName {
		case "mysql_top_query_calls_per_second":
			s.Calls = merge(s.Calls, m.Values, timeseries.Any)
		case "mysql_top_query_time_per_second":
			s.TotalTime = merge(s.TotalTime, m.Values, timeseries.Any)
		case "mysql_top_query_lock_time_per_second":
			s.LockTime = merge(s.LockTime, m.Values, timeseries.Any)
		case "mysql_top_query_rows_examined_per_second":
			s.RowsExamined = merge(s.RowsExamined, m.Values, timeseries.Any)
		case "mysql_top_query_rows_sent_per_second":
			s.RowsSent = merge(s.RowsSent, m.Values, timeseries.Any)
		}
	case "mysql_created_tmp_disk_tables_total":
		instance.Mysql.CreatedTmpDiskTables = merge(instance.Mysql.CreatedTmpDiskTables, m.Values, timeseries.Any)
	case "mysql_locked_queries":
		k := model.MysqlQueryKey{Schema: m.Labels["schema"], Query: m.Labels["query"]}
		instance.Mysql.LockedQueries[k] = merge(instance.Mysql.LockedQueries[k], m.Values, timeseries.Any)
	case "mysql_lock_awaiting_queries":
		k := model.MysqlQueryKey{Schema: m.Labels["schema"], Query: m.Labels["blocking_query"]}
		instance.Mysql.AwaitingQueriesByBlockingQuery[k] = merge(instance.Mysql.AwaitingQueriesByBlockingQuery[k], m.Values, timeseries.Any)
	case "mysql_replication_io_status":
		if instance.Mysql.ReplicationIOStatus == nil {
			instance.Mysql.ReplicationIOStatus = &model.MysqlReplicationStatus{}
		}
		instance.Mysql.ReplicationSourceUUID.Update(m.Values, m.Labels["source_server_uuid"])
		instance.Mysql.ReplicationIOStatus.Status = merge(instance.Mysql.ReplicationIOStatus.Status, m.Values, timeseries.Any)
		instance.Mysql.ReplicationIOStatus.LastError.Update(m.Values, m.Labels["last_error"])
		instance.Mysql.ReplicationIOStatus.LastState.Update(m.Values, m.Labels["state"])
	case "mysql_replication_sql_status":
		if instance.Mysql.ReplicationSQLStatus == nil {
			instance.Mysql.ReplicationSQLStatus = &model.MysqlReplicationStatus{}
		}
		instance.Mysql.ReplicationSourceUUID.Update(m.Values, m.Labels["source_server_uuid"])
		instance.Mysql.ReplicationSQLStatus.Status = merge(instance.Mysql.ReplicationSQLStatus.Status, m.Values, timeseries.Any)
		instance.Mysql.ReplicationSQLStatus.LastError.Update(m.Values, m.Labels["last_error"])
		instance.Mysql.ReplicationSQLStatus.LastState.Update(m.Values, m.Labels["state"])
	case "mysql_replication_lag_seconds":
		instance.Mysql.ReplicationSourceUUID.Update(m.Values, m.Labels["source_server_uuid"])
		instance.Mysql.ReplicationLagSeconds = merge(instance.Mysql.ReplicationLagSeconds, m.Values, timeseries.Any)
	case "mysql_connections_max":
		instance.Mysql.ConnectionsMax = merge(instance.Mysql.ConnectionsMax, m.Values, timeseries.Any)
	case "mysql_connections_current":
		instance.Mysql.ConnectionsCurrent = merge(instance.Mysql.ConnectionsCurrent, m.Values, timeseries.Any)
	case "mysql_connections_total":
		instance.Mysql.ConnectionsNew = merge(instance.Mysql.ConnectionsNew, m.Values, timeseries.Any)
	case "mysql_connections_aborted_total":
		instance.Mysql.ConnectionsAborted = merge(instance.Mysql.ConnectionsAborted, m.Values, timeseries.Any)
	case "mysql_connection_errors_max_connections_total":
		instance.Mysql.ConnectionErrorsMaxConnections = merge(instance.Mysql.ConnectionErrorsMaxConnections, m.Values, timeseries.Any)
	case "mysql_threads_running":
		instance.Mysql.ThreadsRunning = merge(instance.Mysql.ThreadsRunning, m.Values, timeseries.Any)
	case "mysql_innodb_transaction_seconds":
		q := m.Labels["query"]
		instance.Mysql.InnodbTransactions[q] = merge(instance.Mysql.InnodbTransactions[q], m.Values, timeseries.Any)
	case "mysql_wsrep_cluster_size":
		g := getOrCreateMysqlGalera(instance.Mysql)
		g.ClusterSize = merge(g.ClusterSize, m.Values, timeseries.Any)
	case "mysql_wsrep_cluster_status":
		g := getOrCreateMysqlGalera(instance.Mysql)
		g.ClusterStatus.Update(m.Values, m.Labels["status"])
	case "mysql_wsrep_local_state":
		g := getOrCreateMysqlGalera(instance.Mysql)
		g.LocalStateComment.Update(m.Values, m.Labels["state"])
	case "mysql_wsrep_ready":
		g := getOrCreateMysqlGalera(instance.Mysql)
		g.Ready = merge(g.Ready, m.Values, timeseries.Any)
	case "mysql_wsrep_connected":
		g := getOrCreateMysqlGalera(instance.Mysql)
		g.Connected = merge(g.Connected, m.Values, timeseries.Any)
	case "mysql_wsrep_flow_control_paused_seconds_total":
		g := getOrCreateMysqlGalera(instance.Mysql)
		g.FlowControlPaused = merge(g.FlowControlPaused, m.Values, timeseries.Any)
	case "mysql_wsrep_local_recv_queue":
		g := getOrCreateMysqlGalera(instance.Mysql)
		g.LocalRecvQueue = merge(g.LocalRecvQueue, m.Values, timeseries.Any)
	case "mysql_wsrep_local_send_queue":
		g := getOrCreateMysqlGalera(instance.Mysql)
		g.LocalSendQueue = merge(g.LocalSendQueue, m.Values, timeseries.Any)
	case "mysql_wsrep_local_cert_failures_total":
		g := getOrCreateMysqlGalera(instance.Mysql)
		g.CertFailures = merge(g.CertFailures, m.Values, timeseries.Any)
	case "mysql_wsrep_local_bf_aborts_total":
		g := getOrCreateMysqlGalera(instance.Mysql)
		g.BfAborts = merge(g.BfAborts, m.Values, timeseries.Any)
	case "mysql_group_replication_cluster_size":
		g := getOrCreateMysqlGroupReplication(instance.Mysql)
		g.ClusterSize = merge(g.ClusterSize, m.Values, timeseries.Any)
	case "mysql_group_replication_members_online":
		g := getOrCreateMysqlGroupReplication(instance.Mysql)
		g.Online = merge(g.Online, m.Values, timeseries.Any)
	case "mysql_group_replication_member_state":
		g := getOrCreateMysqlGroupReplication(instance.Mysql)
		g.State.Update(m.Values, m.Labels["state"])
	case "mysql_group_replication_transactions_in_queue":
		g := getOrCreateMysqlGroupReplication(instance.Mysql)
		g.TransactionsInQueue = merge(g.TransactionsInQueue, m.Values, timeseries.Any)
	case "mysql_group_replication_transactions_remote_in_applier_queue":
		g := getOrCreateMysqlGroupReplication(instance.Mysql)
		g.TransactionsApplyQueue = merge(g.TransactionsApplyQueue, m.Values, timeseries.Any)
	case "mysql_group_replication_conflicts_detected_total":
		g := getOrCreateMysqlGroupReplication(instance.Mysql)
		g.ConflictsDetected = merge(g.ConflictsDetected, m.Values, timeseries.Any)
	case "mysql_innodb_buffer_pool_read_requests_total":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.BufferPoolReadRequests = merge(inn.BufferPoolReadRequests, m.Values, timeseries.Any)
	case "mysql_innodb_buffer_pool_reads_total":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.BufferPoolReads = merge(inn.BufferPoolReads, m.Values, timeseries.Any)
	case "mysql_innodb_buffer_pool_write_requests_total":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.BufferPoolWriteRequests = merge(inn.BufferPoolWriteRequests, m.Values, timeseries.Any)
	case "mysql_innodb_buffer_pool_pages_total":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.BufferPoolPagesTotal = merge(inn.BufferPoolPagesTotal, m.Values, timeseries.Any)
	case "mysql_innodb_buffer_pool_pages_free":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.BufferPoolPagesFree = merge(inn.BufferPoolPagesFree, m.Values, timeseries.Any)
	case "mysql_innodb_buffer_pool_pages_dirty":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.BufferPoolPagesDirty = merge(inn.BufferPoolPagesDirty, m.Values, timeseries.Any)
	case "mysql_innodb_buffer_pool_pages_data":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.BufferPoolPagesData = merge(inn.BufferPoolPagesData, m.Values, timeseries.Any)
	case "mysql_innodb_buffer_pool_wait_free_total":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.BufferPoolWaitFree = merge(inn.BufferPoolWaitFree, m.Values, timeseries.Any)
	case "mysql_innodb_buffer_pool_pages_flushed_total":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.BufferPoolPagesFlushed = merge(inn.BufferPoolPagesFlushed, m.Values, timeseries.Any)
	case "mysql_innodb_page_size_bytes":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.PageSize = merge(inn.PageSize, m.Values, timeseries.Any)
	case "mysql_innodb_rows_read_total":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.RowsRead = merge(inn.RowsRead, m.Values, timeseries.Any)
	case "mysql_innodb_rows_inserted_total":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.RowsInserted = merge(inn.RowsInserted, m.Values, timeseries.Any)
	case "mysql_innodb_rows_updated_total":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.RowsUpdated = merge(inn.RowsUpdated, m.Values, timeseries.Any)
	case "mysql_innodb_rows_deleted_total":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.RowsDeleted = merge(inn.RowsDeleted, m.Values, timeseries.Any)
	case "mysql_innodb_row_lock_waits_total":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.RowLockWaits = merge(inn.RowLockWaits, m.Values, timeseries.Any)
	case "mysql_innodb_row_lock_time_seconds_total":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.RowLockTime = merge(inn.RowLockTime, m.Values, timeseries.Any)
	case "mysql_innodb_row_lock_current_waits":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.RowLockCurrentWaits = merge(inn.RowLockCurrentWaits, m.Values, timeseries.Any)
	case "mysql_innodb_data_reads_total":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.DataReads = merge(inn.DataReads, m.Values, timeseries.Any)
	case "mysql_innodb_data_writes_total":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.DataWrites = merge(inn.DataWrites, m.Values, timeseries.Any)
	case "mysql_innodb_data_read_bytes_total":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.DataRead = merge(inn.DataRead, m.Values, timeseries.Any)
	case "mysql_innodb_data_written_bytes_total":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.DataWritten = merge(inn.DataWritten, m.Values, timeseries.Any)
	case "mysql_innodb_data_fsyncs_total":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.DataFsyncs = merge(inn.DataFsyncs, m.Values, timeseries.Any)
	case "mysql_innodb_log_waits_total":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.LogWaits = merge(inn.LogWaits, m.Values, timeseries.Any)
	case "mysql_innodb_os_log_written_bytes_total":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.OsLogWritten = merge(inn.OsLogWritten, m.Values, timeseries.Any)
	case "mysql_sort_merge_passes_total":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.SortMergePasses = merge(inn.SortMergePasses, m.Values, timeseries.Any)
	case "mysql_commit_total":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.Commits = merge(inn.Commits, m.Values, timeseries.Any)
	case "mysql_rollback_total":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.Rollbacks = merge(inn.Rollbacks, m.Values, timeseries.Any)
	case "mysql_innodb_deadlocks_total":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.Deadlocks = merge(inn.Deadlocks, m.Values, timeseries.Any)
	case "mysql_innodb_lock_wait_timeouts_total":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.LockWaitTimeouts = merge(inn.LockWaitTimeouts, m.Values, timeseries.Any)
	case "mysql_innodb_history_list_length":
		inn := getOrCreateMysqlInnodb(instance.Mysql)
		inn.HistoryListLength = merge(inn.HistoryListLength, m.Values, timeseries.Any)
	case "mysql_binlog_size_bytes":
		instance.Mysql.BinlogSize = merge(instance.Mysql.BinlogSize, m.Values, timeseries.Any)
	case "mysql_binlog_files":
		instance.Mysql.BinlogFiles = merge(instance.Mysql.BinlogFiles, m.Values, timeseries.Any)
	case "mysql_binlog_expire_seconds":
		instance.Mysql.BinlogExpireSeconds = merge(instance.Mysql.BinlogExpireSeconds, m.Values, timeseries.Any)
	case "mysql_undo_size_bytes":
		instance.Mysql.UndoSize = merge(instance.Mysql.UndoSize, m.Values, timeseries.Any)
	case "mysql_table_locks_waited_total":
		instance.Mysql.TableLocksWaited = merge(instance.Mysql.TableLocksWaited, m.Values, timeseries.Any)
	case "mysql_table_locks_immediate_total":
		instance.Mysql.TableLocksImmediate = merge(instance.Mysql.TableLocksImmediate, m.Values, timeseries.Any)
	case "mysql_traffic_received_bytes_total":
		instance.Mysql.BytesReceived = merge(instance.Mysql.BytesReceived, m.Values, timeseries.Any)
	case "mysql_traffic_sent_bytes_total":
		instance.Mysql.BytesSent = merge(instance.Mysql.BytesSent, m.Values, timeseries.Any)
	case "mysql_queries_total":
		instance.Mysql.Queries = merge(instance.Mysql.Queries, m.Values, timeseries.Any)
	case "mysql_slow_queries_total":
		instance.Mysql.SlowQueries = merge(instance.Mysql.SlowQueries, m.Values, timeseries.Any)
	case "mysql_top_table_io_wait_time_per_second":
		key := model.DbTableKey{Db: m.Labels["schema"], Table: m.Labels["table"]}
		s := instance.Mysql.TablesIOTime[key]
		if s == nil {
			s = &model.MysqlTableIOStats{}
			instance.Mysql.TablesIOTime[key] = s
		}
		switch m.Labels["operation"] {
		case "read":
			s.ReadTimePerSecond = merge(s.ReadTimePerSecond, m.Values, timeseries.Any)
		case "write":
			s.WriteTimePerSecond = merge(s.WriteTimePerSecond, m.Values, timeseries.Any)
		}
	case "mysql_database_size_bytes":
		db := m.Labels["db"]
		instance.Mysql.DatabaseSize[db] = merge(instance.Mysql.DatabaseSize[db], m.Values, timeseries.Any)
	case "mysql_table_size_bytes":
		key := model.DbTableKey{Db: m.Labels["db"], Table: m.Labels["table"]}
		instance.Mysql.TableSize[key] = merge(instance.Mysql.TableSize[key], m.Values, timeseries.Any)
	case "mysql_table_size_growth_bytes_per_second":
		key := model.DbTableKey{Db: m.Labels["db"], Table: m.Labels["table"]}
		instance.Mysql.TableSizeGrowth[key] = merge(instance.Mysql.TableSizeGrowth[key], m.Values, timeseries.Any)
	}
}

func getOrCreateMysqlGalera(m *model.Mysql) *model.MysqlGalera {
	if m.Galera == nil {
		m.Galera = &model.MysqlGalera{}
	}
	return m.Galera
}

func getOrCreateMysqlGroupReplication(m *model.Mysql) *model.MysqlGroupReplication {
	if m.GroupReplication == nil {
		m.GroupReplication = &model.MysqlGroupReplication{}
	}
	return m.GroupReplication
}

func getOrCreateMysqlInnodb(m *model.Mysql) *model.MysqlInnodb {
	if m.Innodb == nil {
		m.Innodb = &model.MysqlInnodb{}
	}
	return m.Innodb
}

func loadMysqlBackups(w *model.World, metrics map[string][]*model.MetricValues) {
	clustersByKey := map[string]*model.Application{}
	for _, app := range w.Applications {
		if app.Cluster.Manager == model.ClusterManagerPerconaXtraDB {
			clustersByKey[app.Id.Namespace+"/"+app.Id.Name] = app
		}
	}
	if len(clustersByKey) == 0 {
		return
	}

	getOrCreateBackup := func(ns, name string) *model.DBBackups {
		app := clustersByKey[ns+"/"+name]
		if app == nil {
			return nil
		}
		if app.Cluster.Backups == nil {
			app.Cluster.Backups = &model.DBBackups{
				Methods:    map[string]*model.DBBackupMethod{},
				Conditions: map[string]model.DBBackupCondition{},
			}
		}
		return app.Cluster.Backups
	}
	method := func(b *model.DBBackups, name string) *model.DBBackupMethod {
		m := b.Methods[name]
		if m == nil {
			m = &model.DBBackupMethod{}
			b.Methods[name] = m
		}
		return m
	}

	for _, m := range metrics["mysql_backup_target_info"] {
		b := getOrCreateBackup(m.Labels["namespace"], m.Labels["name"])
		if b == nil || m.Labels["method"] == "" {
			continue
		}
		t := method(b, m.Labels["method"])
		switch {
		case m.Labels["s3_bucket"] != "":
			t.Destination = "s3://" + m.Labels["s3_bucket"]
			if prefix := m.Labels["s3_prefix"]; prefix != "" {
				t.Destination += "/" + prefix
			}
			t.Endpoint = m.Labels["s3_endpoint"]
		case m.Labels["azure_container"] != "":
			t.Destination = "azure://" + m.Labels["azure_container"]
		}
	}
	for _, m := range metrics["mysql_backup_schedule_info"] {
		b := getOrCreateBackup(m.Labels["namespace"], m.Labels["name"])
		if b == nil || m.Values.Last() != 1 || m.Labels["schedule"] == "" {
			continue
		}
		b.Schedule = m.Labels["schedule"]
		if m.Labels["method"] != "" {
			method(b, m.Labels["method"]).Schedule = m.Labels["schedule"]
		}
	}
	for _, m := range metrics["mysql_cluster_status"] {
		b := getOrCreateBackup(m.Labels["namespace"], m.Labels["name"])
		if b == nil || m.Values.Last() != 1 {
			continue
		}
		b.Conditions["state"] = model.DBBackupCondition{Status: m.Labels["status"]}
	}
	for _, m := range metrics["mysql_backup_pitr_info"] {
		b := getOrCreateBackup(m.Labels["namespace"], m.Labels["name"])
		if b == nil || m.Values.Last() != 1 {
			continue
		}
		b.Conditions["pitr"] = model.DBBackupCondition{Status: m.Labels["enabled"]}
	}

	runsByKey := map[string]*model.DBBackupRun{}
	for _, m := range metrics["mysql_backup_info"] {
		b := getOrCreateBackup(m.Labels["namespace"], m.Labels["cluster"])
		if b == nil || m.Values.Last() != 1 {
			continue
		}
		kind := m.Labels["kind"]
		if kind == "" {
			kind = m.Labels["method"]
		}
		r := &model.DBBackupRun{
			Name:        m.Labels["name"],
			Method:      m.Labels["method"],
			Kind:        kind,
			Destination: m.Labels["path"],
		}
		b.Runs = append(b.Runs, r)
		runsByKey[m.Labels["namespace"]+"/"+m.Labels["name"]] = r
	}
	for _, m := range metrics["mysql_backup_status"] {
		if m.Values.Last() != 1 {
			continue
		}
		if r := runsByKey[m.Labels["namespace"]+"/"+m.Labels["name"]]; r != nil {
			r.Status = m.Labels["status"]
		}
	}
	for _, m := range metrics["mysql_backup_completed_timestamp_seconds"] {
		if r := runsByKey[m.Labels["namespace"]+"/"+m.Labels["name"]]; r != nil {
			r.CompletedAt = pgTs(m.Values)
		}
	}
}
