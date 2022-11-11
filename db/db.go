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

type Type string

const (
	TypeSqlite   Type = "sqlite"
	TypePostgres Type = "postgres"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

type DB struct {
	typ Type
	db  *sql.DB
}

func Open(dataDir string, pgConnString string) (*DB, error) {
	var db *sql.DB
	var err error
	var typ Type
	if pgConnString != "" {
		klog.Infoln("using postgres database")
		typ = TypePostgres
		db, err = postgres(pgConnString)
	} else {
		klog.Infoln("using sqlite database")
		typ = TypeSqlite
		db, err = sqlite(path.Join(dataDir, "db.sqlite"))
	}
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := NewMigrator(typ, db).Migrate(&Project{}, &CheckConfigs{}, &Incident{}); err != nil {
		return nil, err
	}
	return &DB{typ: typ, db: db}, nil
}

func (db *DB) Type() Type {
	return db.typ
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
	Migrate(m *Migrator) error
}

type Migrator struct {
	typ Type
	db  *sql.DB
}

func NewMigrator(t Type, db *sql.DB) *Migrator {
	return &Migrator{typ: t, db: db}
}

func (m *Migrator) Migrate(tables ...Table) error {
	for _, o := range tables {
		err := o.Migrate(m)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Migrator) Exec(query string, args ...any) error {
	_, err := m.db.Exec(query, args...)
	return err
}

func (m *Migrator) AddColumnIfNotExists(table, column, dataType string) error {
	switch m.typ {
	case TypeSqlite:
		rows, err := m.db.Query("SELECT name FROM pragma_table_info('project');")
		if err != nil {
			return nil
		}
		defer rows.Close()
		var name string
		for rows.Next() {
			if err := rows.Scan(&name); err != nil {
				return err
			}
			if name == column {
				return nil
			}
		}
		_, err = m.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, dataType))
		if err != nil {
			return err
		}
	case TypePostgres:
		_, err := m.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s %s", table, column, dataType))
		if err != nil {
			return err
		}
	}
	return nil
}
