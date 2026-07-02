package auditor

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/robfig/cron/v3"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

const pgActiveLockedState = "active (locked)"

const pgBlockSize = 8192

const pgWalSegmentSizeBytes = 16 * 1024 * 1024

const (
	pgQueriesChartTitle           = "Queries per second"
	pgLatencyChartTitle           = "Query latency <selector>, seconds"
	pgQueriesByTotalTimeTitle     = "Queries by total time on <selector>, query seconds/second"
	pgQueriesByIOTimeTitle        = "Queries by I/O time on <selector>, query seconds/second"
	pgConnectionsChartTitle       = "Connections <selector>"
	pgIdleTransactionsTitle       = "Idle transactions by query on <selector>"
	pgActiveQueriesTitle          = "Active connections by query on <selector>"
	pgLockedQueriesTitle          = "Locked queries on <selector>"
	pgBlockingQueriesTitle        = "Blocking queries by the number of awaiting queries on <selector>"
	pgReplicationLagChartTitle    = "Replication lag, bytes"
	pgReplicationStagesChartTitle = "Replication stages <selector>, bytes"
	pgWalThroughputChartTitle     = "WAL throughput, bytes/s"
	pgWalSizeChartTitle           = "WAL size, bytes"
	pgReplicationSlotsTitle       = "Replication slots retained WAL <selector>, bytes"
	pgWalArchivingChartTitle      = "WAL archiving <selector>"
	pgCheckpointsChartTitle       = "Checkpoints <selector>"
	pgCheckpointTriggersTitle     = "Checkpoints by trigger <selector>"
	pgCheckpointerWriteTitle      = "Checkpointer write throughput, bytes/s"
	pgTimeSinceCheckpointTitle    = "Time since last checkpoint, seconds"
	pgWalToReplayTitle            = "WAL to replay in the case of a crash, bytes"
	pgXidAgeChartTitle            = "Transaction ID age, transactions"
	pgMultixactAgeChartTitle      = "Multixact ID age, multixacts"
	pgXminHoldersChartTitle       = "Oldest transaction ID held back <selector>, transactions"
	pgDiskUsageChartTitle         = "Disk usage <selector>, bytes"
	pgTopTablesChartTitle         = "Top tables by size <selector>, bytes"
	pgBloatByDbChartTitle         = "Estimated bloat by database <selector>, bytes"
	pgTopBloatTablesChartTitle    = "Top tables by estimated bloat <selector>, bytes"
	pgTopBloatIndexesChartTitle   = "Top indexes by estimated bloat <selector>, bytes"
	pgDeadTuplesByTableTitle      = "Dead tuples by table <selector>, bytes"
	pgAutovacuumPressureTitle     = "Autovacuum Pressure <selector>, dead tuples ÷ autovacuum trigger threshold"
	pgTimeSinceAutovacuumTitle    = "Time since last autovacuum by table <selector>, seconds"
	pgAutovacuumWorkersTitle      = "Autovacuum workers <selector>"
	pgThrottledByTableTitle       = "Throttled autovacuum workers by table <selector>"
	pgAnalyzePressureTitle        = "Autoanalyze Pressure <selector>, rows modified ÷ autoanalyze trigger threshold"
	pgTimeSinceAnalyzeTitle       = "Time since last analyze by table <selector>, seconds"
)

const pgBackupScheduleGrace = 15 * 60

func pgNextScheduledBackup(b *model.PgBackups, lastSuccess float32) timeseries.Time {
	schedule := b.Schedule
	if schedule == "" {
		for _, m := range b.Methods {
			if m.Schedule != "" {
				schedule = m.Schedule
				break
			}
		}
	}
	if schedule != "" && !timeseries.IsNaN(lastSuccess) && lastSuccess > 0 {
		if sched, err := pgParseCron(schedule); err == nil {
			return timeseries.Time(sched.Next(time.Unix(int64(lastSuccess), 0)).Unix())
		}
	}
	return b.NextScheduledBackup
}

func pgParseCron(schedule string) (cron.Schedule, error) {
	if s, err := cron.ParseStandard(schedule); err == nil {
		return s, nil
	}
	return cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow).Parse(schedule)
}

const pgWraparoundLimit = 1 << 31

const pgBloatMinBytes = 1 * 1024 * 1024 * 1024

var (
	pgConnectionStateColors = map[string]string{
		"idle":                "grey-lighten2",
		pgActiveLockedState:   "red-lighten2",
		"active":              "green",
		"idle in transaction": "lime",
		"reserved":            "blue-lighten3",
	}
)

