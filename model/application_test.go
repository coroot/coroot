package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplication_GetInstance(t *testing.T) {
	app := NewApplication(NewApplicationId("cluster", "default", "Cluster", "database"))
	node1 := NewNode(NodeId{Name: "node-1"})
	node2 := NewNode(NodeId{Name: "node-2"})

	// Create first pod instance on node-1
	instance1 := app.GetOrCreateInstance("database-1", node1)
	instance1.Pod = &Pod{OwnerKind: "Cluster"}
	assert.Equal(t, "node-1", instance1.NodeName())

	// Getting instance with node-1 should return instance1
	assert.Equal(t, instance1, app.GetInstance("database-1", "node-1"))

	// Getting instance with node-2 should not return instance1 when instance has Pod metadata
	assert.Nil(t, app.GetInstance("database-1", "node-2"))

	// Creating instance for the same pod name on node-2 should create a new distinct instance
	instance2 := app.GetOrCreateInstance("database-1", node2)
	instance2.Pod = &Pod{OwnerKind: "Cluster"}
	assert.NotEqual(t, instance1, instance2)
	assert.Equal(t, "node-2", instance2.NodeName())

	// Subsequent lookups by node return the correct incarnation
	assert.Equal(t, instance1, app.GetInstance("database-1", "node-1"))
	assert.Equal(t, instance2, app.GetInstance("database-1", "node-2"))

	// Lookup without node returns the first instance
	assert.Equal(t, instance1, app.GetInstance("database-1", ""))
}
