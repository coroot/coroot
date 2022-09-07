package db

import (
	"database/sql"
	"errors"
	"github.com/coroot/coroot/timeseries"
)

const (
	DefaultRefreshInterval = 30
)

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

type PrometheusQueryState struct {
	ProjectId ProjectId
	Query     string
	LastTs    timeseries.Time
	LastError string
}

func (p *PrometheusQueryState) Migrate(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS prometheus_query_state (
		project_id TEXT NOT NULL REFERENCES project(id),
		query TEXT NOT NULL,
		last_ts INTEGER NOT NULL,
		last_error TEXT NOT NULL,
		PRIMARY KEY(project_id, query)
	)`)
	return err
}

func (db *DB) SaveState(state *PrometheusQueryState) error {
	_, err := db.db.Exec(
		"INSERT OR REPLACE INTO prometheus_query_state (project_id, query, last_ts, last_error) values ($1, $2, $3, $4)",
		state.ProjectId, state.Query, state.LastTs, state.LastError)
	return err
}

func (db *DB) LoadStates(id ProjectId) (map[string]*PrometheusQueryState, error) {
	res := map[string]*PrometheusQueryState{}
	rows, err := db.db.Query("SELECT project_id, query, last_ts, last_error FROM prometheus_query_state WHERE project_id = $1", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		qs := &PrometheusQueryState{}
		if err = rows.Scan(&qs.ProjectId, &qs.Query, &qs.LastTs, &qs.LastError); err != nil {
			return nil, err
		}
		res[qs.Query] = qs
	}
	return res, nil
}

func (db *DB) DeleteState(state *PrometheusQueryState) error {
	_, err := db.db.Exec("DELETE FROM prometheus_query_state WHERE project_id = $1 AND query = $2", state.ProjectId, state.Query)
	return err
}

func (db *DB) GetCacheUpdateTime(id ProjectId) (timeseries.Time, error) {
	var res timeseries.Time
	err := db.db.QueryRow("SELECT last_ts FROM prometheus_query_state WHERE project_id = $1 ORDER BY last_ts LIMIT 1", id).Scan(&res)
	if err == nil {
		return res, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return res, nil
	}
	return res, err
}

func (db *DB) GetCacheError(id ProjectId) (string, error) {
	var res string
	err := db.db.QueryRow("SELECT last_error FROM prometheus_query_state WHERE project_id = $1 AND last_error != '' LIMIT 1", id).Scan(&res)
	if err == nil {
		return res, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return "", err
}
