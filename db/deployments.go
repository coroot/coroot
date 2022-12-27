package db

import (
	"database/sql"
	"encoding/json"
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
		PRIMARY KEY (project_id, application_id, started_at)
	);
`)
}

func (db *DB) SaveApplicationDeployment(projectId ProjectId, appId model.ApplicationId, d *model.ApplicationDeployment) error {
	if d.StartedAt.IsZero() && d.FinishedAt.IsZero() {
		return nil
	}

	appIdStr := appId.String()

	if d.StartedAt.IsZero() {
		_, err := db.db.Exec(
			"UPDATE application_deployment SET finished_at = $1, name = $2 WHERE project_id = $3 AND application_id = $4 AND started_at <= $5 AND finished_at = 0",
			d.FinishedAt, d.Name, projectId, appIdStr, d.FinishedAt)
		return err
	}

	rows, err := db.db.Query(
		"SELECT finished_at FROM application_deployment WHERE project_id = $1 AND application_id = $2 AND started_at = $3 LIMIT 1",
		projectId, appIdStr, d.StartedAt)
	if err != nil {
		return err
	}
	defer func() {
		_ = rows.Close()
	}()

	if !rows.Next() {
		details, err := json.Marshal(d.Details)
		if err != nil {
			return err
		}
		_, err = db.db.Exec(
			"INSERT INTO application_deployment (project_id, application_id, name, started_at, finished_at, details) VALUES ($1, $2, $3, $4, $5, $6)",
			projectId, appIdStr, d.Name, d.StartedAt, d.FinishedAt, string(details))
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
		d.FinishedAt, d.Name, projectId, appIdStr, d.StartedAt)
	return err
}

func (db *DB) GetApplicationDeployments(projectId ProjectId) (map[model.ApplicationId][]*model.ApplicationDeployment, error) {
	q := `
		WITH p AS (
		    SELECT * FROM application_deployment WHERE project_id = $1
		), 
		t AS (
			SELECT 
				application_id, name, started_at, finished_at, details, metrics_snapshot, 
				row_number() OVER (PARTITION BY application_id ORDER BY started_at DESC) AS n 
			FROM 
				p
		) 
		SELECT application_id, name, started_at, finished_at, details, metrics_snapshot 
		FROM t 
		WHERE n <= $2
		ORDER BY application_id, started_at DESC
	`
	rows, err := db.db.Query(q, projectId, DeploymentsLastN)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	res := map[model.ApplicationId][]*model.ApplicationDeployment{}
	var appIdStr, name string
	var startedAt, finishedAt timeseries.Time
	var details, metricsSnapshot sql.NullString
	for rows.Next() {
		if err := rows.Scan(&appIdStr, &name, &startedAt, &finishedAt, &details, &metricsSnapshot); err != nil {
			return res, err
		}
		appId, err := model.NewApplicationIdFromString(appIdStr)
		if err != nil {
			klog.Warningln("invalid application id:", appIdStr)
		}
		d := model.ApplicationDeployment{
			ApplicationId: appId,
			Name:          name,
			StartedAt:     startedAt,
			FinishedAt:    finishedAt,
		}
		if details.String != "" {
			var v model.ApplicationDeploymentDetails
			if err := json.Unmarshal([]byte(details.String), &v); err != nil {
				klog.Warningln(err)
			} else {
				d.Details = &v
			}
		}
		if metricsSnapshot.String != "" {
			var v model.MetricsSnapshot
			if err := json.Unmarshal([]byte(metricsSnapshot.String), &v); err != nil {
				klog.Warningln(err)
			} else {
				d.MetricsSnapshot = &v
			}
		}
		res[appId] = append(res[appId], &d)
	}
	return res, rows.Err()
}

func (db *DB) GetApplicationDeploymentsWithoutMetricsSnapshot(projectId ProjectId) ([]*model.ApplicationDeployment, error) {
	rows, err := db.db.Query(
		"SELECT application_id, name, started_at, finished_at FROM application_deployment WHERE project_id = $1 AND finished_at != 0 AND metrics_snapshot is null",
		projectId)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var res []*model.ApplicationDeployment
	var appIdStr, name string
	var startedAt, finishedAt timeseries.Time
	for rows.Next() {
		if err := rows.Scan(&appIdStr, &name, &startedAt, &finishedAt); err != nil {
			return res, err
		}
		appId, err := model.NewApplicationIdFromString(appIdStr)
		if err != nil {
			klog.Warningln("invalid application id:", appIdStr)
		}
		d := model.ApplicationDeployment{
			ApplicationId: appId,
			Name:          name,
			StartedAt:     startedAt,
			FinishedAt:    finishedAt,
		}
		res = append(res, &d)
	}
	return res, rows.Err()
}

func (db *DB) MarkShortApplicationDeployments(projectId ProjectId, minLifetime timeseries.Duration) error {
	q := `
		WITH p AS (
		    SELECT * FROM application_deployment WHERE project_id = $1
		), 
		t AS (
			SELECT
				application_id, name, started_at,
				lead(started_at) OVER (PARTITION BY application_id ORDER BY started_at) - finished_at AS next_started_in
			FROM p
			WHERE finished_at != 0 AND metrics_snapshot IS NULL
		)
		UPDATE application_deployment SET metrics_snapshot = '{}'
		FROM t
		WHERE
			application_deployment.project_id = $2 and
			application_deployment.application_id = t.application_id and
			application_deployment.started_at = t.started_at and
			t.next_started_in < $3
`
	_, err := db.db.Exec(q, projectId, projectId, minLifetime)
	return err
}

func (db *DB) SaveApplicationDeploymentMetricsSnapshot(projectId ProjectId, appId model.ApplicationId, startedAt timeseries.Time, ms model.MetricsSnapshot) error {
	data, err := json.Marshal(ms)
	if err != nil {
		return err
	}
	_, err = db.db.Exec(
		"UPDATE application_deployment SET metrics_snapshot = $1 WHERE project_id = $2 AND application_id = $3 AND started_at = $4",
		string(data), projectId, appId.String(), startedAt)
	return err
}
