package constructor

import (
	"strings"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func mongodb(instance *model.Instance, queryName string, m *model.MetricValues) {
	if instance == nil {
		return
	}
	if instance.Mongodb == nil {
		instance.Mongodb = model.NewMongodb(false)
	}
	if instance.Mongodb.InternalExporter != metricFromInternalExporter(m.Labels) {
		return
	}
	switch queryName {
	case "mongo_up":
		instance.Mongodb.Up = merge(instance.Mongodb.Up, m.Values[0], timeseries.Any)
	case "mongo_scrape_error":
		instance.Mongodb.Error.Update(m.Values[0], m.Labels["error"])
		instance.Mongodb.Warning.Update(m.Values[0], m.Labels["warning"])
	case "mongo_info":
		instance.Mongodb.Version.Update(m.Values[0], m.Labels["server_version"])
	case "mongo_rs_status":
		instance.Mongodb.ReplicaSet.Update(m.Values[0], m.Labels["rs"])
		state := strings.ToLower(m.Labels["role"])
		instance.Mongodb.State.Update(m.Values[0], state)
		role := state
		if role == "secondary" {
			role = "replica"
		}
		instance.UpdateClusterRole(role, m.Values[0])
	case "mongo_rs_last_applied_timestamp_ms":
		instance.Mongodb.LastApplied = merge(instance.Mongodb.LastApplied, m.Values[0], timeseries.Any)
	}
}
