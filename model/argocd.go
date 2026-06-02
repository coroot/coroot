package model

import "github.com/coroot/coroot/timeseries"

const (
	ApplicationKindArgoApplication ApplicationKind = "ArgoCDApplication"
)

type ArgoCD struct {
	Applications map[ApplicationId]*ArgoApplication
}

func NewArgoCD() *ArgoCD {
	return &ArgoCD{
		Applications: make(map[ApplicationId]*ArgoApplication),
	}
}

func (a *ArgoCD) Merge(other *ArgoCD) {
	for id, v := range other.Applications {
		a.Applications[id] = v
	}
}

type ArgoManagedResource struct {
	SyncStatus   string
	SyncResult   string
	HealthStatus string
}

type ArgoApplication struct {
	Project             LabelLastValue
	SourceType          LabelLastValue
	RepoURL             LabelLastValue
	Path                LabelLastValue
	Chart               LabelLastValue
	DestServer          LabelLastValue
	DestName            LabelLastValue
	DestNamespace       LabelLastValue
	SyncStatus          string
	HealthStatus        string
	OperationPhase      string
	OperationFinishedAt timeseries.Time
	Resources           map[ApplicationId]*ArgoManagedResource
}
