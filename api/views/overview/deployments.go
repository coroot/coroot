package overview

import (
	"fmt"
	"sort"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

type Deployment struct {
	Application Application             `json:"application"`
	Version     string                  `json:"version"`
	Deployed    string                  `json:"deployed"`
	Status      model.Status            `json:"status"`
	Link        *model.RouterLink       `json:"link"`
	Age         string                  `json:"age"`
	Summary     []DeploymentSummaryItem `json:"summary"`

	startedAt timeseries.Time
}

type DeploymentSummaryItem struct {
	Status  string            `json:"status"`
	Message string            `json:"message"`
	Link    *model.RouterLink `json:"link"`
}

func renderDeployments(w *model.World) []*Deployment {
	now := timeseries.Now()
	var deployments []*Deployment
	for _, app := range w.Applications {
		statuses := model.CalcApplicationDeploymentStatuses(app, w.CheckConfigs, now)
		for _, ds := range statuses {
			from, to := ds.Deployment.StartedAt.Add(-30*timeseries.Minute), ds.Deployment.StartedAt.Add(30*timeseries.Minute)
			link := func() *model.RouterLink {
				return model.NewRouterLink("", "overview").
					SetParam("view", "applications").
					SetParam("id", app.Id).
					SetArg("from", from).
					SetArg("to", to)
			}
			d := &Deployment{
				Application: Application{Id: app.Id, Category: app.Category},
				Version:     ds.Deployment.Version(),
				startedAt:   ds.Deployment.StartedAt,
				Deployed:    utils.FormatDuration(now.Sub(ds.Deployment.StartedAt), 1) + " ago",
				Status:      ds.Status,
				Link:        link().SetParam("report", model.AuditReportInstances),
				Age:         utils.FormatDuration(ds.Lifetime, 1),
			}
			deployments = append(deployments, d)
			switch ds.State {
			case model.ApplicationDeploymentStateSummary:
				if len(ds.Summary) > 0 {
					for _, s := range ds.Summary {
						d.Summary = append(d.Summary, DeploymentSummaryItem{
							Status:  s.Emoji(),
							Message: s.Message,
							Link:    link().SetParam("report", s.Report),
						})
					}
				} else {
					d.Summary = append(d.Summary, DeploymentSummaryItem{Message: "No notable changes"})
				}
			case model.ApplicationDeploymentStateDeployed:
				if ds.Last {
					d.Summary = append(d.Summary, DeploymentSummaryItem{Message: "Collecting data..."})
				} else {
					msg := fmt.Sprintf("Not enough data due to the lifetime < %s", utils.FormatDuration(model.ApplicationDeploymentMinLifetime, 1))
					d.Summary = append(d.Summary, DeploymentSummaryItem{Message: msg})
				}
			case model.ApplicationDeploymentStateStuck:
				status := model.ApplicationDeploymentSummary{Ok: false}.Emoji()
				d.Summary = append(d.Summary, DeploymentSummaryItem{Status: status, Message: ds.Message})
			case model.ApplicationDeploymentStateInProgress, model.ApplicationDeploymentStateCancelled:
				d.Summary = append(d.Summary, DeploymentSummaryItem{Message: ds.Message})
			}
		}
	}

	sort.Slice(deployments, func(i, j int) bool {
		return deployments[j].startedAt < deployments[i].startedAt
	})

	return deployments
}
