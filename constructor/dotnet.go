package constructor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func (c *Constructor) loadDotNet(metrics map[string][]*model.MetricValues, containers containerCache) {
	load := func(queryName string, f func(dotnet *model.DotNet, metric *model.MetricValues)) {
		for _, metric := range metrics[queryName] {
			v := containers[metric.NodeContainerId]
			if v.instance == nil {
				continue
			}
			name := metric.Labels["application"]
			if name == "" {
				continue
			}
			if v.instance.DotNet == nil {
				v.instance.DotNet = map[string]*model.DotNet{}
			}
			dotnet := v.instance.DotNet[name]
			if dotnet == nil {
				dotnet = &model.DotNet{
					HeapSize: map[string]*timeseries.TimeSeries{},
					GcCount:  map[string]*timeseries.TimeSeries{},
				}
				v.instance.DotNet[name] = dotnet
			}
			f(dotnet, metric)
		}
	}
	load("container_dotnet_info", func(dotnet *model.DotNet, metric *model.MetricValues) {
		dotnet.RuntimeVersion.Update(metric.Values, metric.Labels["runtime_version"])
		dotnet.Up = merge(dotnet.Up, metric.Values, timeseries.Any)
	})
	load("container_dotnet_memory_allocated_bytes_total", func(dotnet *model.DotNet, metric *model.MetricValues) {
		dotnet.MemoryAllocationRate = merge(dotnet.MemoryAllocationRate, metric.Values, timeseries.Any)
	})
	load("container_dotnet_exceptions_total", func(dotnet *model.DotNet, metric *model.MetricValues) {
		dotnet.Exceptions = merge(dotnet.Exceptions, metric.Values, timeseries.Any)
	})
	load("container_dotnet_memory_heap_size_bytes", func(dotnet *model.DotNet, metric *model.MetricValues) {
		gen := metric.Labels["generation"]
		dotnet.HeapSize[gen] = merge(dotnet.HeapSize[gen], metric.Values, timeseries.Any)
	})
	load("container_dotnet_gc_count_total", func(dotnet *model.DotNet, metric *model.MetricValues) {
		gen := metric.Labels["generation"]
		dotnet.GcCount[gen] = merge(dotnet.GcCount[gen], metric.Values, timeseries.Any)
	})
	load("container_dotnet_heap_fragmentation_percent", func(dotnet *model.DotNet, metric *model.MetricValues) {
		dotnet.HeapFragmentationPercent = merge(dotnet.HeapFragmentationPercent, metric.Values, timeseries.Any)
	})
	load("container_dotnet_monitor_lock_contentions_total", func(dotnet *model.DotNet, metric *model.MetricValues) {
		dotnet.MonitorLockContentions = merge(dotnet.MonitorLockContentions, metric.Values, timeseries.Any)
	})
	load("container_dotnet_thread_pool_completed_items_total", func(dotnet *model.DotNet, metric *model.MetricValues) {
		dotnet.ThreadPoolCompletedItems = merge(dotnet.ThreadPoolCompletedItems, metric.Values, timeseries.Any)
	})
	load("container_dotnet_thread_pool_queue_length", func(dotnet *model.DotNet, metric *model.MetricValues) {
		dotnet.ThreadPoolQueueSize = merge(dotnet.ThreadPoolQueueSize, metric.Values, timeseries.Any)
	})
	load("container_dotnet_thread_pool_size", func(dotnet *model.DotNet, metric *model.MetricValues) {
		dotnet.ThreadPoolSize = merge(dotnet.ThreadPoolSize, metric.Values, timeseries.Any)
	})
}
