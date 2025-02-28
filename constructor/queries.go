package constructor

import (
	"fmt"
	"slices"
	"strings"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	promModel "github.com/prometheus/common/model"
)

const (
	qApplicationCustomSLI                  = "application_custom_sli"
	qRecordingRuleInboundRequestsTotal     = "rr_application_inbound_requests_total"
	qRecordingRuleInboundRequestsHistogram = "rr_application_inbound_requests_histogram"
	qRecordingRuleApplicationLogMessages   = "rr_application_log_messages"
)

var (
	possibleNamespaceLabels  = []string{"namespace", "ns", "kubernetes_namespace", "kubernetes_ns", "k8s_namespace", "k8s_ns"}
	possiblePodLabels        = []string{"pod", "pod_name", "kubernetes_pod", "k8s_pod"}
	possibleDBInstanceLabels = []string{"address", "instance", "rds_instance_id", "ec_instance_id"}
)

type Query struct {
	Name   string
	Query  string
	Labels *utils.StringSet
}

func Q(name, query string, labels ...string) Query {
	ls := utils.NewStringSet(model.LabelMachineId, model.LabelSystemUuid, model.LabelContainerId, model.LabelDestination, model.LabelDestinationIP, model.LabelActualDestination)
	ls.Add(labels...)
	return Query{Name: name, Query: query, Labels: ls}
}

func qPod(name, query string, labels ...string) Query {
	return Q(name, query, slices.Concat([]string{"uid"}, labels)...)
}

func qRDS(name, query string, labels ...string) Query {
	return Q(name, query, slices.Concat([]string{"rds_instance_id"}, labels)...)
}

func qDB(name, query string, labels ...string) Query {
	return Q(name, query, slices.Concat(possibleDBInstanceLabels, possibleNamespaceLabels, possiblePodLabels, labels)...)
}

func qJVM(name, query string, labels ...string) Query {
	return Q(name, query, slices.Concat([]string{"jvm"}, labels)...)
}

func qDotNet(name, query string, labels ...string) Query {
	return Q(name, query, slices.Concat([]string{"application"}, labels)...)
}

func qFargateContainer(name, query string, labels ...string) Query {
	return Q(name, query, slices.Concat([]string{"kubernetes_io_hostname", "namespace", "pod", "container"}, labels)...)
}

