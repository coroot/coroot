package db

type CustomCloudPricing struct {
	Default     bool    `json:"default"`
	PerCPUCore  float32 `json:"per_cpu_core"`
	PerMemoryGb float32 `json:"per_memory_gb"`
}

var defaultCustomCloudPricing = CustomCloudPricing{ //on-demand pricing for GCP (C4 machine family, us-central1)
	Default:     true,
	PerCPUCore:  0.03465,
	PerMemoryGb: 0.003938,
}
