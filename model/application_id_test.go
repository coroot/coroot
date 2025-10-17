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
