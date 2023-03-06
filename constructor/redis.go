package constructor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func redis(w *model.World, queryName string, m model.MetricValues) {
	instance := findInstance(w, m.Labels, model.ApplicationTypeRedis, model.ApplicationTypeKeyDB)
	if instance == nil {
		return
	}
	if instance.Redis == nil {
		instance.Redis = model.NewRedis()
	}
	switch queryName {
	case "redis_up":
		instance.Redis.Up = merge(instance.Redis.Up, m.Values, timeseries.Any)
	case "redis_instance_info":
		instance.Redis.Version.Update(m.Values, m.Labels["redis_version"])
		instance.Redis.Role.Update(m.Values, m.Labels["role"])
	case "redis_commands_duration_seconds_total":
		instance.Redis.CallsTime[m.Labels["cmd"]] = merge(instance.Redis.CallsTime[m.Labels["cmd"]], m.Values, timeseries.Any)
	case "redis_commands_total":
		instance.Redis.Calls[m.Labels["cmd"]] = merge(instance.Redis.Calls[m.Labels["cmd"]], m.Values, timeseries.Any)
	}
}
