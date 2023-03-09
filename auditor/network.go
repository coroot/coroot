package auditor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

type netSummary struct {
	status   model.Status
	rttMin   *timeseries.Aggregate
	rttMax   *timeseries.Aggregate
	rttSum   *timeseries.Aggregate
	rttCount *timeseries.Aggregate
}

func newNetSummary() *netSummary {
	return &netSummary{
		rttMin:   timeseries.NewAggregate(timeseries.Min),
		rttMax:   timeseries.NewAggregate(timeseries.Max),
		rttSum:   timeseries.NewAggregate(timeseries.NanSum),
		rttCount: timeseries.NewAggregate(timeseries.NanSum),
	}
}

func (s *netSummary) addRtt(rtt *timeseries.TimeSeries) {
	s.rttMax.Add(rtt)
	s.rttMin.Add(rtt)
	s.rttSum.Add(rtt)
	s.rttCount.Add(rtt.Map(timeseries.Defined))
}

func (a *appAuditor) network() {
	report := a.addReport(model.AuditReportNetwork)
	upstreams := map[model.ApplicationId]*netSummary{}

	rttCheck := report.CreateCheck(model.Checks.NetworkRTT)
	seenConnections := false
	for _, instance := range a.app.Instances {
		for _, u := range instance.Upstreams {
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
				summary = newNetSummary()
				upstreams[upstreamApp.Id] = summary
			}
			linkStatus := u.Status()
			if linkStatus > summary.status {
				summary.status = linkStatus
			}
			if u.Rtt != nil {
				summary.addRtt(u.Rtt)
			}
			if instance.IsObsolete() || u.IsObsolete() {
				linkStatus = model.UNKNOWN
			}
			if instance.Node != nil && u.RemoteInstance.Node != nil {
				sn := instance.Node
				dn := u.RemoteInstance.Node
				report.GetOrCreateDependencyMap().UpdateLink(
					model.DependencyMapInstance{Id: instance.Name + "@" + instance.NodeName(), Name: instance.Name, Obsolete: instance.IsObsolete()},
					model.DependencyMapNode{Name: sn.Name.Value(), Provider: sn.CloudProvider.Value(), Region: sn.Region.Value(), AZ: sn.AvailabilityZone.Value()},
					model.DependencyMapInstance{Id: u.RemoteInstance.Name + "@" + u.RemoteInstance.NodeName(), Name: u.RemoteInstance.Name, Obsolete: u.IsObsolete()},
					model.DependencyMapNode{Name: dn.Name.Value(), Provider: dn.CloudProvider.Value(), Region: dn.Region.Value(), AZ: dn.AvailabilityZone.Value()},
					linkStatus,
				)
			}
		}
	}
	for appId, summary := range upstreams {
		avg := timeseries.Div(summary.rttSum.Get(), summary.rttCount.Get())
		if avg.Last() > rttCheck.Threshold {
			rttCheck.AddItem(appId.Name)
		}
		report.GetOrCreateChartInGroup("Network round-trip time to <selector>, seconds", appId.Name).
			AddSeries("min", summary.rttMin).
			AddSeries("avg", avg).
			AddSeries("max", summary.rttMax)
	}
	if !seenConnections {
		rttCheck.SetStatus(model.UNKNOWN, "no data")
	}
}