func (a *appAuditor) postgres() {
	isPostgres := a.app.ApplicationTypes()[model.ApplicationTypePostgres]

	if !isPostgres && !a.app.IsPostgres() {
		return
	}

	report := a.addReport(model.AuditReportPostgres)

	report.Instrumentation = model.ApplicationTypePostgres

	if !a.app.IsPostgres() {
		report.Status = model.UNKNOWN
		return
	}

	availabilityCheck := report.CreateCheck(model.Checks.PostgresAvailability)
	latencyCheck := report.CreateCheck(model.Checks.PostgresLatency)
	replicationCheck := report.CreateCheck(model.Checks.PostgresReplicationLag)
	connectionsCheck := report.CreateCheck(model.Checks.PostgresConnections)
	checkpointCheck := report.CreateCheck(model.Checks.PostgresCheckpoint)
	walArchivingCheck := report.CreateCheck(model.Checks.PostgresWalArchiving)
	wraparoundCheck := report.CreateCheck(model.Checks.PostgresWraparound)
	bloatCheck := report.CreateCheck(model.Checks.PostgresBloat)
	autovacuumCheck := report.CreateCheck(model.Checks.PostgresAutovacuum)

	tableColumns := []string{"Instance", "Role", "Status", "Queries", "Latency", "Replication lag", "DB Size"}
	instanceTable := report.GetOrCreateTable(tableColumns...)
	availabilityCheck.AddWidget(instanceTable.Widget())

	primaryLsn := timeseries.NewAggregate(timeseries.Max)
	for _, i := range a.app.Instances {
		if i.Postgres != nil && i.Postgres.WalCurrentLsn != nil {
			primaryLsn.Add(i.Postgres.WalCurrentLsn)
		}
	}

	for _, i := range a.app.Instances {
		if i.Postgres == nil {
			continue
		}
		report.
			GetOrCreateChartInGroup(pgLatencyChartTitle, "overview (avg)", nil).
			Group("Queries", 1).
			Feature().
			AddSeries(i.Name, i.Postgres.Avg)
		if i.Postgres.Avg.Last() > latencyCheck.Threshold {
			latencyCheck.AddItem("%s", i.Name)
		}
		report.
			GetOrCreateChartInGroup(pgLatencyChartTitle, i.Name, nil).
			AddSeries("avg", i.Postgres.Avg).
			AddSeries("p50", i.Postgres.P50).
			AddSeries("p95", i.Postgres.P95).
			AddSeries("p99", i.Postgres.P99)

		qps := sumQueries(i.Postgres.QueriesByDB)
		report.GetOrCreateChart(pgQueriesChartTitle, nil).Group("Queries", 1).AddSeries(i.Name, qps)

		pgQueries(report, i)

		pgConnections(report, i, connectionsCheck)
		pgLocks(report, i)
		pgCheckpoints(report, i, a.app.Instances, primaryLsn.Get(), checkpointCheck)
		primaryLsnTs := primaryLsn.Get()
		lag := pgReplicationLag(primaryLsnTs, i.Postgres.WalReplayLsn)
		report.
			GetOrCreateChart(pgReplicationLagChartTitle, nil).
			Group("Replication", 6).
			Feature().
			AddSeries(i.Name, lag)
		report.
			GetOrCreateChartInGroup(pgReplicationStagesChartTitle, i.Name, nil).
			Group("Replication", 6).
			Stacked().
			AddSeries("shipping (not yet received)", pgReplicationLag(primaryLsnTs, i.Postgres.WalReceiveLsn), "blue-lighten2").
			AddSeries("apply (received, not yet replayed)", pgReplicationLag(i.Postgres.WalReceiveLsn, i.Postgres.WalReplayLsn), "red-lighten2")
		report.
			GetOrCreateChart(pgWalThroughputChartTitle, nil).
			Group("WAL", 5).
			AddSeries(i.Name, i.Postgres.WalThroughput)
		report.
			GetOrCreateChart(pgWalSizeChartTitle, nil).
			Group("WAL", 5).
			AddSeries(i.Name, i.Postgres.WalSize).
			SetThreshold("max_wal_size", pgSettingBytes(i.Postgres.Settings["max_wal_size"]))
		slotsChart := report.
			GetOrCreateChartInGroup(pgReplicationSlotsTitle, i.Name, nil).
			Group("WAL", 5).
			Stacked()
		for name, slot := range i.Postgres.ReplicationSlots {
			slotsChart.AddSeries(name, slot.RetainedWal)
		}
		if !i.Postgres.WalArchivedSegments.IsEmpty() || !i.Postgres.WalArchiveFailures.IsEmpty() {
			report.
				GetOrCreateChartInGroup(pgWalArchivingChartTitle, i.Name, nil).
				Group("WAL", 5).
				Column().
				AddSeries("archived", i.Postgres.WalArchivedSegments).
				AddSeries("failed", i.Postgres.WalArchiveFailures, "red-lighten2")
		}
		if pgArchivingIsFailing(i.Postgres) {
			walArchivingCheck.AddItem("%s", i.Name)
			walArchivingCheck.AddDetail("%s: the last WAL archive attempt failed - check archive_command and the archive storage", i.Name)
		}
		pgWraparound(report, i, wraparoundCheck)

		diskUsageChart := report.GetOrCreateChartGroup(pgDiskUsageChartTitle, nil).Group("Storage", 8)
		tableSizeChart := report.GetOrCreateChartGroup(pgTopTablesChartTitle, nil).Group("Storage", 8)

		if diskUsageChart != nil {
			dbSize := map[string]model.SeriesData{}
			for db, ts := range i.Postgres.DatabaseSize {
				dbSize[db] = ts
			}
			dbSize["WAL"] = i.Postgres.WalSize
			diskUsageChart.GetOrCreateChart(i.Name).Stacked().Sorted().AddMany(dbSize, 20, timeseries.Max)
		}
		if tableSizeChart != nil {
			tableSize := map[string]model.SeriesData{}
			for k, ts := range i.Postgres.TableSize {
				tableSize[k.String()] = ts
			}
			tableSizeChart.GetOrCreateChart(i.Name).Stacked().Sorted().AddMany(tableSize, 20, timeseries.Max)
		}

		if i.IsObsolete() {
			continue
		}

		role := i.ClusterRoleLast()
		roleCell := model.NewTableCell(role.String())
		switch role {
		case model.ClusterRolePrimary:
			roleCell.SetIcon("mdi-database-edit-outline", "rgba(0,0,0,0.87)")
		case model.ClusterRoleReplica:
			roleCell.SetIcon("mdi-database-import-outline", "grey")
		}
		status := model.NewTableCell().SetStatus(model.OK, "up")
		if !i.Postgres.IsUp() {
			availabilityCheck.AddItem("%s", i.Name)
			if v := i.Postgres.Error.Value(); v != "" {
				status.SetStatus(model.WARNING, v)
			} else {
				status.SetStatus(model.WARNING, "down (no metrics)")
			}
		} else {
			if v := i.Postgres.Warning.Value(); v != "" {
				status.SetStatus(model.OK, v)
			}
		}
		lagCell := checkReplicationLag(i, a.app.Instances, primaryLsnTs, lag, role, replicationCheck)
		sizeCell := dbSizeCell(i.Postgres.DatabaseSize)
		report.
			GetOrCreateTable(tableColumns...).
			AddRow(
				model.NewTableCell(i.Name).AddTag("version: %s", i.Postgres.Version.Value()),
				roleCell,
				status,
				model.NewTableCell(utils.FormatFloat(qps.Last())).SetUnit("/s"),
				model.NewTableCell(utils.FormatFloat(i.Postgres.Avg.Last()*1000)).SetUnit("ms"),
				lagCell,
				sizeCell,
			)
	}

	latencyCheck.AddWidget(report.GetOrCreateChartGroup(pgLatencyChartTitle, nil).Widget())
	replicationCheck.AddWidget(report.GetOrCreateChart(pgReplicationLagChartTitle, nil).Widget())
	replicationCheck.AddWidget(report.GetOrCreateChartGroup(pgReplicationStagesChartTitle, nil).Widget())
	pgBloat(report, a.app.Instances, bloatCheck)

	autovacuumCheck.AddWidget(report.GetOrCreateChartGroup(pgAutovacuumPressureTitle, nil).Widget())
	autovacuumCheck.AddWidget(report.GetOrCreateChartGroup(pgDeadTuplesByTableTitle, nil).Widget())
	autovacuumCheck.AddWidget(report.GetOrCreateChartGroup(pgTimeSinceAutovacuumTitle, nil).Widget())
	autovacuumCheck.AddWidget(report.GetOrCreateChartGroup(pgAutovacuumWorkersTitle, nil).Widget())
	autovacuumCheck.AddWidget(report.GetOrCreateChartGroup(pgThrottledByTableTitle, nil).Widget())
	pgAutovacuum(report, a.app.Instances, autovacuumCheck)

	staleStatsCheck := report.CreateCheck(model.Checks.PostgresStaleStatistics)
	staleStatsCheck.AddWidget(report.GetOrCreateChartGroup(pgAnalyzePressureTitle, nil).Widget())
	staleStatsCheck.AddWidget(report.GetOrCreateChartGroup(pgTimeSinceAnalyzeTitle, nil).Widget())
	pgStaleStatistics(report, a.app.Instances, staleStatsCheck)

	if b := a.app.Cluster.Backups; b != nil && len(b.Methods) > 0 {
		backupCheck := report.CreateCheck(model.Checks.PostgresBackups)
		pgBackups(report, b, a.app.Instances, a.app.Id.Name, a.w.Ctx.To, backupCheck)
	}

	checkpointCheck.AddWidget(report.GetOrCreateChart(pgTimeSinceCheckpointTitle, nil).Widget())
	checkpointCheck.AddWidget(report.GetOrCreateChartGroup(pgCheckpointsChartTitle, nil).Widget())
	walArchivingCheck.AddWidget(report.GetOrCreateChartGroup(pgWalArchivingChartTitle, nil).Widget())
	wraparoundCheck.AddWidget(report.GetOrCreateChart(pgXidAgeChartTitle, nil).Widget())
	bloatCheck.AddWidget(report.GetOrCreateChartGroup(pgBloatByDbChartTitle, nil).Widget())

	pgConfigurationHints(report, a.app.Instances)
}

func pgMethodLabel(name string) string {
	if name == "barmanObjectStore" {
		return "Object storage"
	}
	return name
}

func pgBackupFailureHint(conds map[string]model.PgBackupCondition) string {
	for _, t := range []string{"PGBackRestReplicaRepoReady", "PGBackRestRepoHostReady", "PGBackRestReplicaCreate", "LastBackupSucceeded", "ContinuousArchiving"} {
		if c, ok := conds[t]; ok && c.Status == "False" && c.Reason != "" {
			return pgConditionHint(c.Reason)
		}
	}
	return ""
}

func pgConditionHint(reason string) string {
	switch reason {
	case "StanzaNotCreated":
		return "the pgBackRest repository isn't initialized - check the backup storage and credentials"
	case "RepoBackupNotComplete":
		return "the initial repository backup hasn't completed"
	default:
		return reason
	}
}

