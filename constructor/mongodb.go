package constructor

import (
	"strings"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func mongodb(instance *model.Instance, queryName string, m model.MetricValues) {
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
		instance.Mongodb.Up = merge(instance.Mongodb.Up, m.Values, timeseries.Any)
	case "mongo_scrape_error":
		instance.Mongodb.Error.Update(m.Values, m.Labels["error"])
		instance.Mongodb.Warning.Update(m.Values, m.Labels["warning"])
	case "mongodb_members_self":
		instance.Mongodb.ReplicaSet.Update(m.Values, m.Labels["rs_nm"])
		state := strings.ToLower(m.Labels["member_state"])
		instance.Mongodb.State.Update(m.Values, state)
		role := state
		if role == "secondary" {
			role = "replica"
		}
		instance.UpdateClusterRole(role, m.Values)
	case "mongodb_rs_optimes_lastApplied":
		instance.Mongodb.LastApplied = merge(instance.Mongodb.LastApplied, m.Values, timeseries.Any)
	}
}
