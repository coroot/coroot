package model

import "github.com/coroot/coroot/timeseries"

const (
	ApplicationKindFluxGitRepository  ApplicationKind = "GitRepository"
	ApplicationKindFluxOCIRepository  ApplicationKind = "OCIRepository"
	ApplicationKindFluxHelmRepository ApplicationKind = "HelmRepository"
	ApplicationKindFluxHelmChart      ApplicationKind = "HelmChart"
	ApplicationKindFluxHelmRelease    ApplicationKind = "HelmRelease"
	ApplicationKindFluxKustomization  ApplicationKind = "Kustomization"
	ApplicationKindFluxResourceSet    ApplicationKind = "ResourceSet"
)

type Flux struct {
	Repositories   map[ApplicationId]*FluxRepository
	HelmCharts     map[ApplicationId]*FluxHelmChart
	HelmReleases   map[ApplicationId]*FluxHelmRelease
	Kustomizations map[ApplicationId]*FluxKustomization
	ResourceSets   map[ApplicationId]*FluxResourceSet
}

func NewFlux() *Flux {
	return &Flux{
		Repositories:   make(map[ApplicationId]*FluxRepository),
		HelmCharts:     make(map[ApplicationId]*FluxHelmChart),
		HelmReleases:   make(map[ApplicationId]*FluxHelmRelease),
		Kustomizations: make(map[ApplicationId]*FluxKustomization),
		ResourceSets:   make(map[ApplicationId]*FluxResourceSet),
	}
}

func (f *Flux) Merge(other *Flux) {
	for appId, v := range other.Repositories {
		f.Repositories[appId] = v
	}
	for appId, v := range other.HelmCharts {
		f.HelmCharts[appId] = v
	}
	for appId, v := range other.HelmReleases {
		f.HelmReleases[appId] = v
	}
	for appId, v := range other.Kustomizations {
		f.Kustomizations[appId] = v
	}
	for appId, v := range other.ResourceSets {
		f.ResourceSets[appId] = v
	}
}

type FluxStatus struct {
	Status Status
	Reason LabelLastValue
}

type FluxRepository struct {
	Ready     FluxStatus
	Url       LabelLastValue
	Interval  LabelLastValue
	Suspended bool
}

type FluxHelmChart struct {
	Ready        FluxStatus
	Chart        string
	Version      string
	Interval     string
	RepositoryId ApplicationId
	LastInfo     timeseries.Time
	Suspended    bool
}

type FluxHelmRelease struct {
	Ready           FluxStatus
	LastInfo        timeseries.Time
	TargetNamespace string
	Chart           string
	Version         string
	Interval        string
	RepositoryId    ApplicationId
	Suspended       bool
}

type FluxKustomization struct {
	Ready                 FluxStatus
	LastInfo              timeseries.Time
	TargetNamespace       string
	Path                  string
	Interval              string
	LastAppliedRevision   string
	LastAttemptedRevision string
	RepositoryId          ApplicationId
	DependsOn             map[ApplicationId]*FluxKustomization
	InventoryEntries      map[ApplicationId]bool
	Suspended             bool
}

type FluxResourceSet struct {
	LastAppliedRevision LabelLastValue
	Ready               FluxStatus
	DependsOn           map[ApplicationId]bool
	InventoryEntries    map[ApplicationId]bool
}