func pgBackups(report *model.AuditReport, b *model.PgBackups, instances []*model.Instance, cluster string, now timeseries.Time, check *model.Check) {
	age := func(ts float32) float32 {
		a := float32(now) - ts
		if a < 0 {
			a = 0
		}
		return a
	}

	var lastSuccess = timeseries.NaN
	consider := func(t timeseries.Time) {
		if v := float32(t); v > 0 && (timeseries.IsNaN(lastSuccess) || v > lastSuccess) {
			lastSuccess = v
		}
	}
	for _, m := range b.Methods {
		consider(m.LastSuccessfulBackup)
	}
	for _, r := range b.Runs {
		if r.Succeeded() {
			consider(r.CompletedAt)
		}
	}
	var backupAge = timeseries.NaN
	if !timeseries.IsNaN(lastSuccess) {
		backupAge = age(lastSuccess)
		check.SetValue(backupAge)
	}

	lastBackupFailed := b.Conditions["LastBackupSucceeded"].Status == "False"
	_, hasArchivingCondition := b.Conditions["ContinuousArchiving"]
	archivingBroken := b.Conditions["ContinuousArchiving"].Status == "False"
	hasArchivingSignal := hasArchivingCondition
	if !hasArchivingCondition {
		for _, i := range instances {
			if i.Postgres == nil || i.Postgres.WalArchivingStatus.IsEmpty() {
				continue
			}
			hasArchivingSignal = true
			if pgArchivingIsFailing(i.Postgres) {
				archivingBroken = true
			}
		}
	}

	var reasons []string
	switch {
	case timeseries.IsNaN(backupAge):
		reasons = append(reasons, "no successful backup has been recorded")
	case backupAge > check.Threshold:
		reasons = append(reasons, fmt.Sprintf("the last successful backup was %s ago", utils.FormatDuration(timeseries.Duration(backupAge), 1)))
	}
	if lastBackupFailed {
		r := "the last backup attempt failed"
		if reason := b.Conditions["LastBackupSucceeded"].Reason; reason != "" {
			r += " (" + reason + ")"
		}
		reasons = append(reasons, r)
	}
	if archivingBroken {
		reasons = append(reasons, "continuous WAL archiving is failing")
	}
	if b.Conditions["ReadyForBackup"].Status == "False" {
		r := "the cluster is not ready for backups"
		if hint := pgBackupFailureHint(b.Conditions); hint != "" {
			r += " - " + hint
		}
		reasons = append(reasons, r)
	}
	if c := b.Conditions["PGBackRestRepoHostReady"]; c.Status == "False" {
		r := "the pgBackRest repo host is not ready"
		if c.Reason != "" {
			r += " (" + pgConditionHint(c.Reason) + ")"
		}
		reasons = append(reasons, r)
	}

	nextBackup := pgNextScheduledBackup(b, lastSuccess)
	if nextBackup > 0 && float32(now)-float32(nextBackup) > pgBackupScheduleGrace {
		reasons = append(reasons, fmt.Sprintf("the scheduled backup is overdue (was due %s ago) - the backup schedule may not be running", utils.FormatDuration(timeseries.Duration(age(float32(nextBackup))), 1)))
	}
	if len(reasons) > 0 {
		check.AddItem("%s", cluster)
		for _, r := range reasons {
			check.AddDetail("%s: %s", cluster, r)
		}
	}

	methodNames := make([]string, 0, len(b.Methods))
	for name := range b.Methods {
		methodNames = append(methodNames, name)
	}
	slices.Sort(methodNames)

	table := report.GetOrCreateTable("", "").Group("Backup", 9).SetSorted()
	if table == nil {
		return
	}
	status := model.NewTableCell().SetStatus(model.OK, "ok")
	if len(reasons) > 0 {
		status = model.NewTableCell().SetStatus(model.WARNING, reasons[0])
	}
	table.AddRow(model.NewTableCell("Status"), status)

	for _, name := range methodNames {
		m := b.Methods[name]
		prefix := pgMethodLabel(name) + " "

		dest := model.NewTableCell(m.Destination)
		if m.Endpoint != "" {
			dest.AddTag("%s", m.Endpoint)
		}
		table.AddRow(model.NewTableCell(prefix+"destination"), dest)
		if m.Schedule != "" {
			table.AddRow(model.NewTableCell(prefix+"schedule"), model.NewTableCell(m.Schedule))
		}

		lastBackup := timeseries.NaN
		if m.LastSuccessfulBackup > 0 {
			lastBackup = float32(m.LastSuccessfulBackup)
		} else {
			for _, r := range b.Runs {
				if r.Method != name || !r.Succeeded() {
					continue
				}
				if v := float32(r.CompletedAt); v > 0 && (timeseries.IsNaN(lastBackup) || v > lastBackup) {
					lastBackup = v
				}
			}
		}
		if !timeseries.IsNaN(lastBackup) {
			table.AddRow(model.NewTableCell(prefix+"last backup"), pgTimeCell(lastBackup, now))
		} else {
			table.AddRow(model.NewTableCell(prefix+"last backup"), model.NewTableCell("none"))
		}
		if m.FirstRecoverabilityPoint > 0 {
			table.AddRow(model.NewTableCell(prefix+"restorable from"), pgTimeCell(float32(m.FirstRecoverabilityPoint), now))
		}
	}
	if b.Schedule != "" {
		table.AddRow(model.NewTableCell("Schedule"), model.NewTableCell(b.Schedule))
	}
	if nextBackup > 0 {
		table.AddRow(model.NewTableCell("Next backup"), pgTimeCell(float32(nextBackup), now))
	}
	if b.RetentionPolicy != "" {
		table.AddRow(model.NewTableCell("Retention"), model.NewTableCell(b.RetentionPolicy))
	}
	if hasArchivingSignal {
		walArchiving := model.NewTableCell().SetStatus(model.OK, "working")
		if archivingBroken {
			walArchiving = model.NewTableCell().SetStatus(model.WARNING, "failing")
		}
		table.AddRow(model.NewTableCell("WAL archiving"), walArchiving)
	}
	if b.LastFailedBackup > 0 {
		table.AddRow(model.NewTableCell("Last failed backup"), pgTimeCell(float32(b.LastFailedBackup), now))
	}
	check.AddWidget(table.Widget())

	pgBackupRuns(report, b.Runs, now, check)
}

func pgBackupRuns(report *model.AuditReport, runs []*model.PgBackupRun, now timeseries.Time, check *model.Check) {
	if len(runs) == 0 {
		return
	}
	table := report.GetOrCreateTable("Backup", "Type", "Status", "Completed").Group("Backup", 9).SetSorted().SetTitle("Recent backups")
	if table == nil {
		return
	}
	completed := func(r *model.PgBackupRun) float32 {
		return float32(r.CompletedAt)
	}
	sorted := append([]*model.PgBackupRun(nil), runs...)
	slices.SortFunc(sorted, func(a, b *model.PgBackupRun) int {
		return cmp.Compare(completed(b), completed(a))
	})
	const maxRuns = 20
	for i, r := range sorted {
		if i >= maxRuns {
			break
		}
		st := model.UNKNOWN
		switch {
		case r.Succeeded():
			st = model.OK
		case r.Status == "Failed" || r.Status == "failed":
			st = model.WARNING
		}
		completedCell := model.NewTableCell("running")
		if v := completed(r); v > 0 {
			completedCell = pgTimeCell(v, now)
		}
		table.AddRow(
			model.NewTableCell(r.Name),
			model.NewTableCell(r.Kind),
			model.NewTableCell().SetStatus(st, r.Status),
			completedCell,
		)
	}
	check.AddWidget(table.Widget())
}

