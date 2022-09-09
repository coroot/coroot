package model

import "github.com/coroot/coroot/timeseries"

type Rds struct {
	Status LabelLastValue

	Engine        LabelLastValue
	EngineVersion LabelLastValue
	MultiAz       bool

	LifeSpan timeseries.TimeSeries
}
