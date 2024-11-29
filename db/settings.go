package db

import (
	"database/sql"
	"encoding/json"
	"errors"
)

type Setting struct {
	Name  string
	Value string
}

func (s *Setting) Migrate(m *Migrator) error {
	err := m.Exec(`
	CREATE TABLE IF NOT EXISTS settings (
		name TEXT NOT NULL PRIMARY KEY,
		value TEXT NOT NULL
	)`)
	if err != nil {
		return err
	}
	return nil
}

func (db *DB) GetSetting(name string, value any) error {
	var v string
	err := db.db.QueryRow("SELECT value FROM settings WHERE name = $1", name).Scan(&v)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	err = json.Unmarshal([]byte(v), value)
	if err != nil {
		return err
	}
	return nil
}

func (db *DB) SetSetting(name string, value any) error {
	v, err := json.Marshal(value)
	if err != nil {
		return err
	}
	res, err := db.db.Exec("UPDATE settings SET value = $1 WHERE name = $2", v, name)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	_, err = db.db.Exec("INSERT INTO settings(name, value) VALUES ($1, $2)", name, v)
	return err
}
