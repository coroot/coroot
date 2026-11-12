package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplicationGetInstance_StatefulSet(t *testing.T) {
	appId := NewApplicationId("cluster", "default", ApplicationKindStatefulSet, "redis")
	app := NewApplication(appId)

	node1 := NewNode("cluster", NewNodeId("node-1", "node-1"))
	node1.Name = LabelLastValue{v: "node-1"}
	node2 := NewNode("cluster", NewNodeId("node-2", "node-2"))
	node2.Name = LabelLastValue{v: "node-2"}

	inst1 := app.GetOrCreateInstance("redis-0", node1)
	assert.NotNil(t, inst1)
	assert.Equal(t, "node-1", inst1.NodeName())

	// Looking up with same node returns the existing instance
	assert.Equal(t, inst1, app.GetInstance("redis-0", "node-1"))

	// Looking up with empty node returns the existing instance
	assert.Equal(t, inst1, app.GetInstance("redis-0", ""))

	// Looking up with different node returns nil before it is created
	assert.Nil(t, app.GetInstance("redis-0", "node-2"))

	// Creating instance on node-2 creates a distinct instance
	inst2 := app.GetOrCreateInstance("redis-0", node2)
	assert.NotNil(t, inst2)
	assert.NotEqual(t, inst1, inst2)
	assert.Equal(t, "node-2", inst2.NodeName())

	// Retrieval with node names returns the matching instances
	assert.Equal(t, inst1, app.GetInstance("redis-0", "node-1"))
	assert.Equal(t, inst2, app.GetInstance("redis-0", "node-2"))
}

func TestApplicationGetInstance_OperatorManagedPod(t *testing.T) {
	// Operator-managed pod where kind is a CRD (e.g. Cluster, PostgresCluster, ClickHouseInstallation)
	appId := NewApplicationId("cluster", "default", ApplicationKind("Cluster"), "database")
	app := NewApplication(appId)

	node1 := NewNode("cluster", NewNodeId("node-1", "node-1"))
	node1.Name = LabelLastValue{v: "node-1"}
	node2 := NewNode("cluster", NewNodeId("node-2", "node-2"))
	node2.Name = LabelLastValue{v: "node-2"}

	// First pod incarnation on node-1
	inst1 := app.GetOrCreateInstance("database-1", node1)
	inst1.Pod = &Pod{OwnerKind: ApplicationKind("Cluster")}

	// Looking up with same node returns inst1
	assert.Equal(t, inst1, app.GetInstance("database-1", "node-1"))

	// Looking up on node-2 should NOT match inst1 on node-1
	assert.Nil(t, app.GetInstance("database-1", "node-2"))

	// Recreating the pod on node-2 should create a new instance
	inst2 := app.GetOrCreateInstance("database-1", node2)
	inst2.Pod = &Pod{OwnerKind: ApplicationKind("Cluster")}
	assert.NotEqual(t, inst1, inst2)

	// Now both instances can be retrieved by their respective nodes
	assert.Equal(t, inst1, app.GetInstance("database-1", "node-1"))
	assert.Equal(t, inst2, app.GetInstance("database-1", "node-2"))
}

func TestApplicationGetInstance_NonPod(t *testing.T) {
	// Non-pod application (e.g. RDS, ExternalService)
	appId := NewApplicationId("cluster", "default", ApplicationKindExternalService, "external-api")
	app := NewApplication(appId)

	inst := app.GetOrCreateInstance("endpoint-1", nil)
	assert.NotNil(t, inst)

	// In non-pod applications, GetInstance returns the instance regardless of node parameter
	assert.Equal(t, inst, app.GetInstance("endpoint-1", ""))
	assert.Equal(t, inst, app.GetInstance("endpoint-1", "node-1"))
	assert.Equal(t, inst, app.GetInstance("endpoint-1", "node-2"))
}
