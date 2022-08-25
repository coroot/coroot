package application

import (
	"fmt"
	"github.com/coroot/coroot-focus/model"
	"github.com/coroot/coroot-focus/timeseries"
	"github.com/coroot/coroot-focus/utils"
	"github.com/coroot/coroot-focus/views/widgets"
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
		roleCell := widgets.NewTableCell(role.String())
		switch role {
		case model.ClusterRolePrimary:
			roleCell.SetIcon("mdi-database-edit-outline", "rgba(0,0,0,0.87)")
		case model.ClusterRoleReplica:
			roleCell.SetIcon("mdi-database-import-outline", "grey")
		}
		latencyMs := ""
		if i.Postgres.Avg != nil && !i.Postgres.Avg.IsEmpty() {
			latencyMs = utils.FormatFloat(i.Postgres.Avg.Last() * 1000)
		}

		errors := timeseries.Aggregate(
			timeseries.NanSum,
			i.LogMessagesByLevel[model.LogLevelError],
			i.LogMessagesByLevel[model.LogLevelCritical],
		)

		status := widgets.NewTableCell("up").SetStatus(model.OK)
		if !i.Postgres.IsUp() {
			status.SetStatus(model.WARNING).SetValue("down (no metrics)")
		}

		lag := pgReplicationLag(primaryLsn, i.Postgres.WalReplyLsn)
		dash.GetOrCreateChart("Replication lag, bytes").AddSeries(i.Name, lag)

		dash.
			GetOrCreateTable("instance", "role", "status", "queries", "latency", "errors", "replication lag").
			AddRow(
				widgets.NewTableCell(i.Name).AddTag("version: %s", i.Postgres.Version.Value()),
				roleCell,
				status,
				widgets.NewTableCell(utils.FormatFloat(sumQueries(i.Postgres.QueriesByDB).Last())).SetUnit("/s"),
				widgets.NewTableCell(latencyMs).SetUnit("ms"),
				widgets.NewTableCell(fmt.Sprintf("%.0f", timeseries.Reduce(timeseries.NanSum, errors))),
				pgReplicationLagCell(primaryLsn, lag, role),
			)
	}
	return dash
}

func pgReplicationLagCell(primaryLsn, lag timeseries.TimeSeries, role model.ClusterRole) *widgets.TableCell {
	res := &widgets.TableCell{}
	if primaryLsn.IsEmpty() {
		return res
	}
	if role != model.ClusterRoleReplica {
		return res
	}
	last := lag.Last()
	if math.IsNaN(last) {
		return res
	}

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

	lagTime := tCurr.Sub(tPast)
	greaterThanWorldWindow := ""
	if tPast == primaryLsn.Range().From {
		greaterThanWorldWindow = ">"
	}
	res.Value, res.Unit = utils.FormatBytes(last)
	if lagTime > 0 {
		res.Tags = append(res.Tags,
			fmt.Sprintf("%s%s", greaterThanWorldWindow, utils.FormatDuration(lagTime.ToStandart(), 1)))
	}
	return res
}

func pgReplicationLag(primaryLsn, relayLsn timeseries.TimeSeries) timeseries.TimeSeries {
	return timeseries.Aggregate(func(accumulator, v float64) float64 {
		res := accumulator - v
		if res < 0 {
			return 0
		}
		return res
	}, primaryLsn, relayLsn)
}

func sumQueries(byDB map[string]timeseries.TimeSeries) *timeseries.AggregatedTimeseries {
	total := timeseries.Aggregate(timeseries.NanSum)
	for _, qps := range byDB {
		total.AddInput(qps)
	}
	return total
}
