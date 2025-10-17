package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckConfigs_getRaw(t *testing.T) {
	cluster := "cluster-a"
	configs := CheckConfigs{
		NewApplicationId(cluster, "default", ApplicationKindDeployment, "user-service"): {
			Checks.SLOAvailability.Id: json.RawMessage(`{"threshold": 95}`),
		},
		NewApplicationId(cluster, "default", ApplicationKindDeployment, "test-service"): {
			Checks.SLOAvailability.Id: json.RawMessage(`{"threshold": 96}`),
		},
		NewApplicationId(cluster, "default", ApplicationKindDeployment, "*-api"): {
			Checks.SLOAvailability.Id: json.RawMessage(`{"threshold": 98}`),
		},
		NewApplicationId(cluster, "default", ApplicationKindDeployment, "test-*"): {
			Checks.SLOAvailability.Id: json.RawMessage(`{"threshold": 90}`),
		},
		NewApplicationId(cluster, "production", "*", "*"): {
			Checks.SLOAvailability.Id: json.RawMessage(`{"threshold": 99.9}`),
		},
		NewApplicationId(cluster, "staging", ApplicationKindDeployment, "web-*"): {
			Checks.SLOLatency.Id: json.RawMessage(`{"threshold": 85}`),
		},
		ApplicationId{}: {
			Checks.SLOAvailability.Id: json.RawMessage(`{"threshold": 80}`),
			Checks.SLOLatency.Id:      json.RawMessage(`{"threshold": 75}`),
		},
	}

	raw, isDefault := configs.getRaw(
		ApplicationId{ClusterId: cluster, Namespace: "default", Kind: ApplicationKindDeployment, Name: "user-service"},
		Checks.SLOAvailability.Id,
	)
	assert.Equal(t, `{"threshold": 95}`, string(raw))
	assert.False(t, isDefault)

	raw, isDefault = configs.getRaw(
		ApplicationId{ClusterId: cluster, Namespace: "default", Kind: ApplicationKindDeployment, Name: "test-service"},
		Checks.SLOAvailability.Id,
	)
	assert.Equal(t, `{"threshold": 96}`, string(raw))
	assert.False(t, isDefault)

	raw, isDefault = configs.getRaw(
		ApplicationId{ClusterId: cluster, Namespace: "default", Kind: ApplicationKindDeployment, Name: "auth-api"},
		Checks.SLOAvailability.Id,
	)
	assert.Equal(t, `{"threshold": 98}`, string(raw))
	assert.False(t, isDefault)

	raw, isDefault = configs.getRaw(
		ApplicationId{ClusterId: cluster, Namespace: "production", Kind: ApplicationKindDeployment, Name: "payment-service"},
		Checks.SLOAvailability.Id,
	)
	assert.Equal(t, `{"threshold": 99.9}`, string(raw))
	assert.False(t, isDefault)

	raw, isDefault = configs.getRaw(
		ApplicationId{ClusterId: cluster, Namespace: "staging", Kind: ApplicationKindDeployment, Name: "web-frontend"},
		Checks.SLOLatency.Id,
	)
	assert.Equal(t, `{"threshold": 85}`, string(raw))
	assert.False(t, isDefault)

	raw, isDefault = configs.getRaw(
		ApplicationId{ClusterId: cluster, Namespace: "test", Kind: ApplicationKindDeployment, Name: "random-service"},
		Checks.SLOAvailability.Id,
	)
	assert.Equal(t, `{"threshold": 80}`, string(raw))
	assert.True(t, isDefault)

	raw, isDefault = configs.getRaw(
		ApplicationId{ClusterId: cluster, Namespace: "test", Kind: ApplicationKindDeployment, Name: "random-service"},
		Checks.CPUNode.Id,
	)
	assert.Nil(t, raw)
	assert.False(t, isDefault)

	raw, isDefault = configs.getRaw(
		ApplicationId{ClusterId: cluster, Namespace: "default", Kind: ApplicationKindDeployment, Name: "worker-service"},
		Checks.SLOAvailability.Id,
	)
	assert.Equal(t, `{"threshold": 80}`, string(raw))
	assert.True(t, isDefault)

	raw, isDefault = configs.getRaw(
		ApplicationId{ClusterId: cluster, Namespace: "production", Kind: ApplicationKindDeployment, Name: "payment-api"},
		Checks.SLOAvailability.Id,
	)
	assert.Equal(t, `{"threshold": 99.9}`, string(raw))
	assert.False(t, isDefault)

}
