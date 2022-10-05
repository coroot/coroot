package auditor

import "github.com/coroot/coroot/model"

func (a *appAuditor) slo() {
	report := a.addReport("SLO")

	availability := report.CreateCheck(model.Checks.SLOAvailability)
	availability.SetStatus(model.UNKNOWN, "not configured")
}
