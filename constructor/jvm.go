package constructor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
)

func jvm(instance *model.Instance, queryName string, m model.MetricValues) {
	if m.Labels["jvm"] == "" {
		return
	}
	if instance.Jvm == nil {
		instance.Jvm = &model.Jvm{
			Name:   m.Labels["jvm"],
			GcTime: map[string]*timeseries.TimeSeries{},
		}
	}
	if instance.Jvm.Name != m.Labels["jvm"] {
		klog.Warningf(`only one JVM per instance is supported so far, will keep only "%s" (skipping "%s"")`, instance.Jvm.Name, m.Labels["jvm"])
		return
	}
	switch queryName {
	case "container_jvm_info":
		instance.Jvm.JavaVersion.Update(m.Values, m.Labels["java_version"])
	case "container_jvm_heap_size_bytes":
		instance.Jvm.HeapSize = merge(instance.Jvm.HeapSize, m.Values, timeseries.Any)
	case "container_jvm_heap_used_bytes":
		instance.Jvm.HeapUsed = merge(instance.Jvm.HeapUsed, m.Values, timeseries.Any)
	case "container_jvm_gc_time_seconds":
		instance.Jvm.GcTime[m.Labels["gc"]] = merge(instance.Jvm.GcTime[m.Labels["gc"]], m.Values, timeseries.Any)
	case "container_jvm_safepoint_sync_time_seconds":
		instance.Jvm.SafepointSyncTime = merge(instance.Jvm.SafepointSyncTime, m.Values, timeseries.Any)
	case "container_jvm_safepoint_time_seconds":
		instance.Jvm.SafepointTime = merge(instance.Jvm.SafepointTime, m.Values, timeseries.Any)
	}
}
