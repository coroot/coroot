package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"github.com/mattn/go-sqlite3"
	_ "github.com/mattn/go-sqlite3"
)

type Type string

const (
	TypeSqlite   Type = "sqlite"
	TypePostgres Type = "postgres"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
	ErrInvalid  = errors.New("invalid")
)

type DB struct {
	typ Type
	db  *sql.DB

	primaryLockConn *sql.Conn
}

func NewSqlite(dataDir string) (*DB, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=rwc", path.Join(dataDir, "db.sqlite")))
	if err != nil {
		return nil, err
	}
	if _, err = db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return &DB{typ: TypeSqlite, db: db}, nil
}

func NewPostgres(dsn string) (*DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	return &DB{typ: TypePostgres, db: db}, nil
}

func (db *DB) Type() Type {
	return db.typ
}

func (db *DB) DB() *sql.DB {
	return db.db
}

func (db *DB) Migrator() *Migrator {
	return NewMigrator(db.typ, db.db)
}

func (db *DB) Migrate(extraTables ...Table) error {
	defaultTables := []Table{
		&Project{},
		&CheckConfigs{},
		&Incident{},
		&IncidentNotification{},
		&ApplicationDeployment{},
		&ApplicationSettings{},
		&Setting{},
		&User{},
	}
	return db.Migrator().Migrate(append(defaultTables, extraTables...)...)
}

func (db *DB) IsUniqueViolationError(err error) bool {
	switch db.typ {
	case TypePostgres:
		e, ok := err.(*pq.Error)
		return ok && e.Code.Name() == "unique_violation"
	case TypeSqlite:
		e, ok := err.(sqlite3.Error)
		return ok && e.Code == sqlite3.ErrConstraint
	}
	return false
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
	for _, t := range tables {
		err := t.Migrate(m)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Migrator) Exec(query string, args ...any) error {
	switch m.typ {
	case TypePostgres:
		query = strings.ReplaceAll(query, "INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT", "SERIAL PRIMARY KEY")
	}
	_, err := m.db.Exec(query, args...)
	return err
}

func (m *Migrator) AddColumnIfNotExists(table, column, dataType string) error {
	switch m.typ {
	case TypeSqlite:
		rows, err := m.db.Query(fmt.Sprintf("SELECT name FROM pragma_table_info('%s');", table))
		if err != nil {
			return nil
		}
		defer func() {
			_ = rows.Close()
		}()
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

func marshal[T any](v *T) (*string, error) {
	if v == nil {
		return nil, nil
	}
	d, err := json.Marshal(v)
	s := string(d)
	return &s, err
}

func unmarshal[T any](s string, v **T) error {
	if s == "" {
		return nil
	}
	var vv T
	*v = &vv
	return json.Unmarshal([]byte(s), &vv)
}
