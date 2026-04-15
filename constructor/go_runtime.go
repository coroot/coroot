package constructor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func (c *Constructor) loadGoRuntime(metrics map[string][]*model.MetricValues, containers containerCache) {
	load := func(queryName string, f func(g *model.GoRuntime, metric *model.MetricValues)) {
		for _, metric := range metrics[queryName] {
			v := containers[metric.NodeContainerId]
			if v.instance == nil {
				continue
			}
			if v.instance.Go == nil {
				v.instance.Go = &model.GoRuntime{}
			}
			f(v.instance.Go, metric)
		}
	}
	load("container_go_alloc_bytes_total", func(g *model.GoRuntime, metric *model.MetricValues) {
		g.AllocBytes = merge(g.AllocBytes, metric.Values, timeseries.Any)
	})
	load("container_go_alloc_objects_total", func(g *model.GoRuntime, metric *model.MetricValues) {
		g.AllocObjects = merge(g.AllocObjects, metric.Values, timeseries.Any)
	})
}
