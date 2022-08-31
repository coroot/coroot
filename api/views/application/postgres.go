package application

import (
	"fmt"
	widgets2 "github.com/coroot/coroot-focus/api/views/widgets"
	"github.com/coroot/coroot-focus/model"
	"github.com/coroot/coroot-focus/timeseries"
	"github.com/coroot/coroot-focus/utils"
	"math"
	"regexp"
)

const pgActiveLockedState = "active (locked)"

var (
	pgLogErrRegexp = regexp.MustCompile(`.*(ERROR|FATAL|PANIC)\s*:\s*(.+)`)

	pgConnectionStateColors = map[string]string{
		"idle":                "grey-lighten2",
		pgActiveLockedState:   "red-lighten2",
		"active":              "green",
		"idle in transaction": "lime",
		"reserved":            "blue-lighten3",
	}
)

func postgres(ctx timeseries.Context, app *model.Application) *widgets2.Dashboard {
	dash := widgets2.NewDashboard(ctx, "Postgres")

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
		dash.
			GetOrCreateChartInGroup("Postgres query latency <selector>, seconds", "overview").
			Feature().
			AddSeries(i.Name, i.Postgres.Avg)
		dash.
			GetOrCreateChartInGroup("Postgres query latency <selector>, seconds", i.Name).
			AddSeries("avg", i.Postgres.Avg).
			AddSeries("p50", i.Postgres.P50).
			AddSeries("p95", i.Postgres.P95).
			AddSeries("p99", i.Postgres.P99)

		qps := sumQueries(i.Postgres.QueriesByDB)
		dash.GetOrCreateChart("Queries per second").AddSeries(i.Name, qps)

		errors := timeseries.Aggregate(
			timeseries.NanSum,
			i.LogMessagesByLevel[model.LogLevelError],
			i.LogMessagesByLevel[model.LogLevelCritical],
		)
		pgQueries(dash, i)

		dash.
			GetOrCreateChartInGroup("Errors <selector>", "overview").
			Column().
			Feature().
			AddSeries(i.Name, errors)
		dash.
			GetOrCreateChartInGroup("Errors <selector>", i.Name).
			Column().
			AddMany(timeseries.Top(errorsByPattern(i), timeseries.NanSum, 5))

		pgConnections(dash, i)
		pgLocks(dash, i)
		lag := pgReplicationLag(primaryLsn, i.Postgres.WalReplyLsn)
		dash.GetOrCreateChart("Replication lag, bytes").AddSeries(i.Name, lag)

		pgTable(dash, i, primaryLsn, lag, qps, errors)
	}
	return dash
}

func pgTable(dash *widgets2.Dashboard, i *model.Instance, primaryLsn, lag, qps, errors timeseries.TimeSeries) {
	role := i.ClusterRoleLast()
	roleCell := widgets2.NewTableCell(role.String())
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
	status := widgets2.NewTableCell().SetStatus(model.OK, "up")
	if !i.Postgres.IsUp() {
		status.SetStatus(model.WARNING, "down (no metrics)")
	}
	dash.
		GetOrCreateTable("Instance", "Role", "Status", "Queries", "Latency", "Errors", "Replication lag").
		AddRow(
			widgets2.NewTableCell(i.Name).AddTag("version: %s", i.Postgres.Version.Value()),
			roleCell,
			status,
			widgets2.NewTableCell(utils.FormatFloat(qps.Last())).SetUnit("/s"),
			widgets2.NewTableCell(latencyMs).SetUnit("ms"),
			widgets2.NewTableCell(fmt.Sprintf("%.0f", timeseries.Reduce(timeseries.NanSum, errors))),
			pgReplicationLagCell(primaryLsn, lag, role),
		)
}

func errorsByPattern(instance *model.Instance) map[string]timeseries.TimeSeries {
	res := map[string]timeseries.TimeSeries{}
	for _, p := range instance.LogPatterns {
		if p.Level != model.LogLevelError && p.Level != model.LogLevelCritical {
			continue
		}
		if groups := pgLogErrRegexp.FindStringSubmatch(p.Sample); len(groups) == 3 {
			res[groups[1]+": "+groups[2]] = p.Sum
		} else {
			res[p.Sample] = p.Sum
		}
	}
	return res
}

