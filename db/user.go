package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
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

	UserTypeServiceAccount = "service_account"
)

type User struct {
	Id        int
	Email     string
	Name      string
	Roles     []rbac.RoleName
	Type      string
	Anonymous bool
}

type UserApiKey struct {
	Id          int    `json:"id"`
	Description string `json:"description"`
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
	if err = m.AddColumnIfNotExists("users", "type", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	err = m.Exec(`
	CREATE TABLE IF NOT EXISTS user_api_keys (
		id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		hash TEXT NOT NULL UNIQUE,
		description TEXT NOT NULL
	)`)
	if err != nil {
		return err
	}
	return m.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS user_api_keys_user_description ON user_api_keys (user_id, description)`)
}

func (u *User) IsServiceAccount() bool {
	return u.Type == UserTypeServiceAccount
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
	rows, err := db.db.Query("SELECT id, email, name, roles, type FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []*User
	for rows.Next() {
		var u User
		var roles string
		err = rows.Scan(&u.Id, &u.Email, &u.Name, &roles, &u.Type)
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
	err := db.db.QueryRow("SELECT email, name, roles, type FROM users WHERE id = $1", id).Scan(&u.Email, &u.Name, &roles, &u.Type)
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
	if _, err := db.db.Exec("DELETE FROM user_api_keys WHERE user_id = $1", id); err != nil {
		return err
	}
	_, err := db.db.Exec("DELETE FROM users WHERE id = $1", id)
	return err
}

func (db *DB) AddServiceAccount(login, name string, role rbac.RoleName) (int, error) {
	roles, err := json.Marshal([]rbac.RoleName{role})
	if err != nil {
		return 0, err
	}
	res, err := db.db.Exec("INSERT INTO users(email, name, password, roles, type) VALUES($1, $2, '', $3, $4)", login, name, string(roles), UserTypeServiceAccount)
	if db.IsUniqueViolationError(err) {
		return 0, ErrConflict
	}
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil { // postgres doesn't support LastInsertId
		err = db.db.QueryRow("SELECT id FROM users WHERE email = $1", login).Scan(&id)
	}
	return int(id), err
}

func hashApiKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

func (db *DB) GetUserByApiKey(key string) (*User, error) {
	var u User
	var roles string
	err := db.db.QueryRow(
		"SELECT u.id, u.email, u.name, u.roles, u.type FROM users u JOIN user_api_keys k ON k.user_id = u.id WHERE k.hash = $1",
		hashApiKey(key),
	).Scan(&u.Id, &u.Email, &u.Name, &roles, &u.Type)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if err = json.Unmarshal([]byte(roles), &u.Roles); err != nil {
		return nil, err
	}
	return &u, nil
}

func (db *DB) GetUserApiKeys(userId int) ([]UserApiKey, error) {
	rows, err := db.db.Query("SELECT id, description FROM user_api_keys WHERE user_id = $1 ORDER BY id", userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := []UserApiKey{}
	for rows.Next() {
		var k UserApiKey
		if err = rows.Scan(&k.Id, &k.Description); err != nil {
			return nil, err
		}
		res = append(res, k)
	}
	return res, nil
}

func (db *DB) AddUserApiKey(userId int, key, description string) error {
	_, err := db.db.Exec("INSERT INTO user_api_keys(user_id, hash, description) VALUES($1, $2, $3)", userId, hashApiKey(key), description)
	if db.IsUniqueViolationError(err) {
		return ErrConflict
	}
	return err
}

func (db *DB) DeleteUserApiKey(userId, id int) error {
	_, err := db.db.Exec("DELETE FROM user_api_keys WHERE user_id = $1 AND id = $2", userId, id)
	return err
}

func (db *DB) SetUserApiKeys(userId int, keys []ApiKey) error {
	rows, err := db.db.Query("SELECT hash FROM user_api_keys WHERE user_id = $1", userId)
	if err != nil {
		return err
	}
	defer rows.Close()
	existing := map[string]bool{}
	for rows.Next() {
		var hash string
		if err = rows.Scan(&hash); err != nil {
			return err
		}
		existing[hash] = true
	}
	wanted := map[string]ApiKey{}
	for _, k := range keys {
		wanted[hashApiKey(k.Key)] = k
	}
	for hash := range existing {
		if _, ok := wanted[hash]; ok {
			continue
		}
		if _, err = db.db.Exec("DELETE FROM user_api_keys WHERE user_id = $1 AND hash = $2", userId, hash); err != nil {
			return err
		}
	}
	for hash, k := range wanted {
		if existing[hash] {
			continue
		}
		if err = db.AddUserApiKey(userId, k.Key, k.Description); err != nil {
			return err
		}
	}
	return nil
}
