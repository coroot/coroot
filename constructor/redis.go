package constructor

import "github.com/coroot/coroot-focus/model"

func redis(w *model.World, queryName string, m model.MetricValues) {
	instance := findInstance(w, m.Labels, model.ApplicationTypeRedis)
	if instance == nil {
		return
	}
	if instance.Redis == nil {
		instance.Redis = model.NewRedis()
	}
	switch queryName {
	case "redis_up":
		instance.Redis.Up = update(instance.Redis.Up, m.Values)
	case "redis_instance_info":
		instance.Redis.Version.Update(m.Values, m.Labels["redis_version"])
		instance.Redis.Role.Update(m.Values, m.Labels["role"])
	case "redis_commands_duration_seconds_total":
		instance.Redis.CallsTime[m.Labels["cmd"]] = update(instance.Redis.CallsTime[m.Labels["cmd"]], m.Values)
	case "redis_commands_total":
		instance.Redis.Calls[m.Labels["cmd"]] = update(instance.Redis.Calls[m.Labels["cmd"]], m.Values)
	}
}
