package constructor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func memcached(instance *model.Instance, queryName string, m *model.MetricValues) {
	if instance == nil {
		return
	}
	if instance.Memcached == nil {
		instance.Memcached = model.NewMemcached(false)
	}
	if instance.Memcached.InternalExporter != metricFromInternalExporter(m.Labels) {
		return
	}
	switch queryName {
	case "memcached_up":
		instance.Memcached.Up = merge(instance.Memcached.Up, m.Values, timeseries.Any)
	case "memcached_version":
		instance.Memcached.Version.Update(m.Values, m.Labels["version"])
	case "memcached_limit_bytes":
		instance.Memcached.LimitBytes = merge(instance.Memcached.LimitBytes, m.Values, timeseries.Any)
	case "memcached_items_evicted_total":
		instance.Memcached.EvictedItems = merge(instance.Memcached.EvictedItems, m.Values, timeseries.Any)
	case "memcached_commands_total":
		cmd := m.Labels["command"]
		instance.Memcached.Calls[cmd] = merge(instance.Memcached.Calls[cmd], m.Values, timeseries.NanSum)
		switch m.Labels["status"] {
		case "miss":
			instance.Memcached.Misses = merge(instance.Memcached.Misses, m.Values, timeseries.NanSum)
		case "hit":
			instance.Memcached.Hits = merge(instance.Memcached.Hits, m.Values, timeseries.NanSum)
		}
	}
}
