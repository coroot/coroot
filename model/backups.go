package model

import (
	"github.com/coroot/coroot/timeseries"
)

type DBBackups struct {
	Schedule            string
	NextScheduledBackup timeseries.Time
	RetentionPolicy     string
	Methods             map[string]*DBBackupMethod
	LastFailedBackup    timeseries.Time
	Conditions          map[string]DBBackupCondition
	Runs                []*DBBackupRun
}

type DBBackupMethod struct {
	Destination              string
	Endpoint                 string
	Schedule                 string
	LastSuccessfulBackup     timeseries.Time
	FirstRecoverabilityPoint timeseries.Time
}

type DBBackupRun struct {
	Name        string
	Method      string
	Kind        string
	Destination string
	Status      string
	CompletedAt timeseries.Time
}

func (r *DBBackupRun) Succeeded() bool {
	switch r.Status {
	case "Succeeded", "completed", "ready":
		return true
	}
	return false
}

type DBBackupCondition struct {
	Status string
	Reason string
}
