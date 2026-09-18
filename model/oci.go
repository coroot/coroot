package model

import "github.com/coroot/coroot/timeseries"

type OCI struct {
	Configured      bool
	DiscoveryErrors map[string]bool
}

type OCIDB struct {
	Id     string // the OCID
	Status LabelLastValue

	Engine        LabelLastValue
	EngineVersion LabelLastValue

	LifeSpan *timeseries.TimeSeries
}

func (d *OCIDB) ApplicationType() ApplicationType {
	if d == nil {
		return ApplicationTypeUnknown
	}
	switch d.Engine.Value() {
	case "postgres":
		return ApplicationTypePostgres
	case "mysql":
		return ApplicationTypeMysql
	}
	return ApplicationTypeUnknown
}

type OCICache struct {
	Id     string // the OCID
	Status LabelLastValue

	Engine        LabelLastValue
	EngineVersion LabelLastValue

	LifeSpan *timeseries.TimeSeries
}

func (c *OCICache) IsUp() bool {
	return c != nil && c.Status.Value() == "ACTIVE"
}

func (c *OCICache) ApplicationType() ApplicationType {
	if c == nil {
		return ApplicationTypeUnknown
	}
	switch c.Engine.Value() {
	case "redis":
		return ApplicationTypeRedis
	case "valkey":
		return ApplicationTypeValkey
	}
	return ApplicationTypeUnknown
}
