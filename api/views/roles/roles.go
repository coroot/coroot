package roles

import (
	"fmt"
	"slices"

	"github.com/coroot/coroot/rbac"
)

type View struct {
	Roles   []Role   `json:"roles"`
	Actions []Action `json:"actions"`
	Scopes  []Scope  `json:"scopes"`
}

type Role struct {
	Name        rbac.RoleName      `json:"name"`
	Permissions rbac.PermissionSet `json:"permissions"`
	Custom      bool               `json:"custom"`
}

type Action struct {
	Name  string  `json:"name"`
	Roles []ARole `json:"roles"`
}

type ARole struct {
	Name    rbac.RoleName `json:"name"`
	Objects []rbac.Object `json:"objects"`
}

type Scope struct {
	Name    rbac.Scope  `json:"name"`
	Actions []rbac.Verb `json:"actions"`
}

func Render(roles []rbac.Role) *View {
	v := &View{}
	for _, r := range roles {
		v.Roles = append(v.Roles, Role{
			Name:        r.Name,
			Custom:      !r.Name.Builtin(),
			Permissions: r.Permissions,
		})
	}

	actions := []rbac.Verb{rbac.ActionAll, rbac.ActionEdit, rbac.ActionView}
	v.Scopes = append(v.Scopes, Scope{Name: rbac.ScopeAll, Actions: actions})
	v.Scopes = append(v.Scopes, Scope{Name: rbac.ScopeProjectAll, Actions: actions})
	for _, action := range rbac.Actions.List() {
		a := Action{Name: fmt.Sprintf("%s:%s", action.Scope, action.Action)}
		for _, role := range v.Roles {
			a.Roles = append(a.Roles, ARole{Name: role.Name, Objects: role.Permissions.AllowsForObjects(action)})
		}
		v.Actions = append(v.Actions, a)

		s := slices.IndexFunc(v.Scopes, func(scope Scope) bool {
			return scope.Name == action.Scope
		})
		if s == -1 {
			v.Scopes = append(v.Scopes, Scope{Name: action.Scope})
			s = len(v.Scopes) - 1
		}
		v.Scopes[s].Actions = append(v.Scopes[s].Actions, action.Action)
	}

	return v
}
