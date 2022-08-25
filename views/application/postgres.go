package application

import (
	"fmt"
	"github.com/coroot/coroot-focus/model"
	"github.com/coroot/coroot-focus/timeseries"
	"github.com/coroot/coroot-focus/utils"
	"github.com/coroot/coroot-focus/views/widgets"
	"github.com/dustin/go-humanize"
	"math"
)

func postgres(app *model.Application) *widgets.Dashboard {
	dash := &widgets.Dashboard{Name: "Postgres"}

	primaryLsn := timeseries.Aggregate(timeseries.Max)
	for _, i := range app.Instances {
		if i.Postgres != nil && i.Postgres.WalCurrentLsn != nil {
			primaryLsn.AddInput(i.Postgres.WalCurrentLsn)
		}
	}

	for _, i := range app.Instances {
		if i.Postgres == nil {
			continue
		}
		role := i.ClusterRoleLast()
		icon := widgets.Icon{}
		switch role {
		case model.ClusterRolePrimary:
			icon.Icon = "mdi-database-edit-outline"
			icon.Color = "rgba(0,0,0,0.87)"
		case model.ClusterRoleReplica:
			icon.Icon = "mdi-database-import-outline"
			icon.Color = "grey"
		}
		qps := sumQueries(i.Postgres.QueriesByDB)
		latencyMs := "-"
		if i.Postgres.Avg != nil && !i.Postgres.Avg.IsEmpty() {
			latencyMs = utils.FormatFloat(i.Postgres.Avg.Last() * 1000)
		}

		errors := timeseries.Aggregate(
			timeseries.NanSum,
			i.LogMessagesByLevel[model.LogLevelError],
			i.LogMessagesByLevel[model.LogLevelCritical],
		)
		totalErrors := int64(timeseries.Reduce(timeseries.NanSum, errors))
		status := model.OK
		statusMsg := "up"
		if !i.Postgres.IsUp() {
			status = model.WARNING
			statusMsg = "down (no metrics)"
		}

		replicationLag := "-"

		if !primaryLsn.IsEmpty() {
			lag := timeseries.Aggregate(func(accumulator, v float64) float64 {
				res := accumulator - v
				if res < 0 {
					return 0
				}
				return res
			}, primaryLsn, i.Postgres.WalReplyLsn)
			dash.GetOrCreateChart("Replication lag, bytes").AddSeries(i.Name, lag)

			if last := lag.Last(); !math.IsNaN(last) {
				tCurr, vCurr := timeseries.LastNotNull(primaryLsn)
				tPast, vPast := timeseries.Time(0), math.NaN()
				iter := primaryLsn.Iter()
				for iter.Next() {
					tPast, vPast = iter.Value()
					if vPast > vCurr { // wraparound (e.g., complete cluster redeploy)
						continue
					}
					if vPast > vCurr-last {
						break
					}
				}

				if role == model.ClusterRoleReplica {
					lagTime := tCurr.Sub(tPast)
					greaterThanWorldWindow := ""
					if tPast == primaryLsn.Range().From {
						greaterThanWorldWindow = ">"
					}
					replicationLag = humanize.Bytes(uint64(last))
					if lagTime > 0 {
						replicationLag += fmt.Sprintf(" (%s%s)", greaterThanWorldWindow, utils.FormatDuration(lagTime.ToStandart(), 1))
					}
				}
			}
		}

		row := dash.
			GetOrCreateTable("instance", "role", "status", "queries", "latency", "errors", "replication lag").
			AddRow()
		row.
			Text(i.Name).
			WithIcon(role.String(), icon).
			Status(statusMsg, status).
			WithUnit(utils.FormatFloat(qps.Last()), "/s").
			WithUnit(latencyMs, "ms").
			Text(fmt.Sprintf("%d", totalErrors)).
			Text(replicationLag)
	}
	return dash
}

func sumQueries(byDB map[string]timeseries.TimeSeries) *timeseries.AggregatedTimeseries {
	total := timeseries.Aggregate(timeseries.NanSum)
	for _, qps := range byDB {
		total.AddInput(qps)
	}
	return total
}
