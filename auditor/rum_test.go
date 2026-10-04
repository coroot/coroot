package auditor

import (
	"testing"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditRumClientReport(t *testing.T) {
	w := model.NewWorld(0, 600, timeseries.Minute, timeseries.Minute)
	id := model.NewApplicationId(model.ClusterIdExternal, "frontend", model.ApplicationKindRumClient, "demo-web")
	app := w.GetOrCreateApplication(id, true)
	app.RumStats = &model.RumStats{
		LcpP75: timeseries.NewWithData(0, timeseries.Minute, []float32{3000}),
	}

	Audit(w, &db.Project{}, nil, nil)

	var rumReport *model.AuditReport
	for _, r := range app.Reports {
		if r.Name == model.AuditReportRum {
			rumReport = r
			break
		}
	}
	require.NotNil(t, rumReport)
	assert.NotEmpty(t, rumReport.Checks)
	assert.NotEmpty(t, rumReport.Widgets)
}
