package model

type RiskCategory string

const (
	RiskCategorySecurity = "Security"
)

type RiskType string

const (
	RiskTypeDbInternetExposure RiskType = "db-internet-exposure"
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
