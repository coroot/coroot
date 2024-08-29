package forms

import (
	"strings"

	"github.com/coroot/coroot/rbac"
)

type LoginForm struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Action   string `json:"action"`
}

func (f *LoginForm) Valid() bool {
	return f.Email != "" && f.Password != ""
}

type ChangePasswordForm struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

func (f *ChangePasswordForm) Valid() bool {
	return f.OldPassword != "" && f.NewPassword != ""
}

type UserAction string

const (
	UserActionCreate UserAction = "create"
	UserActionUpdate UserAction = "update"
	UserActionDelete UserAction = "delete"
)

type UserForm struct {
	Action   UserAction    `json:"action"`
	Id       int           `json:"id"`
	Email    string        `json:"email"`
	Name     string        `json:"name"`
	Role     rbac.RoleName `json:"role"`
	Password string        `json:"password"`
}

func (f *UserForm) Valid() bool {
	if f.Action == UserActionDelete {
		return true
	}
	f.Email = strings.TrimSpace(f.Email)
	f.Name = strings.TrimSpace(f.Name)
	return f.Email != "" && f.Name != ""
}
