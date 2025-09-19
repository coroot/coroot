package constructor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func (c *Constructor) loadNodejs(metrics map[string][]*model.MetricValues, containers containerCache) {
	load := func(queryName string, f func(nodejs *model.Nodejs, metric *model.MetricValues)) {
		for _, metric := range metrics[queryName] {
			v := containers[metric.NodeContainerId]
			if v.instance == nil {
				continue
			}
			if v.instance.Nodejs == nil {
				v.instance.Nodejs = &model.Nodejs{}
			}
			f(v.instance.Nodejs, metric)
		}
	}
	load("container_nodejs_event_loop_blocked_time_seconds", func(nodejs *model.Nodejs, metric *model.MetricValues) {
		nodejs.EventLoopBlockedTime = merge(nodejs.EventLoopBlockedTime, metric.Values, timeseries.Any)
	})
}
