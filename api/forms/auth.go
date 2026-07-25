package forms

import (
	"encoding/json"
	"fmt"
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

type HandoffCreateForm struct {
	Email       string             `json:"email"`
	Name        string             `json:"name"`
	Role        rbac.RoleName      `json:"role"`
	Redirect    string             `json:"redirect"`
	TTLSeconds  int                `json:"ttl_seconds"`
	Permissions rbac.PermissionSet `json:"permissions"`
}

func (f *HandoffCreateForm) Valid() bool {
	f.Email = strings.TrimSpace(f.Email)
	f.Name = strings.TrimSpace(f.Name)
	return f.Email != ""
}

type RoleAction string

const (
	RoleActionAdd    RoleAction = "add"
	RoleActionEdit   RoleAction = "edit"
	RoleActionDelete RoleAction = "delete"
)

type RoleFormPermission struct {
	Scope  rbac.Scope `json:"scope"`
	Action rbac.Verb  `json:"action"`
	Object any        `json:"object"`
}

type RoleForm struct {
	Action      RoleAction           `json:"action"`
	Id          rbac.RoleName        `json:"id"`
	Name        rbac.RoleName        `json:"name"`
	Permissions []RoleFormPermission `json:"permissions"`
}

func (f *RoleForm) Valid() bool {
	f.Name = rbac.RoleName(strings.TrimSpace(string(f.Name)))
	switch f.Action {
	case RoleActionDelete:
		return f.Id != "" || f.Name != ""
	case RoleActionAdd, RoleActionEdit:
		return f.Name != ""
	default:
		return false
	}
}

func (f *RoleForm) PermissionSet() (rbac.PermissionSet, error) {
	var out rbac.PermissionSet
	for _, p := range f.Permissions {
		if p.Scope == "" || p.Action == "" {
			return nil, fmt.Errorf("permission scope and action are required")
		}
		obj, err := parsePermissionObject(p.Object)
		if err != nil {
			return nil, err
		}
		out = append(out, rbac.NewPermission(p.Scope, p.Action, obj))
	}
	return out, nil
}

func parsePermissionObject(v any) (rbac.Object, error) {
	if v == nil {
		return nil, nil
	}
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		if s == "" || s == "*" {
			return nil, nil
		}
		var obj rbac.Object
		if err := json.Unmarshal([]byte(s), &obj); err != nil {
			return nil, fmt.Errorf("invalid permission object: %w", err)
		}
		return obj, nil
	case map[string]any:
		obj := rbac.Object{}
		for k, val := range t {
			obj[k] = fmt.Sprint(val)
		}
		return obj, nil
	case map[string]string:
		return rbac.Object(t), nil
	default:
		data, err := json.Marshal(t)
		if err != nil {
			return nil, err
		}
		var obj rbac.Object
		if err := json.Unmarshal(data, &obj); err != nil {
			return nil, fmt.Errorf("invalid permission object: %w", err)
		}
		return obj, nil
	}
}
