package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckConfigs_getRaw(t *testing.T) {
	configs := CheckConfigs{
		ApplicationId{Namespace: "default", Kind: "deployment", Name: "user-service"}: {
			Checks.SLOAvailability.Id: json.RawMessage(`{"threshold": 95}`),
		},
		ApplicationId{Namespace: "default", Kind: "deployment", Name: "test-service"}: {
			Checks.SLOAvailability.Id: json.RawMessage(`{"threshold": 96}`),
		},
		ApplicationId{Namespace: "default", Kind: "deployment", Name: "*-api"}: {
			Checks.SLOAvailability.Id: json.RawMessage(`{"threshold": 98}`),
		},
		ApplicationId{Namespace: "default", Kind: "deployment", Name: "test-*"}: {
			Checks.SLOAvailability.Id: json.RawMessage(`{"threshold": 90}`),
		},
		ApplicationId{Namespace: "production", Kind: "*", Name: "*"}: {
			Checks.SLOAvailability.Id: json.RawMessage(`{"threshold": 99.9}`),
		},
		ApplicationId{Namespace: "staging", Kind: "deployment", Name: "web-*"}: {
			Checks.SLOLatency.Id: json.RawMessage(`{"threshold": 85}`),
		},
		ApplicationId{}: {
			Checks.SLOAvailability.Id: json.RawMessage(`{"threshold": 80}`),
			Checks.SLOLatency.Id:      json.RawMessage(`{"threshold": 75}`),
		},
	}

	raw, isDefault := configs.getRaw(
		ApplicationId{Namespace: "default", Kind: "deployment", Name: "user-service"},
		Checks.SLOAvailability.Id,
	)
	assert.Equal(t, `{"threshold": 95}`, string(raw))
	assert.False(t, isDefault)

	raw, isDefault = configs.getRaw(
		ApplicationId{Namespace: "default", Kind: "deployment", Name: "test-service"},
		Checks.SLOAvailability.Id,
	)
	assert.Equal(t, `{"threshold": 96}`, string(raw))
	assert.False(t, isDefault)

	raw, isDefault = configs.getRaw(
		ApplicationId{Namespace: "default", Kind: "deployment", Name: "auth-api"},
		Checks.SLOAvailability.Id,
	)
	assert.Equal(t, `{"threshold": 98}`, string(raw))
	assert.False(t, isDefault)

	raw, isDefault = configs.getRaw(
		ApplicationId{Namespace: "production", Kind: "deployment", Name: "payment-service"},
		Checks.SLOAvailability.Id,
	)
	assert.Equal(t, `{"threshold": 99.9}`, string(raw))
	assert.False(t, isDefault)

	raw, isDefault = configs.getRaw(
		ApplicationId{Namespace: "staging", Kind: "deployment", Name: "web-frontend"},
		Checks.SLOLatency.Id,
	)
	assert.Equal(t, `{"threshold": 85}`, string(raw))
	assert.False(t, isDefault)

	raw, isDefault = configs.getRaw(
		ApplicationId{Namespace: "test", Kind: "deployment", Name: "random-service"},
		Checks.SLOAvailability.Id,
	)
	assert.Equal(t, `{"threshold": 80}`, string(raw))
	assert.True(t, isDefault)

	raw, isDefault = configs.getRaw(
		ApplicationId{Namespace: "test", Kind: "deployment", Name: "random-service"},
		Checks.CPUNode.Id,
	)
	assert.Nil(t, raw)
	assert.False(t, isDefault)

	raw, isDefault = configs.getRaw(
		ApplicationId{Namespace: "default", Kind: "deployment", Name: "worker-service"},
		Checks.SLOAvailability.Id,
	)
	assert.Equal(t, `{"threshold": 80}`, string(raw))
	assert.True(t, isDefault)

	raw, isDefault = configs.getRaw(
		ApplicationId{Namespace: "production", Kind: "deployment", Name: "payment-api"},
		Checks.SLOAvailability.Id,
	)
	assert.Equal(t, `{"threshold": 99.9}`, string(raw))
	assert.False(t, isDefault)

}
