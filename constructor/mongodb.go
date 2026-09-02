package constructor

import (
	"strconv"
	"strings"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func mongodb(instance *model.Instance, queryName string, m *model.MetricValues, pjs promJobStatuses) {
	if instance == nil {
		return
	}
	if instance.Mongodb == nil {
		instance.Mongodb = model.NewMongodb()
	}
	mongo := instance.Mongodb
	switch queryName {
	case "mongo_up":
		mongo.Up = merge(mongo.Up, m.Values, timeseries.Any)
	case "mongo_scrape_error":
		mongo.Error.Update(m.Values, m.Labels["error"])
		mongo.Warning.Update(m.Values, m.Labels["warning"])
	case "mongo_info":
		mongo.Version.Update(m.Values, m.Labels["server_version"])
		mongo.Flavor.Update(m.Values, m.Labels["flavor"])
	case "mongo_rs_status":
		mongo.ReplicaSet.Update(m.Values, m.Labels["rs"])
		state := strings.ToLower(m.Labels["role"])
		mongo.State.Update(m.Values, state)
		role := state
		if role == "secondary" {
			role = "replica"
		}
		instance.UpdateClusterRole(role, m.Values)
	case "mongo_rs_last_applied_timestamp_ms":
		mongo.LastApplied = merge(mongo.LastApplied, m.Values, timeseries.Any)
	case "mongo_rs_member_config_info":
		votes, _ := strconv.ParseInt(m.Labels["votes"], 10, 32)
		mongo.MemberConfigs[m.Labels["member"]] = &model.MongoRsMemberConfig{
			Arbiter: m.Labels["arbiter"] == "true",
			Votes:   votes,
		}
	case "mongo_rs_config_info":
		mongo.WriteConcernMajorityJournalDefault.Update(m.Values, m.Labels["write_concern_majority_journal_default"])
	case "mongo_profiling_level":
		db := m.Labels["db"]
		mongo.ProfilingLevel[db] = merge(mongo.ProfilingLevel[db], m.Values, timeseries.Any)
	case "mongo_oplog_window_seconds":
		mongo.OplogWindow = merge(mongo.OplogWindow, m.Values, timeseries.Any)
	case "mongo_oplog_size_bytes":
		mongo.OplogUsedSize = merge(mongo.OplogUsedSize, m.Values, timeseries.Any)
	case "mongo_oplog_max_size_bytes":
		mongo.OplogMaxSize = merge(mongo.OplogMaxSize, m.Values, timeseries.Any)
	case "mongo_connections_current":
		mongo.ConnectionsCurrent = merge(mongo.ConnectionsCurrent, m.Values, timeseries.Any)
	case "mongo_connections_active":
		mongo.ConnectionsActive = merge(mongo.ConnectionsActive, m.Values, timeseries.Any)
	case "mongo_connections_max":
		mongo.ConnectionsMax = merge(mongo.ConnectionsMax, m.Values, timeseries.Any)
	case "mongo_connections_created_total":
		mongo.ConnectionsCreated = merge(mongo.ConnectionsCreated, m.Values, timeseries.Any)
	case "mongo_connections_rejected_total":
		mongo.ConnectionsRejected = merge(mongo.ConnectionsRejected, m.Values, timeseries.Any)
	case "mongo_connections_by_app":
		app := m.Labels["app"]
		mongo.ConnectionsByApp[app] = merge(mongo.ConnectionsByApp[app], m.Values, timeseries.Any)
	case "mongo_open_transactions":
		app := m.Labels["app"]
		mongo.OpenTransactionsByApp[app] = merge(mongo.OpenTransactionsByApp[app], m.Values, timeseries.Any)
	case "mongo_opcounters_total":
		op := m.Labels["op"]
		mongo.Opcounters[op] = merge(mongo.Opcounters[op], m.Values, timeseries.Any)
	case "mongo_documents_returned_total":
		mongo.DocumentsReturned = merge(mongo.DocumentsReturned, m.Values, timeseries.Any)
	case "mongo_op_latency_seconds_total":
		typ := m.Labels["type"]
		mongo.OpLatencyTime[typ] = merge(mongo.OpLatencyTime[typ], m.Values, timeseries.Any)
	case "mongo_op_latency_ops_total":
		typ := m.Labels["type"]
		mongo.OpLatencyOps[typ] = merge(mongo.OpLatencyOps[typ], m.Values, timeseries.Any)
	case "mongo_queued_operations":
		typ := m.Labels["type"]
		mongo.QueuedOperations[typ] = merge(mongo.QueuedOperations[typ], m.Values, timeseries.Any)
	case "mongo_wt_tickets_available":
		typ := m.Labels["type"]
		mongo.TicketsAvailable[typ] = merge(mongo.TicketsAvailable[typ], m.Values, timeseries.Any)
	case "mongo_wt_cache_used_bytes":
		mongo.CacheUsed = merge(mongo.CacheUsed, m.Values, timeseries.Any)
	case "mongo_wt_cache_dirty_bytes":
		mongo.CacheDirty = merge(mongo.CacheDirty, m.Values, timeseries.Any)
	case "mongo_wt_cache_max_bytes":
		mongo.CacheMax = merge(mongo.CacheMax, m.Values, timeseries.Any)
	case "mongo_wt_pages_evicted_by_app_threads_total":
		mongo.CacheEvictedPagesByApp = merge(mongo.CacheEvictedPagesByApp, m.Values, timeseries.Any)
	case "mongo_wt_app_threads_evicting_seconds_total":
		mongo.CacheAppEvictingTime = merge(mongo.CacheAppEvictingTime, m.Values, timeseries.Any)
	case "mongo_wt_checkpoints_total":
		mongo.Checkpoints = merge(mongo.Checkpoints, m.Values, timeseries.Any)
	case "mongo_wt_checkpoint_seconds_total":
		mongo.CheckpointTime = merge(mongo.CheckpointTime, m.Values, timeseries.Any)
	case "mongo_wt_journal_bytes_written_total":
		mongo.JournalBytesWritten = merge(mongo.JournalBytesWritten, m.Values, timeseries.Any)
	case "mongo_wt_journal_bytes_since_checkpoint":
		mongo.JournalBytesSinceCheckpoint = merge(mongo.JournalBytesSinceCheckpoint, m.Values, timeseries.Any)
	case "mongo_time_since_last_checkpoint_seconds":
		mongo.TimeSinceCheckpoint = merge(mongo.TimeSinceCheckpoint, m.Values, timeseries.Any)
	case "mongo_scanned_keys_total":
		mongo.ScannedKeys = merge(mongo.ScannedKeys, m.Values, timeseries.Any)
	case "mongo_scanned_documents_total":
		mongo.ScannedDocuments = merge(mongo.ScannedDocuments, m.Values, timeseries.Any)
	case "mongo_collection_scans_total":
		mongo.CollectionScans = merge(mongo.CollectionScans, m.Values, timeseries.Any)
	case "mongo_scan_and_order_total":
		mongo.ScanAndOrder = merge(mongo.ScanAndOrder, m.Values, timeseries.Any)
	case "mongo_ttl_deleted_documents_total":
		mongo.TtlDeleted = merge(mongo.TtlDeleted, m.Values, timeseries.Any)
	case "mongo_flow_control_time_acquiring_seconds_total":
		mongo.FlowControlTime = merge(mongo.FlowControlTime, m.Values, timeseries.Any)
	case "mongo_write_conflicts_total":
		mongo.WriteConflicts = merge(mongo.WriteConflicts, m.Values, timeseries.Any)
	case "mongo_wt_cache_bytes_read_into_total":
		mongo.CacheBytesReadInto = merge(mongo.CacheBytesReadInto, m.Values, timeseries.Any)
	case "mongo_repl_apply_ops_total":
		mongo.ReplApplyOps = merge(mongo.ReplApplyOps, m.Values, timeseries.Any)
	case "mongo_fsync_locked":
		mongo.FsyncLocked = merge(mongo.FsyncLocked, m.Values, timeseries.Any)
	case "mongo_prepared_transactions":
		mongo.PreparedTransactions = merge(mongo.PreparedTransactions, m.Values, timeseries.Any)
	case "mongo_repl_buffer_operations":
		mongo.ReplBufferCount = merge(mongo.ReplBufferCount, m.Values, timeseries.Any)
	case "mongo_repl_buffer_bytes":
		mongo.ReplBufferBytes = merge(mongo.ReplBufferBytes, m.Values, timeseries.Any)
	case "mongo_cursors_open":
		mongo.CursorsOpen = merge(mongo.CursorsOpen, m.Values, timeseries.Any)
	case "mongo_cursors_open_no_timeout":
		mongo.CursorsOpenNoTimeout = merge(mongo.CursorsOpenNoTimeout, m.Values, timeseries.Any)
	case "mongo_cursors_timed_out_total":
		mongo.CursorsTimedOut = merge(mongo.CursorsTimedOut, m.Values, timeseries.Any)
	case "mongo_top_query_calls_per_second":
		s := getOrCreateMongoQuery(mongo, m)
		s.Calls = merge(s.Calls, m.Values, timeseries.Any)
	case "mongo_top_query_time_per_second":
		s := getOrCreateMongoQuery(mongo, m)
		s.TotalTime = merge(s.TotalTime, m.Values, timeseries.Any)
	case "mongo_top_query_docs_returned_per_second":
		s := getOrCreateMongoQuery(mongo, m)
		s.DocsReturned = merge(s.DocsReturned, m.Values, timeseries.Any)
	case "mongo_top_query_docs_examined_per_second":
		s := getOrCreateMongoQuery(mongo, m)
		s.DocsExamined = merge(s.DocsExamined, m.Values, timeseries.Any)
	case "mongo_top_query_keys_examined_per_second":
		s := getOrCreateMongoQuery(mongo, m)
		s.KeysExamined = merge(s.KeysExamined, m.Values, timeseries.Any)
	case "mongo_operations_waiting_for_lock":
		db := m.Labels["db"]
		mongo.OperationsWaitingForLock[db] = merge(mongo.OperationsWaitingForLock[db], m.Values, timeseries.Any)
	case "mongo_long_running_operations":
		key := model.MongoLongRunningOpKey{
			DB:         m.Labels["db"],
			Collection: m.Labels["collection"],
			Query:      m.Labels["query"],
			Plan:       m.Labels["plan"],
		}
		mongo.LongRunningOperations[key] = merge(mongo.LongRunningOperations[key], m.Values, timeseries.Any)
	case "mongo_database_size_bytes":
		db := m.Labels["db"]
		mongo.DatabaseSize[db] = merge(mongo.DatabaseSize[db], m.Values, timeseries.Any)
	case "mongo_collection_size_bytes":
		key := model.DbTableKey{Db: m.Labels["db"], Table: m.Labels["collection"]}
		mongo.CollectionSize[key] = merge(mongo.CollectionSize[key], m.Values, timeseries.Any)
	case "mongo_collection_storage_size_bytes":
		key := model.DbTableKey{Db: m.Labels["db"], Table: m.Labels["collection"]}
		mongo.CollectionStorageSize[key] = merge(mongo.CollectionStorageSize[key], m.Values, timeseries.Any)
	case "mongo_collection_size_growth_bytes_per_second":
		key := model.DbTableKey{Db: m.Labels["db"], Table: m.Labels["collection"]}
		mongo.CollectionSizeGrowth[key] = merge(mongo.CollectionSizeGrowth[key], m.Values, timeseries.Any)
	case "mongo_collection_free_storage_bytes":
		key := model.DbTableKey{Db: m.Labels["db"], Table: m.Labels["collection"]}
		mongo.CollectionFreeStorage[key] = merge(mongo.CollectionFreeStorage[key], m.Values, timeseries.Any)
	case "mongo_collection_documents":
		key := model.DbTableKey{Db: m.Labels["db"], Table: m.Labels["collection"]}
		mongo.CollectionDocuments[key] = merge(mongo.CollectionDocuments[key], m.Values, timeseries.Any)
	}
}

func getOrCreateMongoQuery(mongo *model.Mongodb, m *model.MetricValues) *model.MongoQueryStat {
	key := model.MongoQueryKey{
		DB:         m.Labels["db"],
		Collection: m.Labels["collection"],
		Query:      m.Labels["query"],
	}
	stat := mongo.PerQuery[key]
	if stat == nil {
		stat = &model.MongoQueryStat{}
		mongo.PerQuery[key] = stat
	}
	return stat
}

func loadMongoBackups(w *model.World, metrics map[string][]*model.MetricValues, project *db.Project) {
	clustersByKey := map[string]*model.Application{}
	for _, app := range w.Applications {
		if app.Cluster.Manager == model.ClusterManagerPerconaMongoDB {
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

	for _, m := range metrics["mongo_backup_target_info"] {
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
	for _, m := range metrics["mongo_backup_schedule_info"] {
		b := getOrCreateBackup(m.Labels["namespace"], m.Labels["name"])
		if b == nil || m.Values.Last() != 1 || m.Labels["enabled"] != "true" {
			continue
		}
		if m.Labels["schedule"] != "" {
			b.Schedule = m.Labels["schedule"]
			if m.Labels["method"] != "" {
				method(b, m.Labels["method"]).Schedule = m.Labels["schedule"]
			}
		}
	}
	for _, m := range metrics["mongo_cluster_status"] {
		b := getOrCreateBackup(m.Labels["namespace"], m.Labels["name"])
		if b == nil || m.Values.Last() != 1 {
			continue
		}
		b.Conditions["state"] = model.DBBackupCondition{Status: m.Labels["status"]}
	}
	for _, m := range metrics["mongo_backup_pitr_info"] {
		b := getOrCreateBackup(m.Labels["namespace"], m.Labels["name"])
		if b == nil || m.Values.Last() != 1 {
			continue
		}
		b.Conditions["pitr"] = model.DBBackupCondition{Status: m.Labels["enabled"]}
	}

	runsByKey := map[string]*model.DBBackupRun{}
	for _, m := range metrics["mongo_backup_info"] {
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
	for _, m := range metrics["mongo_backup_status"] {
		if m.Values.Last() != 1 {
			continue
		}
		if r := runsByKey[m.Labels["namespace"]+"/"+m.Labels["name"]]; r != nil {
			r.Status = m.Labels["status"]
		}
	}
	for _, m := range metrics["mongo_backup_completed_timestamp_seconds"] {
		if r := runsByKey[m.Labels["namespace"]+"/"+m.Labels["name"]]; r != nil {
			r.CompletedAt = pgTs(m.Values)
		}
	}
}
