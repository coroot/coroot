package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/lib/pq"
	"github.com/mattn/go-sqlite3"
	"strings"
)

const (
	DefaultRefreshInterval = 30
)

type ProjectId string

type Project struct {
	Id   ProjectId
	Name string

	Prometheus Prometheus
	Settings   Settings
}

type Prometheus struct {
	Url             string              `json:"url"`
	RefreshInterval timeseries.Duration `json:"refresh_interval"`
	TlsSkipVerify   bool                `json:"tls_skip_verify"`
	BasicAuth       *BasicAuth          `json:"basic_auth"`
}

type Settings struct {
	ConfigurationHintsMuted map[model.ApplicationType]bool         `json:"configuration_hints_muted"`
	ApplicationCategories   map[model.ApplicationCategory][]string `json:"application_categories"`
	Integrations            Integrations                           `json:"integrations"`
}

type BasicAuth struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

func (p *Project) Migrate(m *Migrator) error {
	err := m.Exec(`
	CREATE TABLE IF NOT EXISTS project (
		id TEXT NOT NULL PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		prometheus TEXT
	)`)
	if err != nil {
		return err
	}
	if err := m.AddColumnIfNotExists("project", "settings", "text"); err != nil {
		return err
	}
	return nil
}

func (db *DB) GetProjects() ([]*Project, error) {
	rows, err := db.db.Query("SELECT id, name, prometheus, settings FROM project")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []*Project
	var prometheus string
	var settings sql.NullString
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.Id, &p.Name, &prometheus, &settings); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(prometheus), &p.Prometheus); err != nil {
			return nil, err
		}
		if p.Prometheus.RefreshInterval == 0 {
			p.Prometheus.RefreshInterval = DefaultRefreshInterval
		}
		if settings.Valid {
			if err := json.Unmarshal([]byte(settings.String), &p.Settings); err != nil {
				return nil, err
			}
		}
		res = append(res, &p)
	}
	return res, nil
}

func (db *DB) GetProjectNames() (map[ProjectId]string, error) {
	rows, err := db.db.Query("SELECT id, name FROM project")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := map[ProjectId]string{}
	var id ProjectId
	var name string
	for rows.Next() {
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		res[id] = name
	}
	return res, nil
}

func (db *DB) GetProject(id ProjectId) (*Project, error) {
	p := Project{Id: id}
	var prometheus string
	var settings sql.NullString
	if err := db.db.QueryRow("SELECT name, prometheus, settings FROM project WHERE id = $1", id).Scan(&p.Name, &prometheus, &settings); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if err := json.Unmarshal([]byte(prometheus), &p.Prometheus); err != nil {
		return nil, err
	}
	if p.Prometheus.RefreshInterval == 0 {
		p.Prometheus.RefreshInterval = DefaultRefreshInterval
	}
	if settings.Valid {
		if err := json.Unmarshal([]byte(settings.String), &p.Settings); err != nil {
			return nil, err
		}
	}
	return &p, nil
}

func (db *DB) SaveProject(p Project) (ProjectId, error) {
	if p.Prometheus.RefreshInterval == 0 {
		p.Prometheus.RefreshInterval = DefaultRefreshInterval
	}
	prometheus, err := json.Marshal(p.Prometheus)
	if err != nil {
		return "", err
	}
	if p.Id == "" {
		p.Id = ProjectId(utils.NanoId(8))
		_, err := db.db.Exec("INSERT INTO project (id, name, prometheus) VALUES ($1, $2, $3)", p.Id, p.Name, string(prometheus))
		if e, ok := err.(sqlite3.Error); ok && e.Code == sqlite3.ErrConstraint {
			return "", ErrConflict
		}
		if e, ok := err.(*pq.Error); ok && e.Code.Name() == "unique_violation" {
			return "", ErrConflict
		}
		return p.Id, err
	}
	if _, err := db.db.Exec("UPDATE project SET name = $1, prometheus = $2 WHERE id = $3", p.Name, string(prometheus), p.Id); err != nil {
		return "", err
	}
	return p.Id, err
}

func (db *DB) DeleteProject(id ProjectId) error {
	tx, err := db.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM check_configs WHERE project_id = $1", id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM incident WHERE project_id = $1", id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM project WHERE id = $1", id); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) ToggleConfigurationHint(id ProjectId, appType model.ApplicationType, mute bool) error {
	p, err := db.GetProject(id)
	if err != nil {
		return err
	}
	if mute {
		if p.Settings.ConfigurationHintsMuted == nil {
			p.Settings.ConfigurationHintsMuted = map[model.ApplicationType]bool{}
		}
		p.Settings.ConfigurationHintsMuted[appType] = true
	} else {
		delete(p.Settings.ConfigurationHintsMuted, appType)
	}
	return db.saveProjectSettings(p)
}

func (db *DB) SaveApplicationCategory(id ProjectId, name, newName model.ApplicationCategory, customPatterns []string) error {
	p, err := db.GetProject(id)
	if err != nil {
		return err
	}

	var ps []string
	for _, p := range customPatterns {
		p = strings.TrimSpace(p)
		if len(p) == 0 {
			continue
		}
		ps = append(ps, p)
	}

	if len(ps) == 0 { // delete
		if p.Settings.ApplicationCategories == nil {
			return nil
		}
		delete(p.Settings.ApplicationCategories, name)
		return db.saveProjectSettings(p)
	}

	if p.Settings.ApplicationCategories == nil {
		p.Settings.ApplicationCategories = map[model.ApplicationCategory][]string{}
	}
	if name != newName && name != model.ApplicationCategoryControlPlane && name != model.ApplicationCategoryMonitoring { // rename
		delete(p.Settings.ApplicationCategories, name)
		name = newName
	}
	p.Settings.ApplicationCategories[name] = ps
	return db.saveProjectSettings(p)
}

func (db *DB) saveProjectSettings(p *Project) error {
	settings, err := json.Marshal(p.Settings)
	if err != nil {
		return err
	}
	_, err = db.db.Exec("UPDATE project SET settings = $1 WHERE id = $2", settings, p.Id)
	return err
}
