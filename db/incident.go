package db

import (
	"database/sql"
	"errors"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"k8s.io/klog"
)

type Incident model.ApplicationIncident

func (i *Incident) Migrate(m *Migrator) error {
	return m.Exec(`
	CREATE TABLE IF NOT EXISTS incident (
		project_id TEXT NOT NULL REFERENCES project(id),
		application_id TEXT NOT NULL,
		key TEXT NOT NULL,
		opened_at INT NOT NULL,
		resolved_at INT NOT NULL DEFAULT 0,
		severity INT NOT NULL,
		PRIMARY KEY (project_id, application_id, opened_at)
	);
	CREATE UNIQUE INDEX IF NOT EXISTS incident_key ON incident (project_id, key);
`)
}

type IncidentNotification struct {
	ProjectId     ProjectId
	ApplicationId model.ApplicationId
	IncidentKey   string
	Status        model.Status
	Destination   IntegrationType
	Timestamp     timeseries.Time
	SentAt        timeseries.Time
	ExternalKey   string
	Details       *IncidentNotificationDetails
}

func (n *IncidentNotification) Migrate(m *Migrator) error {
	return m.Exec(`
	CREATE TABLE IF NOT EXISTS incident_notification (
		project_id TEXT NOT NULL REFERENCES project(id),
		application_id TEXT NOT NULL,
		incident_key TEXT NOT NULL,
		status INT NOT NULL,
		destination TEXT NOT NULL,
		timestamp INT NOT NULL,
		sent_at INT NOT NULL DEFAULT 0,
		external_key TEXT NOT NULL DEFAULT '',
		details TEXT
	);
`)
}

type IncidentNotificationDetails struct {
	Reports []IncidentNotificationDetailsReport `json:"reports"`
}

type IncidentNotificationDetailsReport struct {
	Name    model.AuditReportName `json:"name"`
	Check   string                `json:"check"`
	Message string                `json:"message"`
}

func (db *DB) GetIncidentByKey(projectId ProjectId, key string) (*model.ApplicationIncident, error) {
	i := &model.ApplicationIncident{Key: key}
	err := db.db.QueryRow(
		"SELECT application_id, opened_at, resolved_at, severity FROM incident WHERE project_id = $1 AND key = $2 LIMIT 1",
		projectId, key).Scan(&i.ApplicationId, &i.OpenedAt, &i.ResolvedAt, &i.Severity)
	return i, err
}

func (db *DB) GetApplicationIncidents(projectId ProjectId, from, to timeseries.Time) (map[model.ApplicationId][]*model.ApplicationIncident, error) {
	rows, err := db.db.Query(
		"SELECT application_id, key, opened_at, resolved_at, severity FROM incident WHERE project_id = $1 AND opened_at <= $2 AND (resolved_at = 0 OR resolved_at >= $3)",
		projectId, to, from)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	res := map[model.ApplicationId][]*model.ApplicationIncident{}
	for rows.Next() {
		var i model.ApplicationIncident
		if err := rows.Scan(&i.ApplicationId, &i.Key, &i.OpenedAt, &i.ResolvedAt, &i.Severity); err != nil {
			return nil, err
		}
		res[i.ApplicationId] = append(res[i.ApplicationId], &i)
	}
	return res, err
}

