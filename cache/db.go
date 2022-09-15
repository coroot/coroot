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

type Status struct {
	Error  string
	LagMax timeseries.Duration
	LagAvg timeseries.Duration
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

func (c *Cache) getMinUpdateTime(projectId db.ProjectId) (timeseries.Time, error) {
	var min sql.NullInt64
	err := c.state.QueryRow("SELECT min(last_ts) FROM prometheus_query_state WHERE project_id = $1", projectId).Scan(&min)
	if err != nil {
		return 0, err
	}
	return timeseries.Time(min.Int64), nil
}

func (c *Cache) getStatus(projectId db.ProjectId) (*Status, error) {
	var s Status
	err := c.state.QueryRow("SELECT last_error FROM prometheus_query_state WHERE project_id = $1 AND last_error != '' LIMIT 1", projectId).Scan(&s.Error)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	now := timeseries.Now()
	var max, avg sql.NullFloat64
	if err := c.state.QueryRow("SELECT max($1 - last_ts), avg($1 - last_ts) FROM prometheus_query_state WHERE project_id = $2", now, projectId).Scan(&max, &avg); err != nil {
		return nil, err
	}
	s.LagMax = timeseries.Duration(max.Float64)
	s.LagAvg = timeseries.Duration(avg.Float64)
	return &s, nil
}
