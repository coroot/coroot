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
		instance.Mysql = model.NewMysql(true)
	}
	switch queryName {
	case "mysql_up":
		instance.Mysql.Up = merge(instance.Mysql.Up, m.Values[0], timeseries.Any)
	case "mysql_scrape_error":
		instance.Mysql.Error.Update(m.Values[0], m.Labels["error"])
		instance.Mysql.Warning.Update(m.Values[0], m.Labels["warning"])
	case "mysql_info":
		instance.Mysql.ServerUUID.Update(m.Values[0], m.Labels["server_uuid"])
		instance.Mysql.Version.Update(m.Values[0], m.Labels["server_version"])
	case "mysql_top_query_calls_per_second", "mysql_top_query_time_per_second", "mysql_top_query_lock_time_per_second":
		k := model.MysqlQueryKey{Schema: m.Labels["schema"], Query: m.Labels["query"]}
		s := instance.Mysql.PerQuery[k]
		if s == nil {
			s = &model.MysqlQueryStat{}
			instance.Mysql.PerQuery[k] = s
		}
		switch queryName {
		case "mysql_top_query_calls_per_second":
			s.Calls = merge(s.Calls, m.Values[0], timeseries.Any)
		case "mysql_top_query_time_per_second":
			s.TotalTime = merge(s.TotalTime, m.Values[0], timeseries.Any)
		case "mysql_top_query_lock_time_per_second":
			s.LockTime = merge(s.LockTime, m.Values[0], timeseries.Any)
		}
	case "mysql_replication_io_status":
		if instance.Mysql.ReplicationIOStatus == nil {
			instance.Mysql.ReplicationIOStatus = &model.MysqlReplicationStatus{}
		}
		instance.Mysql.ReplicationSourceUUID.Update(m.Values[0], m.Labels["source_server_uuid"])
		instance.Mysql.ReplicationIOStatus.Status = merge(instance.Mysql.ReplicationIOStatus.Status, m.Values[0], timeseries.Any)
		instance.Mysql.ReplicationIOStatus.LastError.Update(m.Values[0], m.Labels["last_error"])
		instance.Mysql.ReplicationIOStatus.LastState.Update(m.Values[0], m.Labels["state"])
	case "mysql_replication_sql_status":
		if instance.Mysql.ReplicationSQLStatus == nil {
			instance.Mysql.ReplicationSQLStatus = &model.MysqlReplicationStatus{}
		}
		instance.Mysql.ReplicationSourceUUID.Update(m.Values[0], m.Labels["source_server_uuid"])
		instance.Mysql.ReplicationSQLStatus.Status = merge(instance.Mysql.ReplicationSQLStatus.Status, m.Values[0], timeseries.Any)
		instance.Mysql.ReplicationSQLStatus.LastError.Update(m.Values[0], m.Labels["last_error"])
		instance.Mysql.ReplicationSQLStatus.LastState.Update(m.Values[0], m.Labels["state"])
	case "mysql_replication_lag_seconds":
		instance.Mysql.ReplicationSourceUUID.Update(m.Values[0], m.Labels["source_server_uuid"])
		instance.Mysql.ReplicationLagSeconds = merge(instance.Mysql.ReplicationLagSeconds, m.Values[0], timeseries.Any)
	case "mysql_connections_max":
		instance.Mysql.ConnectionsMax = merge(instance.Mysql.ConnectionsMax, m.Values[0], timeseries.Any)
	case "mysql_connections_current":
		instance.Mysql.ConnectionsCurrent = merge(instance.Mysql.ConnectionsCurrent, m.Values[0], timeseries.Any)
	case "mysql_connections_total":
		instance.Mysql.ConnectionsNew = merge(instance.Mysql.ConnectionsNew, m.Values[0], timeseries.Any)
	case "mysql_connections_aborted_total":
		instance.Mysql.ConnectionsAborted = merge(instance.Mysql.ConnectionsAborted, m.Values[0], timeseries.Any)
	case "mysql_traffic_received_bytes_total":
		instance.Mysql.BytesReceived = merge(instance.Mysql.BytesReceived, m.Values[0], timeseries.Any)
	case "mysql_traffic_sent_bytes_total":
		instance.Mysql.BytesSent = merge(instance.Mysql.BytesSent, m.Values[0], timeseries.Any)
	case "mysql_queries_total":
		instance.Mysql.Queries = merge(instance.Mysql.Queries, m.Values[0], timeseries.Any)
	case "mysql_slow_queries_total":
		instance.Mysql.SlowQueries = merge(instance.Mysql.SlowQueries, m.Values[0], timeseries.Any)
	case "mysql_top_table_io_wait_time_per_second":
		key := model.MysqlTable{Schema: m.Labels["schema"], Table: m.Labels["table"]}
		s := instance.Mysql.TablesIOTime[key]
		if s == nil {
			s = &model.MysqlTableIOStats{}
			instance.Mysql.TablesIOTime[key] = s
		}
		switch m.Labels["operation"] {
		case "read":
			s.ReadTimePerSecond = merge(s.ReadTimePerSecond, m.Values[0], timeseries.Any)
		case "write":
			s.WriteTimePerSecond = merge(s.WriteTimePerSecond, m.Values[0], timeseries.Any)
		}
	}
}
