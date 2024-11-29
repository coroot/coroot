package auditor

import (
	"net"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"inet.af/netaddr"
)

func (a *appAuditor) network() {
	report := a.addReport(model.AuditReportNetwork)

	rttCheck := report.CreateCheck(model.Checks.NetworkRTT)
	connectionsCheck := report.CreateCheck(model.Checks.NetworkTCPConnections)
	connectivityCheck := report.CreateCheck(model.Checks.NetworkConnectivity)

	dependencyMap := report.GetOrCreateDependencyMap()
	rttChart := report.GetOrCreateChart("Network round-trip time, seconds", nil)
	connectionLatencyChart := report.GetOrCreateChart("TCP connection latency, seconds", nil)
	activeConnectionsChart := report.GetOrCreateChart("Active TCP connections", nil)
	connectionAttemptsChart := report.GetOrCreateChart("TCP connection attempts, per second", nil)
	failedConnectionsChart := report.GetOrCreateChart("Failed TCP connections, per second", nil)
	trafficChart := report.GetOrCreateChartGroup("Traffic <selector>, bytes/second", nil)
	retransmissionsChart := report.GetOrCreateChart("TCP retransmissions, segments/second", nil)

	seenConnections := false

	type connStats struct {
		failed          *timeseries.Aggregate
		active          *timeseries.Aggregate
		attempts        *timeseries.Aggregate
		totalTime       *timeseries.Aggregate
		retransmissions *timeseries.Aggregate
		bytesSent       *timeseries.Aggregate
		bytesReceived   *timeseries.Aggregate
		rtts            []*timeseries.TimeSeries
	}

	connectionByDest := map[string]*connStats{}

	for _, instance := range a.app.Instances {
		for _, u := range instance.Upstreams {
			dest := net.JoinHostPort(u.ServiceRemoteIP, u.ServiceRemotePort)
			if u.Service != nil {
				dest += " (" + u.Service.Name + ")"
			}
			if u.RemoteApplication != nil {
				if u.RemoteInstance != nil && u.RemoteApplication == a.app {
					dest = u.RemoteInstance.Name
				} else {
					dest = u.RemoteApplication.Id.Name
				}
			}
			stats := connectionByDest[dest]
			if stats == nil {
				stats = &connStats{
					failed:          timeseries.NewAggregate(timeseries.NanSum),
					active:          timeseries.NewAggregate(timeseries.NanSum),
					attempts:        timeseries.NewAggregate(timeseries.NanSum),
					totalTime:       timeseries.NewAggregate(timeseries.NanSum),
					retransmissions: timeseries.NewAggregate(timeseries.NanSum),
					bytesSent:       timeseries.NewAggregate(timeseries.NanSum),
					bytesReceived:   timeseries.NewAggregate(timeseries.NanSum),
				}
				connectionByDest[dest] = stats
			}
			stats.failed.Add(u.FailedConnections)
			stats.active.Add(u.Active)
			stats.attempts.Add(u.SuccessfulConnections, u.FailedConnections)
			stats.retransmissions.Add(u.Retransmissions)
			stats.bytesSent.Add(u.BytesSent)
			stats.bytesReceived.Add(u.BytesReceived)
			stats.totalTime.Add(u.ConnectionTime)
			if !u.Rtt.IsEmpty() {
				stats.rtts = append(stats.rtts, u.Rtt)
			}

			upstreamApp := u.RemoteApplication
			if upstreamApp == nil {
				continue
			}
			seenConnections = true
			if !u.Rtt.IsEmpty() {
				if last := u.Rtt.Last(); last > rttCheck.Value() {
					rttCheck.SetValue(last)
				}
			}
			if u.HasConnectivityIssues() {
				connectivityCheck.AddItem(upstreamApp.Id.String())
			}
			if u.HasFailedConnectionAttempts() {
				connectionsCheck.AddItem(upstreamApp.Id.String())
			}

			if dependencyMap != nil && instance.Node != nil && !u.IsEmpty() {
				linkStatus := model.UNKNOWN
				if !instance.IsObsolete() && !u.IsObsolete() {
					linkStatus, _ = u.Status()
					if linkStatus == model.OK {
						if u.Rtt.Last() > rttCheck.Threshold || u.FailedConnections.Last() > 0 {
							linkStatus = model.WARNING
						}
					}
				}
				dnName := "~unknown"
				var dnCloud, dnRegion, dnAz, dInstanceName, dInstanceId string

				if u.RemoteInstance != nil {
					dInstanceId = u.RemoteInstance.Name + "@" + u.RemoteInstance.NodeName()
					if u.RemoteInstance.Node != nil {
						dnName = u.RemoteInstance.Node.GetName()
						dnCloud = u.RemoteInstance.Node.CloudProvider.Value()
						dnRegion = u.RemoteInstance.Node.Region.Value()
						dnAz = u.RemoteInstance.Node.AvailabilityZone.Value()
					}
					dInstanceName = u.RemoteInstance.Name
					if u.RemoteInstance.Owner.Id.Kind == model.ApplicationKindExternalService {
						if u.Service != nil {
							dInstanceName += " (" + u.Service.Name + ")"
						} else {
							h, _, _ := net.SplitHostPort(u.RemoteInstance.Owner.Id.Name)
							if _, err := netaddr.ParseIP(h); h != "" && err != nil {
								dnName = h
							}
						}

					}
				} else {
					dInstanceName = u.RemoteApplication.Id.Name + " (service)"
					dInstanceId = dInstanceName
				}
				sn := instance.Node
				dependencyMap.UpdateLink(
					model.DependencyMapInstance{Id: instance.Name + "@" + instance.NodeName(), Name: instance.Name, Obsolete: instance.IsObsolete()},
					model.DependencyMapNode{Name: sn.GetName(), Provider: sn.CloudProvider.Value(), Region: sn.Region.Value(), AZ: sn.AvailabilityZone.Value()},
					model.DependencyMapInstance{Id: dInstanceId, Name: dInstanceName, Obsolete: u.IsObsolete()},
					model.DependencyMapNode{Name: dnName, Provider: dnCloud, Region: dnRegion, AZ: dnAz},
					linkStatus,
				)
			}
		}
	}

	for dest, stats := range connectionByDest {
		var sum, count float32
		for _, rtt := range stats.rtts {
			last := rtt.Last()
			if !timeseries.IsNaN(last) {
				sum += last
				count += 1
			}
		}
		if sum > 0 && count > 0 && sum/count > rttCheck.Threshold {
			rttCheck.AddItem(dest)
		}
		if rttChart != nil {
			sum := timeseries.NewAggregate(timeseries.NanSum).Add(stats.rtts...).Get()
			count := timeseries.NewAggregate(timeseries.NanSum)
			for _, rtt := range stats.rtts {
				count.Add(rtt.Map(timeseries.Defined))
			}
			rttChart.AddSeries("→"+dest, timeseries.Div(sum, count.Get()))
		}
		if retransmissionsChart != nil {
			retransmissionsChart.AddSeries("→"+dest, stats.retransmissions)
		}
		if failedConnectionsChart != nil {
			failedConnectionsChart.AddSeries("→"+dest, stats.failed)
		}
		if activeConnectionsChart != nil {
			activeConnectionsChart.AddSeries("→"+dest, stats.active)
		}
		if connectionAttemptsChart != nil && connectionLatencyChart != nil {
			attempts := stats.attempts.Get()
			connectionLatencyChart.AddSeries("→"+dest, timeseries.Div(stats.totalTime.Get(), attempts))
			connectionAttemptsChart.AddSeries("→"+dest, attempts)
		}
		if trafficChart != nil {
			trafficChart.GetOrCreateChart("inbound").Stacked().AddSeries("←"+dest, stats.bytesReceived)
			trafficChart.GetOrCreateChart("outbound").Stacked().AddSeries("→"+dest, stats.bytesSent)
		}
	}
	if !seenConnections {
		a.delReport(model.AuditReportNetwork)
	}
}
