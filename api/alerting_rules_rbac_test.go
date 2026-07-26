package api

import (
	"testing"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/rbac"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplicationIdPatternConfined(t *testing.T) {
	ns := []string{"twenty-production", "twenty-staging"}
	assert.True(t, applicationIdPatternConfined("twenty-production:*:*", ns))
	assert.True(t, applicationIdPatternConfined("twenty-production:Deployment:web", ns))
	assert.True(t, applicationIdPatternConfined("twenty-staging:Deployment:*", ns))
	assert.False(t, applicationIdPatternConfined("other-production:*:*", ns))
	assert.False(t, applicationIdPatternConfined("*:*:*", ns))
	assert.False(t, applicationIdPatternConfined("twenty-*:*:*", ns))
	assert.False(t, applicationIdPatternConfined("twenty-production", ns))
	assert.False(t, applicationIdPatternConfined("", ns))
}

func TestFilterAndValidateAlertingRulesForRestrictedUser(t *testing.T) {
	projectId := "proj1"
	roleName := rbac.RoleName("kubero-twenty-production")
	roles := staticRoles{
		rbac.NewRole(roleName,
			rbac.NewPermission(rbac.ScopeApplication, rbac.ActionView, rbac.Object{
				"project_id":            projectId,
				"application_namespace": "twenty-production",
				"application_kind":      "*",
				"application_name":      "*",
			}),
			rbac.NewPermission(rbac.ScopeProjectAlertingRules, rbac.ActionEdit, rbac.Object{
				"project_id": projectId,
			}),
			rbac.NewPermission(rbac.ScopeProjectAlertingRules, rbac.ActionView, rbac.Object{
				"project_id": projectId,
			}),
		),
	}
	api := &Api{roles: roles}
	user := &db.User{Roles: []rbac.RoleName{roleName}}

	allowedId := model.NewApplicationId(projectId, "twenty-production", model.ApplicationKindDeployment, "twenty-kuberoapp-web")
	deniedId := model.NewApplicationId(projectId, "other-production", model.ApplicationKindDeployment, "other-kuberoapp-web")
	w := &model.World{Applications: map[model.ApplicationId]*model.Application{
		allowedId: model.NewApplication(allowedId),
		deniedId:  model.NewApplication(deniedId),
	}}

	namespaces, restricted := api.restrictedNamespaces(user, projectId, w)
	require.True(t, restricted)
	assert.Equal(t, []string{"twenty-production"}, namespaces)

	own := &model.AlertingRule{
		Id:        "own1",
		ProjectId: projectId,
		Name:      "own",
		Selector: model.AppSelector{
			Type:                  model.AppSelectorTypeApplications,
			ApplicationIdPatterns: []string{"twenty-production:*:*"},
		},
	}
	other := &model.AlertingRule{
		Id:        "other1",
		ProjectId: projectId,
		Name:      "other",
		Selector: model.AppSelector{
			Type:                  model.AppSelectorTypeApplications,
			ApplicationIdPatterns: []string{"other-production:*:*"},
		},
	}
	allApps := &model.AlertingRule{
		Id:        "all1",
		ProjectId: projectId,
		Name:      "all",
		Selector:  model.AppSelector{Type: model.AppSelectorTypeAll},
	}
	builtin := &model.AlertingRule{
		Id:        "builtin1",
		ProjectId: projectId,
		Name:      "builtin",
		Builtin:   true,
		Selector: model.AppSelector{
			Type:                  model.AppSelectorTypeApplications,
			ApplicationIdPatterns: []string{"twenty-production:*:*"},
		},
	}

	assert.True(t, api.canAccessAlertingRule(user, projectId, w, own))
	assert.False(t, api.canAccessAlertingRule(user, projectId, w, other))
	assert.False(t, api.canAccessAlertingRule(user, projectId, w, allApps))
	assert.False(t, api.canAccessAlertingRule(user, projectId, w, builtin))

	filtered := api.filterAlertingRules(user, projectId, w, []*model.AlertingRule{own, other, allApps, builtin})
	require.Len(t, filtered, 1)
	assert.Equal(t, model.AlertingRuleId("own1"), filtered[0].Id)

	// Create validation rejects cross-namespace / all / promql.
	err := api.validateAlertingRuleForUser(user, projectId, w, &model.AlertingRule{
		Source:   model.AlertSource{Type: model.AlertSourceTypeCheck},
		Selector: model.AppSelector{Type: model.AppSelectorTypeAll},
	})
	assert.Error(t, err)

	err = api.validateAlertingRuleForUser(user, projectId, w, &model.AlertingRule{
		Source: model.AlertSource{Type: model.AlertSourceTypePromQL},
		Selector: model.AppSelector{
			Type:                  model.AppSelectorTypeApplications,
			ApplicationIdPatterns: []string{"twenty-production:*:*"},
		},
	})
	assert.Error(t, err)

	ok := &model.AlertingRule{
		Source: model.AlertSource{Type: model.AlertSourceTypeCheck},
		Selector: model.AppSelector{
			Type:                  model.AppSelectorTypeApplications,
			ApplicationIdPatterns: []string{"twenty-production:Deployment:twenty-kuberoapp-web"},
		},
		NotificationCategory: "application",
	}
	require.NoError(t, api.validateAlertingRuleForUser(user, projectId, w, ok))
	assert.Equal(t, model.ApplicationCategory(""), ok.NotificationCategory)

	// Unrestricted viewer still sees everything.
	api.roles = staticRoles(rbac.Roles)
	viewer := &db.User{Roles: []rbac.RoleName{rbac.RoleViewer}}
	_, restricted = api.restrictedNamespaces(viewer, projectId, w)
	assert.False(t, restricted)
	assert.True(t, api.canAccessAlertingRule(viewer, projectId, w, allApps))
	assert.Len(t, api.filterAlertingRules(viewer, projectId, w, []*model.AlertingRule{own, other, allApps, builtin}), 4)
}
