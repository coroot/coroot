package model

import (
	"github.com/coroot/coroot/timeseries"
)

type Mongodb struct {
	InternalExporter bool

	Up          *timeseries.TimeSeries
	Error       LabelLastValue
	Warning     LabelLastValue
	ReplicaSet  LabelLastValue
	State       LabelLastValue
	Version     LabelLastValue
	LastApplied *timeseries.TimeSeries

	DatabaseSize   map[string]*timeseries.TimeSeries
	CollectionSize map[DbTableKey]*timeseries.TimeSeries
}

func NewMongodb(internalExporter bool) *Mongodb {
	return &Mongodb{
		InternalExporter: internalExporter,
		DatabaseSize:     map[string]*timeseries.TimeSeries{},
		CollectionSize:   map[DbTableKey]*timeseries.TimeSeries{},
	}
}

func (m *Mongodb) IsUp() bool {
	return m.Up.Last() > 0
}
