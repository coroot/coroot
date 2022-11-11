package db

import (
	"database/sql"
	"errors"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

type Incident struct {
	Key        string
	OpenedAt   timeseries.Time
	ResolvedAt timeseries.Time
	Severity   model.Status
	SentAt     timeseries.Time
}

func (cc *Incident) Migrate(m *Migrator) error {
	return m.Exec(`
	CREATE TABLE IF NOT EXISTS incident (
		project_id TEXT NOT NULL REFERENCES project(id),
		application_id TEXT NOT NULL,
		key TEXT NOT NULL,
		opened_at INT NOT NULL,
		resolved_at INT NOT NULL DEFAULT 0,
		severity INT NOT NULL,
		sent_at INT NOT NULL DEFAULT 0,
		PRIMARY KEY (project_id, application_id, opened_at)
	);
	CREATE UNIQUE INDEX IF NOT EXISTS incident_key ON incident (project_id, key);
`)
}

func (db *DB) GetIncidentByKey(projectId ProjectId, key string) (*Incident, error) {
	i := &Incident{Key: key}
	err := db.db.QueryRow(
		"SELECT opened_at, resolved_at, severity, sent_at FROM incident WHERE project_id = $1 AND key = $2 LIMIT 1",
		projectId, key).Scan(&i.OpenedAt, &i.ResolvedAt, &i.Severity, &i.SentAt)
	return i, err
}

func (db *DB) GetIncidentsByApp(projectId ProjectId, appId model.ApplicationId, from, to timeseries.Time) ([]Incident, error) {
	rows, err := db.db.Query(
		"SELECT key, opened_at, resolved_at, severity, sent_at FROM incident WHERE project_id = $1 AND application_id = $2 AND opened_at <= $3 AND (resolved_at = 0 OR resolved_at >= $4)",
		projectId, appId.String(), to, from)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Incident
	var i Incident
	for rows.Next() {
		if err := rows.Scan(&i.Key, &i.OpenedAt, &i.ResolvedAt, &i.Severity, &i.SentAt); err != nil {
			return nil, err
		}
		res = append(res, i)
	}
	return res, err
}

func (db *DB) MarkIncidentAsSent(projectId ProjectId, appId model.ApplicationId, i *Incident, now timeseries.Time) error {
	_, err := db.db.Exec(
		"UPDATE incident SET sent_at = $1 WHERE project_id = $2 AND application_id = $3 AND opened_at = $4",
		now, projectId, appId.String(), i.OpenedAt)
	return err
}

func (db *DB) CreateOrUpdateIncident(projectId ProjectId, appId model.ApplicationId, now timeseries.Time, severity model.Status) (*Incident, error) {
	appIdStr := appId.String()
	var last Incident
	err := db.db.QueryRow(
		"SELECT key, opened_at, resolved_at, severity, sent_at FROM incident WHERE project_id = $1 AND application_id = $2 ORDER BY opened_at DESC LIMIT 1",
		projectId, appIdStr).Scan(&last.Key, &last.OpenedAt, &last.ResolvedAt, &last.Severity, &last.SentAt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	if last.OpenedAt.IsZero() || !last.ResolvedAt.IsZero() {
		if severity > model.OK { // open
			i := Incident{Key: utils.NanoId(8), OpenedAt: now, Severity: severity}
			_, err := db.db.Exec(
				"INSERT INTO incident (project_id, application_id, key, opened_at, severity) VALUES ($1, $2, $3, $4, $5)",
				projectId, appIdStr, i.Key, i.OpenedAt, i.Severity)
			return &i, err
		}
		return nil, nil
	}

	if severity == model.OK { // close
		last.ResolvedAt = now
		_, err := db.db.Exec(
			"UPDATE incident SET resolved_at = $1 WHERE project_id = $2 AND application_id = $3 AND opened_at = $4",
			last.ResolvedAt, projectId, appIdStr, last.OpenedAt)
		return &last, err
	}

	if severity != last.Severity { // update severity
		last.Severity = severity
		_, err := db.db.Exec(
			"UPDATE incident SET severity = $1 WHERE project_id = $2 AND application_id = $3 AND opened_at = $4",
			last.Severity, projectId, appIdStr, last.OpenedAt)
		return &last, err
	}

	if last.SentAt.IsZero() {
		return &last, nil
	}

	return nil, nil
}
