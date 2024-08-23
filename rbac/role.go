package rbac

import (
	"slices"
)

const (
	RoleAdmin  RoleName = "Admin"
	RoleEditor RoleName = "Editor"
	RoleViewer RoleName = "Viewer"
)

var (
	Roles = []Role{
		NewRole(RoleAdmin,
			NewPermission(ScopeAll, ActionAll, nil),
		),
		NewRole(RoleEditor,
			NewPermission(ScopeAll, ActionView, nil),
			NewPermission(ScopeProjectApplicationCategories, ActionEdit, nil),
			NewPermission(ScopeProjectCustomApplications, ActionEdit, nil),
			NewPermission(ScopeProjectInspections, ActionEdit, nil),
		),
		NewRole(RoleViewer,
			NewPermission(ScopeAll, ActionView, nil),
		),
	}
)

type RoleName string

func (r RoleName) Valid(roles []Role) bool {
	return slices.ContainsFunc(roles, func(role Role) bool { return role.Name == r })
}

func (r RoleName) Builtin() bool {
	return r.Valid(Roles)
}

type Role struct {
	Name        RoleName      `json:"name"`
	Permissions PermissionSet `json:"permissions"`
}

func NewRole(name RoleName, permissions ...Permission) Role {
	return Role{Name: name, Permissions: permissions}
}

type RoleManager interface {
	GetRoles() ([]Role, error)
}

type StaticRoleManager struct{}

func NewStaticRoleManager() *StaticRoleManager {
	return &StaticRoleManager{}
}

func (mgr *StaticRoleManager) GetRoles() ([]Role, error) {
	return Roles, nil
}
