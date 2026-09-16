package model

import "github.com/coroot/coroot/timeseries"

type GCP struct {
	Configured      bool // the cluster-agent reports the GCP integration
	DiscoveryErrors map[string]bool
}

type CloudSQL struct {
	Id     string // "<project>/<instance>", the value of the cloudsql_instance_id metric label
	Status LabelLastValue

	Engine           LabelLastValue
	EngineVersion    LabelLastValue
	AvailabilityType LabelLastValue

	LifeSpan *timeseries.TimeSeries
}

func (c *CloudSQL) ApplicationType() ApplicationType {
	if c == nil {
		return ApplicationTypeUnknown
	}
	switch c.Engine.Value() {
	case "postgres":
		return ApplicationTypePostgres
	case "mysql":
		return ApplicationTypeMysql
	case "sqlserver":
		return ApplicationTypeMSSQL
	}
	return ApplicationTypeUnknown
}

type Memorystore struct {
	Id     string // "<project>/<region>/<instance>", the value of the memorystore_instance_id metric label
	Status LabelLastValue

	Engine        LabelLastValue
	EngineVersion LabelLastValue

	LifeSpan *timeseries.TimeSeries
}

func (m *Memorystore) IsUp() bool {
	switch m.Status.Value() {
	case "READY", "ACTIVE":
		return true
	}
	return false
}

func (m *Memorystore) ApplicationType() ApplicationType {
	if m == nil {
		return ApplicationTypeUnknown
	}
	switch m.Engine.Value() {
	case "redis":
		return ApplicationTypeRedis
	case "valkey":
		return ApplicationTypeValkey
	case "memcached":
		return ApplicationTypeMemcached
	}
	return ApplicationTypeUnknown
}
