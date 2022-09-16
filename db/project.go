package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/lib/pq"
	"github.com/mattn/go-sqlite3"
)

const (
	DefaultRefreshInterval = 30
)

type ProjectId string

type Project struct {
	Id   ProjectId
	Name string

	Prometheus Prometheus
}

type Prometheus struct {
	Url             string              `json:"url"`
	RefreshInterval timeseries.Duration `json:"refresh_interval"`
	TlsSkipVerify   bool                `json:"tls_skip_verify"`
	BasicAuth       *BasicAuth          `json:"basic_auth"`
}

type BasicAuth struct {
	User     string `json:"user"`
	Password string `json:"password"`
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

func (db *DB) GetProjects() ([]*Project, error) {
	rows, err := db.db.Query("SELECT id, name, prometheus FROM project")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []*Project
	var prometheus string
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.Id, &p.Name, &prometheus); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(prometheus), &p.Prometheus); err != nil {
			return nil, err
		}
		if p.Prometheus.RefreshInterval == 0 {
			p.Prometheus.RefreshInterval = DefaultRefreshInterval
		}
		res = append(res, &p)
	}
	return res, nil
}

func (db *DB) GetProjectIds() (map[ProjectId]bool, error) {
	rows, err := db.db.Query("SELECT id FROM project")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := map[ProjectId]bool{}
	var id ProjectId
	for rows.Next() {
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		res[id] = true
	}
	return res, nil
}

func (db *DB) GetProject(id ProjectId) (*Project, error) {
	p := Project{Id: id}
	var prometheus string
	if err := db.db.QueryRow("SELECT name, prometheus FROM project WHERE id = $1", id).Scan(&p.Name, &prometheus); err != nil {
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
	if _, err := db.db.Exec("DELETE FROM project WHERE id = $1", id); err != nil {
		return err
	}
	return nil
}
