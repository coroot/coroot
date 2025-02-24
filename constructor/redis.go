package constructor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func redis(instance *model.Instance, queryName string, m *model.MetricValues) {
	if instance == nil {
		return
	}
	if instance.Redis == nil {
		instance.Redis = model.NewRedis(false)
	}
	if instance.Redis.InternalExporter != metricFromInternalExporter(m.Labels) {
		return
	}
	switch queryName {
	case "redis_up":
		instance.Redis.Up = merge(instance.Redis.Up, m.Values[0], timeseries.Any)
	case "redis_scrape_error":
		instance.Redis.Error.Update(m.Values[0], m.Labels["err"])
	case "redis_instance_info":
		instance.Redis.Version.Update(m.Values[0], m.Labels["redis_version"])
		role := m.Labels["role"]
		switch role {
		case "master":
			role = "primary"
		case "slave":
			role = "replica"
		}
		instance.Redis.Role.Update(m.Values[0], role)
		instance.UpdateClusterRole(role, m.Values[0])
	case "redis_commands_duration_seconds_total":
		instance.Redis.CallsTime[m.Labels["cmd"]] = merge(instance.Redis.CallsTime[m.Labels["cmd"]], m.Values[0], timeseries.Any)
	case "redis_commands_total":
		instance.Redis.Calls[m.Labels["cmd"]] = merge(instance.Redis.Calls[m.Labels["cmd"]], m.Values[0], timeseries.Any)
	}
}
