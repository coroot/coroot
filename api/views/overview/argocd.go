package overview

import (
	"sort"

	"github.com/coroot/coroot/model"
)

type ArgoManagedResource struct {
	ID          model.ApplicationId `json:"id"`
	CorootAppId model.ApplicationId `json:"coroot_app_id"`
	Issue       string              `json:"issue,omitempty"`
}

type ArgoCDResource struct {
	ID                  model.ApplicationId   `json:"id"`
	Cluster             string                `json:"cluster"`
	Type                string                `json:"type"`
	Name                string                `json:"name"`
	Namespace           string                `json:"namespace"`
	Project             string                `json:"project,omitempty"`
	SourceType          string                `json:"source_type,omitempty"`
	SyncStatus          string                `json:"sync_status,omitempty"`
	HealthStatus        string                `json:"health_status,omitempty"`
	OperationPhase      string                `json:"operation_phase,omitempty"`
	OperationFinishedAt int64                 `json:"operation_finished_at,omitempty"`
	SyncLevel           model.Status          `json:"sync_level"`
	HealthLevel         model.Status          `json:"health_level"`
	OperationLevel      model.Status          `json:"operation_level"`
	Repo                string                `json:"repo,omitempty"`
	Path                string                `json:"path,omitempty"`
	Chart               string                `json:"chart,omitempty"`
	Resources           []ArgoManagedResource `json:"resources,omitempty"`
}

var (
	argoSyncGood       = map[string]bool{"Synced": true}
	argoHealthGood     = map[string]bool{"Healthy": true, "Progressing": true, "Suspended": true}
	argoOpGood         = map[string]bool{"Succeeded": true, "Running": true, "Terminating": true}
	argoSyncResultGood = map[string]bool{"Synced": true, "Pruned": true}
)

func argoStatus(value string, good map[string]bool) model.Status {
	switch {
	case value == "" || value == "Unknown":
		return model.UNKNOWN
	case good[value]:
		return model.OK
	default:
		return model.WARNING
	}
}

func argoAppHasIssue(a *model.ArgoApplication) bool {
	return argoStatus(a.SyncStatus, argoSyncGood) == model.WARNING ||
		argoStatus(a.HealthStatus, argoHealthGood) == model.WARNING ||
		argoStatus(a.OperationPhase, argoOpGood) == model.WARNING
}

func argoResourceIssue(r *model.ArgoManagedResource) string {
	if argoStatus(r.SyncResult, argoSyncResultGood) == model.WARNING {
		return r.SyncResult
	}
	if argoStatus(r.SyncStatus, argoSyncGood) == model.WARNING {
		return r.SyncStatus
	}
	if argoStatus(r.HealthStatus, argoHealthGood) == model.WARNING {
		return r.HealthStatus
	}
	return ""
}

func CountArgoCDIssues(w *model.World) (issues int) {
	if w == nil || w.ArgoCD == nil {
		return
	}
	for _, a := range w.ArgoCD.Applications {
		if argoAppHasIssue(a) {
			issues++
		}
	}
	return
}

func renderArgoCD(w *model.World) []*ArgoCDResource {
	if w == nil || w.ArgoCD == nil {
		return []*ArgoCDResource{}
	}
	argo := w.ArgoCD
	var resources []*ArgoCDResource

	for id, app := range argo.Applications {
		resources = append(resources, &ArgoCDResource{
			ID:                  id,
			Cluster:             w.ClusterName(id.ClusterId),
			Type:                string(id.Kind),
			Name:                id.Name,
			Namespace:           id.Namespace,
			Project:             app.Project.Value(),
			SourceType:          app.SourceType.Value(),
			SyncStatus:          argoStatusOrUnknown(app.SyncStatus),
			HealthStatus:        argoStatusOrUnknown(app.HealthStatus),
			OperationPhase:      app.OperationPhase,
			OperationFinishedAt: int64(app.OperationFinishedAt),
			SyncLevel:           argoStatus(app.SyncStatus, argoSyncGood),
			HealthLevel:         argoStatus(app.HealthStatus, argoHealthGood),
			OperationLevel:      argoStatus(app.OperationPhase, argoOpGood),
			Repo:                app.RepoURL.Value(),
			Path:                app.Path.Value(),
			Chart:               app.Chart.Value(),
			Resources:           getArgoManagedResources(w, app.Resources),
		})
	}
	return resources
}

func getArgoManagedResources(w *model.World, src map[model.ApplicationId]*model.ArgoManagedResource) []ArgoManagedResource {
	entries := make([]ArgoManagedResource, 0, len(src))
	for id, r := range src {
		e := ArgoManagedResource{
			ID:    id,
			Issue: argoResourceIssue(r),
		}
		if app := w.GetApplication(id); app != nil {
			e.CorootAppId = app.Id
		}
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		if (entries[i].Issue != "") != (entries[j].Issue != "") {
			return entries[i].Issue != ""
		}
		return entries[i].ID.String() < entries[j].ID.String()
	})
	return entries
}

func argoStatusOrUnknown(s string) string {
	if s == "" {
		return "Unknown"
	}
	return s
}
