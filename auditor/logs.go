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
	sum := timeseries.NewAggregate(timeseries.NanSum)
	uniqPatterns := map[string]*model.LogPattern{}
	equalTo := map[string]string{}
	uniqErrors := map[string]bool{}
	for _, instance := range a.app.Instances {
		if len(instance.Containers) > 0 {
			seenContainers = true
		}
		for level, msgs := range instance.LogMessages {
			if !level.IsError() {
				continue
			}
			sum.Add(msgs.Messages)
			for hash, pattern := range msgs.Patterns {
				equal := equalTo[hash]
				if equal == "" {
					for h, p := range uniqPatterns {
						if p.Pattern.WeakEqual(pattern.Pattern) {
							equal = h
							break
						}
					}
					if equal == "" {
						equal = hash
						uniqPatterns[hash] = pattern
					}
					equalTo[hash] = equal
				}
				if !uniqErrors[equal] && pattern.Messages.Reduce(timeseries.NanSum) > 0 {
					uniqErrors[equal] = true
				}
			}
		}
	}
	ts := sum.Get()
	check.Inc(int64(ts.Reduce(timeseries.NanSum)))
	check.SetValues(ts)
	for h := range uniqErrors {
		check.AddItem(h)
	}
	if !seenContainers {
		check.SetStatus(model.UNKNOWN, "no data")
	}
}