func pgReplicationLagCell(primaryLsn, lag timeseries.TimeSeries, role model.ClusterRole) *widgets2.TableCell {
	res := &widgets2.TableCell{}
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
			fmt.Sprintf("%s%s", greaterThanWorldWindow, utils.FormatDuration(lagTime.ToStandard(), 1)))
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

func pgConnections(dash *widgets2.Dashboard, instance *model.Instance) {
	connectionByState := map[string]*timeseries.AggregatedTimeseries{}
	for k, v := range instance.Postgres.Connections {
		state := k.State
		if k.State == "active" && k.WaitEventType == "Lock" {
			state = pgActiveLockedState
		}
		byState, ok := connectionByState[state]
		if !ok {
			byState = timeseries.Aggregate(timeseries.NanSum)
			connectionByState[state] = byState
		}
		byState.AddInput(v)
	}
	connectionByState["reserved"] = timeseries.Aggregate(timeseries.NanSum)
	connectionByState["reserved"].AddInput(
		instance.Postgres.Settings["superuser_reserved_connections"].Samples,
		instance.Postgres.Settings["rds.rds_superuser_reserved_connections"].Samples,
	)
	chart := dash.
		GetOrCreateChartInGroup("Postgres connections <selector>", instance.Name).
		Stacked().
		SetThreshold("max_connections", instance.Postgres.Settings["max_connections"].Samples, timeseries.Max)

	for state, v := range connectionByState {
		chart.AddSeries(state, v, pgConnectionStateColors[state])
	}

	idleInTransaction := map[string]timeseries.TimeSeries{}
	locked := map[string]timeseries.TimeSeries{}

	for k, v := range instance.Postgres.Connections {
		switch {
		case k.State == "idle in transaction":
			idleInTransaction[k.String()] = v
		case k.State == "active" && k.WaitEventType == "Lock":
			locked[k.String()] = v
		}
	}
	dash.
		GetOrCreateChartInGroup("Idle transactions on <selector>", instance.Name).
		Stacked().
		AddMany(timeseries.Top(idleInTransaction, timeseries.NanSum, 5))
	dash.
		GetOrCreateChartInGroup("Locked queries on <selector>", instance.Name).
		Stacked().
		AddMany(timeseries.Top(locked, timeseries.NanSum, 5))
}

func pgLocks(dash *widgets2.Dashboard, instance *model.Instance) {
	blockingQueries := make(map[string]timeseries.TimeSeries, len(instance.Postgres.AwaitingQueriesByLockingQuery))
	for k, v := range instance.Postgres.AwaitingQueriesByLockingQuery {
		blockingQueries[k.Query] = v
	}
	dash.
		GetOrCreateChartInGroup("Blocking queries by the number of awaiting queries on <selector>", instance.Name).
		Stacked().
		AddMany(timeseries.Top(blockingQueries, timeseries.NanSum, 5)).
		ShiftColors()
}

func pgQueries(dash *widgets2.Dashboard, instance *model.Instance) {
	totalTime := map[string]timeseries.TimeSeries{}
	ioTime := map[string]timeseries.TimeSeries{}
	for k, stat := range instance.Postgres.PerQuery {
		q := k.String()
		totalTime[q] = stat.TotalTime
		ioTime[q] = stat.IoTime
	}
	dash.
		GetOrCreateChartInGroup("Queries by total time on <selector>, query seconds/second", instance.Name).
		Stacked().
		Sorted().
		AddMany(timeseries.Top(totalTime, timeseries.NanSum, 5))
	dash.
		GetOrCreateChartInGroup("Queries by I/O time on <selector>, query seconds/second", instance.Name).
		Stacked().
		Sorted().
		AddMany(timeseries.Top(ioTime, timeseries.NanSum, 5))
}

func sumQueries(byDB map[string]timeseries.TimeSeries) *timeseries.AggregatedTimeseries {
	total := timeseries.Aggregate(timeseries.NanSum)
	for _, qps := range byDB {
		total.AddInput(qps)
	}
	return total
}
