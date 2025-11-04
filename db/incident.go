package db

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
)

type Incident model.ApplicationIncident

func (i *Incident) Migrate(m *Migrator) error {
	err := m.Exec(`
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
	if err != nil {
		return err
	}
	if err = m.AddColumnIfNotExists("incident", "details", "text"); err != nil {
		return err
	}
	if err = m.AddColumnIfNotExists("incident", "rca", "text"); err != nil {
		return err
	}
	return nil
}

type IncidentNotification struct {
	ProjectId     ProjectId
	ApplicationId model.ApplicationId
	IncidentKey   string
	Status        model.Status
	Destination   IncidentNotificationDestination
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

type IncidentNotificationDestination struct {
	IntegrationType IntegrationType
	SlackChannel    string
}

func (d IncidentNotificationDestination) Value() (driver.Value, error) {
	switch d.IntegrationType {
	case IntegrationTypeSlack:
		return fmt.Sprintf("%s:%s", d.IntegrationType, d.SlackChannel), nil
	}
	return fmt.Sprintf("%s", d.IntegrationType), nil
}

func (d *IncidentNotificationDestination) Scan(src any) error {
	*d = IncidentNotificationDestination{}
	parts := strings.Split(src.(string), ":")
	if len(parts) == 0 {
		return nil
	}
	d.IntegrationType = IntegrationType(parts[0])
	if len(parts) > 1 {
		switch d.IntegrationType {
		case IntegrationTypeSlack:
			d.SlackChannel = parts[1]
		}
	}
	return nil
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
	var d, rca sql.NullString
	err := db.db.QueryRow(
		"SELECT application_id, opened_at, resolved_at, severity, details, rca FROM incident WHERE project_id = $1 AND key = $2 LIMIT 1",
		projectId, key).Scan(&i.ApplicationId, &i.OpenedAt, &i.ResolvedAt, &i.Severity, &d, &rca)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if i.ApplicationId.ClusterId == "" {
		i.ApplicationId.ClusterId = string(projectId)
	}
	if d.String != "" {
		if err = json.Unmarshal([]byte(d.String), &i.Details); err != nil {
			return nil, err
		}
	}
	if rca.String != "" {
		i.RCA = &model.RCA{}
		if err = json.Unmarshal([]byte(rca.String), i.RCA); err != nil {
			return nil, err
		}
	}
	return i, err
}

func (db *DB) GetLatestIncidents(projectId ProjectId, limit int) ([]*model.ApplicationIncident, error) {
	rows, err := db.db.Query(
		"SELECT application_id, key, opened_at, resolved_at, severity, details, rca FROM incident WHERE project_id = $1 ORDER BY (resolved_at = 0) DESC, opened_at DESC LIMIT $2",
		projectId, limit)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	var res []*model.ApplicationIncident
	for rows.Next() {
		var i model.ApplicationIncident
		var d, rca sql.NullString
		if err := rows.Scan(&i.ApplicationId, &i.Key, &i.OpenedAt, &i.ResolvedAt, &i.Severity, &d, &rca); err != nil {
			return nil, err
		}
		if i.ApplicationId.ClusterId == "" {
			i.ApplicationId.ClusterId = string(projectId)
		}
		if d.String != "" {
			if err = json.Unmarshal([]byte(d.String), &i.Details); err != nil {
				return nil, err
			}
		}
		if rca.String != "" {
			i.RCA = &model.RCA{}
			if err = json.Unmarshal([]byte(rca.String), i.RCA); err != nil {
				return nil, err
			}
		}
		res = append(res, &i)
	}
	return res, err
}

func (db *DB) GetApplicationIncidents(projectId ProjectId, from, to timeseries.Time) (map[model.ApplicationId][]*model.ApplicationIncident, error) {
	rows, err := db.db.Query(
		"SELECT application_id, key, opened_at, resolved_at, severity, details, rca FROM incident WHERE project_id = $1 AND opened_at <= $2 AND (resolved_at = 0 OR resolved_at >= $3) ORDER BY opened_at ASC",
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
		var d, rca sql.NullString
		if err := rows.Scan(&i.ApplicationId, &i.Key, &i.OpenedAt, &i.ResolvedAt, &i.Severity, &d, &rca); err != nil {
			return nil, err
		}
		if i.ApplicationId.ClusterId == "" {
			i.ApplicationId.ClusterId = string(projectId)
		}
		if d.String != "" {
			if err = json.Unmarshal([]byte(d.String), &i.Details); err != nil {
				return nil, err
			}
		}
		if rca.String != "" {
			i.RCA = &model.RCA{}
			if err = json.Unmarshal([]byte(rca.String), i.RCA); err != nil {
				return nil, err
			}
		}
		res[i.ApplicationId] = append(res[i.ApplicationId], &i)
	}
	return res, err
}

func (db *DB) GetLastOpenIncident(projectId ProjectId, appId model.ApplicationId) (*model.ApplicationIncident, error) {
	last := model.ApplicationIncident{
		ApplicationId: appId,
	}

	var dd, rca sql.NullString
	err := db.db.QueryRow(
		"SELECT key, opened_at, resolved_at, severity, details, rca FROM incident WHERE project_id = $1 AND (application_id = $2 OR application_id = $3) AND resolved_at = 0 ORDER BY opened_at DESC LIMIT 1",
		projectId, appId.String(), appId.StringWithoutClusterId()).
		Scan(&last.Key, &last.OpenedAt, &last.ResolvedAt, &last.Severity, &dd, &rca)
	switch err {
	case nil:
		if dd.String != "" {
			if err = json.Unmarshal([]byte(dd.String), &last.Details); err != nil {
				return nil, err
			}
		}
		if rca.String != "" {
			last.RCA = &model.RCA{}
			if err = json.Unmarshal([]byte(rca.String), last.RCA); err != nil {
				return nil, err
			}
		}
		return &last, nil
	case sql.ErrNoRows:
		return nil, nil
	default:
		return nil, err
	}
}

func (db *DB) CreateIncident(projectId ProjectId, appId model.ApplicationId, i *model.ApplicationIncident) error {
	appIdStr := appId.String()

	d, _ := json.Marshal(i.Details)
	_, err := db.db.Exec(
		"INSERT INTO incident (project_id, application_id, key, opened_at, severity, details) VALUES ($1, $2, $3, $4, $5, $6)",
		projectId, appIdStr, i.Key, i.OpenedAt, i.Severity, string(d))
	return err
}

func (db *DB) UpdateIncident(projectId ProjectId, key string, severity model.Status, details model.IncidentDetails) error {
	d, _ := json.Marshal(details)
	_, err := db.db.Exec(
		"UPDATE incident SET severity = $1, details = $2 WHERE project_id = $3 AND key = $4", severity, string(d), projectId, key)
	return err
}

func (db *DB) UpdateIncidentRCA(projectId ProjectId, i *model.ApplicationIncident, rca *model.RCA) error {
	i.RCA = rca
	d, err := json.Marshal(i.RCA)
	if err != nil {
		return err
	}
	_, err = db.db.Exec("UPDATE incident SET rca = $1 WHERE project_id = $2 AND key = $3", string(d), projectId, i.Key)
	return err
}

func (db *DB) ResolveIncident(projectId ProjectId, incident *model.ApplicationIncident) error {
	_, err := db.db.Exec(
		"UPDATE incident SET resolved_at = $1 WHERE project_id = $2 AND key = $3",
		incident.ResolvedAt, projectId, incident.Key)
	return err
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
		"UPDATE incident_notification SET sent_at = $1, external_key = $2 WHERE project_id = $3 AND incident_key = $4 AND timestamp = $5 AND destination = $6",
		n.SentAt, n.ExternalKey, n.ProjectId, n.IncidentKey, n.Timestamp, n.Destination,
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
		if n.ApplicationId.ClusterId == "" {
			n.ApplicationId.ClusterId = string(n.ProjectId)
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
		WHERE project_id = $1 AND incident_key = $2 AND destination = $3 AND timestamp < $4 
		ORDER BY timestamp
	`, n.ProjectId, n.IncidentKey, n.Destination, n.Timestamp,
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
		if n.ApplicationId.ClusterId == "" {
			n.ApplicationId.ClusterId = string(n.ProjectId)
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
