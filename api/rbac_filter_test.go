package api

import (
	"testing"

	"github.com/coroot/coroot/api/views/overview"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/rbac"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type staticRoles []rbac.Role

func (r staticRoles) GetRoles() ([]rbac.Role, error) { return r, nil }

func TestRenderSearchFiltersByApplicationPermission(t *testing.T) {
	projectId := "proj1"
	roleName := rbac.RoleName("kubero-twenty-production")
	roles := staticRoles{
		rbac.NewRole(roleName,
			rbac.NewPermission(rbac.ScopeApplication, rbac.ActionView, rbac.Object{
				"project_id":             projectId,
				"application_namespace":  "twenty-production",
				"application_kind":       "*",
				"application_name":       "*",
			}),
		),
	}
	api := &Api{roles: roles}
	user := &db.User{Email: "kubero+u1+twenty-production@handoff.local", Roles: []rbac.RoleName{roleName}}

	allowedId := model.NewApplicationId(projectId, "twenty-production", model.ApplicationKindDeployment, "twenty-kuberoapp-web")
	deniedId := model.NewApplicationId(projectId, "other-production", model.ApplicationKindDeployment, "other-kuberoapp-web")
	allowed := model.NewApplication(allowedId)
	denied := model.NewApplication(deniedId)
	w := &model.World{Applications: map[model.ApplicationId]*model.Application{
		allowed.Id: allowed,
		denied.Id:  denied,
	}}

	search := api.renderSearch(user, projectId, w)
	require.Len(t, search.Applications, 1)
	assert.Equal(t, allowed.Id, search.Applications[0].Id)

	assert.True(t, api.canViewApplication(user, projectId, allowed))
	assert.False(t, api.canViewApplication(user, projectId, denied))

	ov := &overview.Overview{
		Applications: []*overview.ApplicationStatus{
			{Id: allowed.Id},
			{Id: denied.Id},
		},
		Map: []*overview.Application{
			{Id: allowed.Id},
			{Id: denied.Id},
		},
	}
	ov = api.filterOverviewForUser(user, projectId, w, ov)
	require.Len(t, ov.Applications, 1)
	assert.Equal(t, allowed.Id, ov.Applications[0].Id)
	require.Len(t, ov.Map, 1)
	assert.Equal(t, allowed.Id, ov.Map[0].Id)

	assert.True(t, api.hasRestrictedAppAccess(user, projectId, w))
	restricted := api.worldWithViewableApps(user, projectId, w)
	require.Len(t, restricted.Applications, 1)
	assert.NotNil(t, restricted.Applications[allowed.Id])

	// Builtin Viewer still sees everything.
	viewer := &db.User{Roles: []rbac.RoleName{rbac.RoleViewer}}
	api.roles = staticRoles(rbac.Roles)
	search = api.renderSearch(viewer, projectId, w)
	assert.Len(t, search.Applications, 2)
	assert.False(t, api.hasRestrictedAppAccess(viewer, projectId, w))
}