func pgTimeCell(tsUnix float32, now timeseries.Time) *model.TableCell {
	c := model.NewTableCell().SetTimestamp(int64(tsUnix) * 1000)
	if float32(now) >= tsUnix {
		return c.AddTag("%s ago", utils.FormatDuration(timeseries.Duration(float32(now)-tsUnix), 1))
	}
	return c.AddTag("in %s", utils.FormatDuration(timeseries.Duration(tsUnix-float32(now)), 1))
}

func pgConfigurationHints(report *model.AuditReport, instances []*model.Instance) {
	seen, enabled := false, false
	for _, i := range instances {
		if i.Postgres == nil {
			continue
		}
		if s := i.Postgres.Settings["track_io_timing"].Samples; !s.IsEmpty() {
			seen = true
			if s.Last() == 1 {
				enabled = true
			}
		}
	}
	if seen && !enabled {
		report.ConfigurationHint = &model.ConfigurationHint{
			Message:      "Enable track_io_timing to attribute disk I/O to specific queries - without it, the per-query I/O time is always zero.",
			ReadMoreLink: "https://docs.coroot.com/databases/postgres",
		}
	}
}

func pgBloat(report *model.AuditReport, instances []*model.Instance, check *model.Check) {
	tableTotal := map[string]*timeseries.Aggregate{}
	indexTotal := map[string]*timeseries.Aggregate{}
	dbSize := map[string]*timeseries.Aggregate{}
	tables := map[model.DbTableKey]*timeseries.Aggregate{}
	indexes := map[model.DbIndexKey]*timeseries.Aggregate{}
	aggDb := func(m map[string]*timeseries.Aggregate, k string, ts *timeseries.TimeSeries) {
		a := m[k]
		if a == nil {
			a = timeseries.NewAggregate(timeseries.Max)
			m[k] = a
		}
		a.Add(ts)
	}
	for _, i := range instances {
		pg := i.Postgres
		if pg == nil {
			continue
		}
		for db, ts := range pg.DatabaseTableBloat {
			aggDb(tableTotal, db, ts)
		}
		for db, ts := range pg.DatabaseIndexBloat {
			aggDb(indexTotal, db, ts)
		}
		for db, ts := range pg.DatabaseSize {
			aggDb(dbSize, db, ts)
		}
		for k, ts := range pg.TableBloat {
			if tables[k] == nil {
				tables[k] = timeseries.NewAggregate(timeseries.Max)
			}
			tables[k].Add(ts)
		}
		for k, ts := range pg.IndexBloat {
			if indexes[k] == nil {
				indexes[k] = timeseries.NewAggregate(timeseries.Max)
			}
			indexes[k].Add(ts)
		}

		dbTotals := map[string]*timeseries.Aggregate{}
		addSum := func(db string, ts *timeseries.TimeSeries) {
			if dbTotals[db] == nil {
				dbTotals[db] = timeseries.NewAggregate(timeseries.NanSum)
			}
			dbTotals[db].Add(ts)
		}
		for db, ts := range pg.DatabaseTableBloat {
			addSum(db, ts)
		}
		for db, ts := range pg.DatabaseIndexBloat {
			addSum(db, ts)
		}
		dbData := map[string]model.SeriesData{}
		for db, a := range dbTotals {
			dbData[db] = a.Get()
		}
		tblData := map[string]model.SeriesData{}
		for k, ts := range pg.TableBloat {
			tblData[k.String()] = ts
		}
		idxData := map[string]model.SeriesData{}
		for k, ts := range pg.IndexBloat {
			idxData[k.String()] = ts
		}
		report.GetOrCreateChartInGroup(pgBloatByDbChartTitle, i.Name, nil).
			Group("Storage", 8).
			Stacked().
			Sorted().
			AddMany(dbData, 10, timeseries.Max)
		report.GetOrCreateChartInGroup(pgTopBloatTablesChartTitle, i.Name, nil).
			Group("Storage", 8).
			Stacked().
			Sorted().
			AddMany(tblData, 10, timeseries.Max)
		report.GetOrCreateChartInGroup(pgTopBloatIndexesChartTitle, i.Name, nil).
			Group("Storage", 8).
			Stacked().
			Sorted().
			AddMany(idxData, 10, timeseries.Max)
	}
	if len(tableTotal) == 0 && len(indexTotal) == 0 {
		return
	}

	for db := range dbSize {
		size := dbSize[db].Get().Last()
		if timeseries.IsNaN(size) || size <= 0 {
			continue
		}
		bloat := pgLast(tableTotal[db], 0) + pgLast(indexTotal[db], 0)
		if bloat < pgBloatMinBytes {
			continue
		}
		ratio := bloat / size * 100
		if ratio > check.Value() {
			check.SetValue(ratio)
		}
		if ratio <= check.Threshold {
			continue
		}
		check.AddItem("%s", db)
		detail := fmt.Sprintf("%s is %.0f%% bloated (~%s wasted)", db, ratio, pgFormatBytes(bloat))
		if t, b := pgTopBloatInDb(tables, db); t != "" {
			detail += fmt.Sprintf("; table %s ~%s", t, pgFormatBytes(b))
		}
		if ix, b := pgTopBloatIndexInDb(indexes, db); ix != "" {
			detail += fmt.Sprintf("; index %s ~%s", ix, pgFormatBytes(b))
		}
		detail += " - reclaim with pg_repack (online) or VACUUM FULL (locks)"
		check.AddDetail("%s", detail)
	}
}

func pgLast(a *timeseries.Aggregate, defaultValue float32) float32 {
	if a == nil {
		return defaultValue
	}
	if v := a.Get().Last(); !timeseries.IsNaN(v) {
		return v
	}
	return defaultValue
}

func pgTopBloatInDb(tables map[model.DbTableKey]*timeseries.Aggregate, db string) (string, float32) {
	name, max := "", float32(0)
	for k, a := range tables {
		if k.Db != db {
			continue
		}
		if v := pgLast(a, 0); v > max {
			name, max = k.Table, v
		}
	}
	return name, max
}

func pgTopBloatIndexInDb(indexes map[model.DbIndexKey]*timeseries.Aggregate, db string) (string, float32) {
	name, max := "", float32(0)
	for k, a := range indexes {
		if k.Db != db {
			continue
		}
		if v := pgLast(a, 0); v > max {
			name, max = k.Index, v
		}
	}
	return name, max
}

func pgFormatBytes(v float32) string {
	value, unit := utils.FormatBytes(v)
	return value + unit
}

const pgDeadTupleMinBytes = 512 << 20 // 512 MiB
const pgAutovacuumRecentSeconds = 10 * 60

