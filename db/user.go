package db

import (
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/coroot/coroot/rbac"
	"golang.org/x/crypto/bcrypt"
)

const (
	AdminUserLogin           = "admin"
	AdminUserName            = "Admin"
	AdminUserDefaultPassword = ""
	AnonymousUserName        = "Anonymous"
)

type User struct {
	Id        int
	Email     string
	Name      string
	Roles     []rbac.RoleName
	Anonymous bool
}

func (u *User) Migrate(m *Migrator) error {
	err := m.Exec(`
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		email TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		password TEXT NOT NULL,
		roles TEXT NOT NULL
	)`)
	if err != nil {
		return err
	}
	return nil
}

func (u *User) IsDefaultAdmin() bool {
	return u.Email == AdminUserLogin
}

func AnonymousUser(role rbac.RoleName) *User {
	return &User{Name: AnonymousUserName, Roles: []rbac.RoleName{role}, Anonymous: true}
}

func (db *DB) CreateAdminIfNotExists(password string) error {
	var i int
	err := db.db.QueryRow("SELECT 1 FROM users WHERE email = $1", AdminUserLogin).Scan(&i)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return db.AddUser(AdminUserLogin, password, AdminUserName, rbac.RoleAdmin)
}

func (db *DB) SetAdminPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.db.Exec("UPDATE users SET password = $1 WHERE email = $2", string(hash), AdminUserLogin)
	return err
}

func (db *DB) DefaultAdminUserIsTheOnlyUser() (*User, error) {
	rows, err := db.db.Query("SELECT id, email, name, roles, password FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var u User
	var hash, roles string
	for rows.Next() {
		if u.Email != "" { // second iteration
			return nil, nil
		}
		err = rows.Scan(&u.Id, &u.Email, &u.Name, &roles, &hash)
		if err != nil {
			return nil, err
		}
	}
	if !u.IsDefaultAdmin() {
		return nil, nil
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(AdminUserDefaultPassword)) != nil {
		return nil, nil
	}
	err = json.Unmarshal([]byte(roles), &u.Roles)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (db *DB) GetUsers() ([]*User, error) {
	rows, err := db.db.Query("SELECT id, email, name, roles FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []*User
	for rows.Next() {
		var u User
		var roles string
		err = rows.Scan(&u.Id, &u.Email, &u.Name, &roles)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal([]byte(roles), &u.Roles)
		if err != nil {
			return nil, err
		}
		res = append(res, &u)
	}
	return res, nil
}

func (db *DB) AuthUser(email, password string) (int, error) {
	var id int
	var hash string
	err := db.db.QueryRow("SELECT id, password FROM users WHERE email = $1", email).Scan(&id, &hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return 0, ErrNotFound
	}
	return id, nil
}

func (db *DB) GetUser(id int) (*User, error) {
	u := User{Id: id}
	var roles string
	err := db.db.QueryRow("SELECT email, name, roles FROM users WHERE id = $1", id).Scan(&u.Email, &u.Name, &roles)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	err = json.Unmarshal([]byte(roles), &u.Roles)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (db *DB) AddUser(email, password, name string, role rbac.RoleName) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	roles, err := json.Marshal([]rbac.RoleName{role})
	if err != nil {
		return err
	}
	_, err = db.db.Exec("INSERT INTO users(email, name, password, roles) VALUES($1, $2, $3, $4)", email, name, string(hash), string(roles))
	if db.IsUniqueViolationError(err) {
		return ErrConflict
	}
	return err
}

func (db *DB) UpdateUser(id int, email, password, name string, role rbac.RoleName) error {
	roles, err := json.Marshal([]rbac.RoleName{role})
	if err != nil {
		return err
	}
	if password != "" {
		var hash []byte
		hash, err = bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		_, err = db.db.Exec("UPDATE users SET email=$1, name=$2, password=$3, roles = $4 WHERE id = $5", email, name, string(hash), string(roles), id)
	} else {
		_, err = db.db.Exec("UPDATE users SET email=$1, name=$2, roles = $3 WHERE id = $4", email, name, string(roles), id)
	}
	return err
}

func (db *DB) ChangeUserPassword(id int, oldPassword, newPassword string) error {
	var hash []byte
	err := db.db.QueryRow("SELECT password FROM users WHERE id = $1", id).Scan(&hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	if bcrypt.CompareHashAndPassword(hash, []byte(oldPassword)) != nil {
		return ErrInvalid
	}

	if oldPassword == newPassword {
		return ErrConflict
	}

	hash, err = bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.db.Exec("UPDATE users SET password=$1 WHERE id = $2", string(hash), id)
	return err
}

func (db *DB) DeleteUser(id int) error {
	_, err := db.db.Exec("DELETE FROM users WHERE id = $1", id)
	return err
}
