package cache

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/timeseries"
	_ "github.com/mattn/go-sqlite3"
	"os"
	"path"
)

type PrometheusQueryState struct {
	ProjectId db.ProjectId
	Query     string
	LastTs    timeseries.Time
	LastError string
}

func (p *PrometheusQueryState) Migrate(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS prometheus_query_state (
		project_id TEXT NOT NULL,
		query TEXT NOT NULL,
		last_ts INTEGER NOT NULL,
		last_error TEXT NOT NULL,
		PRIMARY KEY(project_id, query)
	)`)
	return err
}

func openStateDB(path string) (*sql.DB, error) {
	database, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=rwc", path))
	if err != nil {
		return nil, err
	}
	database.SetMaxOpenConns(1)
	if err := db.Migrate(database, &PrometheusQueryState{}); err != nil {
		return nil, err
	}
	return database, nil
}

func (c *Cache) saveState(state *PrometheusQueryState) error {
	_, err := c.state.Exec(
		"INSERT OR REPLACE INTO prometheus_query_state (project_id, query, last_ts, last_error) values ($1, $2, $3, $4)",
		state.ProjectId, state.Query, state.LastTs, state.LastError)
	return err
}

func (c *Cache) loadStates(projectId db.ProjectId) (map[string]*PrometheusQueryState, error) {
	res := map[string]*PrometheusQueryState{}
	rows, err := c.state.Query("SELECT project_id, query, last_ts, last_error FROM prometheus_query_state WHERE project_id = $1", projectId)
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

func (c *Cache) deleteState(state *PrometheusQueryState) error {
	_, err := c.state.Exec("DELETE FROM prometheus_query_state WHERE project_id = $1 AND query = $2", state.ProjectId, state.Query)
	return err
}

func (c *Cache) deleteProject(projectId db.ProjectId) error {
	projectDir := path.Join(c.cfg.Path, string(projectId))
	if err := os.RemoveAll(projectDir); err != nil {
		return err
	}
	if _, err := c.state.Exec("DELETE FROM prometheus_query_state WHERE project_id = $1", projectId); err != nil {
		return err
	}
	return nil
}

func (c *Cache) GetUpdateTime(projectId db.ProjectId) (timeseries.Time, error) {
	var res timeseries.Time
	err := c.state.QueryRow("SELECT last_ts FROM prometheus_query_state WHERE project_id = $1 ORDER BY last_ts LIMIT 1", projectId).Scan(&res)
	if err == nil {
		return res, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return res, nil
	}
	return res, err
}

func (c *Cache) GetError(projectId db.ProjectId) (string, error) {
	var res string
	err := c.state.QueryRow("SELECT last_error FROM prometheus_query_state WHERE project_id = $1 AND last_error != '' LIMIT 1", projectId).Scan(&res)
	if err == nil {
		return res, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return "", err
}
