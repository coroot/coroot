package incident

import (
	"github.com/coroot/coroot/model"
)

type View struct {
	Summary
	HeatMap *model.Widget `json:"heatmap"`
}

func Render(w *model.World, app *model.Application, incident *model.ApplicationIncident) *View {
	v := &View{Summary: CalcSummary(w, app, incident), HeatMap: getHeatMap(app)}
	if v.HeatMap == nil {
		return nil
	}
	v.HeatMap.AddAnnotation(model.Annotation{Name: "incident", X1: v.ActualFrom, X2: v.ActualTo})
	return v
}

func getHeatMap(app *model.Application) *model.Widget {
	var sloReport *model.AuditReport
	for _, r := range app.Reports {
		if r.Name == model.AuditReportSLO {
			sloReport = r
		}
	}
	if sloReport == nil {
		return nil
	}
	for _, w := range sloReport.Widgets {
		if w.Heatmap != nil {
			return w
		}
	}
	return nil
}
