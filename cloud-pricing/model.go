package cloud_pricing

import "time"

type InstancePricing struct {
	OnDemand float32 `json:"on_demand"`
	Spot     float32 `json:"spot"`
}

type CloudPricing struct {
	Compute map[string]map[string]*InstancePricing `json:"compute"`
}

type Model struct {
	AWS       *CloudPricing `json:"aws"`
	GCP       *CloudPricing `json:"gcp"`
	Azure     *CloudPricing `json:"azure"`
	timestamp time.Time
}
