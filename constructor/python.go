package constructor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func python(instance *model.Instance, queryName string, m model.MetricValues) {
	if instance.Python == nil {
		instance.Python = &model.Python{}
	}
	switch queryName {
	case "container_python_thread_lock_wait_time_seconds":
		instance.Python.GILWaitTime = merge(instance.Python.GILWaitTime, m.Values, timeseries.Any)
	}
}
