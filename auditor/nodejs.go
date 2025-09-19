package auditor

import (
	"github.com/coroot/coroot/model"
)

func (a *appAuditor) nodejs() {
	if !a.app.IsNodejs() {
		return
	}

	report := a.addReport(model.AuditReportNodejs)
	check := report.CreateCheck(model.Checks.NodejsEventLoopBlockedTime)
	chart := report.GetOrCreateChart("Node.js event loop blocked time, seconds/second", nil)
	for _, i := range a.app.Instances {
		if i.Nodejs == nil {
			continue
		}
		if i.Nodejs.EventLoopBlockedTime.Last() > check.Threshold {
			check.AddItem(i.Name)
		}
		if chart != nil {
			chart.AddSeries(i.Name, i.Nodejs.EventLoopBlockedTime)
		}
	}
}
