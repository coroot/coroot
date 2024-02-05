package db

import (
	"database/sql"
	"fmt"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
)

const (
	DeploymentsLastN = 100
)

type ApplicationDeployment model.ApplicationDeployment

func (ad *ApplicationDeployment) Migrate(m *Migrator) error {
	return m.Exec(`
	CREATE TABLE IF NOT EXISTS application_deployment (
		project_id TEXT NOT NULL REFERENCES project(id),
		application_id TEXT NOT NULL,
		name TEXT NOT NULL,
		started_at INT NOT NULL,
		finished_at INT NOT NULL DEFAULT 0,
		details TEXT,
		metrics_snapshot TEXT,
		notifications TEXT,
		PRIMARY KEY (project_id, application_id, started_at)
	);
`)
}

func (db *DB) SaveApplicationDeployment(projectId ProjectId, d *model.ApplicationDeployment) error {
	if d.StartedAt.IsZero() {
		return fmt.Errorf("invalid deployment")
	}

	rows, err := db.db.Query(
		"SELECT finished_at FROM application_deployment WHERE project_id = $1 AND application_id = $2 AND started_at = $3 LIMIT 1",
		projectId, d.ApplicationId, d.StartedAt)
	if err != nil {
		return err
	}
	defer func() {
		_ = rows.Close()
	}()

	if !rows.Next() {
		details, err := marshal(d.Details)
		if err != nil {
			return err
		}
		notifications, err := marshal(d.Notifications)
		if err != nil {
			return err
		}
		_, err = db.db.Exec(
			"INSERT INTO application_deployment (project_id, application_id, name, started_at, finished_at, details, notifications) VALUES ($1, $2, $3, $4, $5, $6, $7)",
			projectId, d.ApplicationId, d.Name, d.StartedAt, d.FinishedAt, details, notifications)
		return err
	}

	if d.FinishedAt.IsZero() {
		return nil
	}
	var savedFinishedAt timeseries.Time
	if err := rows.Scan(&savedFinishedAt); err != nil {
		return nil
	}
	_ = rows.Close()
	if !savedFinishedAt.IsZero() {
		return nil
	}
	_, err = db.db.Exec(
		"UPDATE application_deployment SET finished_at = $1, name = $2 WHERE project_id = $3 AND application_id = $4 AND started_at = $5",
		d.FinishedAt, d.Name, projectId, d.ApplicationId, d.StartedAt)
	return err
}

func (db *DB) GetApplicationDeployments(projectId ProjectId) (map[model.ApplicationId][]*model.ApplicationDeployment, error) {
	q := `
		WITH p AS (
		    SELECT * FROM application_deployment WHERE project_id = $1
		), 
		t AS (
			SELECT 
				application_id, name, started_at, finished_at, details, metrics_snapshot, notifications,
				row_number() OVER (PARTITION BY application_id ORDER BY started_at DESC) AS n 
			FROM 
				p
		) 
		SELECT application_id, name, started_at, finished_at, details, metrics_snapshot, notifications
		FROM t 
		WHERE n <= $2
		ORDER BY application_id, started_at
	`
	rows, err := db.db.Query(q, projectId, DeploymentsLastN)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	res := map[model.ApplicationId][]*model.ApplicationDeployment{}
	var details, metricsSnapshot, notifications sql.NullString
	for rows.Next() {
		var d model.ApplicationDeployment
		if err := rows.Scan(&d.ApplicationId, &d.Name, &d.StartedAt, &d.FinishedAt, &details, &metricsSnapshot, &notifications); err != nil {
			return res, err
		}
		if err := unmarshal(details.String, &d.Details); err != nil {
			klog.Warningln(err)
		}
		if err := unmarshal(metricsSnapshot.String, &d.MetricsSnapshot); err != nil {
			klog.Warningln(err)
		}
		if err := unmarshal(notifications.String, &d.Notifications); err != nil {
			klog.Warningln(err)
		}
		res[d.ApplicationId] = append(res[d.ApplicationId], &d)
	}
	return res, rows.Err()
}

func (db *DB) SaveApplicationDeploymentMetricsSnapshot(projectId ProjectId, d *model.ApplicationDeployment) error {
	data, err := marshal(d.MetricsSnapshot)
	if err != nil {
		return err
	}
	_, err = db.db.Exec(
		"UPDATE application_deployment SET metrics_snapshot = $1 WHERE project_id = $2 AND application_id = $3 AND started_at = $4",
		data, projectId, d.ApplicationId, d.StartedAt)
	return err
}

func (db *DB) SaveApplicationDeploymentNotifications(projectId ProjectId, d *model.ApplicationDeployment) error {
	data, err := marshal(d.Notifications)
	if err != nil {
		return err
	}
	_, err = db.db.Exec(
		"UPDATE application_deployment SET notifications = $1 WHERE project_id = $2 AND application_id = $3 AND started_at = $4",
		data, projectId, d.ApplicationId, d.StartedAt)
	return err
}
