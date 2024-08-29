package model

import "github.com/coroot/coroot/timeseries"

type AWS struct {
	DiscoveryErrors map[string]bool
}

type Rds struct {
	Status LabelLastValue

	Engine        LabelLastValue
	EngineVersion LabelLastValue
	MultiAz       LabelLastValue

	LifeSpan *timeseries.TimeSeries
}

func (r *Rds) ApplicationType() ApplicationType {
	if r == nil {
		return ApplicationTypeUnknown
	}
	switch r.Engine.Value() {
	case "postgres", "aurora-postgresql":
		return ApplicationTypePostgres
	case "mysql", "aurora-mysql":
		return ApplicationTypeMysql
	}
	return ApplicationTypeUnknown
}

type Elasticache struct {
	Status LabelLastValue

	Engine        LabelLastValue
	EngineVersion LabelLastValue

	LifeSpan *timeseries.TimeSeries
}

func (e *Elasticache) ApplicationType() ApplicationType {
	if e == nil {
		return ApplicationTypeUnknown
	}
	switch e.Engine.Value() {
	case "redis":
		return ApplicationTypeRedis
	case "memcached":
		return ApplicationTypeMemcached
	}
	return ApplicationTypeUnknown
}
