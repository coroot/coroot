package model

type RiskCategory string

const (
	RiskCategorySecurity     = "Security"
	RiskCategoryAvailability = "Availability"
)

type RiskType string

const (
	RiskTypeDbInternetExposure   RiskType = "db-internet-exposure"
	RiskTypeSingleInstanceApp    RiskType = "single-instance-app"
	RiskTypeSingleNodeApp        RiskType = "single-node-app"
	RiskTypeSingleAzApp          RiskType = "single-az-app"
	RiskTypeSpotOnlyApp          RiskType = "spot-only-app"
	RiskTypeUnreplicatedDatabase RiskType = "unreplicated-database"
)

type RiskKey struct {
	Category RiskCategory `json:"category"`
	Type     RiskType     `json:"type"`
}

type RiskOverride struct {
	Key       RiskKey        `json:"risk_key"`
	Dismissal *RiskDismissal `json:"dismissal,omitempty"`
}

type RiskDismissal struct {
	By        string `json:"by"`
	Timestamp int64  `json:"timestamp"`
	Reason    string `json:"reason"`
}
