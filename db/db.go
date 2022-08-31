package db

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	db *sql.DB
}

type Table interface {
	Migrate(db *sql.DB) error
}

func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=rwc", path))
	if err != nil {
		return nil, err
	}
	for _, o := range []Table{
		&Project{},
		&PrometheusQueryState{},
	} {
		err := o.Migrate(db)
		if err != nil {
			return nil, err
		}
	}
	return &DB{db: db}, nil
}
