package auditor

import (
	"github.com/coroot/coroot/model"
)

func (a *appAuditor) python() {
	if !a.app.IsPython() {
		return
	}

	report := a.addReport(model.AuditReportPython)
	gilCheck := report.CreateCheck(model.Checks.PythonGILWaitingTime)
	gilChart := report.GetOrCreateChart("GIL waiting time, seconds/second", nil)
	for _, i := range a.app.Instances {
		if i.Python == nil {
			continue
		}
		if i.Python.GILWaitTime.Last() > gilCheck.Threshold {
			gilCheck.AddItem(i.Name)
		}
		if gilChart != nil {
			gilChart.AddSeries(i.Name, i.Python.GILWaitTime)
		}
	}
}
