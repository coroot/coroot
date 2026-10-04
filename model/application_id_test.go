package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplicationId(t *testing.T) {
	id, _ := NewApplicationIdFromString("1elggi7o:coroot:Deployment:coroot-cluster-agent", "fallback")
	assert.Equal(t, ApplicationId{ClusterId: "1elggi7o", Kind: ApplicationKindDeployment, Name: "coroot-cluster-agent", Namespace: "coroot"}, id)

	id, _ = NewApplicationIdFromString("coroot:Deployment:coroot-cluster-agent", "fallback")
	assert.Equal(t, ApplicationId{ClusterId: "fallback", Kind: ApplicationKindDeployment, Name: "coroot-cluster-agent", Namespace: "coroot"}, id)

	id, _ = NewApplicationIdFromString("external:external:ExternalService:external:30001", "fallback")
	assert.Equal(t, ApplicationId{ClusterId: "external", Kind: ApplicationKindExternalService, Name: "external:30001", Namespace: "external"}, id)

	id, _ = NewApplicationIdFromString("external:ExternalService:external:30001", "fallback")
	assert.Equal(t, ApplicationId{ClusterId: "external", Kind: ApplicationKindExternalService, Name: "external:30001", Namespace: "external"}, id)

	id, _ = NewApplicationIdFromString("external:frontend:RumClient:demo-web", "fallback")
	assert.Equal(t, ApplicationId{ClusterId: "external", Kind: ApplicationKindRumClient, Name: "demo-web", Namespace: "frontend"}, id)
	assert.Equal(t, NewApplicationId(ClusterIdExternal, "frontend", ApplicationKindRumClient, "demo-web"), id)
}

func TestApplicationIdRumClientRoundtrip(t *testing.T) {
	id := NewApplicationId(ClusterIdExternal, "frontend", ApplicationKindRumClient, "demo-web")
	parsed, err := NewApplicationIdFromString(id.String(), "fallback")
	assert.NoError(t, err)
	assert.Equal(t, id, parsed)
}

func TestNewApplicationIdReplicaSet(t *testing.T) {
	id := NewApplicationId("cluster", "default", ApplicationKindReplicaSet, "catalog-6799fc88d8")
	assert.Equal(t, ApplicationId{ClusterId: "cluster", Kind: ApplicationKindDeployment, Name: "catalog", Namespace: "default"}, id)

	id = NewApplicationId("cluster", "default", ApplicationKindReplicaSet, "catalog-blue")
	assert.Equal(t, ApplicationId{ClusterId: "cluster", Kind: ApplicationKindReplicaSet, Name: "catalog-blue", Namespace: "default"}, id)
}
