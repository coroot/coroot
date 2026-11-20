package db

import (
	"testing"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestAlert(t *testing.T, db *DB, projectId ProjectId, id string, openedAt, resolvedAt timeseries.Time) {
	a := &model.Alert{
		Id:            id,
		Fingerprint:   id,
		RuleId:        "rule",
		ApplicationId: model.NewApplicationId("", "default", model.ApplicationKindDeployment, id),
		Severity:      model.WARNING,
		Summary:       id,
	}
	require.NoError(t, db.CreateAlert(projectId, a))
	_, err := db.db.Exec("UPDATE alert SET opened_at = $1, resolved_at = $2 WHERE id = $3", openedAt, resolvedAt, id)
	require.NoError(t, err)
}

func TestQueryAlertsSortByDuration(t *testing.T) {
	forEachDB(t, func(t *testing.T, db *DB) {
		createTestProject(t, db, "p1")
		now := timeseries.Now()
		createTestAlert(t, db, "p1", "resolved-short", now.Add(-10*timeseries.Hour), now.Add(-10*timeseries.Hour+timeseries.Minute))
		createTestAlert(t, db, "p1", "firing-long", now.Add(-5*timeseries.Hour), 0)
		createTestAlert(t, db, "p1", "resolved-medium", now.Add(-3*timeseries.Hour), now.Add(-2*timeseries.Hour))
		createTestAlert(t, db, "p1", "firing-short", now.Add(-timeseries.Minute*10), 0)

		ids := func(res *AlertsResult) []string {
			var ids []string
			for _, a := range res.Alerts {
				ids = append(ids, a.Id)
			}
			return ids
		}

		res, err := db.QueryAlerts("p1", AlertsQuery{IncludeResolved: true, SortBy: "duration", SortDesc: true})
		require.NoError(t, err)
		assert.Equal(t, []string{"firing-long", "resolved-medium", "firing-short", "resolved-short"}, ids(res))

		res, err = db.QueryAlerts("p1", AlertsQuery{IncludeResolved: true, SortBy: "duration"})
		require.NoError(t, err)
		assert.Equal(t, []string{"resolved-short", "firing-short", "resolved-medium", "firing-long"}, ids(res))
	})
}
