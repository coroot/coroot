package db

import (
	"database/sql"
	"fmt"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
)

type AlertNotification struct {
	ProjectId     ProjectId
	AlertId       string
	RuleId        string
	ApplicationId model.ApplicationId
	Status        model.Status
	Destination   IncidentNotificationDestination
	Timestamp     timeseries.Time
	SentAt        timeseries.Time
	ExternalKey   string
	Details       *AlertNotificationDetails
}

type AlertNotificationDetails struct {
	ProjectName string              `json:"project_name,omitempty"`
	RuleName    string              `json:"rule_name"`
	Severity    string              `json:"severity"`
	Summary     string              `json:"summary"`
	Details     []model.AlertDetail `json:"details,omitempty"`
	Duration    string              `json:"duration,omitempty"`
	ResolvedBy  string              `json:"resolved_by,omitempty"`
}

func (n *AlertNotification) Migrate(m *Migrator) error {
	return m.Exec(`
	CREATE TABLE IF NOT EXISTS alert_notification (
		project_id TEXT NOT NULL REFERENCES project(id),
		alert_id TEXT NOT NULL,
		rule_id TEXT NOT NULL,
		application_id TEXT NOT NULL,
		status INT NOT NULL,
		destination TEXT NOT NULL,
		timestamp INT NOT NULL,
		sent_at INT NOT NULL DEFAULT 0,
		external_key TEXT NOT NULL DEFAULT '',
		details TEXT
	);
`)
}

func (db *DB) PutAlertNotification(n AlertNotification) {
	details, err := marshal(n.Details)
	if err != nil {
		klog.Errorln(err)
		return
	}
	_, err = db.db.Exec(
		"INSERT INTO alert_notification (project_id, alert_id, rule_id, application_id, status, destination, timestamp, external_key, details) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
		n.ProjectId, n.AlertId, n.RuleId, n.ApplicationId, n.Status, n.Destination, n.Timestamp, n.ExternalKey, details,
	)
	if err != nil {
		klog.Errorln(err)
	}
}

func (db *DB) UpdateAlertNotification(n AlertNotification) error {
	_, err := db.db.Exec(
		"UPDATE alert_notification SET sent_at = $1, external_key = $2 WHERE project_id = $3 AND alert_id = $4 AND timestamp = $5 AND destination = $6",
		n.SentAt, n.ExternalKey, n.ProjectId, n.AlertId, n.Timestamp, n.Destination,
	)
	return err
}

func (db *DB) GetNotSentAlertNotifications(from timeseries.Time) ([]AlertNotification, error) {
	rows, err := db.db.Query(`
		SELECT project_id, alert_id, rule_id, application_id, status, destination, timestamp, external_key, details
		FROM alert_notification
		WHERE timestamp >= $1 AND sent_at = 0
		ORDER BY project_id, application_id, alert_id, timestamp
	`, from)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	var res []AlertNotification
	var details sql.NullString
	for rows.Next() {
		var n AlertNotification
		if err := rows.Scan(&n.ProjectId, &n.AlertId, &n.RuleId, &n.ApplicationId, &n.Status, &n.Destination, &n.Timestamp, &n.ExternalKey, &details); err != nil {
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

func (db *DB) GetAlertNotificationsByAlertIds(projectId ProjectId, alertIds []string) (map[string][]AlertNotification, error) {
	if len(alertIds) == 0 {
		return nil, nil
	}
	query := `
		SELECT project_id, alert_id, rule_id, application_id, status, destination, timestamp, external_key, details
		FROM alert_notification
		WHERE project_id = $1 AND alert_id IN (`
	args := []any{projectId}
	for i, id := range alertIds {
		if i > 0 {
			query += ", "
		}
		args = append(args, id)
		query += fmt.Sprintf("$%d", len(args))
	}
	query += `) ORDER BY alert_id, timestamp`

	rows, err := db.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string][]AlertNotification)
	var details sql.NullString
	for rows.Next() {
		var n AlertNotification
		if err := rows.Scan(&n.ProjectId, &n.AlertId, &n.RuleId, &n.ApplicationId, &n.Status, &n.Destination, &n.Timestamp, &n.ExternalKey, &details); err != nil {
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
		res[n.AlertId] = append(res[n.AlertId], n)
	}
	return res, nil
}

func (db *DB) GetPreviousAlertNotifications(n AlertNotification) ([]AlertNotification, error) {
	rows, err := db.db.Query(`
		SELECT project_id, alert_id, rule_id, application_id, status, destination, timestamp, external_key, details
		FROM alert_notification
		WHERE project_id = $1 AND alert_id = $2 AND destination = $3 AND timestamp < $4
		ORDER BY timestamp
	`, n.ProjectId, n.AlertId, n.Destination, n.Timestamp,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	var res []AlertNotification
	var details sql.NullString
	for rows.Next() {
		var n AlertNotification
		if err := rows.Scan(&n.ProjectId, &n.AlertId, &n.RuleId, &n.ApplicationId, &n.Status, &n.Destination, &n.Timestamp, &n.ExternalKey, &details); err != nil {
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
