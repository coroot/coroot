package db

import (
	"regexp"
	"testing"

	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
)

var rumApiKeyFormat = regexp.MustCompile(`^[0-9a-z]{5}-[0-9a-z]{5}-[0-9a-z]{5}$`)

func TestRumApiKey(t *testing.T) {
	for i := 0; i < 20; i++ {
		key := RumApiKey()
		assert.True(t, rumApiKeyFormat.MatchString(key), key)
	}
}

func TestApiKeyValidate(t *testing.T) {
	assert.Error(t, (&ApiKey{}).Validate())
	assert.NoError(t, (&ApiKey{Key: "secret-key"}).Validate())
	assert.NoError(t, (&ApiKey{Key: "secret-key", Type: ApiKeyTypeDefault}).Validate())

	assert.Error(t, (&ApiKey{Key: "k", Type: "invalid"}).Validate())

	assert.Error(t, (&ApiKey{Key: "rum-key", Type: ApiKeyTypeRum}).Validate())
	assert.NoError(t, (&ApiKey{
		Key:            "rum-key",
		Type:           ApiKeyTypeRum,
		AllowedOrigins: []string{"https://app.example.com"},
	}).Validate())
}

func TestIsRum(t *testing.T) {
	assert.False(t, (&ApiKey{Key: "k"}).IsRum())
	assert.False(t, (&ApiKey{Key: "k", Type: ApiKeyTypeDefault}).IsRum())
	assert.True(t, (&ApiKey{Key: "k", Type: ApiKeyTypeRum}).IsRum())
}

func TestRumRetentionValidate(t *testing.T) {
	assert.NoError(t, (*RumRetention)(nil).Validate())
	assert.Error(t, (&RumRetention{RawTTL: timeseries.Hour}).Validate())
	assert.Error(t, (&RumRetention{RawTTL: 14 * timeseries.Day, AggregatesTTL: 7 * timeseries.Day}).Validate())
	assert.NoError(t, (&RumRetention{RawTTL: 7 * timeseries.Day, AggregatesTTL: 30 * timeseries.Day}).Validate())
}
