package db

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/coroot/coroot/model"
	"k8s.io/klog"
)

type ApplicationSettings struct{}

func (s *ApplicationSettings) Migrate(m *Migrator) error {
	return m.Exec(`
	CREATE TABLE IF NOT EXISTS application_settings (
		project_id TEXT NOT NULL REFERENCES project(id),
		application_id TEXT NOT NULL,
		settings TEXT NOT NULL,
		PRIMARY KEY (project_id, application_id)
	)`)
}

func (db *DB) GetApplicationSettings(projectId ProjectId, appId model.ApplicationId) (*model.ApplicationSettings, error) {
	var settings sql.NullString
	err := db.db.QueryRow(
		"SELECT settings FROM application_settings WHERE project_id = $1 AND application_id = $2",
		projectId, appId.String(),
	).Scan(&settings)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	var res *model.ApplicationSettings
	if err = unmarshal(settings.String, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (db *DB) SaveApplicationSetting(projectId ProjectId, appId model.ApplicationId, s any) error {
	return db.withTx(func(tx *sql.Tx) error {
		// make sure the row exists so that it can be locked: concurrent saves of different settings must not overwrite each other
		_, err := tx.Exec(
			"INSERT INTO application_settings (project_id, application_id, settings) VALUES ($1, $2, '{}') ON CONFLICT (project_id, application_id) DO NOTHING",
			projectId, appId.String())
		if err != nil {
			return err
		}
		var settings string
		err = tx.QueryRow(
			"SELECT settings FROM application_settings WHERE project_id = $1 AND application_id = $2"+db.forUpdate(),
			projectId, appId.String(),
		).Scan(&settings)
		if err != nil {
			return err
		}
		var as *model.ApplicationSettings
		if err = unmarshal(settings, &as); err != nil {
			return err
		}
		if as == nil {
			as = &model.ApplicationSettings{}
		}
		switch v := s.(type) {
		case *model.ApplicationSettingsProfiling:
			as.Profiling = v
		case *model.ApplicationSettingsTracing:
			as.Tracing = v
		case *model.ApplicationSettingsLogs:
			as.Logs = v
		case *model.ApplicationInstrumentation:
			if as.Instrumentation == nil {
				as.Instrumentation = map[model.ApplicationType]*model.ApplicationInstrumentation{}
			}
			as.Instrumentation[v.Type] = v
		case []model.RiskOverride:
			as.RiskOverrides = v
		default:
			return fmt.Errorf("unsupported type: %T", s)
		}
		data, err := marshal(as)
		if err != nil {
			return err
		}
		_, err = tx.Exec(
			"UPDATE application_settings SET settings = $1 WHERE project_id = $2 AND application_id = $3",
			data, projectId, appId.String())
		return err
	})
}

func (db *DB) GetApplicationSettingsByProject(projectId ProjectId) (map[model.ApplicationId]*model.ApplicationSettings, error) {
	rows, err := db.db.Query("SELECT application_id, settings FROM application_settings WHERE project_id = $1", projectId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := map[model.ApplicationId]*model.ApplicationSettings{}
	var appId model.ApplicationId
	var appIdStr sql.NullString
	var settingsStr sql.NullString
	for rows.Next() {
		err = rows.Scan(&appIdStr, &settingsStr)
		if err != nil {
			return nil, err
		}
		if !settingsStr.Valid {
			continue
		}
		appId, err = model.NewApplicationIdFromString(appIdStr.String, string(projectId))
		if err != nil {
			klog.Warningln(err)
			continue
		}
		var settings *model.ApplicationSettings
		err = unmarshal(settingsStr.String, &settings)
		if err != nil {
			klog.Warningln(err)
			continue
		}
		res[appId] = settings
	}
	return res, rows.Err()
}
