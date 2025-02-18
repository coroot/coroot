package rbac

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPermissions(t *testing.T) {
	p := NewPermission(ScopeProjectAll, ActionView, nil)
	assert.True(t, p.allows(Actions.Project("*").Node("*").View()))
	assert.True(t, p.allows(Actions.Project("foo").Node("foo").View()))
	assert.True(t, p.allows(Actions.Project("bar").Node("bar").View()))

	p = NewPermission(ScopeProjectAll, ActionView, Object{"project_id": "foo"})
	assert.True(t, p.allows(Actions.Project("*").Node("*").View()))
	assert.True(t, p.allows(Actions.Project("foo").Node("foo").View()))
	assert.False(t, p.allows(Actions.Project("bar").Node("foo").View()))

	p = NewPermission(ScopeNode, ActionView, nil)
	assert.True(t, p.allows(Actions.Project("*").Node("*").View()))
	assert.True(t, p.allows(Actions.Project("foo").Node("foo").View()))
	assert.True(t, p.allows(Actions.Project("bar").Node("bar").View()))

	p = NewPermission(ScopeNode, ActionView, Object{"node_name": "*"})
	assert.True(t, p.allows(Actions.Project("*").Node("*").View()))
	assert.True(t, p.allows(Actions.Project("foo").Node("foo").View()))
	assert.True(t, p.allows(Actions.Project("bar").Node("bar").View()))

	p = NewPermission(ScopeNode, ActionView, Object{"node_name": "foo*"})
	assert.True(t, p.allows(Actions.Project("*").Node("*").View()))
	assert.True(t, p.allows(Actions.Project("foo").Node("foo").View()))
	assert.True(t, p.allows(Actions.Project("foo").Node("foobar").View()))
	assert.False(t, p.allows(Actions.Project("bar").Node("bar").View()))

	p = NewPermission(ScopeNode, ActionView, Object{"node_name": ""})
	assert.False(t, p.allows(Actions.Project("*").Node("*").View()))
	assert.False(t, p.allows(Actions.Project("foo").Node("foo").View()))
	assert.False(t, p.allows(Actions.Project("bar").Node("bar").View()))

	p = NewPermission(ScopeApplication, ActionView, nil)
	assert.False(t, p.allows(Actions.Project("*").Node("*").View()))
	assert.False(t, p.allows(Actions.Project("foo").Node("foo").View()))
	assert.False(t, p.allows(Actions.Project("bar").Node("bar").View()))
}
