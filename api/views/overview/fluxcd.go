package overview

import (
	"sort"

	"github.com/coroot/coroot/model"
	"golang.org/x/exp/maps"
)

type InventoryEntry struct {
	ID          model.ApplicationId `json:"id"`
	CorootAppId model.ApplicationId `json:"coroot_app_id"`
}

type FluxCDResource struct {
	ID                    model.ApplicationId   `json:"id"`
	Cluster               string                `json:"cluster"`
	Type                  string                `json:"type"`
	Name                  string                `json:"name"`
	Namespace             string                `json:"namespace"`
	Status                string                `json:"status"`
	Reason                string                `json:"reason"`
	Suspended             bool                  `json:"suspended"`
	URL                   string                `json:"url,omitempty"`
	Interval              string                `json:"interval,omitempty"`
	Chart                 string                `json:"chart,omitempty"`
	Version               string                `json:"version,omitempty"`
	TargetNamespace       string                `json:"target_namespace,omitempty"`
	RepositoryId          model.ApplicationId   `json:"repository_id,omitempty"`
	Dependencies          []model.ApplicationId `json:"dependencies,omitempty"`
	InventoryEntries      []InventoryEntry      `json:"inventory_entries,omitempty"`
	LastAppliedRevision   string                `json:"last_applied_revision"`
	LastAttemptedRevision string                `json:"last_attempted_revision"`
}

func renderFluxCD(w *model.World) []*FluxCDResource {
	if w == nil || w.Flux == nil {
		return []*FluxCDResource{}
	}
	flux := w.Flux
	var resources []*FluxCDResource

	for id, repo := range flux.Repositories {
		resources = append(resources, &FluxCDResource{
			ID:        id,
			Cluster:   w.ClusterName(id.ClusterId),
			Type:      string(id.Kind),
			Name:      id.Name,
			Namespace: id.Namespace,
			Status:    getFluxResourceStatus(&repo.Ready, repo.Suspended),
			Reason:    repo.Ready.Reason.Value(),
			Suspended: repo.Suspended,
			URL:       repo.Url.Value(),
			Interval:  repo.Interval.Value(),
		})
	}

	for id, chart := range flux.HelmCharts {
		resources = append(resources, &FluxCDResource{
			ID:           id,
			Cluster:      w.ClusterName(id.ClusterId),
			Type:         string(id.Kind),
			Name:         id.Name,
			Namespace:    id.Namespace,
			Status:       getFluxResourceStatus(&chart.Ready, chart.Suspended),
			Reason:       chart.Ready.Reason.Value(),
			Suspended:    chart.Suspended,
			Chart:        chart.Chart,
			Version:      chart.Version,
			Interval:     chart.Interval,
			RepositoryId: chart.RepositoryId,
		})
	}

	for id, release := range flux.HelmReleases {
		resources = append(resources, &FluxCDResource{
			ID:              id,
			Cluster:         w.ClusterName(id.ClusterId),
			Type:            string(id.Kind),
			Name:            id.Name,
			Namespace:       id.Namespace,
			Status:          getFluxResourceStatus(&release.Ready, release.Suspended),
			Reason:          release.Ready.Reason.Value(),
			Suspended:       release.Suspended,
			Chart:           release.Chart,
			Version:         release.Version,
			Interval:        release.Interval,
			TargetNamespace: release.TargetNamespace,
			RepositoryId:    release.RepositoryId,
		})
	}

	for id, kustomization := range flux.Kustomizations {
		dependencies := maps.Keys(kustomization.DependsOn)
		sort.Slice(dependencies, func(i, j int) bool {
			return dependencies[i].String() < dependencies[j].String()
		})
		resources = append(resources, &FluxCDResource{
			ID:                    id,
			Cluster:               w.ClusterName(id.ClusterId),
			Type:                  string(id.Kind),
			Name:                  id.Name,
			Namespace:             id.Namespace,
			Status:                getFluxResourceStatus(&kustomization.Ready, kustomization.Suspended),
			Reason:                kustomization.Ready.Reason.Value(),
			Suspended:             kustomization.Suspended,
			Interval:              kustomization.Interval,
			TargetNamespace:       kustomization.TargetNamespace,
			RepositoryId:          kustomization.RepositoryId,
			Dependencies:          dependencies,
			InventoryEntries:      getInventoryEntries(w, kustomization.InventoryEntries),
			LastAppliedRevision:   kustomization.LastAppliedRevision,
			LastAttemptedRevision: kustomization.LastAttemptedRevision,
		})
	}

	for id, resourceSet := range flux.ResourceSets {
		dependencies := maps.Keys(resourceSet.DependsOn)
		sort.Slice(dependencies, func(i, j int) bool {
			return dependencies[i].String() < dependencies[j].String()
		})
		resources = append(resources, &FluxCDResource{
			ID:                  id,
			Cluster:             w.ClusterName(id.ClusterId),
			Type:                string(id.Kind),
			Name:                id.Name,
			Namespace:           id.Namespace,
			Status:              getFluxResourceStatus(&resourceSet.Ready, false),
			Reason:              resourceSet.Ready.Reason.Value(),
			Suspended:           false,
			Dependencies:        dependencies,
			InventoryEntries:    getInventoryEntries(w, resourceSet.InventoryEntries),
			LastAppliedRevision: resourceSet.LastAppliedRevision.Value(),
		})
	}

	return resources
}

func getInventoryEntries(w *model.World, src map[model.ApplicationId]bool) []InventoryEntry {
	entries := make([]InventoryEntry, 0)
	for eId := range src {
		e := InventoryEntry{ID: eId}
		if app := w.GetApplication(eId); app != nil {
			e.CorootAppId = app.Id
		}
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ID.String() < entries[j].ID.String()
	})
	return entries
}

func getFluxResourceStatus(ready *model.FluxStatus, suspended bool) string {
	if suspended {
		return "Suspended"
	}

	switch ready.Status {
	case model.OK:
		return "Ready"
	case model.WARNING:
		return "Failed"
	default:
		return "Unknown"
	}
}
