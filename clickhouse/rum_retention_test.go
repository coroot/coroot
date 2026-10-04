package clickhouse

import (
	"testing"
	"time"

	"github.com/coroot/coroot/config"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
)

func TestEffectiveRumRetention_Defaults(t *testing.T) {
	ttls := EffectiveRumRetention(nil, config.Rum{})
	assert.Equal(t, 7*timeseries.Day, ttls.Raw)
	assert.Equal(t, 7*timeseries.Day, ttls.Replay)
	assert.Equal(t, 30*timeseries.Day, ttls.Aggregates)
}

func TestEffectiveRumRetention_Global(t *testing.T) {
	ttls := EffectiveRumRetention(nil, config.Rum{
		TTL:           3 * timeseries.Day,
		ReplayTTL:     2 * timeseries.Day,
		AggregatesTTL: 60 * timeseries.Day,
	})
	assert.Equal(t, 3*timeseries.Day, ttls.Raw)
	assert.Equal(t, 2*timeseries.Day, ttls.Replay)
	assert.Equal(t, 60*timeseries.Day, ttls.Aggregates)
}

func TestEffectiveRumRetention_ProjectOverride(t *testing.T) {
	p := &db.Project{Settings: db.ProjectSettings{
		Rum: &db.RumProjectSettings{
			Retention: &db.RumRetention{
				RawTTL:        5 * timeseries.Day,
				AggregatesTTL: 14 * timeseries.Day,
			},
		},
	}}
	ttls := EffectiveRumRetention(p, config.Rum{
		TTL:           7 * timeseries.Day,
		ReplayTTL:     7 * timeseries.Day,
		AggregatesTTL: 30 * timeseries.Day,
	})
	assert.Equal(t, 5*timeseries.Day, ttls.Raw)
	assert.Equal(t, 7*timeseries.Day, ttls.Replay)
	assert.Equal(t, 14*timeseries.Day, ttls.Aggregates)
}

func TestEffectiveRumRetention_AggregatesGteRaw(t *testing.T) {
	p := &db.Project{Settings: db.ProjectSettings{
		Rum: &db.RumProjectSettings{
			Retention: &db.RumRetention{
				RawTTL:        30 * timeseries.Day,
				AggregatesTTL: 7 * timeseries.Day,
			},
		},
	}}
	ttls := EffectiveRumRetention(p, config.Rum{
		TTL:           7 * timeseries.Day,
		AggregatesTTL: 30 * timeseries.Day,
	})
	assert.Equal(t, 30*timeseries.Day, ttls.Raw)
	assert.Equal(t, 30*timeseries.Day, ttls.Aggregates)
}

func TestTtlNeedsModify(t *testing.T) {
	assert.False(t, ttlNeedsModify(0, 0))
	assert.True(t, ttlNeedsModify(0, 7*timeseries.Day))
	assert.False(t, ttlNeedsModify(uint64(7*timeseries.Day), 7*timeseries.Day))
	assert.True(t, ttlNeedsModify(uint64(14*timeseries.Day), 7*timeseries.Day))
}

func TestRumTableCleanupPriority(t *testing.T) {
	assert.Equal(t, 0, RumTableCleanupPriority("rum_replay_segments"))
	assert.Equal(t, 1, RumTableCleanupPriority("rum_spans"))
	assert.Equal(t, 1, RumTableCleanupPriority("rum_events"))
	assert.Equal(t, 2, RumTableCleanupPriority("rum_page_stats_1m"))
	assert.Equal(t, 2, RumTableCleanupPriority("rum_events_1m"))
	assert.Equal(t, 3, RumTableCleanupPriority("otel_traces"))
}

func TestRumTableTTL(t *testing.T) {
	ttls := RumRetentionTTLs{
		Raw:        7 * timeseries.Day,
		Replay:     3 * timeseries.Day,
		Aggregates: 30 * timeseries.Day,
	}
	assert.Equal(t, 7*timeseries.Day, rumTableTTL("rum_spans", ttls))
	assert.Equal(t, 7*timeseries.Day, rumTableTTL("rum_events", ttls))
	assert.Equal(t, 3*timeseries.Day, rumTableTTL("rum_replay_segments", ttls))
	assert.Equal(t, 30*timeseries.Day, rumTableTTL("rum_page_stats_1m", ttls))
	assert.Equal(t, timeseries.Duration(0), rumTableTTL("unknown", ttls))
}

func TestExpiredPartitionCutoff(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	ttl := 7 * timeseries.Day
	cutoff := now.Add(-ttl.ToStandard())
	assert.True(t, time.Date(2026, 9, 26, 23, 0, 0, 0, time.UTC).Before(cutoff))
	assert.False(t, time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC).Before(cutoff))
}

func TestRumRetentionValidate(t *testing.T) {
	assert.NoError(t, (*db.RumRetention)(nil).Validate())
	assert.Error(t, (&db.RumRetention{RawTTL: timeseries.Hour}).Validate())
	assert.Error(t, (&db.RumRetention{RawTTL: 14 * timeseries.Day, AggregatesTTL: 7 * timeseries.Day}).Validate())
	assert.NoError(t, (&db.RumRetention{RawTTL: 7 * timeseries.Day, AggregatesTTL: 30 * timeseries.Day}).Validate())
}
