package db

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"k8s.io/klog"
	"path"
)

var (
	ErrConflict = errors.New("conflict")
)

type DB struct {
	db *sql.DB
}

func Open(dir string, pgConnString string) (*DB, error) {
	var db *sql.DB
	var err error
	if pgConnString != "" {
		klog.Infoln("using postgres database")
		db, err = postgres(pgConnString)
	} else {
		klog.Infoln("using sqlite database")
		db, err = sqlite(path.Join(dir, "db.sqlite"))
	}
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := Migrate(db, &Project{}); err != nil {
		return nil, err
	}
	return &DB{db: db}, nil
}

func sqlite(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=rwc", path))
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, err
	}
	return db, nil
}

func postgres(dsn string) (*sql.DB, error) {
	return sql.Open("postgres", dsn)
}

type Table interface {
	Migrate(db *sql.DB) error
}

func Migrate(db *sql.DB, tables ...Table) error {
	for _, o := range tables {
		err := o.Migrate(db)
		if err != nil {
			return err
		}
	}
	return nil
}
