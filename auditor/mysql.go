package auditor

import (
	"fmt"
	"strings"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

func (a *appAuditor) mysql() {
	isMysql := a.app.ApplicationTypes()[model.ApplicationTypeMysql]

	if !isMysql && !a.app.IsMysql() {
		return
	}

	report := a.addReport(model.AuditReportMysql)

	report.Instrumentation = model.ApplicationTypeMysql

	if !a.app.IsMysql() {
		report.Status = model.UNKNOWN
		return
	}

	availabilityCheck := report.CreateCheck(model.Checks.MysqlAvailability)
	latencyCheck := report.CreateCheck(model.Checks.MysqlLatency)
	replicationStatusCheck := report.CreateCheck(model.Checks.MysqlReplicationStatus)
	replicationLagCheck := report.CreateCheck(model.Checks.MysqlReplicationLag)
	connectionsCheck := report.CreateCheck(model.Checks.MysqlConnections)
	connectionErrorsCheck := report.CreateCheck(model.Checks.MysqlConnectionErrors)
	galeraCheck := report.CreateCheck(model.Checks.MysqlGaleraReplication)
	groupReplicationCheck := report.CreateCheck(model.Checks.MysqlGroupReplication)

	table := report.GetOrCreateTable("Instance", "Status", "Queries", "Latency", "Replication status", "Replication lag", "DB Size", "Version")
	qpsChart := report.GetOrCreateChartGroup("Queries <selector>, per second", nil).Group("Queries", 1)
	latencyChart := report.GetOrCreateChart("Average latency, seconds", nil).Group("Queries", 1)
	queriesByTotalTime := report.GetOrCreateChartGroup("Queries by total time <selector>, query seconds/second", nil).Group("Queries", 1)
	tablesByIOTime := report.GetOrCreateChartGroup("I/O time by table <selector>, IO seconds/second", nil).Group("Queries", 1)
	slowQueriesChart := report.GetOrCreateChart("Slow queries, per second", nil).Group("Queries", 1)

	connectionsChart := report.GetOrCreateChartGroup("Connections <selector>", nil).Group("Connections", 2)
	newConnectionsChart := report.GetOrCreateChart("New connections, per second", nil).Group("Connections", 2)
	abortedConnectionsChart := report.GetOrCreateChart("Aborted connections, per second", nil).Group("Connections", 2)

	threadsRunningChart := report.GetOrCreateChart("Running threads", nil).Group("Load", 3)

	lockedQueriesChart := report.GetOrCreateChartGroup("Locked queries on <selector>", nil).Group("Locks", 4)
	blockingQueriesChart := report.GetOrCreateChartGroup("Blocking queries by the number of awaiting queries on <selector>", nil).Group("Locks", 4)
	longTxChart := report.GetOrCreateChartGroup("Long-running transactions on <selector>, seconds", nil).Group("Locks", 4)
	deadlocksChart := report.GetOrCreateChart("InnoDB deadlocks and lock-wait timeouts, per second", nil).Group("Locks", 4)
	historyListChart := report.GetOrCreateChart("InnoDB history list length (unpurged undo records)", nil).Group("Locks", 4)
	tableLocksWaitedChart := report.GetOrCreateChart("Table lock waits, per second", nil).Group("Locks", 4)

	galeraFlowControlChart := report.GetOrCreateChart("Galera flow control, fraction of time paused", nil).Group("Galera", 5)
	galeraQueuesChart := report.GetOrCreateChartGroup("Galera write-set queues <selector>", nil).Group("Galera", 5)
	galeraClusterSizeChart := report.GetOrCreateChart("Galera cluster size", nil).Group("Galera", 5)
	galeraConflictsChart := report.GetOrCreateChart("Galera certification conflicts, per second", nil).Group("Galera", 5)

	grMembersChart := report.GetOrCreateChart("Group Replication members online", nil).Group("Group Replication", 5)
	grQueuesChart := report.GetOrCreateChartGroup("Group Replication queues <selector>", nil).Group("Group Replication", 5)
	grConflictsChart := report.GetOrCreateChart("Group Replication certification conflicts, per second", nil).Group("Group Replication", 5)

	replicationLagChart := report.GetOrCreateChart("Replication lag, seconds", nil).Group("Replication", 6)
	trafficChart := report.GetOrCreateChartGroup("Traffic <selector>, bytes per second", nil).Group("Traffic", 7)
	dbSizeChart := report.GetOrCreateChartGroup("Database size <selector>, bytes", nil).Group("Storage", 8)
	tableSizeChart := report.GetOrCreateChartGroup("Top tables by size <selector>, bytes", nil).Group("Storage", 8)
	binlogSizeChart := report.GetOrCreateChart("Binary logs size, bytes", nil).Group("Storage", 8)
	undoSizeChart := report.GetOrCreateChart("InnoDB undo tablespaces size, bytes", nil).Group("Storage", 8)

	availabilityCheck.AddWidget(table.Widget())
	availabilityCheck.AddWidget(galeraClusterSizeChart.Widget())

	replicationStatusCheck.AddWidget(table.Widget())
	replicationStatusCheck.AddWidget(replicationLagChart.Widget())
	replicationLagCheck.AddWidget(replicationLagChart.Widget())
	connectionsCheck.AddWidget(connectionsChart.Widget())
	connectionErrorsCheck.AddWidget(abortedConnectionsChart.Widget())
	galeraCheck.AddWidget(galeraFlowControlChart.Widget())
	galeraCheck.AddWidget(galeraQueuesChart.Widget())
	galeraCheck.AddWidget(galeraConflictsChart.Widget())
	groupReplicationCheck.AddWidget(grMembersChart.Widget())
	groupReplicationCheck.AddWidget(grQueuesChart.Widget())
	groupReplicationCheck.AddWidget(grConflictsChart.Widget())
	latencyCheck.AddWidget(latencyChart.Widget())
	latencyCheck.AddWidget(queriesByTotalTime.Widget())

	netIssue := appConnectivityIssue(a.app)

	for _, i := range a.app.Instances {
		if i.Mysql == nil {
			continue
		}
		obsolete := i.IsObsolete()
		if !obsolete && !i.Mysql.IsUp() {
			availabilityCheck.AddItem("%s", i.Name)
		}

		if obsolete {
			continue
		}

		threadsRunningChart.AddSeries(i.Name, i.Mysql.ThreadsRunning)
		tableLocksWaitedChart.AddSeries(i.Name, i.Mysql.TableLocksWaited)
		binlogSizeChart.AddSeries(i.Name, i.Mysql.BinlogSize)
		undoSizeChart.AddSeries(i.Name, i.Mysql.UndoSize)
		if inn := i.Mysql.Innodb; inn != nil {
			deadlocksChart.AddSeries(i.Name+" deadlocks", inn.Deadlocks)
			deadlocksChart.AddSeries(i.Name+" lock-wait timeouts", inn.LockWaitTimeouts)
			historyListChart.AddSeries(i.Name, inn.HistoryListLength)
			mysqlInnodb(report, i.Name, inn)
		}
		if g := i.Mysql.Galera; g != nil {
			mysqlGalera(i.Name, g, galeraCheck, netIssue,
				galeraFlowControlChart, galeraQueuesChart, galeraClusterSizeChart, galeraConflictsChart)
		}
		if gr := i.Mysql.GroupReplication; gr != nil {
			mysqlGroupReplication(i.Name, gr, groupReplicationCheck, netIssue,
				grMembersChart, grQueuesChart, grConflictsChart)
		}
		lagCell := model.NewTableCell()

		if !i.Mysql.ReplicationLagSeconds.IsEmpty() {
			if lagTime := i.Mysql.ReplicationLagSeconds.Last(); !timeseries.IsNaN(lagTime) {
				lagCell.SetValue(utils.FormatFloat(lagTime)).SetUnit("s")
				if timeseries.Duration(lagTime) > timeseries.Duration(replicationLagCheck.Threshold) {
					replicationLagCheck.AddItem("%s", i.Name)
					if netIssue != "" {
						replicationLagCheck.AddDetail("%s: %s", i.Name, netIssue)
					}
				}
			}
			if replicationLagChart != nil {
				replicationLagChart.AddSeries(i.Name, i.Mysql.ReplicationLagSeconds)
			}
		}
		currConns := i.Mysql.ConnectionsCurrent.Last()
		maxConns := i.Mysql.ConnectionsMax.Last()
		if !timeseries.IsNaN(currConns) && !timeseries.IsNaN(maxConns) && maxConns > 0 {
			if currConns/maxConns*100 > connectionsCheck.Threshold {
				connectionsCheck.AddItem("%s", i.Name)
				connectionsCheck.AddDetail("%s: %s of %s connections used",
					i.Name, utils.FormatFloat(currConns), utils.FormatFloat(maxConns))
			}
		}
		if !i.Mysql.ConnectionErrorsMaxConnections.IsEmpty() {
			if refused := i.Mysql.ConnectionErrorsMaxConnections.LastNAvg(mysqlRecentPoints, 0); refused > 0 {
				connectionsCheck.AddItem("%s", i.Name)
				if timeseries.IsNaN(maxConns) {
					connectionsCheck.AddDetail("%s: refusing new connections - max_connections reached", i.Name)
				} else {
					connectionsCheck.AddDetail("%s: refusing new connections - max_connections (%s) reached",
						i.Name, utils.FormatFloat(maxConns))
				}
			}
		}
		if !i.Mysql.ConnectionsAborted.IsEmpty() {
			abortedConnectionsChart.AddSeries(i.Name, i.Mysql.ConnectionsAborted)
			if aborted := i.Mysql.ConnectionsAborted.LastNAvg(mysqlRecentPoints, 0); aborted > float32(connectionErrorsCheck.Threshold) {
				connectionErrorsCheck.AddItem("%s", i.Name)
				connectionErrorsCheck.AddDetail("%s: %s/s connection attempts are being aborted - clients are failing to connect (bad credentials, TLS errors, or handshake timeouts)",
					i.Name, utils.FormatFloat(aborted))
			}
		}
		if connectionsChart != nil {
			connectionsChart.GetOrCreateChart("overview").Feature().AddSeries(i.Name, i.Mysql.ConnectionsCurrent)
			connectionsChart.
				GetOrCreateChart(i.Name).
				Stacked().
				AddSeries("current", i.Mysql.ConnectionsCurrent).
				SetThreshold("max", i.Mysql.ConnectionsMax)
		}
		if newConnectionsChart != nil {
			newConnectionsChart.AddSeries(i.Name, i.Mysql.ConnectionsNew)
		}
		if slowQueriesChart != nil {
			slowQueriesChart.AddSeries(i.Name, i.Mysql.SlowQueries)
		}

		replStatusCell := model.NewTableCell()
		if i.Mysql.ReplicationIOStatus != nil && i.Mysql.ReplicationSQLStatus != nil {
			replStatusCell.SetStatus(model.OK, "ok")
		}
		if i.Mysql.ReplicationIOStatus != nil {
			status := i.Mysql.ReplicationIOStatus.Status.Last()
			if status < 1 {
				replicationStatusCheck.AddItem("%s", i.Name)
				msg := i.Mysql.ReplicationIOStatus.LastError.Value()
				if msg == "" {
					msg = i.Mysql.ReplicationIOStatus.LastState.Value()
				}
				if msg == "" {
					msg = "IO replication thread is not running"
				}
				replStatusCell.
					SetStatus(model.WARNING, msg)
			}
		}
		if replStatusCell.Status != nil && *replStatusCell.Status < model.WARNING && i.Mysql.ReplicationSQLStatus != nil {
			status := i.Mysql.ReplicationSQLStatus.Status.Last()
			if status < 1 {
				replicationStatusCheck.AddItem("%s", i.Name)
				msg := i.Mysql.ReplicationSQLStatus.LastError.Value()
				if msg == "" {
					msg = i.Mysql.ReplicationSQLStatus.LastState.Value()
				}
				if msg == "" {
					msg = "SQL replication thread is not running"
				}
				replStatusCell.
					SetStatus(model.WARNING, msg)
			}
		}

		totalQps := timeseries.NewAggregate(timeseries.NanSum).Add(i.Requests.Ok, i.Requests.Failed).Get()
		avgLatency := timeseries.Div(i.Requests.TotalLatency, totalQps)

		if qpsChart != nil {
			qpsChart.GetOrCreateChart("eBPF").Feature().AddSeries(i.Name, totalQps)
			qpsChart.GetOrCreateChart("Mysql status").AddSeries(i.Name, i.Mysql.Queries)
		}
		if latencyChart != nil {
			latencyChart.AddSeries(i.Name, avgLatency)
		}
		if avgLatency.LastNAvg(mysqlRecentPoints, 0) > float32(latencyCheck.Threshold) {
			latencyCheck.AddItem("%s", i.Name)
			for _, cause := range mysqlLatencyCauses(i) {
				latencyCheck.AddDetail("%s: %s", i.Name, cause)
			}
		}

		if table != nil {
			name := model.NewTableCell(i.Name)
			status := model.NewTableCell().SetStatus(model.OK, "up")
			if !i.Mysql.IsUp() {
				if v := i.Mysql.Error.Value(); v != "" {
					status.SetStatus(model.WARNING, v)
				} else {
					status.SetStatus(model.WARNING, "down (no metrics)")
				}
			} else if g := i.Mysql.Galera; g != nil && g.LocalStateComment.Value() != "" {
				state := utils.Capitalize(g.LocalStateComment.Value())
				if strings.EqualFold(g.LocalStateComment.Value(), "synced") {
					status.SetStatus(model.OK, state)
				} else {
					status.SetStatus(model.WARNING, state)
				}
			} else if gr := i.Mysql.GroupReplication; gr != nil && gr.State.Value() != "" {
				state := utils.Capitalize(gr.State.Value())
				if strings.EqualFold(gr.State.Value(), "online") {
					status.SetStatus(model.OK, state)
				} else {
					status.SetStatus(model.WARNING, state)
				}
			} else if v := i.Mysql.Warning.Value(); v != "" {
				status.SetStatus(model.OK, v)
			}
			latencyCell := model.NewTableCell().SetUnit("ms")
			if last := avgLatency.Last(); last > 0 {
				latencyCell.SetValue(utils.FormatFloat(last * 1000))
			}
			table.AddRow(
				name,
				status,
				model.NewTableCell(utils.FormatFloat(totalQps.Last())).SetUnit("/s"),
				latencyCell,
				replStatusCell,
				lagCell,
				dbSizeCell(i.Mysql.DatabaseSize),
				model.NewTableCell(i.Mysql.Version.Value()))
		}
		if queriesByTotalTime != nil {
			totalTime := map[string]model.SeriesData{}
			for k, stat := range i.Mysql.PerQuery {
				q := k.String()
				totalTime[q] = stat.TotalTime
			}
			queriesByTotalTime.GetOrCreateChart(i.Name).Stacked().Sorted().AddMany(totalTime, 5, timeseries.Max)
		}
		if lockedQueriesChart != nil {
			locked := map[string]model.SeriesData{}
			for k, ts := range i.Mysql.LockedQueries {
				locked[k.String()] = ts
			}
			lockedQueriesChart.GetOrCreateChart(i.Name).Stacked().Sorted().AddMany(locked, 5, timeseries.NanSum)
		}
		if blockingQueriesChart != nil {
			blocking := map[string]model.SeriesData{}
			for k, ts := range i.Mysql.AwaitingQueriesByBlockingQuery {
				blocking[k.String()] = ts
			}
			blockingQueriesChart.GetOrCreateChart(i.Name).Stacked().Sorted().AddMany(blocking, 5, timeseries.NanSum).ShiftColors()
		}

		if longTxChart != nil {
			ages := map[string]model.SeriesData{}
			for q, ts := range i.Mysql.InnodbTransactions {
				ages[q] = ts
			}
			longTxChart.GetOrCreateChart(i.Name).Sorted().AddMany(ages, 5, timeseries.Max)
		}

		if tablesByIOTime != nil {
			totalTime := map[string]model.SeriesData{}
			readTime := map[string]model.SeriesData{}
			writeTime := map[string]model.SeriesData{}
			for k, stat := range i.Mysql.TablesIOTime {
				q := k.String()
				readTime[q] = stat.ReadTimePerSecond
				writeTime[q] = stat.WriteTimePerSecond
				totalTime[q] = timeseries.NewAggregate(timeseries.NanSum).
					Add(stat.ReadTimePerSecond, stat.WriteTimePerSecond)
			}
			tablesByIOTime.GetOrCreateChart("total: "+i.Name).Stacked().Sorted().AddMany(totalTime, 5, timeseries.Max)
			tablesByIOTime.GetOrCreateChart("write: "+i.Name).Stacked().Sorted().AddMany(writeTime, 5, timeseries.Max)
			tablesByIOTime.GetOrCreateChart("read: "+i.Name).Stacked().Sorted().AddMany(readTime, 5, timeseries.Max)
		}
		if dbSizeChart != nil {
			dbSize := map[string]model.SeriesData{}
			for db, ts := range i.Mysql.DatabaseSize {
				dbSize[db] = ts
			}
			dbSizeChart.GetOrCreateChart(i.Name).Stacked().Sorted().AddMany(dbSize, 20, timeseries.Max)
		}
		if tableSizeChart != nil {
			tableSize := map[string]model.SeriesData{}
			for k, ts := range i.Mysql.TableSize {
				tableSize[k.String()] = ts
			}
			tableSizeChart.GetOrCreateChart(i.Name).Stacked().Sorted().AddMany(tableSize, 20, timeseries.Max)
		}
		if trafficChart != nil {
			trafficChart.GetOrCreateChart("outbound").AddSeries(i.Name, i.Mysql.BytesSent).Feature()
			trafficChart.GetOrCreateChart("inbound").AddSeries(i.Name, i.Mysql.BytesReceived)
		}
	}

	grTotal, grOnline := 0, 0
	for _, i := range a.app.Instances {
		if i.Mysql == nil || i.Mysql.GroupReplication == nil || i.IsObsolete() {
			continue
		}
		grTotal++
		if strings.EqualFold(i.Mysql.GroupReplication.State.Value(), "online") {
			grOnline++
		}
	}
	if grTotal > 0 && grOnline < grTotal {
		groupReplicationCheck.AddDetail("%d of %d Group Replication members are ONLINE - the group has lost redundancy and is closer to losing quorum", grOnline, grTotal)
	}

	if b := a.app.Cluster.Backups; b != nil && a.app.Cluster.Manager == model.ClusterManagerPerconaXtraDB &&
		(len(b.Methods) > 0 || b.Schedule != "" || len(b.Runs) > 0) {
		backupCheck := report.CreateCheck(model.Checks.MysqlBackups)
		clusterBackups(report, b, a.app.Id.Name, a.w.Ctx.To, backupCheck)
	}
}

const (
	mysqlRecentPoints            = 5
	mysqlLongTxSeconds           = 60
	mysqlGaleraConflictThreshold = 0.1
	mysqlHistoryListThreshold    = 1_000_000 // unpurged undo records; DBAs alarm around 1M
)

func galeraConflictRate(g *model.MysqlGalera) float32 {
	var r float32
	if g.CertFailures != nil {
		r += g.CertFailures.LastNAvg(mysqlRecentPoints, 0)
	}
	if g.BfAborts != nil {
		r += g.BfAborts.LastNAvg(mysqlRecentPoints, 0)
	}
	return r
}

func mysqlLatencyCauses(i *model.Instance) []string {
	m := i.Mysql
	var causes []string

	var topKey model.MysqlQueryKey
	var topTime float32
	for k, s := range m.PerQuery {
		if t := s.TotalTime.LastNAvg(mysqlRecentPoints, 0); t > topTime {
			topTime, topKey = t, k
		}
	}
	if topTime > 0 {
		causes = append(causes, fmt.Sprintf("the most time-consuming query is `%s` (%s query-seconds/second)", topKey.Query, utils.FormatFloat(topTime)))
	}

	locked := timeseries.NewAggregate(timeseries.NanSum)
	for _, ts := range m.LockedQueries {
		locked.Add(ts)
	}
	if locked.Get().LastNAvg(mysqlRecentPoints, 0) >= 1 {
		causes = append(causes, "queries are blocked on InnoDB row locks held by another transaction")
	}

	var oldestTx float32
	for _, ts := range m.InnodbTransactions {
		if age := ts.LastNAvg(mysqlRecentPoints, 0); age > oldestTx {
			oldestTx = age
		}
	}
	if oldestTx > mysqlLongTxSeconds {
		causes = append(causes, fmt.Sprintf("a transaction has been open for %s, holding a read view and possibly row locks", utils.FormatDuration(timeseries.Duration(int64(oldestTx)), 2)))
	}

	if m.CreatedTmpDiskTables.LastNAvg(mysqlRecentPoints, 0) > 1 {
		causes = append(causes, "queries are spilling to on-disk temporary tables (large sorts or joins)")
	}

	if inn := m.Innodb; inn != nil {
		if inn.BufferPoolWaitFree.LastNAvg(mysqlRecentPoints, 0) > 0 {
			causes = append(causes, "the InnoDB buffer pool is full - writes are waiting for free pages (innodb_buffer_pool_size may be undersized)")
		} else if reqs := inn.BufferPoolReadRequests.LastNAvg(mysqlRecentPoints, 0); reqs > 0 {
			if reads := inn.BufferPoolReads.LastNAvg(mysqlRecentPoints, 0); reads > 10 {
				if hit := (1 - reads/reqs) * 100; hit < 99 {
					causes = append(causes, fmt.Sprintf("the InnoDB buffer pool hit rate is %s%% - queries are reading pages from disk (the working set may not fit in innodb_buffer_pool_size)", utils.FormatFloat(hit)))
				}
			}
		}
		if inn.LogWaits.LastNAvg(mysqlRecentPoints, 0) > 0 {
			causes = append(causes, "writes are stalling on the InnoDB redo log (innodb_log_buffer_size too small or the log disk is slow)")
		}
		if hl := inn.HistoryListLength.Last(); hl > mysqlHistoryListThreshold {
			causes = append(causes, fmt.Sprintf("InnoDB purge is lagging (history list length %s) - a long-running transaction is holding undo records, bloating undo and slowing reads", utils.FormatFloat(hl)))
		}
	}

	if m.TableLocksWaited.LastNAvg(mysqlRecentPoints, 0) > 1 {
		causes = append(causes, "queries are waiting on table locks (LOCK TABLES, MyISAM tables, or DDL metadata locks)")
	}

	if m.Galera != nil && m.Galera.FlowControlPaused.LastNAvg(mysqlRecentPoints, 0)*100 > 10 {
		causes = append(causes, "writes are being paused by Galera flow control")
	}

	if cores := mysqlInstanceCpuCores(i); cores > 0 {
		if running := m.ThreadsRunning.LastNAvg(mysqlRecentPoints, 0); running/cores > 1 {
			causes = append(causes, fmt.Sprintf("CPU-bound: ~%s queries running concurrently on %s CPU cores", utils.FormatFloat(running), utils.FormatFloat(cores)))
		}
	}

	if len(causes) == 0 {
		causes = append(causes, "elevated query latency; review the top queries and resource usage")
	}
	return causes
}

func mysqlInstanceCpuCores(i *model.Instance) float32 {
	var cores float32
	known := false
	for _, c := range i.Containers {
		if c.CpuLimit == nil {
			continue
		}
		if v := c.CpuLimit.Last(); !timeseries.IsNaN(v) && v > 0 {
			cores += v
			known = true
		}
	}
	if !known {
		return 0
	}
	return cores
}

func mysqlGalera(name string, g *model.MysqlGalera, galeraCheck *model.Check, netIssue string,
	flowControlChart *model.Chart, queuesChart *model.ChartGroup, clusterSizeChart, conflictsChart *model.Chart) {

	flagged := false
	if status := g.ClusterStatus.Value(); status != "" && status != "primary" {
		galeraCheck.AddItem("%s", name)
		flagged = true
		galeraCheck.AddDetail("%s: node is in a non-Primary Galera component (lost quorum) and refuses queries", name)
	}
	if g.Ready != nil && g.Ready.Last() == 0 {
		galeraCheck.AddItem("%s", name)
		flagged = true
		galeraCheck.AddDetail("%s: wsrep is not ready - the node cannot serve queries", name)
	}
	if strings.HasPrefix(strings.ToLower(g.LocalStateComment.Value()), "joining") {
		galeraCheck.AddItem("%s", name)
		flagged = true
		galeraCheck.AddDetail("%s: Galera node is joining (receiving a state transfer) and not yet serving queries", name)
	}
	if g.FlowControlPaused != nil {
		if paused := g.FlowControlPaused.LastNAvg(mysqlRecentPoints, 0); paused*100 > galeraCheck.Threshold {
			galeraCheck.AddItem("%s", name)
			flagged = true
			galeraCheck.AddDetail("%s: writes paused by flow control %s%% of the time - the cluster is throttled to let a lagging node catch up",
				name, utils.FormatFloat(paused*100))
			if g.LocalRecvQueue != nil {
				if q := g.LocalRecvQueue.LastNAvg(mysqlRecentPoints, 0); q > 1 {
					galeraCheck.AddDetail("%s: %s write-sets queued for apply - this node is falling behind on replication (check its CPU and disk)",
						name, utils.FormatFloat(q))
				}
			}
		}
	}
	if conflicts := galeraConflictRate(g); conflicts > mysqlGaleraConflictThreshold {
		galeraCheck.AddItem("%s", name)
		flagged = true
		galeraCheck.AddDetail("%s: %s/s transactions rolled back by certification conflicts - the same rows are being written on this node and another at once, route all writes through a single node so they serialize instead of racing across nodes",
			name, utils.FormatFloat(conflicts))
	}
	if flagged && netIssue != "" {
		galeraCheck.AddDetail("%s: %s", name, netIssue)
	}

	if flowControlChart != nil {
		flowControlChart.AddSeries(name, g.FlowControlPaused)
	}
	if queuesChart != nil {
		queuesChart.GetOrCreateChart(name).
			AddSeries("recv (apply)", g.LocalRecvQueue).
			AddSeries("send", g.LocalSendQueue)
	}
	if clusterSizeChart != nil {
		clusterSizeChart.AddSeries(name, g.ClusterSize)
	}
	if conflictsChart != nil {
		conflictsChart.AddSeries("cert failures: "+name, g.CertFailures)
		conflictsChart.AddSeries("bf aborts: "+name, g.BfAborts)
	}
}

func mysqlGroupReplication(name string, gr *model.MysqlGroupReplication, grCheck *model.Check, netIssue string,
	membersChart *model.Chart, queuesChart *model.ChartGroup, conflictsChart *model.Chart) {

	state := gr.State.Value()
	flagged := false
	switch state {
	case "error", "offline":
		grCheck.AddItem("%s", name)
		flagged = true
		grCheck.AddDetail("%s: Group Replication member state is %s - it has left the group and is not serving replicated writes",
			name, strings.ToUpper(state))
	case "recovering":
		grCheck.AddItem("%s", name)
		flagged = true
		grCheck.AddDetail("%s: Group Replication member is RECOVERING (applying the backlog after joining) and not yet serving consistent reads", name)
	case "unreachable":
		grCheck.AddItem("%s", name)
		flagged = true
		grCheck.AddDetail("%s: this member is UNREACHABLE from the group - a network partition, if it holds a minority it will be expelled", name)
	}

	if gr.TransactionsApplyQueue != nil {
		if q := gr.TransactionsApplyQueue.LastNAvg(mysqlRecentPoints, 0); q > float32(grCheck.Threshold) {
			grCheck.AddItem("%s", name)
			flagged = true
			grCheck.AddDetail("%s: %s transactions queued to apply - this member is falling behind the group, writes are throttled group-wide until it catches up",
				name, utils.FormatFloat(q))
		}
	}
	if gr.TransactionsInQueue != nil {
		if q := gr.TransactionsInQueue.LastNAvg(mysqlRecentPoints, 0); q > float32(grCheck.Threshold) {
			grCheck.AddItem("%s", name)
			flagged = true
			grCheck.AddDetail("%s: %s transactions queued for certification - write conflicts or a slow member are stalling the group", name, utils.FormatFloat(q))
		}
	}
	if flagged && netIssue != "" {
		grCheck.AddDetail("%s: %s", name, netIssue)
	}

	if membersChart != nil {
		membersChart.AddSeries(name+" online", gr.Online)
		membersChart.AddSeries(name+" total", gr.ClusterSize)
	}
	if queuesChart != nil {
		queuesChart.GetOrCreateChart(name).
			AddSeries("apply", gr.TransactionsApplyQueue).
			AddSeries("certify", gr.TransactionsInQueue)
	}
	if conflictsChart != nil {
		conflictsChart.AddSeries(name, gr.ConflictsDetected)
	}
}

func mysqlInnodb(report *model.AuditReport, name string, inn *model.MysqlInnodb) {
	const group = "InnoDB"
	chart := func(title string) *model.Chart {
		return report.GetOrCreateChart(title, nil).Group(group, 3)
	}
	perInstance := func(title string) *model.Chart {
		return report.GetOrCreateChartGroup(title, nil).Group(group, 3).GetOrCreateChart(name)
	}

	if !inn.BufferPoolReadRequests.IsEmpty() {
		chart("InnoDB buffer pool hit rate, %").AddSeries(name, timeseries.Aggregate2(inn.BufferPoolReads, inn.BufferPoolReadRequests, func(reads, req float32) float32 {
			if req <= 0 {
				return timeseries.NaN
			}
			r := (1 - reads/req) * 100
			if r < 0 {
				r = 0
			}
			return r
		}))
	}
	usedPages := timeseries.Sub(inn.BufferPoolPagesTotal, inn.BufferPoolPagesFree)
	perInstance("InnoDB buffer pool on <selector>, bytes").Stacked().
		AddSeries("used", timeseries.Mul(usedPages, inn.PageSize)).
		AddSeries("free", timeseries.Mul(inn.BufferPoolPagesFree, inn.PageSize))
	chart("InnoDB buffer pool dirty pages, %").AddSeries(name, timeseries.Aggregate2(inn.BufferPoolPagesDirty, inn.BufferPoolPagesTotal, func(dirty, total float32) float32 {
		if total <= 0 {
			return timeseries.NaN
		}
		return dirty / total * 100
	}))
	chart("InnoDB disk reads (buffer pool misses), per second").AddSeries(name, inn.BufferPoolReads)

	chart("InnoDB rows read, per second").AddSeries(name, inn.RowsRead)
	perInstance("InnoDB rows written on <selector>, per second").Stacked().
		AddSeries("inserted", inn.RowsInserted).
		AddSeries("updated", inn.RowsUpdated).
		AddSeries("deleted", inn.RowsDeleted)

	perInstance("Transactions on <selector>, per second").Stacked().
		AddSeries("commit", inn.Commits).
		AddSeries("rollback", inn.Rollbacks)

	chart("InnoDB row lock waits, per second").AddSeries(name, inn.RowLockWaits)
	chart("InnoDB row lock average wait, seconds").AddSeries(name, timeseries.Aggregate2(inn.RowLockTime, inn.RowLockWaits, func(t, w float32) float32 {
		if w <= 0 {
			return timeseries.NaN
		}
		return t / w
	}))
	chart("InnoDB rows locked, current").AddSeries(name, inn.RowLockCurrentWaits)

	perInstance("InnoDB disk I/O on <selector>, bytes per second").Stacked().
		AddSeries("read", inn.DataRead).
		AddSeries("written", inn.DataWritten)
	perInstance("InnoDB disk I/O operations on <selector>, per second").Stacked().
		AddSeries("reads", inn.DataReads).
		AddSeries("writes", inn.DataWrites).
		AddSeries("fsyncs", inn.DataFsyncs)

	chart("InnoDB redo log written, bytes per second").AddSeries(name, inn.OsLogWritten)
	chart("InnoDB redo log waits, per second").AddSeries(name, inn.LogWaits)
	chart("InnoDB sort merge passes, per second").AddSeries(name, inn.SortMergePasses)
}

func mysqlDiskUsageFindings(instance *model.Instance, capacity float32, ctx timeseries.Context, check *model.Check) {
	my := instance.Mysql
	if my == nil {
		return
	}
	growers := tableGrowers("table", my.TableSizeGrowth, my.TableSize)
	if rate, size := seriesGrowthRate(my.BinlogSize, ctx); rate >= diskFindingMinGrowthRateBytesPerSecond {
		name := "binary logs"
		if expire := my.BinlogExpireSeconds.Last(); expire == 0 && !timeseries.IsNaN(expire) {
			name = "binary logs (binlog_expire_logs_seconds=0, they are never purged automatically)"
		} else if files := my.BinlogFiles.Last(); files > 0 && !timeseries.IsNaN(files) {
			name = fmt.Sprintf("binary logs (%s files)", utils.FormatFloat(files))
		}
		growers = append(growers, diskGrower{name, rate, size})
	}
	if rate, size := seriesGrowthRate(my.UndoSize, ctx); rate >= diskFindingMinGrowthRateBytesPerSecond {
		name := "InnoDB undo tablespaces"
		if inn := my.Innodb; inn != nil {
			if hl := inn.HistoryListLength.Last(); hl > mysqlHistoryListThreshold {
				name = "InnoDB undo tablespaces (purge is lagging, a long-running transaction is holding undo records)"
			}
		}
		growers = append(growers, diskGrower{name, rate, size})
	}
	if reportGrowers(instance.Name, check, growers) == 0 {
		reportLargest(instance.Name, "table", capacity, my.TableSize, my.DatabaseSize, check)
	}
}
