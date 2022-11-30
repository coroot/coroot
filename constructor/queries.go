package constructor

import (
	"fmt"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	promModel "github.com/prometheus/common/model"
)

var QUERIES = map[string]string{
	"up": `up`,

	"node_info":                   `node_info`,
	"node_cloud_info":             `node_cloud_info`,
	"node_cpu_cores":              `node_resources_cpu_logical_cores`,
	"node_cpu_usage_percent":      `sum(rate(node_resources_cpu_usage_seconds_total{mode!="idle"}[$RANGE])) without(mode) /sum(rate(node_resources_cpu_usage_seconds_total[$RANGE])) without(mode)*100`,
	"node_cpu_usage_by_mode":      `rate(node_resources_cpu_usage_seconds_total{mode!="idle"}[$RANGE]) / ignoring(mode) group_left sum(rate(node_resources_cpu_usage_seconds_total[$RANGE])) without(mode)*100`,
	"node_memory_total_bytes":     `node_resources_memory_total_bytes`,
	"node_memory_available_bytes": `node_resources_memory_available_bytes`,
	"node_memory_free_bytes":      `node_resources_memory_free_bytes`,
	"node_memory_cached_bytes":    `node_resources_memory_cached_bytes`,
	"node_disk_read_time":         `rate(node_resources_disk_read_time_seconds_total[$RANGE])`,
	"node_disk_write_time":        `rate(node_resources_disk_write_time_seconds_total[$RANGE])`,
	"node_disk_reads":             `rate(node_resources_disk_reads_total[$RANGE])`,
	"node_disk_writes":            `rate(node_resources_disk_writes_total[$RANGE])`,
	"node_disk_read_bytes":        `rate(node_resources_disk_read_bytes_total[$RANGE])`,
	"node_disk_written_bytes":     `rate(node_resources_disk_written_bytes_total[$RANGE])`,
	"node_disk_io_time":           `rate(node_resources_disk_io_time_seconds_total[$RANGE])`,
	"node_net_up":                 `node_net_interface_up`,
	"node_net_ip":                 `node_net_interface_ip`,
	"node_net_rx_bytes":           `rate(node_net_received_bytes_total[$RANGE])`,
	"node_net_tx_bytes":           `rate(node_net_transmitted_bytes_total[$RANGE])`,

	"kube_service_info": `kube_service_info`,

	"kube_pod_info":             `kube_pod_info`,
	"kube_pod_labels":           `kube_pod_labels`,
	"kube_pod_status_phase":     `kube_pod_status_phase`,
	"kube_pod_status_ready":     `kube_pod_status_ready{condition="true"}`,
	"kube_pod_status_scheduled": `kube_pod_status_scheduled{condition="true"} > 0`,

	"container_net_latency":                 `container_net_latency_seconds`,
	"container_net_tcp_successful_connects": `rate(container_net_tcp_successful_connects_total[$RANGE])`,
	"container_net_tcp_active_connections":  `container_net_tcp_active_connections`,
	"container_net_tcp_listen_info":         `container_net_tcp_listen_info`,
	"container_log_messages":                `container_log_messages_total`,
	"container_application_type":            `container_application_type`,
	"container_cpu_limit":                   `container_resources_cpu_limit_cores`,
	"container_cpu_usage":                   `rate(container_resources_cpu_usage_seconds_total[$RANGE])`,
	"container_cpu_delay":                   `rate(container_resources_cpu_delay_seconds_total[$RANGE])`,
	"container_throttled_time":              `rate(container_resources_cpu_throttled_seconds_total[$RANGE])`,
	"container_memory_rss":                  `container_resources_memory_rss_bytes`,
	"container_memory_cache":                `container_resources_memory_cache_bytes`,
	"container_memory_limit":                `container_resources_memory_limit_bytes`,
	"container_oom_kills_total":             `container_oom_kills_total`,
	"container_restarts":                    `container_restarts_total`,
	"container_volume_size":                 `container_resources_disk_size_bytes`,
	"container_volume_used":                 `container_resources_disk_used_bytes`,

	"container_http_requests_count":         `rate(container_http_requests_total[$RANGE])`,
	"container_http_requests_latency":       `rate(container_http_requests_duration_seconds_total_sum [$RANGE]) / rate(container_http_requests_duration_seconds_total_count [$RANGE])`,
	"container_http_requests_histogram":     `rate(container_http_requests_duration_seconds_total_bucket[$RANGE])`,
	"container_postgres_queries_count":      `rate(container_postgres_queries_total[$RANGE])`,
	"container_postgres_queries_latency":    `rate(container_postgres_queries_duration_seconds_total_sum [$RANGE]) / rate(container_postgres_queries_duration_seconds_total_count [$RANGE])`,
	"container_postgres_queries_histogram":  `rate(container_postgres_queries_duration_seconds_total_bucket[$RANGE])`,
	"container_redis_queries_count":         `rate(container_redis_queries_total[$RANGE])`,
	"container_redis_queries_latency":       `rate(container_redis_queries_duration_seconds_total_sum [$RANGE]) / rate(container_redis_queries_duration_seconds_total_count [$RANGE])`,
	"container_redis_queries_histogram":     `rate(container_redis_queries_duration_seconds_total_bucket[$RANGE])`,
	"container_memcached_queries_count":     `rate(container_memcached_queries_total[$RANGE])`,
	"container_memcached_queries_latency":   `rate(container_memcached_queries_duration_seconds_total_sum [$RANGE]) / rate(container_memcached_queries_duration_seconds_total_count [$RANGE])`,
	"container_memcached_queries_histogram": `rate(container_memcached_queries_duration_seconds_total_bucket[$RANGE])`,
	"container_mysql_queries_count":         `rate(container_mysql_queries_total[$RANGE])`,
	"container_mysql_queries_latency":       `rate(container_mysql_queries_duration_seconds_total_sum [$RANGE]) / rate(container_mysql_queries_duration_seconds_total_count [$RANGE])`,
	"container_mysql_queries_histogram":     `rate(container_mysql_queries_duration_seconds_total_bucket[$RANGE])`,
	"container_mongo_queries_count":         `rate(container_mongo_queries_total[$RANGE])`,
	"container_mongo_queries_latency":       `rate(container_mongo_queries_duration_seconds_total_sum [$RANGE]) / rate(container_mongo_queries_duration_seconds_total_count [$RANGE])`,
	"container_mongo_queries_histogram":     `rate(container_mongo_queries_duration_seconds_total_bucket[$RANGE])`,
	"container_kafka_requests_count":        `rate(container_kafka_requests_total[$RANGE])`,
	"container_kafka_requests_latency":      `rate(container_kafka_requests_duration_seconds_total_sum [$RANGE]) / rate(container_kafka_requests_duration_seconds_total_count [$RANGE])`,
	"container_kafka_requests_histogram":    `rate(container_kafka_requests_duration_seconds_total_bucket[$RANGE])`,
	"container_cassandra_queries_count":     `rate(container_cassandra_queries_total[$RANGE])`,
	"container_cassandra_queries_latency":   `rate(container_cassandra_queries_duration_seconds_total_sum [$RANGE]) / rate(container_cassandra_queries_duration_seconds_total_count [$RANGE])`,
	"container_cassandra_queries_histogram": `rate(container_cassandra_queries_duration_seconds_total_bucket[$RANGE])`,

	"kube_pod_init_container_info":                     `kube_pod_init_container_info`,
	"kube_pod_container_status_ready":                  `kube_pod_container_status_ready > 0`,
	"kube_pod_container_status_waiting":                `kube_pod_container_status_waiting > 0`,
	"kube_pod_container_status_running":                `kube_pod_container_status_running > 0 `,
	"kube_pod_container_status_terminated":             `kube_pod_container_status_terminated > 0`,
	"kube_pod_container_status_waiting_reason":         `kube_pod_container_status_waiting_reason > 0`,
	"kube_pod_container_status_last_terminated_reason": `kube_pod_container_status_last_terminated_reason`,
	"kube_deployment_spec_replicas":                    `kube_deployment_spec_replicas`,
	"kube_daemonset_status_desired_number_scheduled":   `kube_daemonset_status_desired_number_scheduled`,
	"kube_statefulset_replicas":                        `kube_statefulset_replicas`,

	"aws_rds_info":                        `aws_rds_info`,
	"aws_rds_status":                      `aws_rds_status`,
	"aws_rds_cpu_cores":                   `aws_rds_cpu_cores`,
	"aws_rds_cpu_usage_percent":           `aws_rds_cpu_usage_percent`,
	"aws_rds_memory_total_bytes":          `aws_rds_memory_total_bytes`,
	"aws_rds_memory_cached_bytes":         `aws_rds_memory_cached_bytes`,
	"aws_rds_memory_free_bytes":           `aws_rds_memory_free_bytes`,
	"aws_rds_io_util_percent":             `aws_rds_io_util_percent`,
	"aws_rds_io_ops_per_second":           `aws_rds_io_ops_per_second`,
	"aws_rds_storage_provisioned_iops":    `aws_rds_storage_provisioned_iops`,
	"aws_rds_allocated_storage_gibibytes": `aws_rds_allocated_storage_gibibytes`,
	"aws_rds_fs_total_bytes":              `aws_rds_fs_total_bytes{mount_point="/rdsdbdata"}`,
	"aws_rds_fs_used_bytes":               `aws_rds_fs_used_bytes{mount_point="/rdsdbdata"}`,
	"aws_rds_io_await_seconds":            `aws_rds_io_await_seconds`,
	"aws_rds_log_messages_total":          `aws_rds_log_messages_total`,
	"aws_rds_net_rx_bytes_per_second":     `aws_rds_net_rx_bytes_per_second`,
	"aws_rds_net_tx_bytes_per_second":     `aws_rds_net_tx_bytes_per_second`,

	"pg_connections":                  `pg_connections{db!="postgres"}`,
	"pg_up":                           `pg_up`,
	"pg_info":                         `pg_info`,
	"pg_setting":                      `pg_setting`,
	"pg_lock_awaiting_queries":        `pg_lock_awaiting_queries`,
	"pg_latency_seconds":              `pg_latency_seconds`,
	"pg_top_query_calls_per_second":   `pg_top_query_calls_per_second`,
	"pg_top_query_time_per_second":    `pg_top_query_time_per_second`,
	"pg_top_query_io_time_per_second": `pg_top_query_io_time_per_second`,
	"pg_db_queries_per_second":        `pg_db_queries_per_second`,
	"pg_wal_current_lsn":              `pg_wal_current_lsn`,
	"pg_wal_receive_lsn":              `pg_wal_receive_lsn`,
	"pg_wal_reply_lsn":                `pg_wal_reply_lsn`,

	"redis_up":                              `redis_up`,
	"redis_instance_info":                   `redis_instance_info`,
	"redis_commands_duration_seconds_total": `rate(redis_commands_duration_seconds_total[$RANGE])`,
	"redis_commands_total":                  `rate(redis_commands_total[$RANGE])`,
}

