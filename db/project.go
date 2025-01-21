package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/utils"
)

const (
	DefaultRefreshInterval = 30
)

type ProjectId string

type Project struct {
	Id   ProjectId
	Name string

	Prometheus IntegrationPrometheus
	Settings   ProjectSettings
}

type ProjectSettings struct {
	ApplicationCategories       map[model.ApplicationCategory][]string                    `json:"application_categories"`
	ApplicationCategorySettings map[model.ApplicationCategory]ApplicationCategorySettings `json:"application_category_settings"`
	Integrations                Integrations                                              `json:"integrations"`
	CustomApplications          map[string]model.CustomApplication                        `json:"custom_applications"`
	ApiKeys                     []ApiKey                                                  `json:"api_keys"`
	Configurable                bool                                                      `json:"configurable"`
}

type ApplicationCategorySettings struct {
	NotifyOfDeployments bool `json:"notify_of_deployments"`
}

type ApiKey struct {
	Key         string `json:"key"`
	Description string `json:"description"`
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

func (p *Project) applyDefaults() {
	if p.Prometheus.RefreshInterval == 0 {
		p.Prometheus.RefreshInterval = DefaultRefreshInterval
	}
	if _, ok := p.Settings.ApplicationCategorySettings[model.ApplicationCategoryApplication]; !ok {
		if p.Settings.ApplicationCategorySettings == nil {
			p.Settings.ApplicationCategorySettings = map[model.ApplicationCategory]ApplicationCategorySettings{}
		}
		p.Settings.ApplicationCategorySettings[model.ApplicationCategoryApplication] = ApplicationCategorySettings{NotifyOfDeployments: true}
	}
	if cfg := p.Settings.Integrations.Slack; cfg != nil {
		if !cfg.Incidents {
			cfg.Incidents = cfg.Enabled
		}
		if !cfg.Deployments {
			cfg.Deployments = cfg.Enabled
		}
	}
}

func (p *Project) GetCustomApplicationName(instance string) string {
	for customAppName, cfg := range p.Settings.CustomApplications {
		if utils.GlobMatch(instance, cfg.InstancePattens...) {
			return customAppName
		}
	}
	return ""
}

func (p *Project) PrometheusConfig(globalPrometheus *IntegrationPrometheus) *IntegrationPrometheus {
	if globalPrometheus != nil {
		gp := *globalPrometheus
		gp.ExtraSelector = fmt.Sprintf(`{coroot_project_id="%s"}`, p.Id)
		gp.ExtraLabels = map[string]string{"coroot_project_id": string(p.Id)}
		return &gp
	}
	return &p.Prometheus
}

func (p *Project) ClickHouseConfig(globalClickHouse *IntegrationClickhouse) *IntegrationClickhouse {
	if globalClickHouse != nil {
		gc := *globalClickHouse
		gc.Database = "coroot_" + string(p.Id)
		return &gc
	}
	return p.Settings.Integrations.Clickhouse
}

func (db *DB) GetProjects() ([]*Project, error) {
	rows, err := db.db.Query("SELECT id, name, prometheus, settings FROM project")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	var res []*Project
	var prometheus sql.NullString
	var settings sql.NullString
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.Id, &p.Name, &prometheus, &settings); err != nil {
			return nil, err
		}
		if prometheus.Valid {
			if err := json.Unmarshal([]byte(prometheus.String), &p.Prometheus); err != nil {
				return nil, err
			}
		}
		if settings.Valid {
			if err := json.Unmarshal([]byte(settings.String), &p.Settings); err != nil {
				return nil, err
			}
		}
		p.applyDefaults()
		res = append(res, &p)
	}
	return res, nil
}

func (db *DB) GetProjectNames() (map[ProjectId]string, error) {
	rows, err := db.db.Query("SELECT id, name FROM project")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
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
	var prometheus sql.NullString
	var settings sql.NullString
	if err := db.db.QueryRow("SELECT name, prometheus, settings FROM project WHERE id = $1", id).Scan(&p.Name, &prometheus, &settings); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if prometheus.Valid {
		if err := json.Unmarshal([]byte(prometheus.String), &p.Prometheus); err != nil {
			return nil, err
		}
	}
	if settings.Valid {
		if err := json.Unmarshal([]byte(settings.String), &p.Settings); err != nil {
			return nil, err
		}
	}
	p.applyDefaults()
	return &p, nil
}

