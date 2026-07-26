package api

import (
	"testing"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/rbac"
	"github.com/stretchr/testify/assert"
)

func TestUserMenuHandoffVsAdmin(t *testing.T) {
	projectId := db.ProjectId("proj1")
	projects := map[db.ProjectId]string{projectId: "shared"}

	handoffRole := rbac.RoleName("kubero-twenty-production")
	api := &Api{roles: staticRoles{
		rbac.NewRole(handoffRole,
			rbac.NewPermission(rbac.ScopeApplication, rbac.ActionView, rbac.Object{
				"project_id":            string(projectId),
				"application_namespace": "twenty-production",
				"application_kind":      "*",
				"application_name":      "*",
			}),
		),
	}}
	handoff := &db.User{Roles: []rbac.RoleName{handoffRole}}
	m := api.userMenu(handoff, projects)
	assert.False(t, m.Settings)
	assert.False(t, m.Project)
	assert.False(t, m.Nodes)
	assert.False(t, m.Kubernetes)
	assert.False(t, m.Costs)
	assert.False(t, m.AlertingRules)

	// Dedicated handoff also has node view.
	api.roles = staticRoles{
		rbac.NewRole(handoffRole,
			rbac.NewPermission(rbac.ScopeApplication, rbac.ActionView, rbac.Object{
				"project_id":            string(projectId),
				"application_namespace": "twenty-production",
				"application_kind":      "*",
				"application_name":      "*",
			}),
			rbac.NewPermission(rbac.ScopeNode, rbac.ActionView, rbac.Object{
				"project_id": string(projectId),
				"node_name":  "*",
			}),
		),
	}
	m = api.userMenu(handoff, projects)
	assert.True(t, m.Nodes)
	assert.False(t, m.Kubernetes)
	assert.False(t, m.Costs)

	api.roles = staticRoles(rbac.Roles)
	admin := &db.User{Roles: []rbac.RoleName{rbac.RoleAdmin}}
	m = api.userMenu(admin, projects)
	assert.True(t, m.Settings)
	assert.True(t, m.Project)
	assert.True(t, m.Nodes)
	assert.True(t, m.Kubernetes)
	assert.True(t, m.Costs)
	assert.True(t, m.AlertingRules)

	viewer := &db.User{Roles: []rbac.RoleName{rbac.RoleViewer}}
	m = api.userMenu(viewer, projects)
	assert.True(t, m.AlertingRules)
}
