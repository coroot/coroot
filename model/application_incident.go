package model

import "github.com/coroot/coroot/timeseries"

type ApplicationIncident struct {
	Key        string
	OpenedAt   timeseries.Time
	ResolvedAt timeseries.Time
	Severity   Status
}

func (i *ApplicationIncident) Resolved() bool {
	return !i.ResolvedAt.IsZero()
}
