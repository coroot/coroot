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

	sum := timeseries.NewAggregate(timeseries.NanSum)
	for level, msgs := range a.app.LogMessages {
		if !level.IsError() {
			continue
		}
		sum.Add(msgs.Messages)
		for hash, pattern := range msgs.Patterns {
			if pattern.Messages.Reduce(timeseries.NanSum) > 0 {
				check.AddItem(hash)
			}
		}
	}
	ts := sum.Get()
	check.Inc(int64(ts.Reduce(timeseries.NanSum)))
	check.SetValues(ts)

	seenContainers := false
	for _, instance := range a.app.Instances {
		if len(instance.Containers) > 0 {
			seenContainers = true
			break
		}
	}
	if !seenContainers {
		check.SetStatus(model.UNKNOWN, "no data")
	}
}
