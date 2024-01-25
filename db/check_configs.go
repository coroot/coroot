package db

import (
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/coroot/coroot/model"
	"k8s.io/klog"
)

type CheckConfigs struct{}

func (cc *CheckConfigs) Migrate(m *Migrator) error {
	return m.Exec(`
	CREATE TABLE IF NOT EXISTS check_configs (
		project_id TEXT NOT NULL REFERENCES project(id),
		application_id TEXT NOT NULL,
		configs TEXT,
		PRIMARY KEY (project_id, application_id)
	)`)
}

func (db *DB) GetCheckConfigs(projectId ProjectId) (model.CheckConfigs, error) {
	rows, err := db.db.Query("SELECT application_id, configs FROM check_configs WHERE project_id = $1", projectId)
	if err != nil {
		return nil, err
	}
	var appId sql.NullString
	var configs sql.NullString
	res := model.CheckConfigs{}
	for rows.Next() {
		if err := rows.Scan(&appId, &configs); err != nil {
			return nil, err
		}
		id, err := model.NewApplicationIdFromString(appId.String)
		if err != nil {
			klog.Warningln(err)
			continue
		}
		if !configs.Valid {
			continue
		}
		var cs map[model.CheckId]json.RawMessage
		if err := json.Unmarshal([]byte(configs.String), &cs); err != nil {
			return nil, err
		}
		res[id] = cs
	}
	return res, nil
}

func (db *DB) SaveCheckConfig(projectId ProjectId, appId model.ApplicationId, checkId model.CheckId, cfg any) error {
	appIdStr := appId.String()
	var configs sql.NullString
	err := db.db.QueryRow("SELECT configs FROM check_configs WHERE project_id = $1 AND application_id = $2", projectId, appIdStr).Scan(&configs)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	cs := map[model.CheckId]json.RawMessage{}
	if configs.Valid {
		if err := json.Unmarshal([]byte(configs.String), &cs); err != nil {
			return err
		}
	}
	c, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if string(c) == "null" {
		delete(cs, checkId)
	} else {
		cs[checkId] = c
	}
	data, err := json.Marshal(cs)
	if err != nil {
		return err
	}
	res, err := db.db.Exec("UPDATE check_configs SET configs = $1 WHERE project_id = $2 AND application_id = $3", string(data), projectId, appIdStr)
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		if _, err := db.db.Exec("INSERT INTO check_configs (project_id, application_id, configs) VALUES ($1, $2, $3)", projectId, appIdStr, string(data)); err != nil {
			return err
		}
	}
	return err
}
