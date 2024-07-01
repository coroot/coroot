package auditor

import (
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
	replicationStatusCheck := report.CreateCheck(model.Checks.MysqlReplicationStatus)
	replicationLagCheck := report.CreateCheck(model.Checks.MysqlReplicationLag)
	connectionsCheck := report.CreateCheck(model.Checks.MysqlConnections)

	table := report.GetOrCreateTable("Instance", "Status", "Queries", "Latency", "Replication status", "Replication lag", "Version")
	qpsChart := report.GetOrCreateChartGroup("Queries <selector>, per second", nil)
	latencyChart := report.GetOrCreateChart("Average latency, seconds", nil)
	queriesByTotalTime := report.GetOrCreateChartGroup("Queries by total time <selector>, query seconds/second", nil)
	tablesByIOTime := report.GetOrCreateChartGroup("I/O time by table <selector>, IO seconds/second", nil)

	trafficChart := report.GetOrCreateChartGroup("Traffic <selector>, bytes per second", nil)
	slowQueriesChart := report.GetOrCreateChart("Slow queries, per second", nil)
	connectionsChart := report.GetOrCreateChartGroup("Connections <selector>", nil)
	newConnectionsChart := report.GetOrCreateChart("New connections, per second", nil)
	replicationLagChart := report.GetOrCreateChart("Replication lag, seconds", nil)

	connectionsByInstance := map[string][]*model.Connection{}
	if table != nil {
		for _, d := range a.app.Downstreams {
			if d.RemoteInstance != nil {
				connectionsByInstance[d.RemoteInstance.Name] = append(connectionsByInstance[d.RemoteInstance.Name], d)
			}
		}
	}

	for _, i := range a.app.Instances {
		if i.Mysql == nil {
			continue
		}
		obsolete := i.IsObsolete()
		if !obsolete && !i.Mysql.IsUp() {
			availabilityCheck.AddItem(i.Name)
		}

		if obsolete {
			continue
		}
		lagCell := model.NewTableCell()

		if !i.Mysql.ReplicationLagSeconds.IsEmpty() {
			if lagTime := i.Mysql.ReplicationLagSeconds.Last(); !timeseries.IsNaN(lagTime) {
				lagCell.SetValue(utils.FormatFloat(lagTime)).SetUnit("s")
				if timeseries.Duration(lagTime) > timeseries.Duration(replicationLagCheck.Threshold) {
					replicationLagCheck.AddItem(i.Name)
				}
			}
			if replicationLagChart != nil {
				replicationLagChart.AddSeries(i.Name, i.Mysql.ReplicationLagSeconds)
			}
		}
		currConns := i.Mysql.ConnectionsCurrent.Last()
		maxConns := i.Mysql.ConnectionsMax.Last()
		if !timeseries.IsNaN(currConns) && !timeseries.IsNaN(maxConns) {
			if currConns/maxConns*100 > connectionsCheck.Threshold {
				connectionsCheck.AddItem(i.Name)
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
				replicationStatusCheck.AddItem(i.Name)
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
				replicationStatusCheck.AddItem(i.Name)
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

		if table != nil {
			name := model.NewTableCell(i.Name)
			status := model.NewTableCell().SetStatus(model.OK, "up")
			if !i.Mysql.IsUp() {
				if v := i.Mysql.Error.Value(); v != "" {
					status.SetStatus(model.WARNING, v)
				} else {
					status.SetStatus(model.WARNING, "down (no metrics)")
				}
			} else {
				if v := i.Mysql.Warning.Value(); v != "" {
					status.SetStatus(model.OK, v)
				}
			}
			protocolFilter := func(protocol model.Protocol) bool {
				return protocol == model.ProtocolMysql
			}
			qps := model.GetConnectionsRequestsSum(connectionsByInstance[i.Name], protocolFilter)
			if qpsChart != nil {
				qpsChart.GetOrCreateChart("eBPF").Feature().AddSeries(i.Name, qps)
				qpsChart.GetOrCreateChart("Mysql status").AddSeries(i.Name, i.Mysql.Queries)
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
				model.NewTableCell(utils.FormatFloat(qps.Last())).SetUnit("/s"),
				latencyCell,
				replStatusCell,
				lagCell,
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
		if trafficChart != nil {
			trafficChart.GetOrCreateChart("outbound").AddSeries(i.Name, i.Mysql.BytesSent).Feature()
			trafficChart.GetOrCreateChart("inbound").AddSeries(i.Name, i.Mysql.BytesReceived)
		}
	}
}