func (db *DB) SaveProject(p *Project) error {
	if p.Prometheus.RefreshInterval == 0 {
		p.Prometheus.RefreshInterval = DefaultRefreshInterval
	}
	if p.Id == "" {
		p.Id = ProjectId(utils.NanoId(8))
		_, err := db.db.Exec("INSERT INTO project (id, name) VALUES ($1, $2)", p.Id, p.Name)
		if db.IsUniqueViolationError(err) {
			return ErrConflict
		}
		if err == nil {
			p.Settings.ApiKeys = []ApiKey{{Key: utils.RandomString(32), Description: "default"}}
			err = db.SaveProjectSettings(p)
		}
		return err
	}
	if _, err := db.db.Exec("UPDATE project SET name = $1 WHERE id = $2", p.Name, p.Id); err != nil {
		return err
	}
	return nil
}

func (db *DB) DeleteProject(id ProjectId) error {
	tx, err := db.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()
	if _, err := tx.Exec("DELETE FROM check_configs WHERE project_id = $1", id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM incident_notification WHERE project_id = $1", id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM incident WHERE project_id = $1", id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM application_deployment WHERE project_id = $1", id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM application_settings WHERE project_id = $1", id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM project WHERE id = $1", id); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) SaveProjectSettings(p *Project) error {
	settings, err := json.Marshal(p.Settings)
	if err != nil {
		return err
	}
	_, err = db.db.Exec("UPDATE project SET settings = $1 WHERE id = $2", string(settings), p.Id)
	return err
}

func (db *DB) SaveApplicationCategory(id ProjectId, category, newName model.ApplicationCategory, customPatterns []string, notifyAboutDeployments bool) error {
	p, err := db.GetProject(id)
	if err != nil {
		return err
	}

	if p.Settings.ApplicationCategorySettings[category].NotifyOfDeployments != notifyAboutDeployments {
		if p.Settings.ApplicationCategorySettings == nil {
			p.Settings.ApplicationCategorySettings = map[model.ApplicationCategory]ApplicationCategorySettings{}
		}
		p.Settings.ApplicationCategorySettings[category] = ApplicationCategorySettings{NotifyOfDeployments: notifyAboutDeployments}
	}

	if !category.Default() {
		var patterns []string
		for _, p := range customPatterns {
			p = strings.TrimSpace(p)
			if len(p) == 0 {
				continue
			}
			patterns = append(patterns, p)
		}

		if len(patterns) == 0 { // delete
			delete(p.Settings.ApplicationCategories, category)
			delete(p.Settings.ApplicationCategorySettings, category)
		} else {
			if p.Settings.ApplicationCategories == nil {
				p.Settings.ApplicationCategories = map[model.ApplicationCategory][]string{}
			}
			if category != newName && !category.Builtin() { // rename
				delete(p.Settings.ApplicationCategories, category)
				p.Settings.ApplicationCategorySettings[newName] = p.Settings.ApplicationCategorySettings[category]
				delete(p.Settings.ApplicationCategorySettings, category)
				category = newName
			}
			p.Settings.ApplicationCategories[category] = patterns
		}
	}

	return db.SaveProjectSettings(p)
}

func (db *DB) SaveCustomApplication(id ProjectId, name, newName string, instancePatterns []string) error {
	p, err := db.GetProject(id)
	if err != nil {
		return err
	}

	var patterns []string
	for _, p := range instancePatterns {
		p = strings.TrimSpace(p)
		if len(p) == 0 {
			continue
		}
		patterns = append(patterns, p)
	}
	if len(patterns) == 0 { // delete
		delete(p.Settings.CustomApplications, name)
	} else {
		if p.Settings.CustomApplications == nil {
			p.Settings.CustomApplications = map[string]model.CustomApplication{}
		}
		if name != newName { // rename
			delete(p.Settings.CustomApplications, name)
			p.Settings.CustomApplications[newName] = p.Settings.CustomApplications[name]
			delete(p.Settings.CustomApplications, name)
			name = newName
		}
		p.Settings.CustomApplications[name] = model.CustomApplication{InstancePattens: patterns}
	}
	return db.SaveProjectSettings(p)
}

func (db *DB) SaveProjectIntegration(p *Project, typ IntegrationType) error {
	if typ == IntegrationTypePrometheus {
		if p.Prometheus.RefreshInterval == 0 {
			p.Prometheus.RefreshInterval = DefaultRefreshInterval
		}
		prometheus, err := json.Marshal(p.Prometheus)
		if err != nil {
			return err
		}
		_, err = db.db.Exec("UPDATE project SET prometheus = $1 WHERE id = $2", string(prometheus), p.Id)
		return err
	}
	return db.SaveProjectSettings(p)
}
