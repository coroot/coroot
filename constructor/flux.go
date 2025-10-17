package constructor

import (
	"strings"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func loadFluxResources(w *model.World, metrics map[string][]*model.MetricValues) {
	flux := model.NewFlux()

	loadFluxRepos(flux, metrics["fluxcd_git_repository_info"], metrics["fluxcd_git_repository_status"], model.ApplicationKindFluxGitRepository)
	loadFluxRepos(flux, metrics["fluxcd_oci_repository_info"], metrics["fluxcd_oci_repository_status"], model.ApplicationKindFluxOCIRepository)
	loadFluxRepos(flux, metrics["fluxcd_helm_repository_info"], metrics["fluxcd_helm_repository_status"], model.ApplicationKindFluxHelmRepository)

	loadFluxHelmCharts(flux, metrics)
	loadFluxHelmReleases(flux, metrics)
	loadFluxKustomizations(flux, metrics)
	loadFluxResourceSets(flux, metrics)

	if len(flux.Repositories) > 0 || len(flux.HelmCharts) > 0 || len(flux.HelmReleases) > 0 || len(flux.Kustomizations) > 0 || len(flux.ResourceSets) > 0 {
		w.Flux = flux
	}
}

func loadFluxResourceSets(flux *model.Flux, metrics map[string][]*model.MetricValues) {
	for _, m := range metrics["fluxcd_resourceset_info"] {
		id := model.ApplicationId{
			ClusterId: "_",
			Namespace: m.Labels["namespace"],
			Kind:      model.ApplicationKindFluxResourceSet,
			Name:      m.Labels["name"],
		}
		rs, ok := flux.ResourceSets[id]
		if !ok {
			rs = &model.FluxResourceSet{
				DependsOn:        map[model.ApplicationId]bool{},
				InventoryEntries: map[model.ApplicationId]bool{},
			}
			flux.ResourceSets[id] = rs
		}
		rs.LastAppliedRevision.Update(m.Values, m.Labels["last_applied_revision"])
	}

	updateStatuses(flux, metrics["fluxcd_resourceset_status"], model.ApplicationKindFluxResourceSet)

	for _, m := range metrics["fluxcd_resourceset_dependency_info"] {
		id := model.ApplicationId{
			ClusterId: "_",
			Namespace: m.Labels["namespace"],
			Kind:      model.ApplicationKindFluxResourceSet,
			Name:      m.Labels["name"],
		}
		rs, ok := flux.ResourceSets[id]
		if !ok {
			continue
		}
		depId := model.ApplicationId{
			ClusterId: "_",
			Namespace: m.Labels["depends_on_namespace"],
			Name:      m.Labels["depends_on_name"],
			Kind:      model.ApplicationKind(m.Labels["depends_on_kind"]),
		}
		if depId.Namespace == "" {
			depId.Namespace = id.Namespace
		}
		rs.DependsOn[depId] = true
	}
	for _, m := range metrics["fluxcd_resourceset_inventory_entry_info"] {
		id := model.ApplicationId{
			Namespace: m.Labels["namespace"],
			Kind:      model.ApplicationKindFluxResourceSet,
			Name:      m.Labels["name"],
		}
		if rs := flux.ResourceSets[id]; rs != nil {
			if eId := parseEntryId(m.Labels["entry_id"]); !eId.IsZero() {
				rs.InventoryEntries[eId] = true
			}
		}
	}
}

func loadFluxKustomizations(flux *model.Flux, metrics map[string][]*model.MetricValues) {
	for _, m := range metrics["fluxcd_kustomization_info"] {
		id := model.ApplicationId{
			Namespace: m.Labels["namespace"],
			Kind:      model.ApplicationKindFluxKustomization,
			Name:      m.Labels["name"],
		}
		k, ok := flux.Kustomizations[id]
		if !ok {
			k = &model.FluxKustomization{
				DependsOn:        map[model.ApplicationId]*model.FluxKustomization{},
				InventoryEntries: map[model.ApplicationId]bool{},
			}
			flux.Kustomizations[id] = k
		}
		t, v := m.Values.LastNotNull()
		if t < k.LastInfo || timeseries.IsNaN(v) {
			continue
		}

		k.LastInfo = t
		sourceId := model.ApplicationId{
			Namespace: m.Labels["source_namespace"],
			Kind:      model.ApplicationKind(m.Labels["source_kind"]),
			Name:      m.Labels["source_name"],
		}
		if sourceId.Namespace == "" {
			sourceId.Namespace = id.Namespace
		}
		k.RepositoryId = sourceId
		k.Interval = m.Labels["interval"]
		k.TargetNamespace = m.Labels["target_namespace"]
		k.LastAppliedRevision = m.Labels["last_applied_revision"]
		k.LastAttemptedRevision = m.Labels["last_attempted_revision"]

		if m.Labels["suspended"] == "true" && m.Values.Last() == 1 {
			k.Suspended = true
		}
	}

	updateStatuses(flux, metrics["fluxcd_kustomization_status"], model.ApplicationKindFluxKustomization)

	for _, m := range metrics["fluxcd_kustomization_dependency_info"] {
		id := model.ApplicationId{
			Namespace: m.Labels["namespace"],
			Kind:      model.ApplicationKindFluxKustomization,
			Name:      m.Labels["name"],
		}
		k, ok := flux.Kustomizations[id]
		if !ok {
			continue
		}
		depId := model.ApplicationId{
			Namespace: m.Labels["depends_on_namespace"],
			Name:      m.Labels["depends_on_name"],
			Kind:      model.ApplicationKindFluxKustomization,
		}
		if depId.Namespace == "" {
			depId.Namespace = id.Namespace
		}
		if dep := flux.Kustomizations[depId]; dep != nil {
			k.DependsOn[depId] = dep
		}

	}
	for _, m := range metrics["fluxcd_kustomization_inventory_entry_info"] {
		id := model.ApplicationId{
			Namespace: m.Labels["namespace"],
			Kind:      model.ApplicationKindFluxKustomization,
			Name:      m.Labels["name"],
		}
		if k := flux.Kustomizations[id]; k != nil {
			if eId := parseEntryId(m.Labels["entry_id"]); !eId.IsZero() {
				k.InventoryEntries[eId] = true
			}
		}
	}
}

func loadFluxHelmReleases(flux *model.Flux, metrics map[string][]*model.MetricValues) {
	for _, m := range metrics["fluxcd_helm_release_info"] {
		id := model.ApplicationId{
			Namespace: m.Labels["namespace"],
			Kind:      model.ApplicationKindFluxHelmRelease,
			Name:      m.Labels["name"],
		}
		r, ok := flux.HelmReleases[id]
		if !ok {
			r = &model.FluxHelmRelease{}
			flux.HelmReleases[id] = r
		}
		t, v := m.Values.LastNotNull()
		if t < r.LastInfo || timeseries.IsNaN(v) {
			continue
		}

		r.LastInfo = t
		sourceId := model.ApplicationId{
			Namespace: m.Labels["source_namespace"],
			Kind:      model.ApplicationKind(m.Labels["source_kind"]),
			Name:      m.Labels["source_name"],
		}
		if refName := m.Labels["chart_ref_name"]; refName != "" {
			sourceId.Name = refName
			sourceId.Namespace = m.Labels["chart_ref_namespace"]
			sourceId.Kind = model.ApplicationKind(m.Labels["chart_ref_kind"])
		}
		if sourceId.Namespace == "" {
			sourceId.Namespace = id.Namespace
		}

		r.RepositoryId = sourceId
		r.Chart = m.Labels["chart"]
		r.Version = m.Labels["version"]
		r.Interval = m.Labels["interval"]
		r.TargetNamespace = m.Labels["target_namespace"]
		if m.Labels["suspended"] == "true" && m.Values.Last() == 1 {
			r.Suspended = true
		}
	}
	updateStatuses(flux, metrics["fluxcd_helm_release_status"], model.ApplicationKindFluxHelmRelease)
}

func loadFluxHelmCharts(flux *model.Flux, metrics map[string][]*model.MetricValues) {
	for _, m := range metrics["fluxcd_helm_chart_info"] {
		id := model.ApplicationId{
			Namespace: m.Labels["namespace"],
			Kind:      model.ApplicationKindFluxHelmChart,
			Name:      m.Labels["name"],
		}
		chart, ok := flux.HelmCharts[id]
		if !ok {
			chart = &model.FluxHelmChart{}
			flux.HelmCharts[id] = chart
		}
		t, v := m.Values.LastNotNull()
		if t < chart.LastInfo || timeseries.IsNaN(v) {
			continue
		}

		chart.LastInfo = t
		sourceId := model.ApplicationId{
			Namespace: m.Labels["source_namespace"],
			Kind:      model.ApplicationKind(m.Labels["source_kind"]),
			Name:      m.Labels["source_name"],
		}
		if sourceId.Namespace == "" {
			sourceId.Namespace = id.Namespace
		}
		chart.RepositoryId = sourceId
		chart.Chart = m.Labels["chart"]
		chart.Version = m.Labels["version"]
		chart.Interval = m.Labels["interval"]
		if m.Labels["suspended"] == "true" && m.Values.Last() == 1 {
			chart.Suspended = true
		}
	}
	updateStatuses(flux, metrics["fluxcd_helm_chart_status"], model.ApplicationKindFluxHelmChart)
}

func loadFluxRepos(flux *model.Flux, info, status []*model.MetricValues, kind model.ApplicationKind) {
	for _, m := range info {
		id := model.ApplicationId{Namespace: m.Labels["namespace"], Kind: kind, Name: m.Labels["name"]}
		r, ok := flux.Repositories[id]
		if !ok {
			r = &model.FluxRepository{}
			flux.Repositories[id] = r
		}
		r.Url.Update(m.Values, m.Labels["url"])
		r.Interval.Update(m.Values, m.Labels["interval"])
		if m.Labels["suspended"] == "true" && m.Values.Last() == 1 {
			r.Suspended = true
		}
	}
	updateStatuses(flux, status, kind)
}

func updateStatuses(flux *model.Flux, metrics []*model.MetricValues, kind model.ApplicationKind) {
	for _, m := range metrics {
		id := model.ApplicationId{Namespace: m.Labels["namespace"], Kind: kind, Name: m.Labels["name"]}
		var ready *model.FluxStatus
		switch kind {
		case model.ApplicationKindFluxGitRepository, model.ApplicationKindFluxHelmRepository, model.ApplicationKindFluxOCIRepository:
			if o := flux.Repositories[id]; o != nil {
				ready = &o.Ready
			}
		case model.ApplicationKindFluxKustomization:
			if o := flux.Kustomizations[id]; o != nil {
				ready = &o.Ready
			}
		case model.ApplicationKindFluxHelmChart:
			if o := flux.HelmCharts[id]; o != nil {
				ready = &o.Ready
			}
		case model.ApplicationKindFluxHelmRelease:
			if o := flux.HelmReleases[id]; o != nil {
				ready = &o.Ready
			}
		case model.ApplicationKindFluxResourceSet:
			if o := flux.ResourceSets[id]; o != nil {
				ready = &o.Ready
			}
		}
		if ready == nil {
			return
		}
		switch t := m.Labels["type"]; t {
		case "Ready":
			ready.Reason.Update(m.Values, m.Labels["reason"])
			switch l := m.Values.Last(); l {
			case 0.:
				ready.Status = model.WARNING
			case 1.:
				ready.Status = model.OK
			}
		}
	}
}

func parseEntryId(id string) model.ApplicationId {
	parts := strings.SplitN(id, "_", 4)
	if len(parts) != 4 {
		return model.ApplicationId{}
	}
	return model.ApplicationId{
		Namespace: parts[0],
		Name:      parts[1],
		Kind:      model.ApplicationKind(parts[3]),
	}
}
