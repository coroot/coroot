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

type StartUsageAmountGB int64

type DataTransferPricing struct {
	IngressPerGB float64 `json:"ingress_per_gb"`
	EgressPerGB  float64 `json:"egress_per_gb"`
}

type CloudPricing struct {
	Compute                 map[Region]map[InstanceType]*InstancePricing              `json:"compute"`
	ManagedDB               map[Region]map[Engine]map[InstanceType]*DBInstancePricing `json:"managed_db"`
	ManagedCache            map[Region]map[Engine]map[InstanceType]*InstancePricing   `json:"managed_cache"`
	InternetEgress          map[Region]map[StartUsageAmountGB]float64                 `json:"internet_egress"`
	IntraRegionDataTransfer map[Region]DataTransferPricing                            `json:"inter_region_data_transfer"`
}

type Model struct {
	AWS       *CloudPricing `json:"aws"`
	GCP       *CloudPricing `json:"gcp"`
	Azure     *CloudPricing `json:"azure"`
	timestamp time.Time
}
