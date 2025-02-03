package auditor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

func (a *appAuditor) deployments() {
	if len(a.app.Deployments) == 0 {
		return
	}

	report := a.addReport(model.AuditReportDeployments)

	statusCheck := report.CreateCheck(model.Checks.DeploymentStatus)

	table := report.GetOrCreateTable("Deployment", "Deployed", "Summary").SetSorted()

	now := timeseries.Now()
	statuses := model.CalcApplicationDeploymentStatuses(a.app, a.w.CheckConfigs, now)
	for i := len(statuses) - 1; i >= 0; i-- {
		ds := statuses[i]
		version := model.NewTableCell().SetStatus(ds.Status, ds.Deployment.Version()).AddTag("age: %s", utils.FormatDuration(ds.Lifetime, 1))
		from, to := ds.Deployment.StartedAt.Add(-30*timeseries.Minute), ds.Deployment.StartedAt.Add(30*timeseries.Minute)
		version.Link = model.NewRouterLink(ds.Deployment.Version(), "overview").
			SetParam("view", "applications").
			SetParam("report", model.AuditReportInstances).
			SetArg("from", from).SetArg("to", to).
			SetParam("id", a.app.Id)
		deployed := model.NewTableCell(utils.FormatDuration(now.Sub(ds.Deployment.StartedAt), 1) + " ago")

		summary := model.NewTableCell()
		switch ds.State {
		case model.ApplicationDeploymentStateSummary:
			if len(ds.Summary) > 0 {
				summary.DeploymentSummaries = ds.Summary
			} else {
				summary.SetStub("No notable changes")
			}
		case model.ApplicationDeploymentStateDeployed:
			version.UpdateStatus(model.UNKNOWN)
			if i == len(statuses)-1 {
				summary.SetStub("Collecting data...")
			} else {
				summary.SetStub("Not enough data due to the lifetime < %s", utils.FormatDuration(model.ApplicationDeploymentMinLifetime, 1))
			}
		case model.ApplicationDeploymentStateStuck:
			statusCheck.SetValue(float32(now.Sub(ds.Deployment.StartedAt)))
			summary.DeploymentSummaries = append(summary.DeploymentSummaries, model.ApplicationDeploymentSummary{
				Report:  model.AuditReportInstances,
				Ok:      false,
				Message: ds.Message,
				Time:    ds.Deployment.StartedAt,
			})
		case model.ApplicationDeploymentStateInProgress, model.ApplicationDeploymentStateCancelled:
			summary.SetStub(ds.Message)
		}

		table.AddRow(version, deployed, summary).SetId(ds.Deployment.Id())
	}
}
