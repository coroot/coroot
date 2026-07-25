package db

import (
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/coroot/coroot/rbac"
)

type CustomRole struct {
	Name        rbac.RoleName
	Permissions rbac.PermissionSet
}

func (r *CustomRole) Migrate(m *Migrator) error {
	return m.Exec(`
	CREATE TABLE IF NOT EXISTS roles (
		name TEXT NOT NULL PRIMARY KEY,
		permissions TEXT NOT NULL
	)`)
}

func (db *DB) GetCustomRoles() ([]rbac.Role, error) {
	rows, err := db.db.Query("SELECT name, permissions FROM roles")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []rbac.Role
	for rows.Next() {
		var name string
		var perms string
		if err = rows.Scan(&name, &perms); err != nil {
			return nil, err
		}
		var permissions rbac.PermissionSet
		if err = json.Unmarshal([]byte(perms), &permissions); err != nil {
			return nil, err
		}
		res = append(res, rbac.NewRole(rbac.RoleName(name), permissions...))
	}
	return res, nil
}

func (db *DB) UpsertCustomRole(name rbac.RoleName, permissions rbac.PermissionSet) error {
	if name == "" {
		return ErrInvalid
	}
	if name.Builtin() {
		return ErrConflict
	}
	data, err := json.Marshal(permissions)
	if err != nil {
		return err
	}
	res, err := db.db.Exec("UPDATE roles SET permissions = $1 WHERE name = $2", string(data), string(name))
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
	_, err = db.db.Exec("INSERT INTO roles(name, permissions) VALUES($1, $2)", string(name), string(data))
	return err
}

func (db *DB) DeleteCustomRole(name rbac.RoleName) error {
	if name.Builtin() {
		return ErrConflict
	}
	res, err := db.db.Exec("DELETE FROM roles WHERE name = $1", string(name))
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (db *DB) GetCustomRole(name rbac.RoleName) (*rbac.Role, error) {
	var perms string
	err := db.db.QueryRow("SELECT permissions FROM roles WHERE name = $1", string(name)).Scan(&perms)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var permissions rbac.PermissionSet
	if err = json.Unmarshal([]byte(perms), &permissions); err != nil {
		return nil, err
	}
	role := rbac.NewRole(name, permissions...)
	return &role, nil
}
