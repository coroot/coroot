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

	rttCheck := report.CreateCheck(model.Checks.NetworkRTT)
	connectionsCheck := report.CreateCheck(model.Checks.NetworkTCPConnections)
	connectivityCheck := report.CreateCheck(model.Checks.NetworkConnectivity)

	rttChart := report.GetOrCreateChart("Network round-trip time, seconds", nil)
	connectionLatencyChart := report.GetOrCreateChart("TCP connection latency, seconds", nil)
	activeConnectionsChart := report.GetOrCreateChart("Active TCP connections", nil)
	connectionAttemptsChart := report.GetOrCreateChart("TCP connection attempts, per second", nil)
	failedConnectionsChart := report.GetOrCreateChart("Failed TCP connections, per second", nil)
	trafficChart := report.GetOrCreateChartGroup("Traffic <selector>, bytes/second", nil)
	retransmissionsChart := report.GetOrCreateChart("TCP retransmissions, segments/second", nil)
	for _, u := range a.app.Upstreams {
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
			trafficChart.GetOrCreateChart("outbound").Stacked().AddSeries("→"+legend, u.BytesSent)
		}
	}
}
