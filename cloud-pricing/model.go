package cloud_pricing

import "time"

type Region string
type InstanceType string
type PurchaseOption string
type DBDeploymentOption string
type Engine string

type InstancePricing struct {
	OnDemand float32 `json:"on_demand"`
	Spot     float32 `json:"spot"`
}

type DBInstancePricing struct {
	SingleAz *InstancePricing `json:"single_az"`
	MultiAz  *InstancePricing `json:"multi_az"`
}

type CloudPricing struct {
	Compute      map[Region]map[InstanceType]*InstancePricing              `json:"compute"`
	ManagedDB    map[Region]map[Engine]map[InstanceType]*DBInstancePricing `json:"managed_db"`
	ManagedCache map[Region]map[Engine]map[InstanceType]*InstancePricing   `json:"managed_cache"`
}

type Model struct {
	AWS       *CloudPricing `json:"aws"`
	GCP       *CloudPricing `json:"gcp"`
	Azure     *CloudPricing `json:"azure"`
	timestamp time.Time
}
