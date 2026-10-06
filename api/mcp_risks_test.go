package api

import (
	"testing"

	"github.com/coroot/coroot/api/views/overview"
	"github.com/coroot/coroot/model"
	"github.com/stretchr/testify/assert"
)

func TestMCPFilterRisks(t *testing.T) {
	app := model.NewApplicationId("cluster", "default", model.ApplicationKindDeployment, "app")
	other := model.NewApplicationId("cluster", "default", model.ApplicationKindDeployment, "other")
	active := &overview.Risk{ApplicationId: app, Severity: model.WARNING}
	dismissed := &overview.Risk{ApplicationId: app, Severity: model.OK, Dismissal: &model.RiskDismissal{Reason: "reviewed"}}
	hidden := &overview.Risk{ApplicationId: other, Severity: model.CRITICAL}
	risks := []*overview.Risk{active, dismissed, hidden}
	visible := func(r *overview.Risk) bool { return r != hidden }

	assert.Equal(t, []*overview.Risk{active}, mcpFilterRisks(risks, "active", nil, visible))
	assert.Equal(t, []*overview.Risk{dismissed}, mcpFilterRisks(risks, "dismissed", nil, visible))
	assert.Equal(t, []*overview.Risk{active, dismissed}, mcpFilterRisks(risks, "any", nil, visible))
	assert.Equal(t, []*overview.Risk{active}, mcpFilterRisks(risks, "active", &app, visible))
	assert.Empty(t, mcpFilterRisks(risks, "active", &other, visible))
}
