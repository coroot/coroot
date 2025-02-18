package constructor

import (
	"strings"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func (c *Constructor) loadJVM(metrics map[string][]*model.MetricValues, containers containerCache) {
	load := func(queryName string, f func(jvm *model.Jvm, metric *model.MetricValues)) {
		for _, metric := range metrics[queryName] {
			v := containers[metric.NodeContainerId]
			if v.instance == nil {
				continue
			}
			name := metric.Labels["jvm"]
			if name == "" {
				continue
			}
			if v.instance.Jvms == nil {
				v.instance.Jvms = map[string]*model.Jvm{}
			}
			name = jvmName(name)
			jvm := v.instance.Jvms[name]
			if jvm == nil {
				jvm = &model.Jvm{GcTime: map[string]*timeseries.TimeSeries{}}
				v.instance.Jvms[name] = jvm
			}
			f(jvm, metric)
		}
	}
	load("container_jvm_info", func(jvm *model.Jvm, metric *model.MetricValues) {
		jvm.JavaVersion.Update(metric.Values, metric.Labels["java_version"])
	})
	load("container_jvm_heap_size_bytes", func(jvm *model.Jvm, metric *model.MetricValues) {
		jvm.HeapSize = merge(jvm.HeapSize, metric.Values, timeseries.Any)
	})
	load("container_jvm_heap_used_bytes", func(jvm *model.Jvm, metric *model.MetricValues) {
		jvm.HeapUsed = merge(jvm.HeapUsed, metric.Values, timeseries.Any)
	})
	load("container_jvm_gc_time_seconds", func(jvm *model.Jvm, metric *model.MetricValues) {
		gc := metric.Labels["gc"]
		jvm.GcTime[gc] = merge(jvm.GcTime[gc], metric.Values, timeseries.Any)
	})
	load("container_jvm_safepoint_time_seconds", func(jvm *model.Jvm, metric *model.MetricValues) {
		jvm.SafepointTime = merge(jvm.SafepointTime, metric.Values, timeseries.Any)
	})
	load("container_jvm_safepoint_sync_time_seconds", func(jvm *model.Jvm, metric *model.MetricValues) {
		jvm.SafepointSyncTime = merge(jvm.SafepointSyncTime, metric.Values, timeseries.Any)
	})
}

func jvmName(s string) string {
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return "unknown"
	}
	if strings.HasSuffix(parts[0], ".jar") {
		parts = strings.Split(parts[0], "/")
		return parts[len(parts)-1]
	}
	parts = strings.Split(parts[0], ".")
	return parts[len(parts)-1]
}
