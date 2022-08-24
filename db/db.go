package db

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
)

type PrometheusQueryState struct {
	Query     string
	LastTs    int64
	LastError string
}

type DB struct {
	db *sql.DB
}

func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=rwc", path))
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS prometheus_query_state (
    	query TEXT NOT NULL PRIMARY KEY,
    	last_ts INTEGER NOT NULL,
    	last_error TEXT NOT NULL
  	)`)
	if err != nil {
		return nil, err
	}
	return &DB{db: db}, nil
}

func (db *DB) SaveState(state *PrometheusQueryState) error {
	_, err := db.db.Exec(
		"INSERT OR REPLACE INTO prometheus_query_state (query, last_ts, last_error) values ($1, $2, $3)",
		state.Query, state.LastTs, state.LastError)
	return err
}

func (db *DB) LoadStates() (map[string]*PrometheusQueryState, error) {
	res := map[string]*PrometheusQueryState{}
	rows, err := db.db.Query("SELECT query, last_ts, last_error FROM  prometheus_query_state")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		qs := &PrometheusQueryState{}
		if err = rows.Scan(&qs.Query, &qs.LastTs, &qs.LastError); err != nil {
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
