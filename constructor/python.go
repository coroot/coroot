package constructor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func (c *Constructor) loadPython(metrics map[string][]*model.MetricValues, containers containerCache) {
	load := func(queryName string, f func(python *model.Python, metric *model.MetricValues)) {
		for _, metric := range metrics[queryName] {
			v := containers[metric.NodeContainerId]
			if v.instance == nil {
				continue
			}
			if v.instance.Python == nil {
				v.instance.Python = &model.Python{}
			}
			f(v.instance.Python, metric)
		}
	}
	load("container_python_thread_lock_wait_time_seconds", func(python *model.Python, metric *model.MetricValues) {
		python.GILWaitTime = merge(python.GILWaitTime, metric.Values, timeseries.Any)
	})
}
