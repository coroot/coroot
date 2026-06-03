package constructor

import (
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func loadArgoCDResources(w *model.World, metrics map[string][]*model.MetricValues, project *db.Project) {
	argo := model.NewArgoCD()

	loadArgoApplications(argo, metrics, project)
	loadArgoApplicationResources(argo, metrics, project)

	if len(argo.Applications) > 0 {
		w.ArgoCD = argo
	}
}

func loadArgoApplications(argo *model.ArgoCD, metrics map[string][]*model.MetricValues, project *db.Project) {
	for _, m := range metrics["argocd_application_info"] {
		id := argoAppId(project, m)
		app, ok := argo.Applications[id]
		if !ok {
			app = &model.ArgoApplication{Resources: map[model.ApplicationId]*model.ArgoManagedResource{}}
			argo.Applications[id] = app
		}
		app.Project.Update(m.Values, m.Labels["project"])
		app.SourceType.Update(m.Values, m.Labels["source_type"])
		app.RepoURL.Update(m.Values, m.Labels["repo"])
		app.Path.Update(m.Values, m.Labels["path"])
		app.Chart.Update(m.Values, m.Labels["chart"])
		app.DestServer.Update(m.Values, m.Labels["dest_server"])
		app.DestName.Update(m.Values, m.Labels["dest_name"])
		app.DestNamespace.Update(m.Values, m.Labels["dest_namespace"])
	}

	for _, m := range metrics["argocd_application_sync_status"] {
		if app := argoActiveApp(argo, project, m); app != nil {
			app.SyncStatus = m.Labels["sync_status"]
		}
	}
	for _, m := range metrics["argocd_application_health_status"] {
		if app := argoActiveApp(argo, project, m); app != nil {
			app.HealthStatus = m.Labels["health_status"]
		}
	}
	for _, m := range metrics["argocd_application_operation_status"] {
		if app := argoActiveApp(argo, project, m); app != nil {
			app.OperationPhase = m.Labels["operation_phase"]
		}
	}
	for _, m := range metrics["argocd_application_operation_finished_timestamp_seconds"] {
		app := argo.Applications[argoAppId(project, m)]
		if app == nil {
			continue
		}
		if _, v := m.Values.LastNotNull(); !timeseries.IsNaN(v) && v > 0 {
			app.OperationFinishedAt = timeseries.Time(int64(v))
		}
	}
}

func loadArgoApplicationResources(argo *model.ArgoCD, metrics map[string][]*model.MetricValues, project *db.Project) {
	for _, m := range metrics["argocd_application_resource_info"] {
		argoResource(argo, project, m)
	}
	for _, m := range metrics["argocd_application_resource_sync_status"] {
		if r := argoActiveResource(argo, project, m); r != nil {
			r.SyncStatus = m.Labels["sync_status"]
		}
	}
	for _, m := range metrics["argocd_application_resource_health_status"] {
		if r := argoActiveResource(argo, project, m); r != nil {
			r.HealthStatus = m.Labels["health_status"]
		}
	}
	for _, m := range metrics["argocd_application_resource_status"] {
		if r := argoActiveResource(argo, project, m); r != nil {
			r.SyncResult = m.Labels["status"]
		}
	}
}

func argoAppId(project *db.Project, m *model.MetricValues) model.ApplicationId {
	return model.NewApplicationId(project.ClusterId(), m.Labels["namespace"], model.ApplicationKindArgoApplication, m.Labels["name"])
}

func argoActiveApp(argo *model.ArgoCD, project *db.Project, m *model.MetricValues) *model.ArgoApplication {
	if m.Values.Last() != 1 {
		return nil
	}
	return argo.Applications[argoAppId(project, m)]
}

func argoResource(argo *model.ArgoCD, project *db.Project, m *model.MetricValues) *model.ArgoManagedResource {
	app := argo.Applications[argoAppId(project, m)]
	if app == nil {
		return nil
	}
	rid := model.NewApplicationId(project.ClusterId(), m.Labels["resource_namespace"], model.ApplicationKind(m.Labels["resource_kind"]), m.Labels["resource_name"])
	if rid.Kind == "" || rid.Name == "" {
		return nil
	}
	r := app.Resources[rid]
	if r == nil {
		r = &model.ArgoManagedResource{}
		app.Resources[rid] = r
	}
	return r
}

func argoActiveResource(argo *model.ArgoCD, project *db.Project, m *model.MetricValues) *model.ArgoManagedResource {
	if m.Values.Last() != 1 {
		return nil
	}
	return argoResource(argo, project, m)
}
