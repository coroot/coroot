package auditor

import (
	"math"
	"testing"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func TestPgCheckpointsExcludesArchivePadding(t *testing.T) {
	const from = timeseries.Time(10000)
	const step = timeseries.Duration(5)
	const points = 21
	constant := func(value float32) *timeseries.TimeSeries {
		values := make([]float32, points)
		for i := range values {
			values[i] = value
		}
		return timeseries.NewWithData(from, step, values)
	}
	stale := timeseries.NewWithData(from, step, []float32{16777064})
	future := timeseries.NewWithData(from.Add(105), step, []float32{16777064})
	tests := []struct {
		name         string
		recordBytes  *timeseries.TimeSeries
		attempts     float32
		wantFindings int
	}{
		{name: "idle_primary_archive_padding", recordBytes: constant(16777064), attempts: 1, wantFindings: 0},
		{name: "idle_standby_with_no_primary_checkpoint_attempts", recordBytes: constant(16777064), wantFindings: 0},
		{name: "record_distance_at_existing_segment_boundary", recordBytes: constant(16777216), wantFindings: 0},
		{name: "record_distance_above_existing_segment_boundary", recordBytes: constant(16777218), wantFindings: 1},
		{name: "stalled_checkpointer", recordBytes: constant(75332296), wantFindings: 1},
		{name: "failed_attempts_with_real_wal_debt", recordBytes: constant(75332296), attempts: 1, wantFindings: 1},
		{name: "missing_record_metric", attempts: 1, wantFindings: 1},
		{name: "missing_record_samples", recordBytes: constant(timeseries.NaN), attempts: 1, wantFindings: 1},
		{name: "invalid_infinite_distance", recordBytes: constant(float32(math.Inf(1))), attempts: 1, wantFindings: 1},
		{name: "negative_distance", recordBytes: constant(-1), attempts: 1, wantFindings: 1},
		{name: "stale_record_distance", recordBytes: stale, attempts: 1, wantFindings: 1},
		{name: "future_record_distance", recordBytes: future, attempts: 1, wantFindings: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pg := model.NewPostgres()
			pg.Settings["checkpoint_timeout"] = model.PgSetting{Samples: constant(30)}
			ages := make([]float32, points)
			for i := range ages {
				ages[i] = float32(i * 5)
			}
			pg.TimeSinceLastCheckpoint = timeseries.NewWithData(from, step, ages)
			// Native PG18 archive-timeout reproduction: the REDO pointer is
			// over one segment behind while distance after the checkpoint
			// record remains below a segment. Attempts alone can also advance
			// on unsuccessful checkpoints and must not clear real WAL debt.
			pg.WalSinceLastCheckpoint = constant(33150320)
			pg.WalSinceLastCheckpointRecord = test.recordBytes
			pg.Checkpoints = constant(0)
			pg.CheckpointsScheduledByType["timed"] = constant(test.attempts)
			instance := &model.Instance{Name: "fixture", Postgres: pg}
			report := model.NewAuditReport(&model.Application{}, timeseries.NewContext(from, from.Add(100), step), nil, model.AuditReportPostgres, true)
			check := report.CreateCheck(model.Checks.PostgresCheckpoint)
			check.Threshold = 3
			pgCheckpoints(report, instance, []*model.Instance{instance}, nil, check)
			if got := check.Items().Len(); got != test.wantFindings {
				t.Fatalf("checkpoint findings = %d, want %d", got, test.wantFindings)
			}
		})
	}
}
