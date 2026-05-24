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
}

func TestNewApplicationIdReplicaSet(t *testing.T) {
	id := NewApplicationId("cluster", "default", ApplicationKindReplicaSet, "catalog-6799fc88d8")
	assert.Equal(t, ApplicationId{ClusterId: "cluster", Kind: ApplicationKindDeployment, Name: "catalog", Namespace: "default"}, id)

	id = NewApplicationId("cluster", "default", ApplicationKindReplicaSet, "catalog-blue")
	assert.Equal(t, ApplicationId{ClusterId: "cluster", Kind: ApplicationKindReplicaSet, Name: "catalog-blue", Namespace: "default"}, id)

	id = NewApplicationId("cluster", "default", ApplicationKindReplicaSet, "catalog-a")
	assert.Equal(t, ApplicationId{ClusterId: "cluster", Kind: ApplicationKindReplicaSet, Name: "catalog-a", Namespace: "default"}, id)

	id = NewApplicationId("cluster", "default", ApplicationKindReplicaSet, "abcdef1234")
	assert.Equal(t, ApplicationId{ClusterId: "cluster", Kind: ApplicationKindReplicaSet, Name: "abcdef1234", Namespace: "default"}, id)
}

func TestNewApplicationIdJob(t *testing.T) {
	id := NewApplicationId("cluster", "default", ApplicationKindJob, "backup-1716480000")
	assert.Equal(t, ApplicationId{ClusterId: "cluster", Kind: ApplicationKindCronJob, Name: "backup", Namespace: "default"}, id)

	id = NewApplicationId("cluster", "default", ApplicationKindJob, "db-migration-1")
	assert.Equal(t, ApplicationId{ClusterId: "cluster", Kind: ApplicationKindJob, Name: "db-migration-1", Namespace: "default"}, id)

	id = NewApplicationId("cluster", "default", ApplicationKindJob, "1716480000")
	assert.Equal(t, ApplicationId{ClusterId: "cluster", Kind: ApplicationKindJob, Name: "1716480000", Namespace: "default"}, id)
}
