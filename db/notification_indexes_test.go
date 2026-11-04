package db

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationIndexes(t *testing.T) {
	alertsByIdsQuery, alertsByIdsArgs := alertNotificationsByAlertIdsQuery("p", []string{"a1", "a2"})
	cases := []struct {
		name  string
		index string
		query string
		args  []any
	}{
		{
			name:  "not sent alert notifications",
			index: "alert_notification_not_sent",
			query: queryNotSentAlertNotifications,
			args:  []any{0},
		},
		{
			name:  "update alert notification",
			index: "alert_notification_alert",
			query: queryUpdateAlertNotification,
			args:  []any{1, "", "p", "a", 1, "d"},
		},
		{
			name:  "alert notifications by alert ids",
			index: "alert_notification_alert",
			query: alertsByIdsQuery,
			args:  alertsByIdsArgs,
		},
		{
			name:  "previous alert notifications",
			index: "alert_notification_alert",
			query: queryPreviousAlertNotifications,
			args:  []any{"p", "a", "d", 1},
		},
		{
			name:  "not sent incident notifications",
			index: "incident_notification_not_sent",
			query: queryNotSentIncidentNotifications,
			args:  []any{0},
		},
		{
			name:  "update incident notification",
			index: "incident_notification_incident",
			query: queryUpdateIncidentNotification,
			args:  []any{1, "", "p", "i", 1, "d"},
		},
		{
			name:  "previous incident notifications",
			index: "incident_notification_incident",
			query: queryPreviousIncidentNotifications,
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
