package auditor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

func (a *appAuditor) mongodb() {
	isMongo := a.app.ApplicationTypes()[model.ApplicationTypeMongodb]

	if !isMongo && !a.app.IsMongodb() {
		return
	}

	report := a.addReport(model.AuditReportMongodb)

	report.Instrumentation = model.ApplicationTypeMongodb

	if !a.app.IsMongodb() {
		report.Status = model.UNKNOWN
		return
	}

	availabilityCheck := report.CreateCheck(model.Checks.MongodbAvailability)
	replicationLagCheck := report.CreateCheck(model.Checks.MongodbReplicationLag)

	table := report.GetOrCreateTable("Instance", "Status", "ReplicaSet", "State", "Queries", "Latency", "Replication lag", "Version")
	qpsChart := report.GetOrCreateChart("Queries, per second", nil)
	latencyChart := report.GetOrCreateChart("Average latency, seconds", nil)
	replicationLagChart := report.GetOrCreateChart("Replication lag, seconds", nil)

	connectionsByInstance := map[string][]*model.Connection{}
	if table != nil {
		for _, d := range a.app.Downstreams {
			if d.RemoteInstance != nil {
				connectionsByInstance[d.RemoteInstance.Name] = append(connectionsByInstance[d.RemoteInstance.Name], d)
			}
		}
	}

	primaryLastApplied := calcMongoPrimaryBaseline(a.app)

	for _, i := range a.app.Instances {
		if i.Mongodb == nil {
			continue
		}
		obsolete := i.IsObsolete()
		if !obsolete && !i.Mongodb.IsUp() {
			availabilityCheck.AddItem(i.Name)
		}

		if obsolete {
			continue
		}
		rs := i.Mongodb.ReplicaSet.Value()
		var lag *timeseries.TimeSeries
		lagCell := model.NewTableCell()

		if rs != "" && !i.Mongodb.LastApplied.IsEmpty() {
			if primary := primaryLastApplied[rs].Get(); !primary.IsEmpty() {
				lag = timeseries.Sub(i.Mongodb.LastApplied, primary).MapInPlace(
					func(t timeseries.Time, v float32) float32 {
						if v < float32(a.w.Ctx.Step) {
							return 0
						}
						return v
					},
				)
			}
		}
		if !lag.IsEmpty() {
			if lagTime := lag.Last(); !timeseries.IsNaN(lagTime) {
				lagCell.SetValue(utils.FormatFloat(lagTime)).SetUnit("s")
				if timeseries.Duration(lagTime) > timeseries.Duration(replicationLagCheck.Threshold) {
					replicationLagCheck.AddItem(i.Name)
				}
			}
			if replicationLagChart != nil {
				replicationLagChart.AddSeries(i.Name, lag)
			}
		}

		if table != nil {
			name := model.NewTableCell(i.Name)
			state := model.NewTableCell(i.Mongodb.State.Value())
			switch i.Mongodb.State.Value() {
			case "primary":
				state.SetIcon("mdi-database-edit-outline", "rgba(0,0,0,0.87)")
			case "secondary":
				state.SetIcon("mdi-database-import-outline", "grey")
			case "arbiter":
				state.SetIcon("mdi-database-eye-outline", "grey")
			}
			status := model.NewTableCell().SetStatus(model.OK, "up")
			if !i.Mongodb.IsUp() {
				if v := i.Mongodb.Error.Value(); v != "" {
					status.SetStatus(model.WARNING, v)
				} else {
					status.SetStatus(model.WARNING, "down (no metrics)")
				}
			} else {
				if v := i.Mongodb.Warning.Value(); v != "" {
					status.SetStatus(model.OK, v)
				}
			}

			protocolFilter := func(protocol model.Protocol) bool {
				return protocol == model.ProtocolMongodb
			}
			qps := model.GetConnectionsRequestsSum(connectionsByInstance[i.Name], protocolFilter)
			if qpsChart != nil {
				qpsChart.AddSeries(i.Name, qps)
			}
			latency := model.GetConnectionsRequestsLatency(connectionsByInstance[i.Name], protocolFilter)
			if latencyChart != nil {
				latencyChart.AddSeries(i.Name, latency)
			}
			latencyCell := model.NewTableCell().SetUnit("ms")
			if last := latency.Last(); last > 0 {
				latencyCell.SetValue(utils.FormatFloat(last * 1000))
			}
			table.AddRow(
				name,
				status,
				model.NewTableCell(rs),
				state,
				model.NewTableCell(utils.FormatFloat(qps.Last())).SetUnit("/s"),
				latencyCell,
				lagCell,
				model.NewTableCell(i.Mongodb.Version.Value()))
		}
	}
}

func calcMongoPrimaryBaseline(app *model.Application) map[string]*timeseries.Aggregate {
	res := map[string]*timeseries.Aggregate{}
	for _, i := range app.Instances {
		if i.Mongodb != nil && !i.Mongodb.LastApplied.IsEmpty() {
			if rs := i.Mongodb.ReplicaSet.Value(); rs != "" {
				p := res[rs]
				if p == nil {
					p = timeseries.NewAggregate(timeseries.Max)
					res[rs] = p
				}
				v := timeseries.Mul(
					i.Mongodb.LastApplied,
					i.ClusterRole().Map(func(t timeseries.Time, v float32) float32 {
						if v > 1 {
							return 0
						}
						return v
					}),
				)
				p.Add(v)
			}
		}
	}
	return res
}
