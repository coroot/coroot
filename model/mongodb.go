package model

import (
	"github.com/coroot/coroot/timeseries"
)

type Mongodb struct {
	Up          *timeseries.TimeSeries
	ReplicaSet  LabelLastValue
	State       LabelLastValue
	LastApplied *timeseries.TimeSeries
}

func NewMongodb() *Mongodb {
	return &Mongodb{}
}

func (m *Mongodb) IsUp() bool {
	return m.Up.Last() > 0
}
