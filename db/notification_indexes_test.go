package db

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationIndexes(t *testing.T) {
	cases := []struct {
		name  string
		index string
		query string
		args  []any
	}{
		{
			name:  "not sent alert notifications",
			index: "alert_notification_not_sent",
			query: "SELECT alert_id FROM alert_notification WHERE timestamp >= $1 AND sent_at = 0",
			args:  []any{0},
		},
		{
			name:  "update alert notification",
			index: "alert_notification_alert",
			query: "UPDATE alert_notification SET sent_at = $1, external_key = $2 WHERE project_id = $3 AND alert_id = $4 AND timestamp = $5 AND destination = $6",
			args:  []any{1, "", "p", "a", 1, "d"},
		},
		{
			name:  "alert notifications by alert ids",
			index: "alert_notification_alert",
			query: "SELECT alert_id FROM alert_notification WHERE project_id = $1 AND alert_id IN ($2, $3) ORDER BY alert_id, timestamp",
			args:  []any{"p", "a1", "a2"},
		},
		{
			name:  "not sent incident notifications",
			index: "incident_notification_not_sent",
			query: "SELECT incident_key FROM incident_notification WHERE timestamp >= $1 AND sent_at = 0",
			args:  []any{0},
		},
		{
			name:  "update incident notification",
			index: "incident_notification_incident",
			query: "UPDATE incident_notification SET sent_at = $1, external_key = $2 WHERE project_id = $3 AND incident_key = $4 AND timestamp = $5 AND destination = $6",
			args:  []any{1, "", "p", "i", 1, "d"},
		},
		{
			name:  "previous incident notifications",
			index: "incident_notification_incident",
			query: "SELECT incident_key FROM incident_notification WHERE project_id = $1 AND incident_key = $2 AND destination = $3 AND timestamp < $4 ORDER BY timestamp",
			args:  []any{"p", "i", "d", 1},
		},
	}

	forEachDB(t, func(t *testing.T, db *DB) {
		// migrations must stay idempotent
		require.NoError(t, db.Migrate())

		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				plan := explain(t, db, c.query, c.args...)
				assert.Contains(t, plan, c.index, plan)
			})
		}
	})
}

func explain(t *testing.T, db *DB, query string, args ...any) string {
	ctx := context.Background()
	conn, err := db.DB().Conn(ctx)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	var explainQuery string
	switch db.Type() {
	case TypeSqlite:
		explainQuery = "EXPLAIN QUERY PLAN " + query
	case TypePostgres:
		// the tables are empty, so make the planner prefer indexes the same way it would on real data
		_, err = conn.ExecContext(ctx, "SET enable_seqscan = off")
		require.NoError(t, err)
		defer func() { _, _ = conn.ExecContext(ctx, "RESET enable_seqscan") }()
		explainQuery = "EXPLAIN " + query
	}
	rows, err := conn.QueryContext(ctx, explainQuery, args...)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	cols, err := rows.Columns()
	require.NoError(t, err)
	var lines []string
	for rows.Next() {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		require.NoError(t, rows.Scan(ptrs...))
		line, _ := values[len(values)-1].(string)
		if b, ok := values[len(values)-1].([]byte); ok {
			line = string(b)
		}
		lines = append(lines, line)
	}
	require.NoError(t, rows.Err())
	return strings.Join(lines, "\n")
}