var RecordingRules = map[string]func(w *model.World) []model.MetricValues{
	"rr_application_inbound_requests_total": func(w *model.World) []model.MetricValues {
		var res []model.MetricValues
		for _, app := range w.Applications {
			connections := app.GetClientsConnections()
			if len(connections) == 0 {
				continue
			}
			sum := map[string]timeseries.TimeSeries{}
			for _, c := range connections {
				for _, byStatus := range c.RequestsCount {
					for status, ts := range byStatus {
						sum[status] = timeseries.Merge(sum[status], ts, timeseries.NanSum)
					}
				}
			}
			appId := app.Id.String()
			for status, ts := range sum {
				if !timeseries.IsEmpty(ts) {
					ls := model.Labels{"application": appId, "status": status}
					res = append(res, model.MetricValues{Labels: ls, LabelsHash: promModel.LabelsToSignature(ls), Values: timeseries.NewCopy(ts)})
				}
			}
		}
		return res
	},
	"rr_application_inbound_requests_histogram": func(w *model.World) []model.MetricValues {
		var res []model.MetricValues
		for _, app := range w.Applications {
			connections := app.GetClientsConnections()
			if len(connections) == 0 {
				continue
			}
			sum := map[float64]timeseries.TimeSeries{}
			for _, c := range connections {
				for _, byLe := range c.RequestsHistogram {
					for le, ts := range byLe {
						sum[le] = timeseries.Merge(sum[le], ts, timeseries.NanSum)
					}
				}
			}
			appId := app.Id.String()
			for le, ts := range sum {
				if !timeseries.IsEmpty(ts) {
					ls := model.Labels{"application": appId, "le": fmt.Sprintf("%f", le)}
					res = append(res, model.MetricValues{Labels: ls, LabelsHash: promModel.LabelsToSignature(ls), Values: timeseries.NewCopy(ts)})
				}
			}
		}
		return res
	},
}
