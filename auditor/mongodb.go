package auditor

import (
	"fmt"
	"math"
	"strings"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

const (
	mongoQueriesChartTitle        = "Operations <selector>, per second"
	mongoLatencyChartTitle        = "Operation latency <selector>, seconds"
	mongoTopQueriesByTimeTitle    = "Top queries by total time <selector>, query seconds/second"
	mongoQueryEfficiencyTitle     = "Documents scanned vs returned <selector>, per second"
	mongoCollscanTitle            = "Collection scans and in-memory sorts <selector>, per second"
	mongoRunningOpsTitle          = "Long-running operations <selector>, seconds/second"
	mongoConnectionsChartTitle    = "Connections <selector>"
	mongoConnectionsByAppTitle    = "Connections by client application <selector>"
	mongoNewConnectionsTitle      = "New connections <selector>, per second"
	mongoQueuedOpsTitle           = "Queued operations <selector>"
	mongoOpenTransactionsTitle    = "Open transactions by client application"
	mongoTicketsTitle             = "Available WiredTiger tickets <selector>"
	mongoCacheTitle               = "WiredTiger cache <selector>, bytes"
	mongoCacheEvictionsTitle      = "Cache pages evicted by application threads, pages/second"
	mongoCheckpointsTitle         = "WiredTiger checkpoints, per second"
	mongoCheckpointTimeTitle      = "WiredTiger checkpoint time, seconds/second"
	mongoJournalWriteRateTitle    = "Journal write rate, bytes/second"
	mongoJournalRecoveryTitle     = "Journal to replay on crash recovery, bytes"
	mongoTimeSinceCheckpointTitle = "Time since last checkpoint, seconds"
	mongoWriteConflictsTitle      = "Write conflicts, per second"
	mongoCacheReadIntoTitle       = "Cache bytes read from disk, bytes/second"
	mongoReplApplyTitle           = "Oplog apply rate, operations/second"
	mongoReplBufferTitle          = "Oplog apply buffer, operations"
	mongoCursorsOpenTitle         = "Open cursors"
	mongoCursorsTimedOutTitle     = "Cursors timed out, per second"
	mongoReplicationLagChartTitle = "Replication lag, seconds"
	mongoOplogWindowChartTitle    = "Oplog window, seconds"
	mongoFsyncLockedTitle         = "fsyncLock held (blocks oplog apply)"
	mongoOplogWriteRateChartTitle = "Oplog write rate, bytes/second"
	mongoDbSizeChartTitle         = "Database size <selector>, bytes"
	mongoCollectionSizeChartTitle = "Top collections by size <selector>, bytes"
	mongoCollectionDocsChartTitle = "Top collections by documents <selector>"
	mongoFragmentationChartTitle  = "Reclaimable collection storage <selector>, bytes"
)

const (
	mongoFragmentationMinBytes     = 1 * 1024 * 1024 * 1024
	mongoOplogFullRatio            = 0.95
	mongoCheckpointBusyFraction    = 0.5
	mongoLatencyDetailQueryMinTime = 0.001
	mongoConnectionStormPerSecond  = 20
	mongoSlowDiskAwaitSeconds      = 0.02
	mongoSaturationRecentPoints    = 3
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
	latencyCheck := report.CreateCheck(model.Checks.MongodbLatency)
	replicationLagCheck := report.CreateCheck(model.Checks.MongodbReplicationLag)
	oplogWindowCheck := report.CreateCheck(model.Checks.MongodbOplogWindow)
	connectionsCheck := report.CreateCheck(model.Checks.MongodbConnections)
	saturationCheck := report.CreateCheck(model.Checks.MongodbSaturation)
	fragmentationCheck := report.CreateCheck(model.Checks.MongodbFragmentation)

	table := report.GetOrCreateTable("Instance", "Status", "ReplicaSet", "State", "Queries", "Latency", "Replication lag", "DB Size", "Version")
	availabilityCheck.AddWidget(table.Widget())

	primaries := mongoPrimaries(a.app)
	primaryLastApplied := calcMongoPrimaryBaseline(a.app)

	mongoClusterAvailability(a.app, primaries, availabilityCheck)

	anyFsyncLocked := false
	anyPreparedTx := false

	for _, i := range a.app.Instances {
		if i.Mongodb == nil {
			continue
		}
		obsolete := i.IsObsolete()
		if !obsolete && !i.Mongodb.IsUp() {
			availabilityCheck.AddItem("%s", i.Name)
		}
		if obsolete {
			continue
		}

		mongoQueries(report, i, latencyCheck)
		mongoConnections(report, i, connectionsCheck)
		mongoOperations(report, i, saturationCheck)
		mongoCache(report, i)
		mongoOplog(report, i, oplogWindowCheck)
		mongoStorage(report, i, fragmentationCheck)

		lag := mongoInstanceLag(a.w.Ctx, i, primaryLastApplied)
		lagCell := model.NewTableCell()
		if !lag.IsEmpty() {
			if lagTime := lag.Last(); !timeseries.IsNaN(lagTime) {
				lagCell.SetValue(utils.FormatFloat(lagTime)).SetUnit("s")
				if timeseries.Duration(lagTime) > timeseries.Duration(replicationLagCheck.Threshold) {
					replicationLagCheck.AddItem("%s", i.Name)
					for _, cause := range mongoLagCauses(primaries[i.Mongodb.ReplicaSet.Value()], i, lagTime) {
						replicationLagCheck.AddDetail("%s: %s", i.Name, cause)
					}
				}
			}
			report.GetOrCreateChart(mongoReplicationLagChartTitle, nil).Group("Replication", 5).AddSeries(i.Name, lag)
		}

		if fl := i.Mongodb.FsyncLocked; !fl.IsEmpty() {
			report.GetOrCreateChart(mongoFsyncLockedTitle, nil).Group("Replication", 5).Stacked().AddSeries(i.Name, fl)
			if fl.Reduce(timeseries.Max) > 0 {
				anyFsyncLocked = true
			}
		}
		if pt := i.Mongodb.PreparedTransactions; !pt.IsEmpty() && pt.Reduce(timeseries.Max) > 0 {
			anyPreparedTx = true
		}

		if table != nil {
			mongoInstanceRow(table, i, lagCell)
		}
	}

	latencyCheck.AddWidget(report.GetOrCreateChartGroup(mongoLatencyChartTitle, nil).Widget())
	latencyCheck.AddWidget(report.GetOrCreateChartGroup(mongoTopQueriesByTimeTitle, nil).Widget())
	connectionsCheck.AddWidget(report.GetOrCreateChartGroup(mongoConnectionsChartTitle, nil).Widget())
	connectionsCheck.AddWidget(report.GetOrCreateChartGroup(mongoConnectionsByAppTitle, nil).Widget())
	saturationCheck.AddWidget(report.GetOrCreateChartGroup(mongoQueuedOpsTitle, nil).Widget())
	saturationCheck.AddWidget(report.GetOrCreateChartGroup(mongoTicketsTitle, nil).Widget())
	replicationLagCheck.AddWidget(report.GetOrCreateChart(mongoReplicationLagChartTitle, nil).Widget())
	if anyFsyncLocked {
		w := report.GetOrCreateChart(mongoFsyncLockedTitle, nil).Widget()
		replicationLagCheck.AddWidget(w)
		saturationCheck.AddWidget(w)
	}
	if mongoOpenTransactionsChart(report, a.app) {
		w := report.GetOrCreateChart(mongoOpenTransactionsTitle, nil).Widget()
		saturationCheck.AddWidget(w)
		if anyPreparedTx {
			replicationLagCheck.AddWidget(w)
		}
	}
	oplogWindowCheck.AddWidget(report.GetOrCreateChart(mongoOplogWindowChartTitle, nil).Widget())
	oplogWindowCheck.AddWidget(report.GetOrCreateChart(mongoOplogWriteRateChartTitle, nil).Widget())
	oplogWindowCheck.AddWidget(report.GetOrCreateChartGroup(mongoQueriesChartTitle, nil).Widget())
	fragmentationCheck.AddWidget(report.GetOrCreateChartGroup(mongoFragmentationChartTitle, nil).Widget())

	if b := a.app.Cluster.Backups; b != nil && a.app.Cluster.Manager == model.ClusterManagerPerconaMongoDB &&
		(len(b.Methods) > 0 || b.Schedule != "" || len(b.Runs) > 0) {
		backupCheck := report.CreateCheck(model.Checks.MongodbBackups)
		mongoBackups(report, b, a.app.Id.Name, a.w.Ctx.To, backupCheck)
	}

	mongoConfigurationHints(report, a.app)
}

func mongoInstanceRow(table *model.Table, i *model.Instance, lagCell *model.TableCell) {
	mongo := i.Mongodb
	name := model.NewTableCell(i.Name)
	state := model.NewTableCell(mongo.State.Value())
	switch mongo.State.Value() {
	case "primary":
		state.SetIcon("mdi-database-edit-outline", "rgba(0,0,0,0.87)")
	case "secondary":
		state.SetIcon("mdi-database-import-outline", "grey")
	case "arbiter":
		state.SetIcon("mdi-database-eye-outline", "grey")
	}
	status := model.NewTableCell().SetStatus(model.OK, "up")
	if !mongo.IsUp() {
		if v := mongo.Error.Value(); v != "" {
			status.SetStatus(model.WARNING, v)
		} else {
			status.SetStatus(model.WARNING, "down (no metrics)")
		}
	} else if v := mongo.Warning.Value(); v != "" {
		status.SetStatus(model.OK, v)
	}

	qps := mongoOpsPerSecond(mongo)
	if qps.IsEmpty() {
		qps = timeseries.NewAggregate(timeseries.NanSum).Add(i.Requests.Ok, i.Requests.Failed).Get()
	}
	avgLatency := mongoAvgLatency(mongo)
	if avgLatency.IsEmpty() {
		avgLatency = timeseries.Div(i.Requests.TotalLatency, qps)
	}
	latencyCell := model.NewTableCell().SetUnit("ms")
	if last := avgLatency.Last(); last > 0 {
		latencyCell.SetValue(utils.FormatFloat(last * 1000))
	}
	table.AddRow(
		name,
		status,
		model.NewTableCell(mongo.ReplicaSet.Value()),
		state,
		model.NewTableCell(utils.FormatFloat(qps.Last())).SetUnit("/s"),
		latencyCell,
		lagCell,
		dbSizeCell(mongo.DatabaseSize),
		model.NewTableCell(mongo.Version.Value()))
}

func mongoOpsPerSecond(mongo *model.Mongodb) *timeseries.TimeSeries {
	total := timeseries.NewAggregate(timeseries.NanSum)
	for _, ts := range mongo.Opcounters {
		total.Add(ts)
	}
	return total.Get()
}

func mongoAvgLatency(mongo *model.Mongodb) *timeseries.TimeSeries {
	time := timeseries.NewAggregate(timeseries.NanSum)
	ops := timeseries.NewAggregate(timeseries.NanSum)
	for _, ts := range mongo.OpLatencyTime {
		time.Add(ts)
	}
	for _, ts := range mongo.OpLatencyOps {
		ops.Add(ts)
	}
	return timeseries.Div(time.Get(), ops.Get())
}

func mongoPrimaries(app *model.Application) map[string]*model.Instance {
	res := map[string]*model.Instance{}
	for _, i := range app.Instances {
		if i.Mongodb != nil && !i.IsObsolete() && i.Mongodb.IsUp() && i.Mongodb.State.Value() == "primary" {
			if rs := i.Mongodb.ReplicaSet.Value(); rs != "" {
				res[rs] = i
			}
		}
	}
	return res
}

func mongoInstanceLag(ctx timeseries.Context, i *model.Instance, primaryBaseline map[string]*timeseries.Aggregate) *timeseries.TimeSeries {
	if i.Mongodb.State.Value() == "primary" {
		return nil
	}
	rs := i.Mongodb.ReplicaSet.Value()
	if rs == "" || i.Mongodb.LastApplied.IsEmpty() {
		return nil
	}
	baseline := primaryBaseline[rs].Get()
	if baseline.IsEmpty() {
		return nil
	}
	return timeseries.Sub(i.Mongodb.LastApplied, baseline).MapInPlace(func(t timeseries.Time, v float32) float32 {
		if v < float32(ctx.Step) {
			return 0
		}
		return v
	})
}

func mongoLagCauses(primary *model.Instance, i *model.Instance, lagSeconds float32) []string {
	var causes []string
	mongo := i.Mongodb

	applyStalled := mongo.ReplApplyOps.Last() < 1 && mongo.ReplBufferCount.Last() > 0
	switch {
	case mongo.FsyncLocked.Last() > 0:
		causes = append(causes, "db.fsyncLock() is holding a global lock (typically a backup in progress), blocking the oplog applier, it resumes once the lock is released")
	case mongo.PreparedTransactions.Last() > 0:
		if app := mongoTopOpenTxApp(primary); app != "" {
			causes = append(causes, fmt.Sprintf("a prepared transaction from %s is holding locks, blocking the oplog applier until it commits or aborts", app))
		} else {
			causes = append(causes, "a prepared transaction is holding locks, blocking the oplog applier until it commits or aborts")
		}
	case applyStalled:
		causes = append(causes, "oplog apply is stalled, not slow (apply rate ~0, buffer backing up), a long-running op is likely holding a lock, check its long-running operations")
	}

	if net := appConnectivityIssue(i.Owner); net != "" {
		causes = append(causes, net)
	}
	if state := mongo.State.Value(); state != "secondary" && state != "primary" && state != "" {
		causes = append(causes, fmt.Sprintf("the member is in the %s state", strings.ToUpper(state)))
	}
	if primary != nil && primary.Mongodb != nil {
		if window := primary.Mongodb.OplogWindow.Last(); window > 0 && lagSeconds > 0 && lagSeconds > 0.5*window {
			causes = append(causes, fmt.Sprintf("the lag is approaching the oplog window (%s): the member is at risk of requiring a full resync",
				utils.FormatDuration(timeseries.Duration(window), 1)))
		}
		if primary.Mongodb.FlowControlTime.Last() > 0 {
			causes = append(causes, "flow control on the primary is throttling writes to bound the lag")
		}
	}
	if mongoQueuedOps(mongo) > 0 {
		causes = append(causes, "the secondary is saturated: operations are queuing (waiting for a lock or a ticket)")
	}
	if mongo.CacheEvictedPagesByApp.Last() > 0 {
		causes = append(causes, "application threads on the secondary are evicting WiredTiger cache pages")
	}
	if await := mongoInstanceDiskAwait(i); await > mongoSlowDiskAwaitSeconds {
		causes = append(causes, fmt.Sprintf("the secondary's data disk is slow (I/O latency ~%s), so it applies the oplog slower than the primary produces it",
			utils.FormatFloat(await*1000)+"ms"))
	}
	if !mongo.IsUp() {
		causes = append(causes, "the secondary is not responding to monitoring queries")
	}
	if len(causes) == 0 {
		causes = append(causes, "the secondary is applying the oplog slower than the primary produces it")
	}
	return causes
}

func mongoQueuedOps(mongo *model.Mongodb) float32 {
	var total float32
	for _, ts := range mongo.QueuedOperations {
		if v := ts.Last(); !timeseries.IsNaN(v) {
			total += v
		}
	}
	return total
}

func mongoTopOpenTxApp(primary *model.Instance) string {
	if primary == nil || primary.Mongodb == nil {
		return ""
	}
	best, bestN := "", float32(0)
	for app, ts := range primary.Mongodb.OpenTransactionsByApp {
		if v := ts.Last(); !timeseries.IsNaN(v) && v > bestN {
			best, bestN = app, v
		}
	}
	return best
}

func mongoOpenTransactionsChart(report *model.AuditReport, app *model.Application) bool {
	byApp := map[string]*timeseries.Aggregate{}
	for _, i := range app.Instances {
		if i.Mongodb == nil {
			continue
		}
		for a, ts := range i.Mongodb.OpenTransactionsByApp {
			if byApp[a] == nil {
				byApp[a] = timeseries.NewAggregate(timeseries.NanSum)
			}
			byApp[a].Add(ts)
		}
	}
	if len(byApp) == 0 {
		return false
	}
	chart := report.GetOrCreateChart(mongoOpenTransactionsTitle, nil).Group("Concurrency", 3).Stacked()
	for a, agg := range byApp {
		chart.AddSeries(a, agg.Get())
	}
	return true
}

func mongoInstanceDiskAwait(i *model.Instance) float32 {
	if i.Node == nil {
		return 0
	}
	var maxAwait float32
	for _, v := range i.Volumes {
		disk := i.Node.Disks[v.Device.Value()]
		if disk == nil {
			continue
		}
		if a := disk.Await.Last(); !timeseries.IsNaN(a) && a > maxAwait {
			maxAwait = a
		}
	}
	return maxAwait
}

func mongoClusterAvailability(app *model.Application, primaries map[string]*model.Instance, check *model.Check) {
	rsUp := map[string]bool{}
	for _, i := range app.Instances {
		if i.Mongodb == nil || i.IsObsolete() {
			continue
		}
		rs := i.Mongodb.ReplicaSet.Value()
		if rs == "" {
			continue
		}
		if _, ok := rsUp[rs]; !ok {
			rsUp[rs] = false
		}
		if i.Mongodb.IsUp() {
			rsUp[rs] = true
		}
		switch state := i.Mongodb.State.Value(); state {
		case "recovering", "rollback", "startup", "startup2", "down", "unknown":
			check.AddItem("%s", i.Name)
			check.AddDetail("member %s is in the %s state and is not replicating normally", i.Name, strings.ToUpper(state))
		}
	}

	for rs, up := range rsUp {
		if up && primaries[rs] == nil {
			check.AddItem("%s", rs)
			check.AddDetail("replica set %s has no primary", rs)
		}
	}
}

func mongoQueries(report *model.AuditReport, i *model.Instance, latencyCheck *model.Check) {
	mongo := i.Mongodb

	avgLatency := mongoAvgLatency(mongo)
	report.
		GetOrCreateChartInGroup(mongoLatencyChartTitle, "overview (avg)", nil).
		Group("Queries", 1).
		Feature().
		AddSeries(i.Name, avgLatency)
	latencyByType := report.GetOrCreateChartInGroup(mongoLatencyChartTitle, i.Name, nil).Group("Queries", 1)
	if latencyByType != nil {
		for typ, time := range mongo.OpLatencyTime {
			latencyByType.AddSeries(typ, timeseries.Div(time, mongo.OpLatencyOps[typ]))
		}
	}

	ops := map[string]model.SeriesData{}
	for op, ts := range mongo.Opcounters {
		ops[op] = ts
	}
	report.
		GetOrCreateChartInGroup(mongoQueriesChartTitle, i.Name, nil).
		Group("Queries", 1).
		Stacked().
		AddMany(ops, 10, timeseries.NanSum)

	totalTime := map[string]model.SeriesData{}
	for k, stat := range mongo.PerQuery {
		totalTime[k.String()] = stat.TotalTime
	}
	report.
		GetOrCreateChartInGroup(mongoTopQueriesByTimeTitle, i.Name, nil).
		Group("Queries", 1).
		Stacked().
		Sorted().
		AddMany(totalTime, 5, timeseries.NanSum)

	report.
		GetOrCreateChartInGroup(mongoQueryEfficiencyTitle, i.Name, nil).
		Group("Queries", 1).
		AddSeries("documents scanned", mongo.ScannedDocuments).
		AddSeries("index keys scanned", mongo.ScannedKeys).
		AddSeries("documents returned", mongo.DocumentsReturned)
	report.
		GetOrCreateChartInGroup(mongoCollscanTitle, i.Name, nil).
		Group("Queries", 1).
		AddSeries("collection scans", mongo.CollectionScans).
		AddSeries("in-memory sorts", mongo.ScanAndOrder)

	if avgLatency.Last() > latencyCheck.Threshold {
		latencyCheck.AddItem("%s", i.Name)
		for _, cause := range mongoLatencyCauses(mongo) {
			latencyCheck.AddDetail("%s: %s", i.Name, cause)
		}
	}
}

func mongoLatencyCauses(mongo *model.Mongodb) []string {
	var causes []string
	if mongoQueuedOps(mongo) > 0 {
		causes = append(causes, "operations are queuing (waiting for a lock or a ticket)")
	}
	if mongo.CacheEvictedPagesByApp.Last() > 0 {
		causes = append(causes, "application threads are evicting WiredTiger cache pages")
	}
	var topQuery string
	var topTime float32
	for k, stat := range mongo.PerQuery {
		if v := stat.TotalTime.Last(); !timeseries.IsNaN(v) && v > topTime {
			topTime = v
			topQuery = k.String()
		}
	}
	if topQuery != "" && topTime > mongoLatencyDetailQueryMinTime {
		causes = append(causes, fmt.Sprintf("the most time-consuming query: %s", topQuery))
	}
	return causes
}

func mongoConnections(report *model.AuditReport, i *model.Instance, check *model.Check) {
	mongo := i.Mongodb
	if mongo.ConnectionsCurrent.IsEmpty() {
		return
	}
	limit := mongo.ConnectionsMax
	report.
		GetOrCreateChartInGroup(mongoConnectionsChartTitle, i.Name, nil).
		Group("Connections", 2).
		SetThreshold("limit", limit).
		AddSeries("current", mongo.ConnectionsCurrent).
		AddSeries("active", mongo.ConnectionsActive)

	report.
		GetOrCreateChartInGroup(mongoNewConnectionsTitle, i.Name, nil).
		Group("Connections", 2).
		AddSeries("created", mongo.ConnectionsCreated).
		AddSeries("rejected", mongo.ConnectionsRejected)

	report.GetOrCreateChart(mongoCursorsOpenTitle, nil).Group("Connections", 2).AddSeries(i.Name, mongo.CursorsOpen)
	report.GetOrCreateChart(mongoCursorsTimedOutTitle, nil).Group("Connections", 2).AddSeries(i.Name, mongo.CursorsTimedOut)

	byApp := map[string]model.SeriesData{}
	for app, ts := range mongo.ConnectionsByApp {
		byApp[app] = ts
	}
	report.
		GetOrCreateChartInGroup(mongoConnectionsByAppTitle, i.Name, nil).
		Group("Connections", 2).
		Stacked().
		AddMany(byApp, 10, timeseries.NanSum)

	current, limitLast := mongo.ConnectionsCurrent.Last(), limit.Last()
	if current > 0 && limitLast > 0 && current/limitLast*100 > check.Threshold {
		check.AddItem("%s", i.Name)
		if mongo.ConnectionsRejected.Last() > 0 {
			check.AddDetail("%s: connections are being rejected", i.Name)
		}
		if created := mongo.ConnectionsCreated.Last(); created > mongoConnectionStormPerSecond {
			check.AddDetail("%s: connections are being created at %s/s", i.Name, utils.FormatFloat(created))
		}
		var topApp string
		var topCount float32
		for app, ts := range mongo.ConnectionsByApp {
			if v := ts.Last(); !timeseries.IsNaN(v) && v > topCount {
				topCount = v
				topApp = app
			}
		}
		if topApp != "" {
			check.AddDetail("%s: the top client application by connection count: %s (%s connections) - check its connection pool settings",
				i.Name, topApp, utils.FormatFloat(topCount))
		}
	}
}

func mongoOperations(report *model.AuditReport, i *model.Instance, check *model.Check) {
	mongo := i.Mongodb

	queued := timeseries.NewAggregate(timeseries.NanSum)
	for _, ts := range mongo.QueuedOperations {
		queued.Add(ts)
	}
	queuedTotal := queued.Get()

	queuedChart := report.GetOrCreateChartInGroup(mongoQueuedOpsTitle, i.Name, nil).Group("Concurrency", 3).Stacked()
	for typ, ts := range mongo.QueuedOperations {
		queuedChart.AddSeries(typ, ts)
	}
	ticketsChart := report.GetOrCreateChartInGroup(mongoTicketsTitle, i.Name, nil).Group("Concurrency", 3)
	for typ, ts := range mongo.TicketsAvailable {
		ticketsChart.AddSeries(typ, ts)
	}
	report.GetOrCreateChart(mongoWriteConflictsTitle, nil).Group("Concurrency", 3).AddSeries(i.Name, mongo.WriteConflicts)

	mongoRunningOpsChart(report, i)

	if queuedTotal.LastNAvg(mongoSaturationRecentPoints, 0) > check.Threshold {
		check.AddItem("%s", i.Name)
		ticketsExhausted := false
		for _, ts := range mongo.TicketsAvailable {
			if ts.LastNAvg(mongoSaturationRecentPoints, math.MaxFloat32) <= 0 {
				ticketsExhausted = true
			}
		}
		if ticketsExhausted {
			check.AddDetail("%s: out of WiredTiger tickets - the storage engine is the bottleneck (slow disk, cache pressure, or too many concurrent ops)", i.Name)
		} else {
			check.AddDetail("%s: queued on a lock, not out of tickets - a lock holder is blocking (fsyncLock/backup, a long DDL/index build, or a long transaction)", i.Name)
		}
		var waiting float32
		for _, ts := range mongo.OperationsWaitingForLock {
			if v := ts.Last(); !timeseries.IsNaN(v) {
				waiting += v
			}
		}
		if waiting > 0 {
			check.AddDetail("%s: %s operations are waiting for locks", i.Name, utils.FormatFloat(waiting))
		}
		if d := mongo.CheckpointTime.Last(); !timeseries.IsNaN(d) && d > mongoCheckpointBusyFraction {
			check.AddDetail("%s: WiredTiger spent %ss of every second in checkpoints", i.Name, utils.FormatFloat(d))
		}
		count := 0
		for k, ts := range mongo.LongRunningOperations {
			v := ts.Last()
			if timeseries.IsNaN(v) || v <= 0 {
				continue
			}
			plan := ""
			if k.Plan != "" {
				plan = " (" + k.Plan + ")"
			}
			check.AddDetail("%s: an operation on %s.%s has been running for %s%s: %s",
				i.Name, k.DB, k.Collection, utils.FormatDuration(timeseries.Duration(v), 1), plan, k.Query)
			count++
			if count >= 3 {
				break
			}
		}
	}
}

func mongoRunningOpsChart(report *model.AuditReport, i *model.Instance) {
	byShape := map[string]model.SeriesData{}
	for k, ts := range i.Mongodb.LongRunningOperations {
		if peak := ts.Reduce(timeseries.Max); timeseries.IsNaN(peak) || peak <= 0 {
			continue
		}
		label := k.DB + "." + k.Collection + " " + k.Query
		if k.Plan != "" {
			label = k.Plan + " " + label
		}
		byShape[label] = ts
	}
	if len(byShape) == 0 {
		return
	}
	report.
		GetOrCreateChartInGroup(mongoRunningOpsTitle, i.Name, nil).
		Group("Queries", 1).
		Stacked().
		Sorted().
		AddMany(byShape, 10, timeseries.NanSum)
}

func mongoCache(report *model.AuditReport, i *model.Instance) {
	mongo := i.Mongodb
	if mongo.CacheMax.IsEmpty() {
		return
	}
	report.
		GetOrCreateChartInGroup(mongoCacheTitle, i.Name, nil).
		Group("WiredTiger", 4).
		SetThreshold("cache size", mongo.CacheMax).
		AddSeries("used", mongo.CacheUsed).
		AddSeries("dirty", mongo.CacheDirty)

	report.GetOrCreateChart(mongoCacheEvictionsTitle, nil).Group("WiredTiger", 4).AddSeries(i.Name, mongo.CacheEvictedPagesByApp)
	report.GetOrCreateChart(mongoCacheReadIntoTitle, nil).Group("WiredTiger", 4).AddSeries(i.Name, mongo.CacheBytesReadInto)
	report.GetOrCreateChart(mongoCheckpointsTitle, nil).Group("WiredTiger", 4).AddSeries(i.Name, mongo.Checkpoints)
	report.GetOrCreateChart(mongoCheckpointTimeTitle, nil).Group("WiredTiger", 4).AddSeries(i.Name, mongo.CheckpointTime)
	report.GetOrCreateChart(mongoJournalWriteRateTitle, nil).Group("WiredTiger", 4).AddSeries(i.Name, mongo.JournalBytesWritten)
	report.GetOrCreateChart(mongoJournalRecoveryTitle, nil).Group("WiredTiger", 4).AddSeries(i.Name, mongo.JournalBytesSinceCheckpoint)
	report.GetOrCreateChart(mongoTimeSinceCheckpointTitle, nil).Group("WiredTiger", 4).AddSeries(i.Name, mongo.TimeSinceCheckpoint)
}

func mongoOplog(report *model.AuditReport, i *model.Instance, check *model.Check) {
	mongo := i.Mongodb
	if mongo.OplogWindow.IsEmpty() {
		return
	}
	report.GetOrCreateChart(mongoOplogWindowChartTitle, nil).Group("Replication", 5).AddSeries(i.Name, mongo.OplogWindow)
	report.GetOrCreateChart(mongoOplogWriteRateChartTitle, nil).Group("Replication", 5).
		AddSeries(i.Name, timeseries.Div(mongo.OplogUsedSize, mongo.OplogWindow))
	report.GetOrCreateChart(mongoReplApplyTitle, nil).Group("Replication", 5).AddSeries(i.Name, mongo.ReplApplyOps)
	report.GetOrCreateChart(mongoReplBufferTitle, nil).Group("Replication", 5).AddSeries(i.Name, mongo.ReplBufferCount)

	window := mongo.OplogWindow.Last()
	if timeseries.IsNaN(window) {
		return
	}
	used, maxSize := mongo.OplogUsedSize.Last(), mongo.OplogMaxSize.Last()
	oplogFull := used > 0 && maxSize > 0 && used >= mongoOplogFullRatio*maxSize
	if window < check.Threshold && oplogFull {
		check.AddItem("%s", i.Name)
		if window > 0 && maxSize > 0 {
			churn, churnUnit := utils.FormatBytes(maxSize / window * 3600)
			size, sizeUnit := utils.FormatBytes(maxSize)
			check.AddDetail("%s: the %s%s oplog covers only %s of changes at the current write rate (%s%s/hour) - any member lagging or down for longer will require a full resync, consider increasing the oplog size",
				i.Name, size, sizeUnit, utils.FormatDuration(timeseries.Duration(window), 1), churn, churnUnit)
		}
		if ttl := mongo.TtlDeleted.Last(); ttl > 1 {
			check.AddDetail("%s: TTL indexes delete %s documents/s, contributing to the oplog churn", i.Name, utils.FormatFloat(ttl))
		}
	}
}

func mongoStorage(report *model.AuditReport, i *model.Instance, check *model.Check) {
	mongo := i.Mongodb

	dbSize := map[string]model.SeriesData{}
	for db, ts := range mongo.DatabaseSize {
		dbSize[db] = ts
	}
	report.
		GetOrCreateChartInGroup(mongoDbSizeChartTitle, i.Name, nil).
		Group("Storage", 6).
		Stacked().
		Sorted().
		AddMany(dbSize, 20, timeseries.Max)

	collSize := map[string]model.SeriesData{}
	for k, ts := range mongo.CollectionSize {
		collSize[k.String()] = ts
	}
	report.
		GetOrCreateChartInGroup(mongoCollectionSizeChartTitle, i.Name, nil).
		Group("Storage", 6).
		Stacked().
		Sorted().
		AddMany(collSize, 20, timeseries.Max)

	collDocs := map[string]model.SeriesData{}
	for k, ts := range mongo.CollectionDocuments {
		collDocs[k.String()] = ts
	}
	report.
		GetOrCreateChartInGroup(mongoCollectionDocsChartTitle, i.Name, nil).
		Group("Storage", 6).
		Stacked().
		Sorted().
		AddMany(collDocs, 20, timeseries.Max)

	free := map[string]model.SeriesData{}
	for k, ts := range mongo.CollectionFreeStorage {
		free[k.String()] = ts
	}
	report.
		GetOrCreateChartInGroup(mongoFragmentationChartTitle, i.Name, nil).
		Group("Storage", 6).
		Stacked().
		Sorted().
		AddMany(free, 20, timeseries.Max)

	if i.Mongodb.State.Value() != "primary" {
		return
	}
	for k, freeTs := range mongo.CollectionFreeStorage {
		freeBytes := freeTs.Last()
		storageBytes := mongo.CollectionStorageSize[k].Last()
		if timeseries.IsNaN(freeBytes) || timeseries.IsNaN(storageBytes) || storageBytes == 0 {
			continue
		}
		if freeBytes > mongoFragmentationMinBytes && freeBytes/storageBytes*100 > check.Threshold {
			check.AddItem("%s", k.String())
			f, fu := utils.FormatBytes(freeBytes)
			s, su := utils.FormatBytes(storageBytes)
			check.AddDetail("%s: %s%s of the %s%s allocated storage is reclaimable free space - run compact (rolling, secondaries first) to reclaim it",
				k.String(), f, fu, s, su)
		}
	}
}

func mongoBackups(report *model.AuditReport, b *model.PgBackups, cluster string, now timeseries.Time, check *model.Check) {
	var lastSuccess timeseries.Time
	for _, m := range b.Methods {
		if m.LastSuccessfulBackup > lastSuccess {
			lastSuccess = m.LastSuccessfulBackup
		}
	}
	var lastRun *model.PgBackupRun
	for _, r := range b.Runs {
		if r.Succeeded() && r.CompletedAt > lastSuccess {
			lastSuccess = r.CompletedAt
		}
		if lastRun == nil || r.CompletedAt > lastRun.CompletedAt {
			lastRun = r
		}
	}
	var backupAge = timeseries.NaN
	if lastSuccess > 0 {
		backupAge = float32(now.Sub(lastSuccess))
		check.SetValue(backupAge)
	}

	var reasons []string
	switch {
	case timeseries.IsNaN(backupAge):
		reasons = append(reasons, "no successful backup has been recorded")
	case backupAge > check.Threshold:
		reasons = append(reasons, fmt.Sprintf("the last successful backup was %s ago", utils.FormatDuration(timeseries.Duration(backupAge), 1)))
	}
	if lastRun != nil && !lastRun.Succeeded() {
		switch lastRun.Status {
		case "error", "Failed", "failed":
			reasons = append(reasons, "the last backup attempt failed")
		}
	}
	if b.Conditions["state"].Status == "error" {
		reasons = append(reasons, "the cluster is in the error state")
	}
	if overdue, overdueBy := pgBackupOverdue(b, lastSuccess, now); overdue {
		reasons = append(reasons, fmt.Sprintf("the scheduled backup is overdue (was due %s ago) - the backup schedule may not be running", utils.FormatDuration(overdueBy, 1)))
	}
	if len(reasons) > 0 {
		check.AddItem("%s", cluster)
		for _, r := range reasons {
			check.AddDetail("%s: %s", cluster, r)
		}
	}

	table := report.GetOrCreateTable("", "").Group("Backup", 7).SetSorted()
	if table == nil {
		return
	}
	status := model.NewTableCell().SetStatus(model.OK, "ok")
	if len(reasons) > 0 {
		status = model.NewTableCell().SetStatus(model.WARNING, reasons[0])
	}
	table.AddRow(model.NewTableCell("Status"), status)

	for name, m := range b.Methods {
		prefix := name + " "
		dest := model.NewTableCell(m.Destination)
		if m.Endpoint != "" {
			dest.AddTag("%s", m.Endpoint)
		}
		table.AddRow(model.NewTableCell(prefix+"destination"), dest)
		if m.Schedule != "" {
			table.AddRow(model.NewTableCell(prefix+"schedule"), model.NewTableCell(m.Schedule))
		}
	}
	if lastSuccess > 0 {
		table.AddRow(model.NewTableCell("Last successful backup"), pgTimeCell(float32(lastSuccess), now))
	} else {
		table.AddRow(model.NewTableCell("Last successful backup"), model.NewTableCell("none"))
	}
	if next := pgNextScheduledBackup(b, now); next > 0 {
		table.AddRow(model.NewTableCell("Next backup"), pgTimeCell(float32(next), now))
	}
	if pitr, ok := b.Conditions["pitr"]; ok {
		cell := model.NewTableCell().SetStatus(model.OK, "enabled")
		if pitr.Status != "true" {
			cell = model.NewTableCell().SetStatus(model.WARNING, "disabled")
		}
		table.AddRow(model.NewTableCell("Point-in-time recovery"), cell)
	}
	check.AddWidget(table.Widget())

	pgBackupRuns(report, b.Runs, now, check)
}

func mongoConfigurationHints(report *model.AuditReport, app *model.Application) {
	if report.ConfigurationHint != nil {
		return
	}
	profilingSeen, profilingEnabled, isPercona := false, false, false
	for _, i := range app.Instances {
		if i.Mongodb == nil || i.IsObsolete() {
			continue
		}
		if i.Mongodb.Flavor.Value() == "percona" {
			isPercona = true
		}
		for _, level := range i.Mongodb.ProfilingLevel {
			if !level.IsEmpty() {
				profilingSeen = true
				if level.Last() > 0 {
					profilingEnabled = true
				}
			}
		}
	}
	if profilingSeen && !profilingEnabled {
		msg := "Enable the database profiler so Coroot can collect per-query statistics - without it, the top-queries view stays empty: " +
			"set operationProfiling to mode: slowOp, slowOpThresholdMs: 200."
		if isPercona {
			msg = "Enable the database profiler so Coroot can collect per-query statistics - without it, the top-queries view stays empty. " +
				"Percona Server for MongoDB supports sampling fast operations: set operationProfiling to mode: all, slowOpThresholdMs: 200, rateLimit: 100."
		}
		report.ConfigurationHint = &model.ConfigurationHint{
			Message:      msg,
			ReadMoreLink: "https://docs.coroot.com/databases/mongodb",
		}
	}
}

func calcMongoPrimaryBaseline(app *model.Application) map[string]*timeseries.Aggregate {
	res := map[string]*timeseries.Aggregate{}
	for _, i := range app.Instances {
		if i.Mongodb == nil || i.Mongodb.LastApplied.IsEmpty() {
			continue
		}
		rs := i.Mongodb.ReplicaSet.Value()
		if rs == "" {
			continue
		}
		p := res[rs]
		if p == nil {
			p = timeseries.NewAggregate(timeseries.Max)
			res[rs] = p
		}
		p.Add(timeseries.Mul(
			i.Mongodb.LastApplied,
			i.ClusterRole().Map(func(t timeseries.Time, v float32) float32 {
				if v > 1 {
					return 0
				}
				return v
			}),
		))
	}
	return res
}