func pgAutovacuum(report *model.AuditReport, instances []*model.Instance, check *model.Check) {
	for _, i := range instances {
		pg := i.Postgres
		if pg == nil {
			continue
		}
		vacThreshold, ok1 := pgSettingFloat(pg, "autovacuum_vacuum_threshold")
		vacScale, ok2 := pgSettingFloat(pg, "autovacuum_vacuum_scale_factor")
		settingsOK := ok1 && ok2
		xminHolder, _ := pgTopXminHolder(pg)

		maxWorkers, hasMaxWorkers := pgSettingFloat(pg, "autovacuum_max_workers")
		workersSaturated := false
		if hasMaxWorkers && maxWorkers > 0 {
			if avg := pg.AutovacuumWorkers.LastNAvg(3, 0); !timeseries.IsNaN(avg) && avg >= maxWorkers-0.5 {
				workersSaturated = true
			}
		}
		if !pg.AutovacuumWorkers.IsEmpty() {
			if g := report.GetOrCreateChartGroup(pgAutovacuumWorkersTitle, nil).Group("Vacuum", 7); g != nil {
				g.GetOrCreateChart(i.Name).
					Stacked().
					AddSeries("workers running", pg.AutovacuumWorkers).
					SetThreshold("autovacuum_max_workers", pg.Settings["autovacuum_max_workers"].Samples)
			}
		}
		inProgress, throttledVacuums := 0, 0
		for tk, ts := range pg.TableVacuumInProgress {
			if ts.Last() == 1 {
				inProgress++
				if pg.TableVacuumThrottled[tk].Average() >= 0.5 {
					throttledVacuums++
				}
			}
		}
		workersThrottled := inProgress > 0 && throttledVacuums*2 >= inProgress

		deadData := map[string]model.SeriesData{}
		pressureData := map[string]model.SeriesData{}
		sinceData := map[string]model.SeriesData{}
		var worst struct {
			table    string
			dead     float32
			pressure float32
			cause    string
			found    bool
		}
		for k, deadTs := range pg.TableDeadTupleBytes {
			deadData[k.String()] = deadTs
			if ts := pg.TableSecondsSinceAutovacuum[k]; ts != nil {
				sinceData[k.String()] = ts
			}
			if !settingsOK {
				continue
			}
			nd, reltuples := pg.TableDeadTuples[k], pg.TableReltuples[k]
			if nd.IsEmpty() || reltuples == nil || reltuples.IsEmpty() {
				continue
			}
			effThreshold, effScale := vacThreshold, vacScale
			if v := pgTableSetting(pg, k, "autovacuum_vacuum_threshold"); !timeseries.IsNaN(v) {
				effThreshold = v
			}
			if v := pgTableSetting(pg, k, "autovacuum_vacuum_scale_factor"); !timeseries.IsNaN(v) {
				effScale = v
			}
			pTs := timeseries.Div(nd, reltuples.Map(func(_ timeseries.Time, v float32) float32 {
				return effThreshold + effScale*v
			}))
			pressureData[k.String()] = pTs

			dead := deadTs.Last()
			if timeseries.IsNaN(dead) || dead < pgDeadTupleMinBytes {
				continue
			}
			pLast := pTs.Last()
			if timeseries.IsNaN(pLast) {
				continue
			}
			if pLast > check.Value() {
				check.SetValue(pLast)
			}
			if pLast < check.Threshold || (worst.found && dead <= worst.dead) {
				continue
			}
			cause := ""
			avTs := pg.TableSecondsSinceAutovacuum[k]
			hasAge := avTs != nil && !timeseries.IsNaN(avTs.Last())
			recentlyVacuumed := hasAge && avTs.Last() < pgAutovacuumRecentSeconds
			switch {
			case pgTableSetting(pg, k, "autovacuum_disabled") == 1:
				cause = "; autovacuum is disabled on this table (autovacuum_enabled=false)"
			case recentlyVacuumed && xminHolder != "":
				cause = fmt.Sprintf("; a %s is blocking cleanup (holds the vacuum horizon)", pgXminHolderLabel(xminHolder))
				check.AddWidget(report.GetOrCreateChartGroup(pgXminHoldersChartTitle, nil).Widget())
				if xminHolder == "replication_slot" {
					check.AddWidget(report.GetOrCreateChartGroup(pgReplicationSlotsTitle, nil).Widget())
				} else {
					check.AddWidget(report.GetOrCreateChartGroup(pgIdleTransactionsTitle, nil).Widget())
					check.AddWidget(report.GetOrCreateChartGroup(pgActiveQueriesTitle, nil).Widget())
				}
			case pg.TableVacuumInProgress[k] != nil && pg.TableVacuumInProgress[k].Last() == 1:
				if pg.TableVacuumThrottled[k].Average() < 0.5 {
					cause = "; vacuum running but too slow (large table or slow storage)"
				} else if o := pgTableCostOverride(pg, k); o != "" {
					cause = fmt.Sprintf("; vacuum throttled by this table's %s, adjust the cost settings", o)
				} else {
					cause = "; vacuum throttled by the cost limits - tune autovacuum_vacuum_cost_delay/limit"
				}
			case workersSaturated && workersThrottled && !recentlyVacuumed:
				if o := pgTableCostOverride(pg, k); o != "" {
					cause = fmt.Sprintf("; workers busy but throttled by this table's %s, adjust the cost settings (more workers won't help)", o)
				} else {
					cause = "; workers busy but throttled by the cost limits, tune autovacuum_vacuum_cost_delay/limit (more workers won't help)"
				}
			case workersSaturated && !recentlyVacuumed:
				cause = fmt.Sprintf("; all %.0f autovacuum workers are busy, raise autovacuum_max_workers", maxWorkers)
			case hasAge:
				cause = fmt.Sprintf("; autovacuum last ran %s ago", utils.FormatDurationShort(timeseries.Duration(avTs.Last()), 1))
			}
			worst.table, worst.dead, worst.pressure, worst.cause, worst.found = k.String(), dead, pLast, cause, true
		}
		report.GetOrCreateChartInGroup(pgAutovacuumPressureTitle, i.Name, nil).
			Group("Vacuum", 7).
			Sorted().
			AddMany(pressureData, 10, timeseries.Max)

		report.GetOrCreateChartInGroup(pgDeadTuplesByTableTitle, i.Name, nil).
			Group("Vacuum", 7).
			Sorted().
			Stacked().
			AddMany(deadData, 10, timeseries.Max)

		report.GetOrCreateChartInGroup(pgTimeSinceAutovacuumTitle, i.Name, nil).
			Group("Vacuum", 7).
			Sorted().
			AddMany(sinceData, 10, timeseries.Max)

		throttledData := map[string]model.SeriesData{}
		for k, ts := range pg.TableVacuumThrottled {
			throttledData[k.String()] = ts
		}
		report.GetOrCreateChartInGroup(pgThrottledByTableTitle, i.Name, nil).
			Group("Vacuum", 7).
			Stacked().
			Sorted().
			AddMany(throttledData, 10, timeseries.Max)

		if worst.found {
			check.AddItem("%s", i.Name)
			check.AddDetail("%s: %s: %.0fx over the autovacuum trigger threshold, ~%s of dead rows%s", i.Name, worst.table, worst.pressure, pgFormatBytes(worst.dead), worst.cause)
		}
	}
}

func pgTableCostOverride(pg *model.Postgres, k model.DbTableKey) string {
	if cd := pgTableSetting(pg, k, "autovacuum_vacuum_cost_delay"); cd > 0 {
		s := fmt.Sprintf("cost_delay=%.0fms", cd)
		if cl := pgTableSetting(pg, k, "autovacuum_vacuum_cost_limit"); !timeseries.IsNaN(cl) {
			s += fmt.Sprintf(", cost_limit=%.0f", cl)
		}
		return s
	}
	return ""
}

func pgTableSetting(pg *model.Postgres, k model.DbTableKey, name string) float32 {
	if m := pg.TableSettings[k]; m != nil {
		if v, ok := m[name]; ok {
			return v
		}
	}
	return timeseries.NaN
}

const pgStaleStatsMinRows = 100000

