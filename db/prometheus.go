package db

import (
	"database/sql"
	"github.com/coroot/coroot-focus/timeseries"
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
	LastTs    int64
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
		"INSERT OR REPLACE INTO prometheus_query_state (query, last_ts, last_error) values ($1, $2, $3)",
		state.Query, state.LastTs, state.LastError)
	return err
}

func (db *DB) LoadStates() (map[string]*PrometheusQueryState, error) {
	res := map[string]*PrometheusQueryState{}
	rows, err := db.db.Query("SELECT project_id, query, last_ts, last_error FROM prometheus_query_state")
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
	_, err := db.db.Exec("DELETE FROM prometheus_query_state WHERE query = $1", state.Query)
	return err
}
