package db

import (
	"database/sql"
	"encoding/json"
	"github.com/coroot/coroot/utils"
)

type ProjectId string

type Project struct {
	Id   ProjectId
	Name string

	Prometheus Prometheus
}

func (p *Project) Migrate(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS project (
		id TEXT NOT NULL PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		prometheus TEXT
	)`)
	return err
}

func (db *DB) GetProjects() ([]Project, error) {
	rows, err := db.db.Query("SELECT id, name, prometheus FROM project")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Project
	var p Project
	var prometheus string
	for rows.Next() {
		if err := rows.Scan(&p.Id, &p.Name, &prometheus); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(prometheus), &p.Prometheus); err != nil {
			return nil, err
		}
		if p.Prometheus.RefreshInterval == 0 {
			p.Prometheus.RefreshInterval = DefaultRefreshInterval
		}
		res = append(res, p)
	}
	return res, nil
}

func (db *DB) GetProject(id ProjectId) (*Project, error) {
	p := Project{Id: id}
	var prometheus string
	if err := db.db.QueryRow("SELECT name, prometheus FROM project WHERE id = $1", id).Scan(&p.Name, &prometheus); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(prometheus), &p.Prometheus); err != nil {
		return nil, err
	}
	if p.Prometheus.RefreshInterval == 0 {
		p.Prometheus.RefreshInterval = DefaultRefreshInterval
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
		return p.Id, err
	}
	if _, err := db.db.Exec("UPDATE project SET name = $1, prometheus = $2 WHERE id = $3", p.Name, string(prometheus), p.Id); err != nil {
		return "", err
	}
	return p.Id, err
}