func pgStaleStatistics(report *model.AuditReport, instances []*model.Instance, check *model.Check) {
	for _, i := range instances {
		pg := i.Postgres
		if pg == nil {
			continue
		}
		anThreshold, ok1 := pgSettingFloat(pg, "autovacuum_analyze_threshold")
		anScale, ok2 := pgSettingFloat(pg, "autovacuum_analyze_scale_factor")
		settingsOK := ok1 && ok2

		pressureData := map[string]model.SeriesData{}
		sinceData := map[string]model.SeriesData{}
		var worst struct {
			table    string
			pressure float32
			cause    string
			found    bool
		}
		for k, modsTs := range pg.TableModsSinceAnalyze {
			if ts := pg.TableSecondsSinceAnalyze[k]; ts != nil {
				sinceData[k.String()] = ts
			}
			if !settingsOK {
				continue
			}
			reltuples := pg.TableReltuples[k]
			if modsTs.IsEmpty() || reltuples == nil || reltuples.IsEmpty() {
				continue
			}
			effThreshold, effScale := anThreshold, anScale
			if v := pgTableSetting(pg, k, "autovacuum_analyze_threshold"); !timeseries.IsNaN(v) {
				effThreshold = v
			}
			if v := pgTableSetting(pg, k, "autovacuum_analyze_scale_factor"); !timeseries.IsNaN(v) {
				effScale = v
			}
			pTs := timeseries.Div(modsTs, reltuples.Map(func(_ timeseries.Time, v float32) float32 {
				return effThreshold + effScale*v
			}))
			pressureData[k.String()] = pTs

			rows := reltuples.Last()
			if timeseries.IsNaN(rows) || rows < pgStaleStatsMinRows {
				continue
			}
			pLast := pTs.Last()
			if timeseries.IsNaN(pLast) {
				continue
			}
			if pLast > check.Value() {
				check.SetValue(pLast)
			}
			if pLast < check.Threshold || (worst.found && pLast <= worst.pressure) {
				continue
			}
			cause := "; run ANALYZE"
			if pgTableSetting(pg, k, "autovacuum_disabled") == 1 {
				cause = "; autoanalyze is disabled on this table (autovacuum_enabled=false), run ANALYZE"
			} else if ts := pg.TableSecondsSinceAnalyze[k]; ts != nil && !timeseries.IsNaN(ts.Last()) {
				cause = fmt.Sprintf("; last analyzed %s ago, run ANALYZE", utils.FormatDurationShort(timeseries.Duration(ts.Last()), 1))
			}
			worst.table, worst.pressure, worst.cause, worst.found = k.String(), pLast, cause, true
		}
		report.GetOrCreateChartInGroup(pgAnalyzePressureTitle, i.Name, nil).
			Group("Vacuum", 7).
			Sorted().
			AddMany(pressureData, 10, timeseries.Max)
		report.GetOrCreateChartInGroup(pgTimeSinceAnalyzeTitle, i.Name, nil).
			Group("Vacuum", 7).
			Sorted().
			AddMany(sinceData, 10, timeseries.Max)
		if worst.found {
			check.AddItem("%s", i.Name)
			check.AddDetail("%s: %s: %.0fx over the autoanalyze threshold, planner statistics are stale%s", i.Name, worst.table, worst.pressure, worst.cause)
		}
	}
}

func pgSettingFloat(pg *model.Postgres, name string) (float32, bool) {
	s, ok := pg.Settings[name]
	if !ok || s.Samples.IsEmpty() {
		return 0, false
	}
	if v := s.Samples.Last(); !timeseries.IsNaN(v) {
		return v, true
	}
	return 0, false
}

func pgTopXminHolder(pg *model.Postgres) (string, float32) {
	holder, age := "", float32(0)
	for h, ts := range pg.OldestXminAge {
		if last := ts.Last(); !timeseries.IsNaN(last) && last > age {
			holder, age = h, last
		}
	}
	return holder, age
}

func pgWraparound(report *model.AuditReport, instance *model.Instance, check *model.Check) {
	pg := instance.Postgres

	xidAge, worstXidDb := pgMaxByKey(pg.XidAge)
	mxidAge, worstMxidDb := pgMaxByKey(pg.MultixactAge)

	report.
		GetOrCreateChart(pgXidAgeChartTitle, nil).
		Group("Vacuum", 7).
		AddSeries(instance.Name, xidAge).
		SetThreshold("autovacuum_freeze_max_age", pg.Settings["autovacuum_freeze_max_age"].Samples)
	if !mxidAge.IsEmpty() {
		report.
			GetOrCreateChart(pgMultixactAgeChartTitle, nil).
			Group("Vacuum", 7).
			AddSeries(instance.Name, mxidAge).
			SetThreshold("autovacuum_multixact_freeze_max_age", pg.Settings["autovacuum_multixact_freeze_max_age"].Samples)
	}

	holders := report.GetOrCreateChartInGroup(pgXminHoldersChartTitle, instance.Name, nil).Group("Vacuum", 7)
	for _, h := range []string{"running_transaction", "standby_feedback", "replication_slot", "prepared_transaction"} {
		holders.AddSeries(pgXminHolderLabel(h), pg.OldestXminAge[h])
	}

	kind, worstDb, worstAge := "transaction ID", worstXidDb, xidAge.Last()
	if m := mxidAge.Last(); !timeseries.IsNaN(m) && (timeseries.IsNaN(worstAge) || m > worstAge) {
		kind, worstDb, worstAge = "multixact ID", worstMxidDb, m
	}
	if timeseries.IsNaN(worstAge) {
		return
	}
	pct := worstAge / pgWraparoundLimit * 100
	if pct > check.Value() {
		check.SetValue(pct)
	}
	if pct <= check.Threshold {
		return
	}
	check.AddItem("%s", instance.Name)
	detail := fmt.Sprintf("%s: database %q is %.0f%% toward %s wraparound (age %s)", instance.Name, worstDb, pct, kind, pgFormatCount(worstAge))
	if kind == "transaction ID" {
		detail += "; " + pgDominantXminHolder(pg, worstAge)
	}
	check.AddDetail("%s", detail)
}

func pgMaxByKey(m map[string]*timeseries.TimeSeries) (*timeseries.TimeSeries, string) {
	agg := timeseries.NewAggregate(timeseries.Max)
	worstKey, worstLast := "", float32(0)
	for k, ts := range m {
		agg.Add(ts)
		if last := ts.Last(); !timeseries.IsNaN(last) && (worstKey == "" || last > worstLast) {
			worstKey, worstLast = k, last
		}
	}
	return agg.Get(), worstKey
}

func pgDominantXminHolder(pg *model.Postgres, frozenAge float32) string {
	holder, holderAge := pgTopXminHolder(pg)
	if holder == "" || holderAge < frozenAge*0.8 {
		return "autovacuum is not freezing fast enough, check autovacuum settings and dead tuples"
	}
	switch holder {
	case "replication_slot":
		return "a replication slot is holding the oldest transaction, drop or advance the lagging/inactive slot"
	case "running_transaction":
		return "a long-running transaction is holding the oldest transaction - end it"
	case "prepared_transaction":
		return "a prepared transaction is holding the oldest transaction, commit or roll it back"
	case "standby_feedback":
		return "a standby with hot_standby_feedback is holding the oldest transaction"
	}
	return ""
}

func pgXminHolderLabel(holder string) string {
	switch holder {
	case "running_transaction":
		return "running transaction"
	case "standby_feedback":
		return "standby feedback"
	case "replication_slot":
		return "replication slot"
	case "prepared_transaction":
		return "prepared transaction"
	}
	return holder
}

func pgFormatCount(v float32) string {
	switch {
	case v >= 1e9:
		return fmt.Sprintf("%.2fB", v/1e9)
	case v >= 1e6:
		return fmt.Sprintf("%.0fM", v/1e6)
	default:
		return utils.FormatFloat(v)
	}
}

func checkReplicationLag(instance *model.Instance, instances []*model.Instance, primaryLsn, lag *timeseries.TimeSeries, role model.ClusterRole, check *model.Check) *model.TableCell {
	res := &model.TableCell{}
	if primaryLsn.IsEmpty() {
		return res
	}
	if role != model.ClusterRoleReplica {
		return res
	}
	last := lag.Last()
	if timeseries.IsNaN(last) {
		return res
	}

	tCurr, vCurr := primaryLsn.LastNotNull()
	t, tPast, vPast := timeseries.Time(0), timeseries.Time(0), timeseries.NaN
	iter := primaryLsn.Iter()
	for iter.Next() {
		t, vPast = iter.Value()
		if vPast > vCurr { // wraparound (e.g., complete cluster redeploy)
			continue
		}
		if vPast > vCurr-last {
			break
		}
		tPast = t
	}

	lagTime := tCurr.Sub(tPast)
	greaterThanWorldWindow := ""
	if tPast.IsZero() {
		greaterThanWorldWindow = ">"
	}
	if lagTime > timeseries.Duration(check.Threshold) {
		check.AddItem("%s", instance.Name)
		if cause := pgWalStallCause(instance, instances, primaryLsn); cause != "" {
			check.AddDetail("%s: %s", instance.Name, cause)
		}
	}
	res.Value, res.Unit = utils.FormatBytes(last)
	if lagTime > 0 {
		res.Tags = append(res.Tags,
			fmt.Sprintf("%s%s", greaterThanWorldWindow, utils.FormatDuration(lagTime, 1)))
	}
	return res
}