var QUERIES = []Query{
	Q("node_agent_info", `node_agent_info`, "version"),

	Q("up", `up`, "job", "instance"),

	Q("node_info", `node_info`, "hostname", "kernel_version"),
	Q("node_cloud_info", `node_cloud_info`, "provider", "region", "availability_zone", "instance_type", "instance_life_cycle"),
	Q("node_uptime_seconds", `node_uptime_second`),
	Q("node_cpu_cores", `node_resources_cpu_logical_cores`, ""),
	Q("node_cpu_usage_percent", `sum(rate(node_resources_cpu_usage_seconds_total{mode!="idle"}[$RANGE])) without(mode) /sum(rate(node_resources_cpu_usage_seconds_total[$RANGE])) without(mode)*100`),
	Q("node_cpu_usage_by_mode", `rate(node_resources_cpu_usage_seconds_total{mode!="idle"}[$RANGE]) / ignoring(mode) group_left sum(rate(node_resources_cpu_usage_seconds_total[$RANGE])) without(mode)*100`, "mode"),
	Q("node_memory_total_bytes", `node_resources_memory_total_bytes`),
	Q("node_memory_available_bytes", `node_resources_memory_available_bytes`),
	Q("node_memory_free_bytes", `node_resources_memory_free_bytes`),
	Q("node_memory_cached_bytes", `node_resources_memory_cached_bytes`),
	Q("node_disk_read_time", `rate(node_resources_disk_read_time_seconds_total[$RANGE])`, "device"),
	Q("node_disk_write_time", `rate(node_resources_disk_write_time_seconds_total[$RANGE])`, "device"),
	Q("node_disk_reads", `rate(node_resources_disk_reads_total[$RANGE])`, "device"),
	Q("node_disk_writes", `rate(node_resources_disk_writes_total[$RANGE])`, "device"),
	Q("node_disk_read_bytes", `rate(node_resources_disk_read_bytes_total[$RANGE])`, "device"),
	Q("node_disk_written_bytes", `rate(node_resources_disk_written_bytes_total[$RANGE])`, "device"),
	Q("node_disk_io_time", `rate(node_resources_disk_io_time_seconds_total[$RANGE])`, "device"),
	Q("node_net_up", `node_net_interface_up`, "interface"),
	Q("node_net_ip", `node_net_interface_ip`, "interface", "ip"),
	Q("node_net_rx_bytes", `rate(node_net_received_bytes_total[$RANGE])`, "interface"),
	Q("node_net_tx_bytes", `rate(node_net_transmitted_bytes_total[$RANGE])`, "interface"),

	Q("ip_to_fqdn", `sum by(fqdn, ip) (ip_to_fqdn)`, "ip", "fqdn"),

	Q("fargate_node_machine_cpu_cores", `machine_cpu_cores{eks_amazonaws_com_compute_type="fargate"}`, "eks_amazonaws_com_compute_type", "kubernetes_io_hostname", "topology_kubernetes_io_region", "topology_kubernetes_io_zone"),
	Q("fargate_node_machine_memory_bytes", `machine_memory_bytes{eks_amazonaws_com_compute_type="fargate"}`, "eks_amazonaws_com_compute_type", "kubernetes_io_hostname", "topology_kubernetes_io_region", "topology_kubernetes_io_zone"),

	qFargateContainer("fargate_container_spec_cpu_limit_cores", `container_spec_cpu_quota{eks_amazonaws_com_compute_type="fargate"}/container_spec_cpu_period{eks_amazonaws_com_compute_type="fargate"}`),
	qFargateContainer("fargate_container_cpu_usage_seconds", `rate(container_cpu_usage_seconds_total{eks_amazonaws_com_compute_type="fargate"}[$RANGE])`),
	qFargateContainer("fargate_container_cpu_cfs_throttled_seconds", `rate(container_cpu_cfs_throttled_seconds_total{eks_amazonaws_com_compute_type="fargate"}[$RANGE])`),
	qFargateContainer("fargate_container_spec_memory_limit_bytes", `container_spec_memory_limit_bytes{eks_amazonaws_com_compute_type="fargate"}`),
	qFargateContainer("fargate_container_memory_rss", `container_memory_rss{eks_amazonaws_com_compute_type="fargate"}`),
	qFargateContainer("fargate_container_memory_cache", `container_memory_cache{eks_amazonaws_com_compute_type="fargate"}`),
	qFargateContainer("fargate_container_oom_events_total", `container_oom_events_total{eks_amazonaws_com_compute_type="fargate"}`, "job", "instance"),

	Q("kube_node_info", `kube_node_info`, "node", "kernel_version"),
	Q("kube_service_info", `kube_service_info`, "namespace", "service", "cluster_ip"),
	Q("kube_service_spec_type", `kube_service_spec_type`, "namespace", "service", "type"),
	Q("kube_endpoint_address", `kube_endpoint_address`, "namespace", "endpoint", "ip"),
	Q("kube_service_status_load_balancer_ingress", `kube_service_status_load_balancer_ingress`, "namespace", "service", "ip"),
	Q("kube_deployment_spec_replicas", `kube_deployment_spec_replicas`, "namespace", "deployment"),
	Q("kube_daemonset_status_desired_number_scheduled", `kube_daemonset_status_desired_number_scheduled`, "namespace", "daemonset"),
	Q("kube_statefulset_replicas", `kube_statefulset_replicas`, "namespace", "statefulset"),

	qPod("kube_pod_info", `kube_pod_info`, "namespace", "pod", "created_by_name", "created_by_kind", "node", "pod_ip", "host_ip"),
	qPod("kube_pod_labels", `kube_pod_labels`,
		"label_postgres_operator_crunchydata_com_cluster", "label_postgres_operator_crunchydata_com_role",
		"label_cluster_name", "label_team", "label_application", "label_spilo_role",
		"label_role",
		"label_k8s_enterprisedb_io_cluster",
		"label_cnpg_io_cluster",
		"label_stackgres_io_cluster_name",
		"label_app_kubernetes_io_managed_by",
		"label_app_kubernetes_io_instance",
		"label_helm_sh_chart",
		"label_app_kubernetes_io_name",
		"label_app_kubernetes_io_component", "label_app_kubernetes_io_part_of",
	),
	qPod("kube_pod_status_phase", `kube_pod_status_phase`, "phase"),
	qPod("kube_pod_status_ready", `kube_pod_status_ready{condition="true"}`),
	qPod("kube_pod_status_scheduled", `kube_pod_status_scheduled{condition="true"} > 0`),
	qPod("kube_pod_init_container_info", `kube_pod_init_container_info`, "namespace", "pod", "container"),
	qPod("kube_pod_container_resource_requests", `kube_pod_container_resource_requests`, "namespace", "pod", "container", "resource"),
	qPod("kube_pod_container_status_ready", `kube_pod_container_status_ready > 0`, "namespace", "pod", "container"),
	qPod("kube_pod_container_status_running", `kube_pod_container_status_running > 0`, "namespace", "pod", "container"),
	qPod("kube_pod_container_status_waiting", `kube_pod_container_status_waiting > 0`, "namespace", "pod", "container"),
	qPod("kube_pod_container_status_waiting_reason", `kube_pod_container_status_waiting_reason > 0`, "namespace", "pod", "container", "reason"),
	qPod("kube_pod_container_status_terminated", `kube_pod_container_status_terminated > 0`, "namespace", "pod", "container"),
	qPod("kube_pod_container_status_terminated_reason", `kube_pod_container_status_terminated_reason > 0`, "namespace", "pod", "container", "reason"),
	qPod("kube_pod_container_status_last_terminated_reason", `kube_pod_container_status_last_terminated_reason`, "namespace", "pod", "container", "reason"),

	Q("container_info", `container_info`, "image", "systemd_triggered_by"),
	Q("container_application_type", `container_application_type`, "application_type"),
	Q("container_cpu_limit", `container_resources_cpu_limit_cores`),
	Q("container_cpu_usage", `rate(container_resources_cpu_usage_seconds_total[$RANGE])`),
	Q("container_cpu_delay", `rate(container_resources_cpu_delay_seconds_total[$RANGE])`),
	Q("container_throttled_time", `rate(container_resources_cpu_throttled_seconds_total[$RANGE])`),
	Q("container_memory_limit", `container_resources_memory_limit_bytes`),
	Q("container_memory_rss", `container_resources_memory_rss_bytes`),
	Q("container_memory_cache", `container_resources_memory_cache_bytes`),
	Q("container_oom_kills_total", `container_oom_kills_total % 10000000`, "job", "instance"),
	Q("container_restarts", `container_restarts_total % 10000000`, "job", "instance"),
	Q("container_volume_size", `container_resources_disk_size_bytes`, "mount_point", "volume", "device"),
	Q("container_volume_used", `container_resources_disk_used_bytes`, "mount_point", "volume", "device"),
	Q("container_net_tcp_listen_info", `container_net_tcp_listen_info`, "listen_addr", "proxy"),
	Q("container_net_latency", `container_net_latency_seconds`),
	Q("container_net_tcp_successful_connects", `rate(container_net_tcp_successful_connects_total[$RANGE])`),
	Q("container_net_tcp_failed_connects", `rate(container_net_tcp_failed_connects_total[$RANGE])`),
	Q("container_net_tcp_active_connections", `container_net_tcp_active_connections`),
	Q("container_net_tcp_connection_time_seconds", `rate(container_net_tcp_connection_time_seconds_total[$RANGE])`),
	Q("container_net_tcp_bytes_sent", `rate(container_net_tcp_bytes_sent_total[$RANGE])`),
	Q("container_net_tcp_bytes_received", `rate(container_net_tcp_bytes_received_total[$RANGE])`),
	Q("container_net_tcp_retransmits", `rate(container_net_tcp_retransmits_total[$RANGE])`),
	Q("container_log_messages", `container_log_messages_total % 10000000`, "level", "pattern_hash", "sample", "job", "instance"),

	Q("container_http_requests_count", `rate(container_http_requests_total[$RANGE])`, "status"),
	Q("container_http_requests_latency", `rate(container_http_requests_duration_seconds_total_sum [$RANGE]) / rate(container_http_requests_duration_seconds_total_count [$RANGE])`),
	Q("container_http_requests_histogram", `rate(container_http_requests_duration_seconds_total_bucket[$RANGE])`, "le"),
	Q("container_postgres_queries_count", `rate(container_postgres_queries_total[$RANGE])`, "status"),
	Q("container_postgres_queries_latency", `rate(container_postgres_queries_duration_seconds_total_sum [$RANGE]) / rate(container_postgres_queries_duration_seconds_total_count [$RANGE])`),
	Q("container_postgres_queries_histogram", `rate(container_postgres_queries_duration_seconds_total_bucket[$RANGE])`, "le"),
	Q("container_redis_queries_count", `rate(container_redis_queries_total[$RANGE])`, "status"),
	Q("container_redis_queries_latency", `rate(container_redis_queries_duration_seconds_total_sum [$RANGE]) / rate(container_redis_queries_duration_seconds_total_count [$RANGE])`),
	Q("container_redis_queries_histogram", `rate(container_redis_queries_duration_seconds_total_bucket[$RANGE])`, "le"),
	Q("container_memcached_queries_count", `rate(container_memcached_queries_total[$RANGE])`, "status"),
	Q("container_memcached_queries_latency", `rate(container_memcached_queries_duration_seconds_total_sum [$RANGE]) / rate(container_memcached_queries_duration_seconds_total_count [$RANGE])`),
	Q("container_memcached_queries_histogram", `rate(container_memcached_queries_duration_seconds_total_bucket[$RANGE])`, "le"),
	Q("container_mysql_queries_count", `rate(container_mysql_queries_total[$RANGE])`, "status"),
	Q("container_mysql_queries_latency", `rate(container_mysql_queries_duration_seconds_total_sum [$RANGE]) / rate(container_mysql_queries_duration_seconds_total_count [$RANGE])`),
	Q("container_mysql_queries_histogram", `rate(container_mysql_queries_duration_seconds_total_bucket[$RANGE])`, "le"),
	Q("container_mongo_queries_count", `rate(container_mongo_queries_total[$RANGE])`, "status"),
	Q("container_mongo_queries_latency", `rate(container_mongo_queries_duration_seconds_total_sum [$RANGE]) / rate(container_mongo_queries_duration_seconds_total_count [$RANGE])`),
	Q("container_mongo_queries_histogram", `rate(container_mongo_queries_duration_seconds_total_bucket[$RANGE])`, "le"),
	Q("container_kafka_requests_count", `rate(container_kafka_requests_total[$RANGE])`, "status"),
	Q("container_kafka_requests_latency", `rate(container_kafka_requests_duration_seconds_total_sum [$RANGE]) / rate(container_kafka_requests_duration_seconds_total_count [$RANGE])`),
	Q("container_kafka_requests_histogram", `rate(container_kafka_requests_duration_seconds_total_bucket[$RANGE])`, "le"),
	Q("container_cassandra_queries_count", `rate(container_cassandra_queries_total[$RANGE])`, "status"),
	Q("container_cassandra_queries_latency", `rate(container_cassandra_queries_duration_seconds_total_sum [$RANGE]) / rate(container_cassandra_queries_duration_seconds_total_count [$RANGE])`),
	Q("container_cassandra_queries_histogram", `rate(container_cassandra_queries_duration_seconds_total_bucket[$RANGE])`, "le"),
	Q("container_clickhouse_queries_count", `rate(container_clickhouse_queries_total[$RANGE])`, "status"),
	Q("container_clickhouse_queries_latency", `rate(container_clickhouse_queries_duration_seconds_total_sum [$RANGE]) / rate(container_clickhouse_queries_duration_seconds_total_count [$RANGE])`),
	Q("container_clickhouse_queries_histogram", `rate(container_clickhouse_queries_duration_seconds_total_bucket[$RANGE])`, "le"),
	Q("container_zookeeper_requests_count", `rate(container_zookeeper_requests_total[$RANGE])`, "status"),
	Q("container_zookeeper_requests_latency", `rate(container_zookeeper_requests_duration_seconds_total_sum [$RANGE]) / rate(container_zookeeper_requests_duration_seconds_total_count [$RANGE])`),
	Q("container_zookeeper_requests_histogram", `rate(container_zookeeper_requests_duration_seconds_total_bucket[$RANGE])`, "le"),
	Q("container_rabbitmq_messages", `rate(container_rabbitmq_messages_total[$RANGE])`, "status", "method"),
	Q("container_nats_messages", `rate(container_nats_messages_total[$RANGE])`, "status", "method"),

	Q("container_dns_requests_total", `rate(container_dns_requests_total[$RANGE])`, "request_type", "domain", "status"),
	Q("container_dns_requests_latency", `rate(container_dns_requests_duration_seconds_total_bucket[$RANGE])`, "le"),

	Q("aws_discovery_error", `aws_discovery_error`, "error"),
	qRDS("aws_rds_info", `aws_rds_info`, "cluster_id", "ipv4", "port", "engine", "engine_version", "instance_type", "storage_type", "region", "availability_zone", "multi_az"),
	qRDS("aws_rds_status", `aws_rds_status`, "status"),
	qRDS("aws_rds_cpu_cores", `aws_rds_cpu_cores`),
	qRDS("aws_rds_cpu_usage_percent", `aws_rds_cpu_usage_percent`, "mode"),
	qRDS("aws_rds_memory_total_bytes", `aws_rds_memory_total_bytes`),
	qRDS("aws_rds_memory_cached_bytes", `aws_rds_memory_cached_bytes`),
	qRDS("aws_rds_memory_free_bytes", `aws_rds_memory_free_bytes`),
	qRDS("aws_rds_storage_provisioned_iops", `aws_rds_storage_provisioned_iops`),
	qRDS("aws_rds_allocated_storage_gibibytes", `aws_rds_allocated_storage_gibibytes`),
	qRDS("aws_rds_fs_total_bytes", `aws_rds_fs_total_bytes{mount_point="/rdsdbdata"}`),
	qRDS("aws_rds_fs_used_bytes", `aws_rds_fs_used_bytes{mount_point="/rdsdbdata"}`),
	qRDS("aws_rds_io_util_percent", `aws_rds_io_util_percent`, "device"),
	qRDS("aws_rds_io_ops_per_second", `aws_rds_io_ops_per_second`, "device", "operation"),
	qRDS("aws_rds_io_await_seconds", `aws_rds_io_await_seconds`, "device"),
	qRDS("aws_rds_net_rx_bytes_per_second", `aws_rds_net_rx_bytes_per_second`, "interface"),
	qRDS("aws_rds_net_tx_bytes_per_second", `aws_rds_net_tx_bytes_per_second`, "interface"),
	qRDS("aws_rds_log_messages_total", `aws_rds_log_messages_total % 10000000`, "level", "pattern_hash", "sample", "job", "instance"),

	Q("aws_elasticache_info", `aws_elasticache_info`, "ec_instance_id", "cluster_id", "ipv4", "port", "engine", "engine_version", "instance_type", "region", "availability_zone"),
	Q("aws_elasticache_status", `aws_elasticache_status`, "ec_instance_id", "status"),

	qDB("pg_up", `pg_up`),
	qDB("pg_scrape_error", `pg_scrape_error`, "error", "warning"),
	qDB("pg_info", `pg_info`, "server_version"),
	qDB("pg_setting", `pg_setting`, "name", "unit"),
	qDB("pg_connections", `pg_connections{db!="postgres"}`, "db", "user", "state", "query", "wait_event_type"),
	qDB("pg_lock_awaiting_queries", `pg_lock_awaiting_queries`, "db", "user", "blocking_query"),
	qDB("pg_latency_seconds", `pg_latency_seconds`, "summary"),
	qDB("pg_top_query_calls_per_second", `pg_top_query_calls_per_second`, "db", "user", "query"),
	qDB("pg_top_query_time_per_second", `pg_top_query_time_per_second`, "db", "user", "query"),
	qDB("pg_top_query_io_time_per_second", `pg_top_query_io_time_per_second`, "db", "user", "query"),
	qDB("pg_db_queries_per_second", `pg_db_queries_per_second`, "db"),
	qDB("pg_wal_current_lsn", `pg_wal_current_lsn`),
	qDB("pg_wal_receive_lsn", `pg_wal_receive_lsn`),
	qDB("pg_wal_reply_lsn", `pg_wal_reply_lsn`),

	qDB("mysql_up", `mysql_up`),
	qDB("mysql_scrape_error", `mysql_scrape_error`, "error", "warning"),
	qDB("mysql_info", `mysql_info`, "server_uuid", "server_version"),
	qDB("mysql_top_query_calls_per_second", `mysql_top_query_calls_per_second`, "schema", "query"),
	qDB("mysql_top_query_time_per_second", `mysql_top_query_time_per_second`, "schema", "query"),
	qDB("mysql_top_query_lock_time_per_second", `mysql_top_query_lock_time_per_second`, "schema", "query"),
	qDB("mysql_replication_io_status", `mysql_replication_io_status`, "source_server_uuid", "last_error", "state"),
	qDB("mysql_replication_sql_status", `mysql_replication_sql_status`, "source_server_uuid", "last_error", "state"),
	qDB("mysql_replication_lag_seconds", `mysql_replication_lag_seconds`, "source_server_uuid"),
	qDB("mysql_connections_max", `mysql_connections_max`),
	qDB("mysql_connections_current", `mysql_connections_current`),
	qDB("mysql_connections_total", `rate(mysql_connections_total[$RANGE])`),
	qDB("mysql_connections_aborted_total", `rate(mysql_connections_aborted_total[$RANGE])`),
	qDB("mysql_traffic_received_bytes_total", `rate(mysql_traffic_received_bytes_total[$RANGE])`),
	qDB("mysql_traffic_sent_bytes_total", `rate(mysql_traffic_sent_bytes_total[$RANGE])`),
	qDB("mysql_queries_total", `rate(mysql_queries_total[$RANGE])`),
	qDB("mysql_slow_queries_total", `rate(mysql_slow_queries_total[$RANGE])`),
	qDB("mysql_top_table_io_wait_time_per_second", `mysql_top_table_io_wait_time_per_second`, "schema", "table", "operation"),

	qDB("redis_up", `redis_up`),
	qDB("redis_scrape_error", `redis_exporter_last_scrape_error`, "err"),
	qDB("redis_instance_info", `redis_instance_info`, "redis_version", "role"),
	qDB("redis_commands_duration_seconds_total", `rate(redis_commands_duration_seconds_total[$RANGE])`, "cmd"),
	qDB("redis_commands_total", `rate(redis_commands_total[$RANGE])`, "cmd"),

	qDB("mongo_up", `mongo_up`),
	qDB("mongo_scrape_error", `mongo_scrape_error`, "error", "warning"),
	qDB("mongo_info", `mongo_info`, "server_version"),
	qDB("mongo_rs_status", `mongo_rs_status`, "rs", "role"),
	qDB("mongo_rs_last_applied_timestamp_ms", `timestamp(mongo_rs_last_applied_timestamp_ms) - mongo_rs_last_applied_timestamp_ms/1000`),

	qDB("memcached_up", `memcached_up`),
	qDB("memcached_version", `memcached_version`, "version"),
	qDB("memcached_limit_bytes", `memcached_limit_bytes`),
	qDB("memcached_items_evicted_total", `rate(memcached_items_evicted_total[$RANGE])`),
	qDB("memcached_commands_total", `rate(memcached_commands_total[$RANGE])`, "command", "status"),

	qJVM("container_jvm_info", `container_jvm_info`, "java_version"),
	qJVM("container_jvm_heap_size_bytes", `container_jvm_heap_size_bytes`),
	qJVM("container_jvm_heap_used_bytes", `container_jvm_heap_used_bytes`),
	qJVM("container_jvm_gc_time_seconds", `rate(container_jvm_gc_time_seconds[$RANGE])`, "gc"),
	qJVM("container_jvm_safepoint_time_seconds", `rate(container_jvm_safepoint_time_seconds[$RANGE])`),
	qJVM("container_jvm_safepoint_sync_time_seconds", `rate(container_jvm_safepoint_sync_time_seconds[$RANGE])`),

	qDotNet("container_dotnet_info", `container_dotnet_info`, "runtime_version"),
	qDotNet("container_dotnet_memory_allocated_bytes_total", `rate(container_dotnet_memory_allocated_bytes_total[$RANGE])`),
	qDotNet("container_dotnet_exceptions_total", `rate(container_dotnet_exceptions_total[$RANGE])`),
	qDotNet("container_dotnet_memory_heap_size_bytes", `container_dotnet_memory_heap_size_bytes`, "generation"),
	qDotNet("container_dotnet_gc_count_total", `rate(container_dotnet_gc_count_total[$RANGE])`, "generation"),
	qDotNet("container_dotnet_heap_fragmentation_percent", `container_dotnet_heap_fragmentation_percent`),
	qDotNet("container_dotnet_monitor_lock_contentions_total", `rate(container_dotnet_monitor_lock_contentions_total[$RANGE])`),
	qDotNet("container_dotnet_thread_pool_completed_items_total", `rate(container_dotnet_thread_pool_completed_items_total[$RANGE])`),
	qDotNet("container_dotnet_thread_pool_queue_length", `container_dotnet_thread_pool_queue_length`),
	qDotNet("container_dotnet_thread_pool_size", `container_dotnet_thread_pool_size`),

	Q("container_python_thread_lock_wait_time_seconds", `rate(container_python_thread_lock_wait_time_seconds[$RANGE])`),
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