func (db *DB) CreateOrUpdateIncident(projectId ProjectId, appId model.ApplicationId, now timeseries.Time, severity model.Status) (*model.ApplicationIncident, error) {
	appIdStr := appId.String()
	var last model.ApplicationIncident
	err := db.db.QueryRow(
		"SELECT key, opened_at, resolved_at, severity FROM incident WHERE project_id = $1 AND application_id = $2 ORDER BY opened_at DESC LIMIT 1",
		projectId, appIdStr).Scan(&last.Key, &last.OpenedAt, &last.ResolvedAt, &last.Severity)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	if last.OpenedAt.IsZero() || last.Resolved() {
		if severity > model.OK { // open
			i := model.ApplicationIncident{Key: utils.NanoId(8), OpenedAt: now, Severity: severity}
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

	return nil, nil
}

func (db *DB) PutIncidentNotification(n IncidentNotification) {
	details, err := marshal(n.Details)
	if err != nil {
		klog.Errorln(err)
		return
	}
	_, err = db.db.Exec(
		"INSERT INTO incident_notification (project_id, application_id, incident_key, status, destination, timestamp, external_key, details) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		n.ProjectId, n.ApplicationId, n.IncidentKey, n.Status, n.Destination, n.Timestamp, n.ExternalKey, details,
	)
	if err != nil {
		klog.Errorln(err)
	}
}

func (db *DB) UpdateIncidentNotification(n IncidentNotification) error {
	_, err := db.db.Exec(
		"UPDATE incident_notification SET sent_at = $1, external_key = $2 WHERE project_id = $3 AND application_id = $4 AND incident_key = $5 AND timestamp = $6 AND destination = $7",
		n.SentAt, n.ExternalKey, n.ProjectId, n.ApplicationId, n.IncidentKey, n.Timestamp, n.Destination,
	)
	return err
}

func (db *DB) GetNotSentIncidentNotifications(from timeseries.Time) ([]IncidentNotification, error) {
	rows, err := db.db.Query(`
		SELECT project_id, application_id, incident_key, status, destination, timestamp, external_key, details 
		FROM incident_notification 
		WHERE timestamp >= $1 AND sent_at = 0 
		ORDER BY project_id, application_id, incident_key, timestamp
	`, from)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	var res []IncidentNotification
	var details sql.NullString
	for rows.Next() {
		var n IncidentNotification
		if err := rows.Scan(&n.ProjectId, &n.ApplicationId, &n.IncidentKey, &n.Status, &n.Destination, &n.Timestamp, &n.ExternalKey, &details); err != nil {
			return nil, err
		}
		if details.String != "" {
			if err := unmarshal(details.String, &n.Details); err != nil {
				klog.Warningln(err)
			}
		}
		res = append(res, n)
	}
	return res, nil
}

func (db *DB) GetPreviousIncidentNotifications(n IncidentNotification) ([]IncidentNotification, error) {
	rows, err := db.db.Query(`
		SELECT project_id, application_id, incident_key, status, destination, timestamp, external_key, details 
		FROM incident_notification 
		WHERE project_id = $1 AND application_id = $2 AND incident_key = $3 AND destination = $4 AND timestamp < $5 
		ORDER BY timestamp
	`, n.ProjectId, n.ApplicationId, n.IncidentKey, n.Destination, n.Timestamp,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	var res []IncidentNotification
	var details sql.NullString
	for rows.Next() {
		var n IncidentNotification
		if err := rows.Scan(&n.ProjectId, &n.ApplicationId, &n.IncidentKey, &n.Status, &n.Destination, &n.Timestamp, &n.ExternalKey, &details); err != nil {
			return nil, err
		}
		if details.String != "" {
			if err := unmarshal(details.String, &n.Details); err != nil {
				klog.Warningln(err)
			}
		}
		res = append(res, n)
	}
	return res, nil
}

func (db *DB) GetSentIncidentNotificationsStat(from timeseries.Time) map[IntegrationType]int {
	rows, err := db.db.Query("SELECT destination, count(*) FROM incident_notification WHERE timestamp >= $1 AND sent_at > 0 GROUP BY destination", from)
	if err != nil {
		klog.Errorln(err)
		return nil
	}
	defer func() {
		_ = rows.Close()
	}()
	res := map[IntegrationType]int{}
	var destination IntegrationType
	var count int
	for rows.Next() {
		if err := rows.Scan(&destination, &count); err != nil {
			klog.Errorln(err)
			return nil
		}
		res[destination] = count
	}
	return res
}
