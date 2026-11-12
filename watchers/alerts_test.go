package watchers

import (
	"testing"

	"github.com/coroot/coroot/auditor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/require"
)

func checkAlertFixture(t *testing.T) (*db.DB, *db.Project, *Alerts, *model.World, *model.Application, *model.AlertingRule) {
	t.Helper()
	database, err := db.NewSqlite(t.TempDir(), false)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.DB().Close()) })
	require.NoError(t, database.Migrate())
	project := &db.Project{Id: "test-project", Name: "test-project"}
	_, err = database.Exec("INSERT INTO project (id, name) VALUES ($1, $2)", project.Id, project.Name)
	require.NoError(t, err)
	rule := &model.AlertingRule{
		Id:       "test-check",
		Name:     "test check",
		Enabled:  true,
		Severity: model.WARNING,
		Source: model.AlertSource{
			Type:  model.AlertSourceTypeCheck,
			Check: &model.CheckSource{CheckId: model.Checks.MemoryOOM.Id},
		},
		Selector: model.AppSelector{Type: model.AppSelectorTypeAll},
	}
	require.NoError(t, database.CreateAlertingRule(project.Id, rule))
	now := timeseries.Now()
	world := model.NewWorld(now.Add(-timeseries.Hour), now, timeseries.Minute, timeseries.Minute)
	app := world.GetOrCreateApplication(model.NewApplicationId(string(project.Id), "default", model.ApplicationKindDeployment, "service"), false)
	return database, project, NewAlerts(database, nil, nil, nil, nil), world, app, rule
}

func TestCheckAlertUnknownDoesNotRecover(t *testing.T) {
	for _, tc := range []struct {
		name       string
		severity   model.Status
		suppressed bool
		keepFiring timeseries.Duration
	}{
		{name: "warning", severity: model.WARNING},
		{name: "critical after keep firing expires", severity: model.CRITICAL, keepFiring: 10 * timeseries.Minute},
		{name: "suppressed warning", severity: model.WARNING, suppressed: true},
		{name: "suppressed critical", severity: model.CRITICAL, suppressed: true, keepFiring: 10 * timeseries.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			database, project, watcher, world, app, rule := checkAlertFixture(t)
			rule.Severity = tc.severity
			rule.KeepFiringFor = tc.keepFiring
			require.NoError(t, database.UpdateAlertingRule(project.Id, rule))
			app.Reports = []*model.AuditReport{{Name: model.AuditReportMemory, Checks: []*model.Check{{Id: rule.Source.Check.CheckId, Status: tc.severity, Message: "synthetic failure"}}}}
			watcher.Check(project, world, world.Ctx.From, world.Ctx.To, world.Ctx.Step)
			alerts, err := database.GetLatestAlertsByRule(project.Id, string(rule.Id))
			require.NoError(t, err)
			require.Len(t, alerts, 1)
			before := alerts[0]
			if tc.suppressed {
				require.NoError(t, database.SuppressAlerts(project.Id, []string{before.Id}, "test"))
				before, err = database.GetAlert(project.Id, before.Id)
				require.NoError(t, err)
			}

			// Exercise the real auditor's missing-container path, rather than
			// treating a hand-built OK check as a stand-in for unavailable data.
			app.Reports = nil
			auditor.Audit(world, project, nil, nil)
			check, report := app.GetCheckWithReport(rule.Source.Check.CheckId)
			require.NotNil(t, check)
			require.Equal(t, model.UNKNOWN, check.Status)
			watcher.evaluateCheckAlert(project, rule, app, check, report, before.UpdatedAt.Add(timeseries.Hour))

			after, err := database.GetAlert(project.Id, before.Id)
			require.NoError(t, err)
			require.Zero(t, after.ResolvedAt, "missing telemetry is not recovery")
			require.Equal(t, tc.suppressed, after.Suppressed, "missing telemetry must not clear suppression")
			require.Equal(t, before.UpdatedAt, after.UpdatedAt, "no successful or failing observation was made")
		})
	}
}

