//go:build integration

package clickhouse

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/coroot/coroot/ch"
	"github.com/coroot/coroot/config"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/stretchr/testify/require"
)

// Integration tests require a ClickHouse instance.
//
//	CLICKHOUSE_ADDR=localhost:9000 go test -tags=integration ./clickhouse/ -run RumIntegration -count=1
func TestRumIntegration_MigrateIdempotent(t *testing.T) {
	addr := os.Getenv("CLICKHOUSE_ADDR")
	if addr == "" {
		t.Skip("CLICKHOUSE_ADDR not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cfg := config.CollectorConfig{
		TracesTTL:   7 * timeseries.Day,
		LogsTTL:     7 * timeseries.Day,
		ProfilesTTL: 7 * timeseries.Day,
		MetricsTTL:  7 * timeseries.Day,
		Rum: config.Rum{
			TTL:           7 * timeseries.Day,
			ReplayTTL:     7 * timeseries.Day,
			AggregatesTTL: 30 * timeseries.Day,
		},
	}
	dbName := "coroot_rum_itest"
	bootstrap, err := ch.NewLowLevelClient(ctx, &db.IntegrationClickhouse{
		Addr:     addr,
		Database: "default",
		Auth:     utils.BasicAuth{User: "default"},
	})
	require.NoError(t, err)
	require.NoError(t, bootstrap.Exec(ctx, "CREATE DATABASE IF NOT EXISTS "+dbName))
	bootstrap.Close()

	llc, err := ch.NewLowLevelClient(ctx, &db.IntegrationClickhouse{
		Addr:     addr,
		Database: dbName,
		Auth:     utils.BasicAuth{User: "default"},
	})
	require.NoError(t, err)
	defer llc.Close()

	require.NoError(t, llc.Migrate(ctx, cfg))
	require.NoError(t, llc.Migrate(ctx, cfg))

	for _, name := range []string{"rum_spans", "rum_events", "rum_replay_segments", "rum_page_stats_1m", "rum_events_1m"} {
		require.NoError(t, llc.Exec(ctx, "EXISTS TABLE "+name), name)
	}
}