func pgReplicationLag(primaryLsn, replayLsn *timeseries.TimeSeries) *timeseries.TimeSeries {
	return timeseries.Aggregate2(
		primaryLsn, replayLsn,
		func(primary, replay float32) float32 {
			res := primary - replay
			if res < 0 {
				return 0
			}
			return res
		})
}

func pgConnections(report *model.AuditReport, instance *model.Instance, connectionsCheck *model.Check) {
	connectionByState := map[string]*timeseries.Aggregate{}
	var total float32
	for k, v := range instance.Postgres.Connections {
		if last := v.Last(); !timeseries.IsNaN(last) {
			total += last
		}
		state := k.State
		if k.State == "active" && k.WaitEventType == "Lock" {
			state = pgActiveLockedState
		}
		byState, ok := connectionByState[state]
		if !ok {
			byState = timeseries.NewAggregate(timeseries.NanSum)
			connectionByState[state] = byState
		}
		byState.Add(v)
	}
	connectionByState["reserved"] = timeseries.NewAggregate(timeseries.NanSum)

	for _, setting := range []string{"superuser_reserved_connections", "rds.rds_superuser_reserved_connections"} {
		v := instance.Postgres.Settings[setting].Samples
		connectionByState["reserved"].Add(v)
		if last := v.Last(); !timeseries.IsNaN(last) {
			total += last
		}
	}
	if max := instance.Postgres.Settings["max_connections"].Samples.Last(); max > 0 && total > 0 {
		if total/max*100 > connectionsCheck.Threshold {
			connectionsCheck.AddItem("%s", instance.Name)
		}
	}

	chart := report.
		GetOrCreateChartInGroup(pgConnectionsChartTitle, instance.Name, nil).
		Group("Connections", 2).
		Stacked().
		SetThreshold("max_connections", instance.Postgres.Settings["max_connections"].Samples)

	for state, v := range connectionByState {
		chart.AddSeries(state, v, pgConnectionStateColors[state])
	}

	idleInTransaction := map[string]model.SeriesData{}
	active := map[string]model.SeriesData{}
	locked := map[string]model.SeriesData{}

	for k, v := range instance.Postgres.Connections {
		switch {
		case k.State == "idle in transaction":
			idleInTransaction[k.String()] = v
		case k.State == "active" && k.WaitEventType == "Lock":
			locked[k.String()] = v
		case k.State == "active":
			active[k.String()] = v
		}
	}
	report.
		GetOrCreateChartInGroup(pgActiveQueriesTitle, instance.Name, nil).
		Group("Connections", 2).
		Stacked().
		AddMany(active, 5, timeseries.NanSum)
	report.
		GetOrCreateChartInGroup(pgIdleTransactionsTitle, instance.Name, nil).
		Group("Connections", 2).
		Stacked().
		AddMany(idleInTransaction, 5, timeseries.NanSum)
	report.
		GetOrCreateChartInGroup(pgLockedQueriesTitle, instance.Name, nil).
		Group("Locks", 3).
		Stacked().
		AddMany(locked, 5, timeseries.NanSum)
}

func pgLocks(report *model.AuditReport, instance *model.Instance) {
	blockingQueries := map[string]model.SeriesData{}
	for k, v := range instance.Postgres.AwaitingQueriesByLockingQuery {
		blockingQueries[k.Query] = v
	}
	report.
		GetOrCreateChartInGroup(pgBlockingQueriesTitle, instance.Name, nil).
		Group("Locks", 3).
		Stacked().
		AddMany(blockingQueries, 5, timeseries.NanSum).
		ShiftColors()
}

func pgCheckpoints(report *model.AuditReport, instance *model.Instance, instances []*model.Instance, primaryLsn *timeseries.TimeSeries, checkpointCheck *model.Check) {
	pg := instance.Postgres

	cpChart := report.
		GetOrCreateChartInGroup(pgCheckpointsChartTitle, instance.Name, nil).
		Group("Checkpoints", 4).
		Column().
		AddSeries("checkpoint", pg.Checkpoints).
		AddSeries("restartpoint (standby)", pg.Restartpoints)

	report.
		GetOrCreateChartInGroup(pgCheckpointTriggersTitle, instance.Name, nil).
		Group("Checkpoints", 4).
		Column().
		AddSeries("timed (by checkpoint_timeout)", pg.CheckpointsScheduledByType["timed"]).
		AddSeries("requested (by max_wal_size)", pg.CheckpointsScheduledByType["requested"])

	if chart := report.GetOrCreateChart(pgCheckpointerWriteTitle, nil).Group("Checkpoints", 4); chart != nil {
		chart.AddSeries(
			instance.Name,
			pg.BuffersWrittenBySource["checkpointer"].
				Map(func(_ timeseries.Time, v float32) float32 {
					return v * pgBlockSize
				}),
		)
	}

	if chart := report.GetOrCreateChart(pgWalToReplayTitle, nil).Group("Checkpoints", 4); chart != nil {
		chart.AddSeries(instance.Name, pg.WalSinceLastCheckpoint)
		if maxWalSize := pgSettingBytes(pg.Settings["max_wal_size"]); !maxWalSize.IsEmpty() {
			chart.SetThreshold("max_wal_size", maxWalSize)
		}
	}

	chart := report.
		GetOrCreateChart(pgTimeSinceCheckpointTitle, nil).
		Group("Checkpoints", 4).
		AddSeries(instance.Name, pg.TimeSinceLastCheckpoint)

	timeout := pg.Settings["checkpoint_timeout"].Samples
	if !timeout.IsEmpty() {
		mult := float32(checkpointCheck.Threshold)
		chart.SetThreshold(
			fmt.Sprintf("threshold (%.0f * checkpoint_timeout)", mult),
			timeout.Map(
				func(t timeseries.Time, v float32) float32 { return v * mult },
			),
		)
		if last := pg.TimeSinceLastCheckpoint.Last(); !timeseries.IsNaN(last) && last > timeout.Last()*mult {
			if wal := pg.WalSinceLastCheckpoint.Last(); wal > pgWalSegmentSizeBytes {
				checkpointCheck.AddItem("%s", instance.Name)
				if cause := pgCheckpointStallCause(instance, instances, primaryLsn); cause != "" {
					checkpointCheck.AddDetail("%s: %s", instance.Name, cause)
				}
				cpChart.Feature()
			}
		}
	}
}

func pgSettingBytes(s model.PgSetting) *timeseries.TimeSeries {
	var mult float32
	switch s.Unit {
	case "B":
		mult = 1
	case "kB":
		mult = 1 << 10
	case "8kB":
		mult = 8 << 10
	case "MB":
		mult = 1 << 20
	case "GB":
		mult = 1 << 30
	default:
		return nil
	}
	return s.Samples.Map(func(_ timeseries.Time, v float32) float32 { return v * mult })
}

func pgWalStallCause(instance *model.Instance, instances []*model.Instance, primaryLsn *timeseries.TimeSeries) string {
	pg := instance.Postgres
	if pg.WalReplayPaused.Last() > 0 {
		return "WAL replay is paused"
	}
	if instance.ClusterRoleLast() != model.ClusterRoleReplica {
		return ""
	}
	if pg.WalReceiverStatus.Last() == 0 {
		msg := "the standby is not connected to the primary"
		if primary := pgPrimaryInstance(instances); primary != nil {
			msg = fmt.Sprintf("the standby is not connected to the primary (%s)", primary.Name)
		}
		if net := pgConnectivityIssue(instance.Owner); net != "" {
			msg += " - " + net
		}
		return msg
	}
	apply := pgReplicationLag(pg.WalReceiveLsn, pg.WalReplayLsn).Last()
	shipping := pgReplicationLag(primaryLsn, pg.WalReceiveLsn).Last()
	switch {
	case apply > shipping:
		return "WAL replay is slow or blocked (check standby queries and disk I/O)"
	case shipping > apply:
		return "WAL shipping can't keep up (network or primary load)"
	}
	return ""
}