func TestCheckAlertUnknownRestartsPendingPeriod(t *testing.T) {
	database, project, watcher, _, app, rule := checkAlertFixture(t)
	rule.For = 10 * timeseries.Minute
	check := &model.Check{Id: rule.Source.Check.CheckId, Status: model.WARNING, Message: "synthetic failure"}
	now := timeseries.Now()
	watcher.evaluateCheckAlert(project, rule, app, check, model.AuditReportMemory, now)
	check.Status = model.UNKNOWN
	watcher.evaluateCheckAlert(project, rule, app, check, model.AuditReportMemory, now.Add(5*timeseries.Minute))
	check.Status = model.WARNING
	watcher.evaluateCheckAlert(project, rule, app, check, model.AuditReportMemory, now.Add(11*timeseries.Minute))
	alerts, err := database.GetLatestAlertsByRule(project.Id, string(rule.Id))
	require.NoError(t, err)
	require.Empty(t, alerts, "unobserved time must not count toward a continuous breach")
	watcher.evaluateCheckAlert(project, rule, app, check, model.AuditReportMemory, now.Add(21*timeseries.Minute))
	alerts, err = database.GetLatestAlertsByRule(project.Id, string(rule.Id))
	require.NoError(t, err)
	require.Len(t, alerts, 1)
}

func TestCheckAlertOKStillRecoversAfterKeepFiring(t *testing.T) {
	database, project, watcher, _, app, rule := checkAlertFixture(t)
	rule.KeepFiringFor = 10 * timeseries.Minute
	check := &model.Check{Id: rule.Source.Check.CheckId, Status: model.WARNING, Message: "synthetic failure"}
	watcher.evaluateCheckAlert(project, rule, app, check, model.AuditReportMemory, timeseries.Now())
	alerts, err := database.GetLatestAlertsByRule(project.Id, string(rule.Id))
	require.NoError(t, err)
	require.Len(t, alerts, 1)
	before := alerts[0]
	check.Status = model.OK
	watcher.evaluateCheckAlert(project, rule, app, check, model.AuditReportMemory, before.UpdatedAt.Add(timeseries.Minute))
	after, err := database.GetAlert(project.Id, before.Id)
	require.NoError(t, err)
	require.Zero(t, after.ResolvedAt)
	watcher.evaluateCheckAlert(project, rule, app, check, model.AuditReportMemory, before.UpdatedAt.Add(11*timeseries.Minute))
	after, err = database.GetAlert(project.Id, before.Id)
	require.NoError(t, err)
	require.NotZero(t, after.ResolvedAt)
}

func TestCheckAlertDisabledRuleStillRetiresUnknown(t *testing.T) {
	database, project, watcher, world, app, rule := checkAlertFixture(t)
	app.Reports = []*model.AuditReport{{Name: model.AuditReportMemory, Checks: []*model.Check{{Id: rule.Source.Check.CheckId, Status: model.WARNING, Message: "synthetic failure"}}}}
	watcher.Check(project, world, world.Ctx.From, world.Ctx.To, world.Ctx.Step)
	alerts, err := database.GetLatestAlertsByRule(project.Id, string(rule.Id))
	require.NoError(t, err)
	require.Len(t, alerts, 1)
	app.Reports = nil
	auditor.Audit(world, project, nil, nil)
	rule.Enabled = false
	require.NoError(t, database.UpdateAlertingRule(project.Id, rule))
	watcher.Check(project, world, world.Ctx.From, world.Ctx.To, world.Ctx.Step)
	after, err := database.GetAlert(project.Id, alerts[0].Id)
	require.NoError(t, err)
	require.NotZero(t, after.ResolvedAt)
}

func TestCheckAlertMissingReportIsNotRecovery(t *testing.T) {
	database, project, watcher, world, app, rule := checkAlertFixture(t)
	rule.Source.Check.CheckId = model.Checks.PostgresAvailability.Id
	require.NoError(t, database.UpdateAlertingRule(project.Id, rule))
	app.Reports = []*model.AuditReport{{Name: model.AuditReportPostgres, Checks: []*model.Check{{Id: rule.Source.Check.CheckId, Status: model.WARNING, Message: "synthetic failure"}}}}
	watcher.Check(project, world, world.Ctx.From, world.Ctx.To, world.Ctx.Step)
	alerts, err := database.GetLatestAlertsByRule(project.Id, string(rule.Id))
	require.NoError(t, err)
	require.Len(t, alerts, 1)
	app.Reports = nil
	auditor.Audit(world, project, nil, nil)
	check, _ := app.GetCheckWithReport(rule.Source.Check.CheckId)
	require.Nil(t, check)
	watcher.Check(project, world, world.Ctx.From, world.Ctx.To, world.Ctx.Step)
	after, err := database.GetAlert(project.Id, alerts[0].Id)
	require.NoError(t, err)
	require.Zero(t, after.ResolvedAt)
}
