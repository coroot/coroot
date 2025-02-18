package constructor

import (
	"fmt"
	"strings"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	promModel "github.com/prometheus/common/model"
)

const (
	qApplicationCustomSLI                  = "application_custom_sli"
	qRecordingRuleInboundRequestsTotal     = "rr_application_inbound_requests_total"
	qRecordingRuleInboundRequestsHistogram = "rr_application_inbound_requests_histogram"
	qRecordingRuleApplicationLogMessages   = "rr_application_log_messages"
)

var QUERIES = map[string]string{
	"node_agent_info": `node_agent_info`,

	"up": `up`,

	"node_info":                   `node_info`,
	"node_cloud_info":             `node_cloud_info`,
	"node_uptime_seconds":         `node_uptime_seconds`,
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
	"ip_to_fqdn":                  `sum by(fqdn, ip) (ip_to_fqdn)`,

	"fargate_node_machine_cpu_cores":    `machine_cpu_cores{eks_amazonaws_com_compute_type="fargate"}`,
	"fargate_node_machine_memory_bytes": `machine_memory_bytes{eks_amazonaws_com_compute_type="fargate"}`,

	"fargate_container_spec_memory_limit_bytes":   `container_spec_memory_limit_bytes{eks_amazonaws_com_compute_type="fargate"}`,
	"fargate_container_spec_cpu_limit_cores":      `container_spec_cpu_quota{eks_amazonaws_com_compute_type="fargate"}/container_spec_cpu_period{eks_amazonaws_com_compute_type="fargate"}`,
	"fargate_container_oom_events_total":          `container_oom_events_total{eks_amazonaws_com_compute_type="fargate"}`,
	"fargate_container_memory_rss":                `container_memory_rss{eks_amazonaws_com_compute_type="fargate"}`,
	"fargate_container_memory_cache":              `container_memory_cache{eks_amazonaws_com_compute_type="fargate"}`,
	"fargate_container_cpu_usage_seconds":         `rate(container_cpu_usage_seconds_total{eks_amazonaws_com_compute_type="fargate"}[$RANGE])`,
	"fargate_container_cpu_cfs_throttled_seconds": `rate(container_cpu_cfs_throttled_seconds_total{eks_amazonaws_com_compute_type="fargate"}[$RANGE])`,

	"kube_node_info":                            `kube_node_info`,
	"kube_service_info":                         `kube_service_info`,
	"kube_service_spec_type":                    `kube_service_spec_type`,
	"kube_endpoint_address":                     `kube_endpoint_address`,
	"kube_service_status_load_balancer_ingress": `kube_service_status_load_balancer_ingress`,

	"kube_pod_info":             `kube_pod_info`,
	"kube_pod_labels":           `kube_pod_labels`,
	"kube_pod_status_phase":     `kube_pod_status_phase`,
	"kube_pod_status_ready":     `kube_pod_status_ready{condition="true"}`,
	"kube_pod_status_scheduled": `kube_pod_status_scheduled{condition="true"} > 0`,

	"container_info":                            `container_info`,
	"container_net_latency":                     `container_net_latency_seconds`,
	"container_net_tcp_successful_connects":     `rate(container_net_tcp_successful_connects_total[$RANGE])`,
	"container_net_tcp_failed_connects":         `rate(container_net_tcp_failed_connects_total[$RANGE])`,
	"container_net_tcp_active_connections":      `container_net_tcp_active_connections`,
	"container_net_tcp_connection_time_seconds": `rate(container_net_tcp_connection_time_seconds_total[$RANGE])`,
	"container_net_tcp_bytes_sent":              `rate(container_net_tcp_bytes_sent_total[$RANGE])`,
	"container_net_tcp_bytes_received":          `rate(container_net_tcp_bytes_received_total[$RANGE])`,
	"container_net_tcp_listen_info":             `container_net_tcp_listen_info`,
	"container_net_tcp_retransmits":             `rate(container_net_tcp_retransmits_total[$RANGE])`,
	"container_log_messages":                    `container_log_messages_total % 10000000`,
	"container_application_type":                `container_application_type`,
	"container_cpu_limit":                       `container_resources_cpu_limit_cores`,
	"container_cpu_usage":                       `rate(container_resources_cpu_usage_seconds_total[$RANGE])`,
	"container_cpu_delay":                       `rate(container_resources_cpu_delay_seconds_total[$RANGE])`,
	"container_throttled_time":                  `rate(container_resources_cpu_throttled_seconds_total[$RANGE])`,
	"container_memory_rss":                      `container_resources_memory_rss_bytes`,
	"container_memory_cache":                    `container_resources_memory_cache_bytes`,
	"container_memory_limit":                    `container_resources_memory_limit_bytes`,
	"container_oom_kills_total":                 `container_oom_kills_total % 10000000`,
	"container_restarts":                        `container_restarts_total % 10000000`,
	"container_volume_size":                     `container_resources_disk_size_bytes`,
	"container_volume_used":                     `container_resources_disk_used_bytes`,

	"container_http_requests_count":          `rate(container_http_requests_total[$RANGE])`,
	"container_http_requests_latency":        `rate(container_http_requests_duration_seconds_total_sum [$RANGE]) / rate(container_http_requests_duration_seconds_total_count [$RANGE])`,
	"container_http_requests_histogram":      `rate(container_http_requests_duration_seconds_total_bucket[$RANGE])`,
	"container_postgres_queries_count":       `rate(container_postgres_queries_total[$RANGE])`,
	"container_postgres_queries_latency":     `rate(container_postgres_queries_duration_seconds_total_sum [$RANGE]) / rate(container_postgres_queries_duration_seconds_total_count [$RANGE])`,
	"container_postgres_queries_histogram":   `rate(container_postgres_queries_duration_seconds_total_bucket[$RANGE])`,
	"container_redis_queries_count":          `rate(container_redis_queries_total[$RANGE])`,
	"container_redis_queries_latency":        `rate(container_redis_queries_duration_seconds_total_sum [$RANGE]) / rate(container_redis_queries_duration_seconds_total_count [$RANGE])`,
	"container_redis_queries_histogram":      `rate(container_redis_queries_duration_seconds_total_bucket[$RANGE])`,
	"container_memcached_queries_count":      `rate(container_memcached_queries_total[$RANGE])`,
	"container_memcached_queries_latency":    `rate(container_memcached_queries_duration_seconds_total_sum [$RANGE]) / rate(container_memcached_queries_duration_seconds_total_count [$RANGE])`,
	"container_memcached_queries_histogram":  `rate(container_memcached_queries_duration_seconds_total_bucket[$RANGE])`,
	"container_mysql_queries_count":          `rate(container_mysql_queries_total[$RANGE])`,
	"container_mysql_queries_latency":        `rate(container_mysql_queries_duration_seconds_total_sum [$RANGE]) / rate(container_mysql_queries_duration_seconds_total_count [$RANGE])`,
	"container_mysql_queries_histogram":      `rate(container_mysql_queries_duration_seconds_total_bucket[$RANGE])`,
	"container_mongo_queries_count":          `rate(container_mongo_queries_total[$RANGE])`,
	"container_mongo_queries_latency":        `rate(container_mongo_queries_duration_seconds_total_sum [$RANGE]) / rate(container_mongo_queries_duration_seconds_total_count [$RANGE])`,
	"container_mongo_queries_histogram":      `rate(container_mongo_queries_duration_seconds_total_bucket[$RANGE])`,
	"container_kafka_requests_count":         `rate(container_kafka_requests_total[$RANGE])`,
	"container_kafka_requests_latency":       `rate(container_kafka_requests_duration_seconds_total_sum [$RANGE]) / rate(container_kafka_requests_duration_seconds_total_count [$RANGE])`,
	"container_kafka_requests_histogram":     `rate(container_kafka_requests_duration_seconds_total_bucket[$RANGE])`,
	"container_cassandra_queries_count":      `rate(container_cassandra_queries_total[$RANGE])`,
	"container_cassandra_queries_latency":    `rate(container_cassandra_queries_duration_seconds_total_sum [$RANGE]) / rate(container_cassandra_queries_duration_seconds_total_count [$RANGE])`,
	"container_cassandra_queries_histogram":  `rate(container_cassandra_queries_duration_seconds_total_bucket[$RANGE])`,
	"container_rabbitmq_messages":            `rate(container_rabbitmq_messages_total[$RANGE])`,
	"container_nats_messages":                `rate(container_nats_messages_total[$RANGE])`,
	"container_dns_requests_total":           `rate(container_dns_requests_total[$RANGE])`,
	"container_dns_requests_latency":         `rate(container_dns_requests_duration_seconds_total_bucket[$RANGE])`,
	"container_clickhouse_queries_count":     `rate(container_clickhouse_queries_total[$RANGE])`,
	"container_clickhouse_queries_latency":   `rate(container_clickhouse_queries_duration_seconds_total_sum [$RANGE]) / rate(container_clickhouse_queries_duration_seconds_total_count [$RANGE])`,
	"container_clickhouse_queries_histogram": `rate(container_clickhouse_queries_duration_seconds_total_bucket[$RANGE])`,
	"container_zookeeper_requests_count":     `rate(container_zookeeper_requests_total[$RANGE])`,
	"container_zookeeper_requests_latency":   `rate(container_zookeeper_requests_duration_seconds_total_sum [$RANGE]) / rate(container_zookeeper_requests_duration_seconds_total_count [$RANGE])`,
	"container_zookeeper_requests_histogram": `rate(container_zookeeper_requests_duration_seconds_total_bucket[$RANGE])`,

	"kube_pod_init_container_info":                     `kube_pod_init_container_info`,
	"kube_pod_container_resource_requests":             `kube_pod_container_resource_requests`,
	"kube_pod_container_status_ready":                  `kube_pod_container_status_ready > 0`,
	"kube_pod_container_status_waiting":                `kube_pod_container_status_waiting > 0`,
	"kube_pod_container_status_running":                `kube_pod_container_status_running > 0 `,
	"kube_pod_container_status_terminated":             `kube_pod_container_status_terminated > 0`,
	"kube_pod_container_status_terminated_reason":      `kube_pod_container_status_terminated_reason > 0`,
	"kube_pod_container_status_waiting_reason":         `kube_pod_container_status_waiting_reason > 0`,
	"kube_pod_container_status_last_terminated_reason": `kube_pod_container_status_last_terminated_reason`,
	"kube_deployment_spec_replicas":                    `kube_deployment_spec_replicas`,
	"kube_daemonset_status_desired_number_scheduled":   `kube_daemonset_status_desired_number_scheduled`,
	"kube_statefulset_replicas":                        `kube_statefulset_replicas`,

	"aws_discovery_error": `aws_discovery_error`,

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
	"aws_rds_log_messages_total":          `aws_rds_log_messages_total % 10000000`,
	"aws_rds_net_rx_bytes_per_second":     `aws_rds_net_rx_bytes_per_second`,
	"aws_rds_net_tx_bytes_per_second":     `aws_rds_net_tx_bytes_per_second`,

	"aws_elasticache_info":   `aws_elasticache_info`,
	"aws_elasticache_status": `aws_elasticache_status`,

	"pg_connections":                  `pg_connections{db!="postgres"}`,
	"pg_up":                           `pg_up`,
	"pg_scrape_error":                 `pg_scrape_error`,
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
	"redis_scrape_error":                    `redis_exporter_last_scrape_error`,
	"redis_instance_info":                   `redis_instance_info`,
	"redis_commands_duration_seconds_total": `rate(redis_commands_duration_seconds_total[$RANGE])`,
	"redis_commands_total":                  `rate(redis_commands_total[$RANGE])`,

	"container_jvm_info":                        `container_jvm_info`,
	"container_jvm_heap_size_bytes":             `container_jvm_heap_size_bytes`,
	"container_jvm_heap_used_bytes":             `container_jvm_heap_used_bytes`,
	"container_jvm_gc_time_seconds":             `rate(container_jvm_gc_time_seconds[$RANGE])`,
	"container_jvm_safepoint_sync_time_seconds": `rate(container_jvm_safepoint_sync_time_seconds[$RANGE])`,
	"container_jvm_safepoint_time_seconds":      `rate(container_jvm_safepoint_time_seconds[$RANGE])`,

	"mongo_up":                           `mongo_up`,
	"mongo_info":                         `mongo_info`,
	"mongo_scrape_error":                 `mongo_scrape_error`,
	"mongo_rs_status":                    `mongo_rs_status`,
	"mongo_rs_last_applied_timestamp_ms": `timestamp(mongo_rs_last_applied_timestamp_ms) - mongo_rs_last_applied_timestamp_ms/1000`,

	"container_dotnet_info":                              `container_dotnet_info`,
	"container_dotnet_memory_allocated_bytes_total":      `rate(container_dotnet_memory_allocated_bytes_total[$RANGE])`,
	"container_dotnet_exceptions_total":                  `rate(container_dotnet_exceptions_total[$RANGE])`,
	"container_dotnet_memory_heap_size_bytes":            `container_dotnet_memory_heap_size_bytes`,
	"container_dotnet_gc_count_total":                    `rate(container_dotnet_gc_count_total[$RANGE])`,
	"container_dotnet_heap_fragmentation_percent":        `container_dotnet_heap_fragmentation_percent`,
	"container_dotnet_monitor_lock_contentions_total":    `rate(container_dotnet_monitor_lock_contentions_total[$RANGE])`,
	"container_dotnet_thread_pool_completed_items_total": `rate(container_dotnet_thread_pool_completed_items_total[$RANGE])`,
	"container_dotnet_thread_pool_queue_length":          `container_dotnet_thread_pool_queue_length`,
	"container_dotnet_thread_pool_size":                  `container_dotnet_thread_pool_size`,

	"memcached_up":                  `memcached_up`,
	"memcached_version":             `memcached_version`,
	"memcached_limit_bytes":         `memcached_limit_bytes`,
	"memcached_commands_total":      `rate(memcached_commands_total[$RANGE])`,
	"memcached_items_evicted_total": `rate(memcached_items_evicted_total[$RANGE])`,

	"mysql_up":                                `mysql_up`,
	"mysql_scrape_error":                      `mysql_scrape_error`,
	"mysql_info":                              `mysql_info`,
	"mysql_top_query_calls_per_second":        `mysql_top_query_calls_per_second`,
	"mysql_top_query_time_per_second":         `mysql_top_query_time_per_second`,
	"mysql_top_query_lock_time_per_second":    `mysql_top_query_lock_time_per_second`,
	"mysql_replication_io_status":             `mysql_replication_io_status`,
	"mysql_replication_sql_status":            `mysql_replication_sql_status`,
	"mysql_replication_lag_seconds":           `mysql_replication_lag_seconds`,
	"mysql_connections_max":                   `mysql_connections_max`,
	"mysql_connections_current":               `mysql_connections_current`,
	"mysql_connections_total":                 `rate(mysql_connections_total[$RANGE])`,
	"mysql_connections_aborted_total":         `rate(mysql_connections_aborted_total[$RANGE])`,
	"mysql_traffic_received_bytes_total":      `rate(mysql_traffic_received_bytes_total[$RANGE])`,
	"mysql_traffic_sent_bytes_total":          `rate(mysql_traffic_sent_bytes_total[$RANGE])`,
	"mysql_queries_total":                     `rate(mysql_queries_total[$RANGE])`,
	"mysql_slow_queries_total":                `rate(mysql_slow_queries_total[$RANGE])`,
	"mysql_top_table_io_wait_time_per_second": `mysql_top_table_io_wait_time_per_second`,

	"container_python_thread_lock_wait_time_seconds": `rate(container_python_thread_lock_wait_time_seconds[$RANGE])`,
}

