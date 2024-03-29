package model

import "github.com/coroot/coroot/timeseries"

type Rds struct {
	Status LabelLastValue

	Engine        LabelLastValue
	EngineVersion LabelLastValue
	MultiAz       LabelLastValue

	LifeSpan *timeseries.TimeSeries
}

type Elasticache struct {
	Status LabelLastValue

	Engine        LabelLastValue
	EngineVersion LabelLastValue

	LifeSpan *timeseries.TimeSeries
}
