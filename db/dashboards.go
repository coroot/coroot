package db

import (
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/coroot/coroot/utils"
)

type Dashboards struct{}

func (s *Dashboards) Migrate(m *Migrator) error {
	return m.Exec(`
	CREATE TABLE IF NOT EXISTS dashboards (
		project_id TEXT NOT NULL REFERENCES project(id),
		id TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		config TEXT NOT NULL DEFAULT '{}',
		PRIMARY KEY (project_id, id)
	)`)
}

type Dashboard struct {
	Id          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Config      DashboardConfig `json:"config"`
}

type DashboardConfig struct {
	Groups []DashboardPanelGroup `json:"groups"`
}

type DashboardPanelGroup struct {
	Name      string           `json:"name"`
	Panels    []DashboardPanel `json:"panels"`
	Collapsed bool             `json:"collapsed"`
}

type DashboardPanel struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Source      DashboardPanelSource `json:"source"`
	Widget      DashboardPanelWidget `json:"widget"`
	Box         DashboardPanelBox    `json:"box"`
}

type DashboardPanelBox struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type DashboardPanelSource struct {
	Metrics *DashboardPanelSourceMetrics `json:"metrics,omitempty"`
}

type DashboardPanelSourceMetrics struct {
	Queries []DashboardPanelSourceMetricsQuery `json:"queries"`
}

type DashboardPanelSourceMetricsQuery struct {
	DataSource string `json:"datasource"`
	Query      string `json:"query"`
	Legend     string `json:"legend"`
	Color      string `json:"color"`
}

type DashboardPanelWidget struct {
	Chart *DashboardPanelChart `json:"chart,omitempty"`
}

type DashboardPanelChart struct {
	Display string `json:"display"`
	Stacked bool   `json:"stacked"`
}

func (db *DB) GetDashboards(projectId ProjectId) ([]*Dashboard, error) {
	rows, err := db.Query("SELECT id, name, description FROM dashboards WHERE project_id = $1", projectId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ds []*Dashboard
	for rows.Next() {
		var d Dashboard
		if err = rows.Scan(&d.Id, &d.Name, &d.Description); err != nil {
			return nil, err
		}
		ds = append(ds, &d)
	}
	return ds, nil
}

func (db *DB) GetDashboard(projectId ProjectId, id string) (*Dashboard, error) {
	d := &Dashboard{Id: id}
	var config string
	err := db.db.QueryRow("SELECT name, config FROM dashboards WHERE project_id = $1 AND id = $2", projectId, id).Scan(&d.Name, &config)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if err = json.Unmarshal([]byte(config), &d.Config); err != nil {
		return nil, err
	}
	return d, nil
}

func (db *DB) CreateDashboard(projectId ProjectId, name, description string) (string, error) {
	id := utils.NanoId(8)
	_, err := db.db.Exec("INSERT INTO dashboards (project_id, id, name, description) VALUES ($1, $2, $3, $4)", projectId, id, name, description)
	return id, err
}

func (db *DB) UpdateDashboard(projectId ProjectId, id, name, description string) error {
	_, err := db.db.Exec("UPDATE dashboards SET name = $1, description=$2 WHERE project_id = $3 AND id = $4", name, description, projectId, id)
	return err
}

func (db *DB) SaveDashboardConfig(projectId ProjectId, id string, config DashboardConfig) error {
	cfg, err := json.Marshal(config)
	if err != nil {
		return err
	}
	_, err = db.db.Exec("UPDATE dashboards SET config = $1 WHERE project_id = $2 AND id = $3", string(cfg), projectId, id)
	return err
}

func (db *DB) DeleteDashboard(projectId ProjectId, id string) error {
	_, err := db.db.Exec("DELETE FROM dashboards WHERE project_id = $1 AND id = $2", projectId, id)
	return err
}
