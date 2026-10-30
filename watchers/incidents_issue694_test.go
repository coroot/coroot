package watchers

import (
	"testing"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/notifications"
	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func issue694Values(points int, value float32) []float32 {
	values := make([]float32, points)
	for i := range values {
		values[i] = value
	}
	return values
}

func issue694Status(burnRates []model.BurnRate) model.Status {
	status := model.UNKNOWN
	for _, burnRate := range burnRates {
		if burnRate.Severity > status {
			status = burnRate.Severity
		}
	}
	return status
}

func TestIssue694IncidentPersistence(t *testing.T) {
	const objectivePercentage = float32(99)

	step := timeseries.Minute
	evaluationTime := timeseries.Time(1_700_000_000).Truncate(step)
	from := evaluationTime.Add(-model.MaxAlertRuleWindow)
	points := int(model.MaxAlertRuleWindow/step) + 1

	zeroTraffic := func() (*timeseries.TimeSeries, *timeseries.TimeSeries) {
		return timeseries.NewWithData(from, step, issue694Values(points, 0)),
			timeseries.NewWithData(from, step, issue694Values(points, 0))
	}
	coverageSeries := func(defined int, value float32) *timeseries.TimeSeries {
		const coveragePoints = 100
		values := issue694Values(coveragePoints, timeseries.NaN)
		for i := coveragePoints - defined; i < coveragePoints; i++ {
			values[i] = value
		}
		return timeseries.NewWithData(evaluationTime.Add(-(coveragePoints-1)*timeseries.Second), timeseries.Second, values)
	}
	errorAt := func(age timeseries.Duration) (*timeseries.TimeSeries, *timeseries.TimeSeries) {
		total := issue694Values(points, 0)
		failed := issue694Values(points, 0)
		index := points - 1 - int(age/step)
		total[index] = 100
		failed[index] = 100
		return timeseries.NewWithData(from, step, total), timeseries.NewWithData(from, step, failed)
	}

	tests := []struct {
		name                  string
		telemetry             func() (*timeseries.TimeSeries, *timeseries.TimeSeries)
		evaluationOffset      timeseries.Duration
		wantEmptyTail         bool
		wantStatus            model.Status
		wantSeverities        []model.Status
		wantOpen              bool
		wantTotalUnavailable  bool
		wantFailedUnavailable bool
		checkTotalSum         bool
		wantTotalSum          float32
		checkFailedSum        bool
		wantFailedSum         float32
	}{
		{
			name: "missing_telemetry",
			telemetry: func() (*timeseries.TimeSeries, *timeseries.TimeSeries) {
				return timeseries.New(from, points, step), timeseries.New(from, points, step)
			},
			wantEmptyTail:  true,
			wantStatus:     model.UNKNOWN,
			wantSeverities: nil,
			wantOpen:       true,
		},
		{
			name:           "defined_zero_requests_and_failures",
			telemetry:      zeroTraffic,
			wantEmptyTail:  false,
			wantStatus:     model.OK,
			wantSeverities: []model.Status{model.OK, model.OK},
			wantOpen:       false,
		},
		{
			name: "healthy_traffic",
			telemetry: func() (*timeseries.TimeSeries, *timeseries.TimeSeries) {
				return timeseries.NewWithData(from, step, issue694Values(points, 100)),
					timeseries.NewWithData(from, step, issue694Values(points, 0))
			},
			wantEmptyTail:  false,
			wantStatus:     model.OK,
			wantSeverities: []model.Status{model.OK, model.OK},
			wantOpen:       false,
		},
		{
			name: "insufficient_telemetry_coverage",
			telemetry: func() (*timeseries.TimeSeries, *timeseries.TimeSeries) {
				total := issue694Values(points, timeseries.NaN)
				total[len(total)-1] = 0
				return timeseries.NewWithData(from, step, total),
					timeseries.NewWithData(from, step, issue694Values(points, 0))
			},
			wantEmptyTail:        false,
			wantStatus:           model.UNKNOWN,
			wantSeverities:       nil,
			wantOpen:             true,
			wantTotalUnavailable: true,
		},
		{
			name: "total_requests_49_percent_coverage",
			telemetry: func() (*timeseries.TimeSeries, *timeseries.TimeSeries) {
				return coverageSeries(49, 0), coverageSeries(100, 0)
			},
			wantEmptyTail:        false,
			wantStatus:           model.UNKNOWN,
			wantSeverities:       nil,
			wantOpen:             true,
			wantTotalUnavailable: true,
		},
		{
			name: "total_requests_50_percent_coverage",
			telemetry: func() (*timeseries.TimeSeries, *timeseries.TimeSeries) {
				return coverageSeries(50, 0), coverageSeries(100, 0)
			},
			wantEmptyTail:  false,
			wantStatus:     model.OK,
			wantSeverities: []model.Status{model.OK, model.OK},
			wantOpen:       false,
			checkTotalSum:  true,
			wantTotalSum:   0,
		},
		{
			name: "failed_requests_49_percent_coverage",
			telemetry: func() (*timeseries.TimeSeries, *timeseries.TimeSeries) {
				return coverageSeries(100, 1), coverageSeries(49, 0)
			},
			wantEmptyTail:         false,
			wantStatus:            model.UNKNOWN,
			wantSeverities:        nil,
			wantOpen:              true,
			wantFailedUnavailable: true,
		},
		{
			name: "failed_requests_50_percent_coverage",
			telemetry: func() (*timeseries.TimeSeries, *timeseries.TimeSeries) {
				return coverageSeries(100, 1), coverageSeries(50, 0)
			},
			wantEmptyTail:  false,
			wantStatus:     model.OK,
			wantSeverities: []model.Status{model.OK, model.OK},
			wantOpen:       false,
			checkFailedSum: true,
			wantFailedSum:  0,
		},
		{
			name: "evaluation_window_with_no_points",
			telemetry: func() (*timeseries.TimeSeries, *timeseries.TimeSeries) {
				return zeroTraffic()
			},
			evaluationOffset:     model.MaxAlertRuleWindow + step,
			wantEmptyTail:        false,
			wantStatus:           model.UNKNOWN,
			wantSeverities:       nil,
			wantOpen:             true,
			wantTotalUnavailable: true,
		},
		{
			name: "historical_error_inside_fifteen_minute_short_window",
			telemetry: func() (*timeseries.TimeSeries, *timeseries.TimeSeries) {
				return errorAt(10 * timeseries.Minute)
			},
			wantEmptyTail:  false,
			wantStatus:     model.CRITICAL,
			wantSeverities: []model.Status{model.OK, model.CRITICAL},
			wantOpen:       true,
		},
		{
			name: "historical_error_outside_both_short_windows",
			telemetry: func() (*timeseries.TimeSeries, *timeseries.TimeSeries) {
				return errorAt(20 * timeseries.Minute)
			},
			wantEmptyTail:  false,
			wantStatus:     model.OK,
			wantSeverities: []model.Status{model.OK, model.OK},
			wantOpen:       false,
		},
		{
			name: "zero_total_with_positive_failures",
			telemetry: func() (*timeseries.TimeSeries, *timeseries.TimeSeries) {
				failed := issue694Values(points, 0)
				failed[len(failed)-1] = 1
				return timeseries.NewWithData(from, step, issue694Values(points, 0)),
					timeseries.NewWithData(from, step, failed)
			},
			wantEmptyTail:  false,
			wantStatus:     model.CRITICAL,
			wantSeverities: []model.Status{model.CRITICAL, model.CRITICAL},
			wantOpen:       true,
		},
		{
			name: "zero_total_with_missing_failed_samples",
			telemetry: func() (*timeseries.TimeSeries, *timeseries.TimeSeries) {
				return timeseries.NewWithData(from, step, issue694Values(points, 0)),
					timeseries.New(from, points, step)
			},
			wantEmptyTail:         false,
			wantStatus:            model.UNKNOWN,
			wantSeverities:        nil,
			wantOpen:              true,
			wantFailedUnavailable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			database, err := db.NewSqlite(t.TempDir(), false)
			require.NoError(t, err)
			t.Cleanup(func() { _ = database.DB().Close() })
			require.NoError(t, database.Migrate())

			project := &db.Project{Name: "issue-694"}
			require.NoError(t, database.SaveProject(project))

			appID := model.NewApplicationId(string(project.Id), "default", model.ApplicationKindDeployment, "quiet-service")
			incident := &model.ApplicationIncident{
				ApplicationId: appID,
				Key:           "issue-694",
				OpenedAt:      evaluationTime.Add(-timeseries.Hour),
				Severity:      model.CRITICAL,
			}
			require.NoError(t, database.CreateIncident(project.Id, appID, incident))

			evaluateAt := evaluationTime.Add(tt.evaluationOffset)
			world := model.NewWorld(from, evaluateAt, step, step)
			app := world.GetOrCreateApplication(appID, false)
			app.Category = model.ApplicationCategoryApplication
			assert.False(t, app.AlertingDisabled())
			total, failed := tt.telemetry()
			assert.Equal(t, tt.wantEmptyTail, total.TailIsEmpty())
			app.AvailabilitySLIs = []*model.AvailabilitySLI{{
				Config: model.CheckConfigSLOAvailability{
					ObjectivePercentage: objectivePercentage,
				},
				TotalRequestsRaw:  total,
				FailedRequestsRaw: failed,
			}}

			burnRates, failedF, totalF := availability(world.Ctx, app)
			var severities []model.Status
			for _, burnRate := range burnRates {
				severities = append(severities, burnRate.Severity)
			}
			assert.Equal(t, tt.wantSeverities, severities)
			assert.Equal(t, tt.wantStatus, issue694Status(burnRates))
			if tt.wantTotalUnavailable {
				require.NotNil(t, totalF)
				assert.True(t, timeseries.IsNaN(totalF(evaluateAt.Add(-model.MaxAlertRuleWindow))))
			}
			if tt.wantFailedUnavailable {
				require.NotNil(t, failedF)
				assert.True(t, timeseries.IsNaN(failedF(evaluateAt.Add(-model.MaxAlertRuleWindow))))
			}
			if tt.checkTotalSum {
				require.NotNil(t, totalF)
				assert.Equal(t, tt.wantTotalSum, totalF(evaluateAt.Add(-model.MaxAlertRuleWindow)))
			}
			if tt.checkFailedSum {
				require.NotNil(t, failedF)
				assert.Equal(t, tt.wantFailedSum, failedF(evaluateAt.Add(-model.MaxAlertRuleWindow)))
			}

			watcher := &Incidents{
				db:       database,
				notifier: &notifications.IncidentNotifier{},
			}
			watcher.Check(project, world)

			openIncident, err := database.GetLastOpenIncident(project.Id, appID)
			require.NoError(t, err)
			if tt.wantOpen {
				require.NotNil(t, openIncident)
				assert.True(t, openIncident.ResolvedAt.IsZero())
			} else {
				assert.Nil(t, openIncident)
			}

			persistedByApp, err := database.GetApplicationIncidents(project.Id, 0, timeseries.Time(2_100_000_000))
			require.NoError(t, err)
			require.Len(t, persistedByApp[appID], 1)
			assert.Equal(t, tt.wantOpen, persistedByApp[appID][0].ResolvedAt.IsZero())
		})
	}
}