func pgCheckpointStallCause(instance *model.Instance, instances []*model.Instance, primaryLsn *timeseries.TimeSeries) string {
	if cause := pgWalStallCause(instance, instances, primaryLsn); cause != "" {
		return cause
	}
	if instance.ClusterRoleLast() != model.ClusterRolePrimary {
		return ""
	}
	if instance.Postgres.BuffersWrittenBySource["checkpointer"].Last() > 0 {
		return "a checkpoint is running but can't complete (check disk I/O)"
	}
	return "the checkpointer appears stuck"
}

func pgPrimaryInstance(instances []*model.Instance) *model.Instance {
	for _, i := range instances {
		if i.Postgres != nil && i.ClusterRoleLast() == model.ClusterRolePrimary {
			return i
		}
	}
	return nil
}

func pgConnectivityIssue(app *model.Application) string {
	if app == nil {
		return ""
	}
	for _, conns := range []map[model.ApplicationId]*model.AppToAppConnection{app.Upstreams, app.Downstreams} {
		c := conns[app.Id]
		if c == nil {
			continue
		}
		switch {
		case c.HasConnectivityIssues():
			return "no network connectivity to the primary"
		case c.HasFailedConnectionAttempts():
			return "connection attempts to the primary are failing"
		}
	}
	return ""
}

const pgDiskFindingMinGrowthBytes = 256 * 1024 * 1024
const pgDiskFindingSignificantFraction = 0.02

func pgDiskUsageFindings(instance *model.Instance, capacity float32, check *model.Check) {
	if instance.Postgres == nil {
		return
	}
	minGrowth := float32(pgDiskFindingMinGrowthBytes)
	if capacity > 0 && capacity*pgDiskFindingSignificantFraction > minGrowth {
		minGrowth = capacity * pgDiskFindingSignificantFraction
	}
	type consumer struct {
		name   string
		growth float32
		size   float32
	}
	var consumers []consumer
	for k, ts := range instance.Postgres.TableSize {
		if growth, size := seriesGrowth(ts); growth > minGrowth {
			name := "table " + k.String()
			if bloatGrowth, _ := seriesGrowth(instance.Postgres.TableBloat[k]); bloatGrowth > growth/2 {
				name += " (mostly bloat, consider reclaiming with pg_repack)"
			}
			consumers = append(consumers, consumer{name: name, growth: growth, size: size})
		}
	}
	if growth, size := seriesGrowth(instance.Postgres.WalSize); growth > minGrowth {
		name := "WAL directory"
		if pgArchivingIsFailing(instance.Postgres) {
			name = "WAL directory (archiving is failing, WAL cannot be recycled)"
		}
		consumers = append(consumers, consumer{name: name, growth: growth, size: size})
	}
	for name, slot := range instance.Postgres.ReplicationSlots {
		if growth, size := seriesGrowth(slot.RetainedWal); growth > minGrowth {
			state := "active"
			if slot.Active.Value() != "true" {
				state = "inactive"
			}
			if ws := slot.WalStatus.Value(); ws != "" && ws != "reserved" {
				state += ", " + ws
			}
			consumers = append(consumers, consumer{name: fmt.Sprintf("replication slot '%s' (%s)", name, state), growth: growth, size: size})
		}
	}
	if len(consumers) == 0 {
		for db, ts := range instance.Postgres.DatabaseSize {
			if growth, size := seriesGrowth(ts); growth > minGrowth {
				consumers = append(consumers, consumer{name: "database " + db, growth: growth, size: size})
			}
		}
	}
	slices.SortFunc(consumers, func(a, b consumer) int { return cmp.Compare(b.growth, a.growth) })
	for _, c := range consumers[:min(3, len(consumers))] {
		check.AddDetail("%s: %s grew by %s (%s total)", instance.Name, c.name,
			humanize.Bytes(uint64(c.growth)), humanize.Bytes(uint64(c.size)))
	}
}

const pgIOFindingMinWait = 0.5
const pgIOFindingMinWriteShare = 0.3

func pgIOFindings(instance *model.Instance, disk *model.DiskStats, ioLoad float32, check *model.Check) {
	pg := instance.Postgres
	if pg == nil || ioLoad <= 0 {
		return
	}
	var topKey model.QueryKey
	var topIO float32
	for k, stat := range pg.PerQuery {
		if io := stat.IoTime.LastNAvg(5, 0); io > topIO {
			topIO, topKey = io, k
		}
	}
	if topIO > pgIOFindingMinWait {
		check.AddDetail("%s: the most I/O time consuming query → %s", instance.Name, utils.TruncateUtf8(topKey.Query, 80))
	}
	if disk == nil {
		return
	}
	w, r := disk.WrittenBytes.LastNAvg(5, 0), disk.ReadBytes.LastNAvg(5, 0)
	if w <= 0 || w <= r {
		return
	}
	source, bps := "WAL", pg.WalThroughput.LastNAvg(5, 0)
	if ckpt := pg.BuffersWrittenBySource["checkpointer"].LastNAvg(5, 0) * pgBlockSize; ckpt > bps {
		source, bps = "checkpointer flushes", ckpt
	}
	if bps < w*pgIOFindingMinWriteShare {
		return
	}
	check.AddDetail("%s: disk writes are mostly %s (%s/s of %s/s written)", instance.Name, source, pgFormatBytes(bps), pgFormatBytes(w))
}

func pgArchivingIsFailing(pg *model.Postgres) bool {
	return pg.WalArchivingStatus.Last() == 0
}

func seriesGrowth(ts *timeseries.TimeSeries) (float32, float32) {
	if ts.IsEmpty() {
		return timeseries.NaN, timeseries.NaN
	}
	first := timeseries.NaN
	iter := ts.Iter()
	for iter.Next() {
		if _, v := iter.Value(); !timeseries.IsNaN(v) {
			first = v
			break
		}
	}
	_, last := ts.LastNotNull()
	return last - first, last
}

func pgQueries(report *model.AuditReport, instance *model.Instance) {
	totalTime := map[string]model.SeriesData{}
	ioTime := map[string]model.SeriesData{}
	for k, stat := range instance.Postgres.PerQuery {
		q := k.String()
		totalTime[q] = stat.TotalTime
		ioTime[q] = stat.IoTime
	}
	report.
		GetOrCreateChartInGroup(pgQueriesByTotalTimeTitle, instance.Name, nil).
		Group("Queries", 1).
		Stacked().
		Sorted().
		AddMany(totalTime, 5, timeseries.NanSum)
	report.
		GetOrCreateChartInGroup(pgQueriesByIOTimeTitle, instance.Name, nil).
		Group("Queries", 1).
		Stacked().
		Sorted().
		AddMany(ioTime, 5, timeseries.NanSum)
}

func sumQueries(byDB map[string]*timeseries.TimeSeries) *timeseries.TimeSeries {
	total := timeseries.NewAggregate(timeseries.NanSum)
	for _, qps := range byDB {
		total.Add(qps)
	}
	return total.Get()
}

func dbSizeCell(databaseSize map[string]*timeseries.TimeSeries) *model.TableCell {
	cell := model.NewTableCell()
	var total float32
	for _, ts := range databaseSize {
		if v := ts.Last(); !timeseries.IsNaN(v) {
			total += v
		}
	}
	if total > 0 {
		v, u := utils.FormatBytes(total)
		cell.SetValue(v).SetUnit(u)
	}
	return cell
}
