package auditor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func (a *appAuditor) logs() {
	report := a.addReport(model.AuditReportLogs)
	report.Custom = true
	check := report.CreateCheck(model.Checks.LogErrors)
	report.AddWidget(&model.Widget{Logs: &model.Logs{ApplicationId: a.app.Id, Check: check}, Width: "100%"})

	seenContainers := false
	for _, instance := range a.app.Instances {
		if len(instance.Containers) > 0 {
			seenContainers = true
		}
		for level, msgs := range instance.LogMessages {
			if level.IsError() {
				check.Inc(int64(msgs.Messages.Reduce(timeseries.NanSum)))
			}
		}
	}
	if !seenContainers {
		check.SetStatus(model.UNKNOWN, "no data")
	}
}
