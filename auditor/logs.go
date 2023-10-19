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
	byHash := map[string]*model.LogPattern{}
	for _, instance := range a.app.Instances {
		if len(instance.Containers) > 0 {
			seenContainers = true
		}
		for level, msgs := range instance.LogMessages {
			if !level.IsError() {
				continue
			}
			count := msgs.Messages.Reduce(timeseries.NanSum)
			if timeseries.IsNaN(count) || count == 0 {
				continue
			}
			sum.Add(msgs.Messages)
			for hash, pattern := range msgs.Patterns {
				if byHash[hash] != nil {
					continue
				}
				var found bool
				for _, p := range byHash {
					if p.Pattern.WeakEqual(pattern.Pattern) {
						found = true
						break
					}
				}
				byHash[hash] = pattern
				if !found {
					check.AddItem(hash)
				}
			}
		}
	}
	ts := sum.Get()
	check.Inc(int64(ts.Reduce(timeseries.NanSum)))
	check.SetValues(ts)
	if !seenContainers {
		check.SetStatus(model.UNKNOWN, "no data")
	}
}
