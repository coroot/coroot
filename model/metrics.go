package model

import "github.com/coroot/coroot/timeseries"

type Metric struct {
	Name     string
	Query    string
	FillFunc timeseries.FillFunc
}

var Metrics = map[string][]Metric{
	"container": {
		{Name: "cpu_limit", Query: "container_resources_cpu_limit_cores"},
		{Name: "cpu_usage", Query: "rate(container_resources_cpu_usage_seconds_total[$RANGE])"},
		{Name: "cpu_delay", Query: "rate(container_resources_cpu_delay_seconds_total[$RANGE])"},
		{Name: "cpu_throttling", Query: "rate(container_resources_cpu_throttled_seconds_total[$RANGE])"},
		{Name: "memory_limit", Query: "container_resources_memory_limit_bytes"},
		{Name: "memory_rss", Query: "container_resources_memory_rss_bytes"},
		{Name: "memory_cache", Query: "container_resources_memory_cache_bytes"},
		{Name: "oom_kills", Query: "container_oom_kills_total % 10000000"},
		{Name: "restarts", Query: "container_restarts_total % 10000000"},
	},
	"container_volume": {
		{Name: "size", Query: "container_resources_disk_size_bytes"},
		{Name: "used", Query: "container_resources_disk_used_bytes"},
	},
	"container_net": {
		{Name: "successful_connects", Query: "rate(container_net_tcp_successful_connects_total[$RANGE])"},
		{Name: "active_connections", Query: "container_net_tcp_active_connections"},
		{Name: "connection_time", Query: "rate(container_net_tcp_connection_time_seconds_total[$RANGE])"},
		{Name: "bytes_sent", Query: "rate(container_net_tcp_bytes_sent_total[$RANGE])"},
		{Name: "bytes_received", Query: "rate(container_net_tcp_bytes_received_total[$RANGE])"},
		{Name: "retransmits", Query: "rate(container_net_tcp_retransmits_total[$RANGE])"},
	},
}
