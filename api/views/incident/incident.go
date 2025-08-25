package incident

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

type Incident struct {
	model.ApplicationIncident
	Impact              float32                   `json:"impact"`
	ShortDescription    string                    `json:"short_description"`
	ApplicationCategory model.ApplicationCategory `json:"application_category"`
	Duration            timeseries.Duration       `json:"duration"`
}

func RenderList(w *model.World, incidents []*model.ApplicationIncident) []Incident {
	res := make([]Incident, 0, len(incidents))

	for _, i := range incidents {
		res = append(res, renderIncident(w, i))
	}
	return res
}

func renderIncident(w *model.World, incident *model.ApplicationIncident) Incident {
	i := Incident{
		ApplicationIncident: *incident,
		ShortDescription:    incident.ShortDescription(),
		Impact: max(
			incident.Details.LatencyImpact.AffectedRequestPercentage,
			incident.Details.AvailabilityImpact.AffectedRequestPercentage,
		),
	}
	i.ApplicationCategory = model.ApplicationCategoryApplication
	if app := w.GetApplication(i.ApplicationId); app != nil {
		i.ApplicationCategory = app.Category
	}
	to := timeseries.Now()
	if i.Resolved() {
		to = i.ResolvedAt
	}
	i.Duration = to.Sub(i.OpenedAt)
	if i.RCA != nil && i.RCA.Status == "" && i.RCA.RootCause != "" {
		i.RCA.Status = "OK"
	}
	return i
}

type SLODetails struct {
	Objective  string  `json:"objective"`
	Compliance string  `json:"compliance"`
	Violated   bool    `json:"violated"`
	Threshold  float32 `json:"threshold"`
}

type View struct {
	Incident
	AvailabilitySLO *SLODetails     `json:"availability_slo,omitempty"`
	LatencySLO      *SLODetails     `json:"latency_slo,omitempty"`
	ActualFrom      timeseries.Time `json:"actual_from"`
	ActualTo        timeseries.Time `json:"actual_to"`

	Widgets []*model.Widget `json:"widgets"`
}

func Render(w *model.World, app *model.Application, incident *model.ApplicationIncident) *View {
	to := timeseries.Now()
	if incident.Resolved() {
		to = incident.ResolvedAt
	}
	v := &View{
		Incident: renderIncident(w, incident),
		ActualTo: to,
		Widgets:  incidentWidgets(w, app),
	}
	if len(app.AvailabilitySLIs) > 0 {
		sli := app.AvailabilitySLIs[0]
		v.AvailabilitySLO = &SLODetails{
			Objective:  utils.FormatPercentage(sli.Config.ObjectivePercentage) + " of requests should not fail",
			Compliance: "100%",
		}
		for _, br := range incident.Details.AvailabilityBurnRates {
			if br.Severity > model.OK {
				if t := incident.OpenedAt.Add(-br.ShortWindow); v.ActualFrom.IsZero() || t.After(v.ActualFrom) {
					v.ActualFrom = t
				}
				v.AvailabilitySLO.Violated = true
				v.AvailabilitySLO.Compliance = utils.FormatPercentage(100 - br.LongWindowPercentage)
				break
			}
		}
	}

	if len(app.LatencySLIs) > 0 {
		sli := app.LatencySLIs[0]
		v.LatencySLO = &SLODetails{
			Objective:  utils.FormatPercentage(sli.Config.ObjectivePercentage) + " of requests should be served faster than " + utils.FormatLatency(sli.Config.ObjectiveBucket),
			Compliance: "100%",
			Threshold:  sli.Config.ObjectiveBucket,
		}
		for _, br := range incident.Details.LatencyBurnRates {
			if v.ActualFrom.IsZero() {
				v.ActualFrom = incident.OpenedAt.Add(-br.ShortWindow)
			}
			if br.Severity > model.OK {
				if t := incident.OpenedAt.Add(-br.ShortWindow); v.ActualFrom.IsZero() || t.After(v.ActualFrom) {
					v.ActualFrom = t
				}
				v.LatencySLO.Violated = true
				v.LatencySLO.Compliance = utils.FormatPercentage(100 - br.LongWindowPercentage)
				break
			}
		}
	}
	for _, widget := range v.Widgets {
		widget.AddAnnotation(model.Annotation{Name: "incident", X1: v.ActualFrom, X2: to})
	}
	return v
}

func incidentWidgets(w *model.World, app *model.Application) []*model.Widget {
	var res []*model.Widget
	if len(app.LatencySLIs) > 0 {
		ch := model.NewChart(w.Ctx, "Latency, seconds").
			PercentilesFrom(app.LatencySLIs[0].Histogram, 0.25, 0.5, 0.75, 0.95, 0.99)
		res = append(res, &model.Widget{Chart: ch})
	}
	if len(app.AvailabilitySLIs) > 0 {
		res = append(res, &model.Widget{
			Chart: model.NewChart(w.Ctx, "Errors, per second").
				AddSeries("errors", app.AvailabilitySLIs[0].FailedRequests.Map(timeseries.NanToZero), "black").
				Stacked(),
		})
	}
	return res
}
