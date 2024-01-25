package constructor

import (
	"strings"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func jvm(instance *model.Instance, queryName string, m model.MetricValues) {
	if m.Labels["jvm"] == "" {
		return
	}
	if instance.Jvms == nil {
		instance.Jvms = map[string]*model.Jvm{}
	}
	name := jvmName(m.Labels["jvm"])
	j := instance.Jvms[name]
	if j == nil {
		j = &model.Jvm{GcTime: map[string]*timeseries.TimeSeries{}}
		instance.Jvms[name] = j
	}
	switch queryName {
	case "container_jvm_info":
		j.JavaVersion.Update(m.Values, m.Labels["java_version"])
	case "container_jvm_heap_size_bytes":
		j.HeapSize = merge(j.HeapSize, m.Values, timeseries.Any)
	case "container_jvm_heap_used_bytes":
		j.HeapUsed = merge(j.HeapUsed, m.Values, timeseries.Any)
	case "container_jvm_gc_time_seconds":
		j.GcTime[m.Labels["gc"]] = merge(j.GcTime[m.Labels["gc"]], m.Values, timeseries.Any)
	case "container_jvm_safepoint_sync_time_seconds":
		j.SafepointSyncTime = merge(j.SafepointSyncTime, m.Values, timeseries.Any)
	case "container_jvm_safepoint_time_seconds":
		j.SafepointTime = merge(j.SafepointTime, m.Values, timeseries.Any)
	}
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
