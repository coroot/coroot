package auditor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func (a *appAuditor) network() {
	if len(a.app.Upstreams) == 0 {
		return
	}
	report := a.addReport(model.AuditReportNetwork)

	rttCheckInCluster := report.CreateCheck(model.Checks.NetworkRTT)
	rttCheckExternal := report.CreateCheck(model.Checks.NetworkRTTExternal)
	rttCheckOtherClusters := report.CreateCheck(model.Checks.NetworkRTTOtherClusters)

	connectionsCheck := report.CreateCheck(model.Checks.NetworkTCPConnections)
	connectivityCheck := report.CreateCheck(model.Checks.NetworkConnectivity)

	rttInClusterChart := report.GetOrCreateChart("Network RTT (in-cluster), seconds", nil)
	rttExternalChart := report.GetOrCreateChart("Network RTT (external), seconds", nil)
	rttOtherClustersChart := report.GetOrCreateChart("Network RTT (cross-cluster), seconds", nil)
	connectionLatencyChart := report.GetOrCreateChart("TCP connection latency, seconds", nil)
	activeConnectionsChart := report.GetOrCreateChart("Active TCP connections", nil)
	connectionAttemptsChart := report.GetOrCreateChart("TCP connection attempts, per second", nil)
	failedConnectionsChart := report.GetOrCreateChart("Failed TCP connections, per second", nil)
	trafficChart := report.GetOrCreateChartGroup("Traffic <selector>, bytes/second", nil)
	retransmissionsChart := report.GetOrCreateChart("TCP retransmissions, segments/second", nil)

	rttCheckInCluster.AddWidget(rttInClusterChart.Widget())
	rttCheckExternal.AddWidget(rttExternalChart.Widget())
	rttCheckOtherClusters.AddWidget(rttOtherClustersChart.Widget())
	connectionsCheck.AddWidget(failedConnectionsChart.Widget())
	connectivityCheck.AddWidget(activeConnectionsChart.Widget())

	for _, u := range a.app.Upstreams {
		var rttCheck *model.Check
		var rttChart *model.Chart
		switch {
		case u.RemoteApplication.Id.Kind == model.ApplicationKindExternalService:
			rttCheck = rttCheckExternal
			rttChart = rttExternalChart
		case a.app.Id.ClusterId != u.RemoteApplication.Id.ClusterId:
			rttCheck = rttCheckOtherClusters
			rttChart = rttOtherClustersChart
		default:
			rttCheck = rttCheckInCluster
			rttChart = rttInClusterChart
		}
		if last := u.Rtt.Last(); !timeseries.IsNaN(last) {
			if last > rttCheck.Value() {
				rttCheck.SetValue(last)
			}
			if last > rttCheck.Threshold {
				rttCheck.AddItem(u.RemoteApplication.Id.String())
			}
		}
		if u.HasConnectivityIssues() {
			connectivityCheck.AddItem(u.RemoteApplication.Id.String())
		}
		if u.HasFailedConnectionAttempts() {
			connectionsCheck.AddItem(u.RemoteApplication.Id.String())
		}
		legend := "→" + u.RemoteApplication.Id.Name
		if rttChart != nil {
			rttChart.AddSeries(legend, u.Rtt)
		}
		if retransmissionsChart != nil {
			retransmissionsChart.AddSeries(legend, u.Retransmissions)
		}
		if failedConnectionsChart != nil {
			failedConnectionsChart.AddSeries(legend, u.FailedConnections)
		}
		if activeConnectionsChart != nil {
			activeConnectionsChart.AddSeries(legend, u.Active)
		}
		if connectionAttemptsChart != nil && connectionLatencyChart != nil {
			connectionLatencyChart.AddSeries(legend, timeseries.Div(u.ConnectionTime.Get(), u.SuccessfulConnections))
			connectionAttemptsChart.AddSeries(legend, u.SuccessfulConnections)
		}
		if trafficChart != nil {
			trafficChart.GetOrCreateChart("inbound").Stacked().AddSeries("←"+u.RemoteApplication.Id.Name, u.BytesReceived)
			trafficChart.GetOrCreateChart("outbound").Stacked().AddSeries("→"+u.RemoteApplication.Id.Name, u.BytesSent)
		}
	}
}
