package db

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/coroot/coroot/model"
)

type ApplicationSettings struct {
	Profiling *ApplicationSettingsProfiling `json:"profiling,omitempty"`
	Tracing   *ApplicationSettingsTracing   `json:"tracing,omitempty"`
	Logs      *ApplicationSettingsLogs      `json:"logs,omitempty"`
}

func (s *ApplicationSettings) Migrate(m *Migrator) error {
	return m.Exec(`
	CREATE TABLE IF NOT EXISTS application_settings (
		project_id TEXT NOT NULL REFERENCES project(id),
		application_id TEXT NOT NULL,
		settings TEXT NOT NULL,
		PRIMARY KEY (project_id, application_id)
	)`)
}

type ApplicationSettingsProfiling struct {
	Service string `json:"service"`
}

type ApplicationSettingsTracing struct {
	Service string `json:"service"`
}

type ApplicationSettingsLogs struct {
	Service string `json:"service"`
}

func (db *DB) GetApplicationSettings(projectId ProjectId, appId model.ApplicationId) (*ApplicationSettings, error) {
	var settings sql.NullString
	err := db.db.QueryRow(
		"SELECT settings FROM application_settings WHERE project_id = $1 AND application_id = $2",
		projectId, appId.String(),
	).Scan(&settings)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	var res *ApplicationSettings
	if err := unmarshal(settings.String, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (db *DB) SaveApplicationSetting(projectId ProjectId, appId model.ApplicationId, s any) error {
	as, err := db.GetApplicationSettings(projectId, appId)
	if err != nil {
		return err
	}
	insert := false
	if as == nil {
		insert = true
		as = &ApplicationSettings{}
	}
	switch v := s.(type) {
	case *ApplicationSettingsProfiling:
		as.Profiling = v
	case *ApplicationSettingsTracing:
		as.Tracing = v
	case *ApplicationSettingsLogs:
		as.Logs = v
	default:
		return fmt.Errorf("unsupported type: %T", s)
	}
	settings, err := marshal(as)
	if err != nil {
		return err
	}
	if insert {
		_, err = db.db.Exec(
			"INSERT INTO application_settings (project_id, application_id, settings) VALUES ($1, $2, $3)",
			projectId, appId.String(), settings)
	} else {
		_, err = db.db.Exec(
			"UPDATE application_settings SET settings = $1 WHERE project_id = $2 AND application_id = $3",
			settings, projectId, appId.String())
	}
	return err
}
