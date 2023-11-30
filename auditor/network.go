package auditor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"net"
)

type netSummary struct {
	retransmissions []*timeseries.TimeSeries
	rtts            []*timeseries.TimeSeries
}

func (a *appAuditor) network() {
	report := a.addReport(model.AuditReportNetwork)

	rttCheck := report.CreateCheck(model.Checks.NetworkRTT)

	dependencyMap := report.GetOrCreateDependencyMap()
	failedConnectionsChart := report.GetOrCreateChart("Failed TCP connections, per second")
	rttChart := report.GetOrCreateChart("Network round-trip time, seconds")
	retransmissionsChart := report.GetOrCreateChart("TCP retransmissions, segments/second")

	seenConnections := false
	upstreams := map[model.ApplicationId]*netSummary{}

	failedConnectionByDest := map[string]*timeseries.Aggregate{}

	for _, instance := range a.app.Instances {
		for _, u := range instance.Upstreams {
			if failedConnectionsChart != nil && u.FailedConnections != nil {
				dest := net.JoinHostPort(u.ServiceRemoteIP, u.ServiceRemotePort)
				if u.Service != nil {
					dest += " (" + u.Service.Name + ")"
				}
				if u.RemoteInstance != nil {
					if u.RemoteInstance.OwnerId == a.app.Id {
						dest = u.RemoteInstance.Name
					} else {
						dest = u.RemoteInstance.OwnerId.Name
					}
				}
				v := failedConnectionByDest[dest]
				if v == nil {
					v = timeseries.NewAggregate(timeseries.NanSum)
					failedConnectionByDest[dest] = v
				}
				v.Add(u.FailedConnections)
			}

			if u.RemoteInstance == nil {
				continue
			}
			upstreamApp := a.w.GetApplication(u.RemoteInstance.OwnerId)
			if upstreamApp == nil {
				continue
			}
			seenConnections = true
			summary := upstreams[upstreamApp.Id]
			if summary == nil {
				summary = &netSummary{}
				upstreams[upstreamApp.Id] = summary
			}
			if u.Rtt != nil {
				if last := u.Rtt.Last(); last > rttCheck.Value() {
					rttCheck.SetValue(last)
				}
				summary.rtts = append(summary.rtts, u.Rtt)
			}
			if !u.Retransmissions.IsEmpty() {
				summary.retransmissions = append(summary.retransmissions, u.Retransmissions)
			}

			if instance.Node != nil && u.RemoteInstance.Node != nil && dependencyMap != nil {
				linkStatus := model.UNKNOWN
				if !instance.IsObsolete() && !u.IsObsolete() {
					linkStatus = u.Status()
					if linkStatus == model.OK && !u.Rtt.IsEmpty() && u.Rtt.Last() > rttCheck.Threshold {
						linkStatus = model.WARNING
					}
				}
				sn := instance.Node
				dn := u.RemoteInstance.Node
				dependencyMap.UpdateLink(
					model.DependencyMapInstance{Id: instance.Name + "@" + instance.NodeName(), Name: instance.Name, Obsolete: instance.IsObsolete()},
					model.DependencyMapNode{Name: sn.GetName(), Provider: sn.CloudProvider.Value(), Region: sn.Region.Value(), AZ: sn.AvailabilityZone.Value()},
					model.DependencyMapInstance{Id: u.RemoteInstance.Name + "@" + u.RemoteInstance.NodeName(), Name: u.RemoteInstance.Name, Obsolete: u.IsObsolete()},
					model.DependencyMapNode{Name: dn.GetName(), Provider: dn.CloudProvider.Value(), Region: dn.Region.Value(), AZ: dn.AvailabilityZone.Value()},
					linkStatus,
				)
			}
		}
	}
	if failedConnectionsChart != nil {
		for dest, v := range failedConnectionByDest {
			failedConnectionsChart.AddSeries("→"+dest, v)
		}
	}

	for appId, summary := range upstreams {
		var sum, count float32
		for _, rtt := range summary.rtts {
			last := rtt.Last()
			if !timeseries.IsNaN(last) {
				sum += last
				count += 1
			}
		}
		if sum > 0 && count > 0 && sum/count > rttCheck.Threshold {
			rttCheck.AddItem(appId.Name)
		}
		if rttChart != nil {
			sum := timeseries.NewAggregate(timeseries.NanSum).Add(summary.rtts...).Get()
			count := sum.Map(timeseries.Defined)
			avg := timeseries.Div(sum, count)
			rttChart.AddSeries("→"+appId.Name, avg)
		}
		if retransmissionsChart != nil {
			sum := timeseries.NewAggregate(timeseries.NanSum).Add(summary.retransmissions...).Get()
			retransmissionsChart.AddSeries("→"+appId.Name, sum)
		}
	}
	if !seenConnections {
		rttCheck.SetStatus(model.UNKNOWN, "no data")
	}
}
