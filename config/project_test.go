package config

import (
	"testing"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestAlertingRuleDurations(t *testing.T) {
	var r AlertingRule
	require.NoError(t, yaml.Unmarshal([]byte("id: dns-latency\nfor: 0s\nkeepFiringFor: 2m"), &r))
	assert.Equal(t, RuleDuration(0), *r.For)
	assert.Equal(t, RuleDuration(2*timeseries.Minute), *r.KeepFiringFor)

	merged := applyConfigOverrides(&model.AlertingRule{For: 5 * timeseries.Minute, KeepFiringFor: 5 * timeseries.Minute}, r)
	assert.Equal(t, timeseries.Duration(0), merged.For)
	assert.Equal(t, 2*timeseries.Minute, merged.KeepFiringFor)

	out, err := yaml.Marshal(r)
	require.NoError(t, err)
	assert.Contains(t, string(out), "for: 0s")
	assert.Contains(t, string(out), "keepFiringFor: 2m")

	assert.Error(t, yaml.Unmarshal([]byte("id: dns-latency\nfor: soon"), &r))
	assert.Error(t, yaml.Unmarshal([]byte("id: dns-latency\nfor: -1m"), &r))
}
