package constructor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func dotnet(instance *model.Instance, queryName string, m model.MetricValues) {
	name := m.Labels["application"]
	if name == "" {
		return
	}
	if instance.DotNet == nil {
		instance.DotNet = map[string]*model.DotNet{}
	}
	runtime := instance.DotNet[name]
	if runtime == nil {
		runtime = &model.DotNet{
			HeapSize: map[string]*timeseries.TimeSeries{},
			GcCount:  map[string]*timeseries.TimeSeries{},
		}
		instance.DotNet[name] = runtime
	}
	switch queryName {
	case "container_dotnet_info":
		runtime.RuntimeVersion.Update(m.Values, m.Labels["runtime_version"])
		runtime.Up = merge(runtime.Up, m.Values, timeseries.Any)
	case "container_dotnet_memory_allocated_bytes_total":
		runtime.MemoryAllocationRate = merge(runtime.MemoryAllocationRate, m.Values, timeseries.Any)
	case "container_dotnet_exceptions_total":
		runtime.Exceptions = merge(runtime.Exceptions, m.Values, timeseries.Any)
	case "container_dotnet_memory_heap_size_bytes":
		gen := m.Labels["generation"]
		runtime.HeapSize[gen] = merge(runtime.HeapSize[gen], m.Values, timeseries.Any)
	case "container_dotnet_gc_count_total":
		gen := m.Labels["generation"]
		runtime.GcCount[gen] = merge(runtime.GcCount[gen], m.Values, timeseries.Any)
	case "container_dotnet_heap_fragmentation_percent":
		runtime.HeapFragmentationPercent = merge(runtime.HeapFragmentationPercent, m.Values, timeseries.Any)
	case "container_dotnet_monitor_lock_contentions_total":
		runtime.MonitorLockContentions = merge(runtime.MonitorLockContentions, m.Values, timeseries.Any)
	case "container_dotnet_thread_pool_completed_items_total":
		runtime.ThreadPoolCompletedItems = merge(runtime.ThreadPoolCompletedItems, m.Values, timeseries.Any)
	case "container_dotnet_thread_pool_queue_length":
		runtime.ThreadPoolQueueSize = merge(runtime.ThreadPoolQueueSize, m.Values, timeseries.Any)
	case "container_dotnet_thread_pool_size":
		runtime.ThreadPoolSize = merge(runtime.ThreadPoolSize, m.Values, timeseries.Any)
	}
}