var RecordingRules = map[string]func(p *db.Project, w *model.World) []*model.MetricValues{

	qRecordingRuleInboundRequestsTotal: func(p *db.Project, w *model.World) []*model.MetricValues {
		var res []*model.MetricValues
		for _, app := range w.Applications {
			byClient := app.GetClientsConnections()
			if len(byClient) == 0 {
				continue
			}
			appCategory := model.CalcApplicationCategory(app.Id, p.Settings.ApplicationCategories)
			sum := map[string]*timeseries.Aggregate{}
			for client, connections := range byClient {
				clientCategory := model.CalcApplicationCategory(client, p.Settings.ApplicationCategories)
				if !appCategory.Monitoring() && clientCategory.Monitoring() {
					continue
				}
				for _, c := range connections {
					for _, byStatus := range c.RequestsCount {
						for status, ts := range byStatus {
							if sum[status] == nil {
								sum[status] = timeseries.NewAggregate(timeseries.NanSum)
							}
							sum[status].Add(ts)
						}
					}
				}
			}
			appId := app.Id.String()
			for status, agg := range sum {
				ts := agg.Get()
				if !ts.IsEmpty() {
					ls := model.Labels{"application": appId, "status": status}
					res = append(res, &model.MetricValues{Labels: ls, LabelsHash: promModel.LabelsToSignature(ls), Values: ts})
				}
			}
		}
		return res
	},

	qRecordingRuleInboundRequestsHistogram: func(p *db.Project, w *model.World) []*model.MetricValues {
		var res []*model.MetricValues
		for _, app := range w.Applications {
			byClient := app.GetClientsConnections()
			if len(byClient) == 0 {
				continue
			}
			appCategory := model.CalcApplicationCategory(app.Id, p.Settings.ApplicationCategories)
			sum := map[float32]*timeseries.Aggregate{}
			for client, connections := range byClient {
				clientCategory := model.CalcApplicationCategory(client, p.Settings.ApplicationCategories)
				if !appCategory.Monitoring() && clientCategory.Monitoring() {
					continue
				}
				for _, c := range connections {
					for _, byLe := range c.RequestsHistogram {
						for le, ts := range byLe {
							dest := sum[le]
							if dest == nil {
								dest = timeseries.NewAggregate(timeseries.NanSum)
								sum[le] = dest
							}
							dest.Add(ts)
						}
					}
				}
			}
			appId := app.Id.String()
			for le, agg := range sum {
				ts := agg.Get()
				if !ts.IsEmpty() {
					ls := model.Labels{"application": appId, "le": fmt.Sprintf("%f", le)}
					res = append(res, &model.MetricValues{Labels: ls, LabelsHash: promModel.LabelsToSignature(ls), Values: ts})
				}
			}
		}
		return res
	},
	qRecordingRuleApplicationLogMessages: func(p *db.Project, w *model.World) []*model.MetricValues {
		var res []*model.MetricValues
		for _, app := range w.Applications {
			appId := app.Id.String()
			for level, msgs := range app.LogMessages {
				if len(msgs.Patterns) == 0 {
					if msgs.Messages.Reduce(timeseries.NanSum) > 0 {
						ls := model.Labels{"application": appId, "level": string(level)}
						res = append(res, &model.MetricValues{Labels: ls, LabelsHash: promModel.LabelsToSignature(ls), Values: msgs.Messages})
					}
				} else {
					for _, pattern := range msgs.Patterns {
						if pattern.Messages.Reduce(timeseries.NanSum) > 0 {
							ls := model.Labels{"application": appId, "level": string(level)}
							ls["multiline"] = fmt.Sprintf("%t", pattern.Multiline)
							ls["similar"] = strings.Join(pattern.SimilarPatternHashes.Items(), " ")
							ls["sample"] = pattern.Sample
							ls["words"] = pattern.Pattern.String()
							res = append(res, &model.MetricValues{Labels: ls, LabelsHash: promModel.LabelsToSignature(ls), Values: pattern.Messages})
						}
					}
				}
			}
		}
		return res
	},
}
