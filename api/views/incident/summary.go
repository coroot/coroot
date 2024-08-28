package incident

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

type SLODetails struct {
	Objective  string  `json:"objective"`
	Compliance string  `json:"compliance"`
	Violated   bool    `json:"violated"`
	Threshold  float32 `json:"threshold"`
}

type Summary struct {
	model.ApplicationIncident
	ApplicationCategory        model.ApplicationCategory `json:"application_category"`
	Duration                   timeseries.Duration       `json:"duration"`
	AffectedRequestPercent     float32                   `json:"affected_request_percent"`
	ErrorBudgetConsumedPercent float32                   `json:"error_budget_consumed_percent"`
	AvailabilitySLO            *SLODetails               `json:"availability_slo,omitempty"`
	LatencySLO                 *SLODetails               `json:"latency_slo,omitempty"`

	ActualFrom timeseries.Time `json:"actual_from"`
	ActualTo   timeseries.Time `json:"actual_to"`

	failedRequests            float32
	failedRequestsPercent     float32
	failedRequestsErrorBudget float32

	slowRequests            float32
	slowRequestsPercent     float32
	slowRequestsErrorBudget float32
}

func (s *Summary) errorBudgetConsumedPercent() float32 {
	var vLatency, vFailedRequests float32
	if s.slowRequestsErrorBudget > 0 {
		vLatency = s.slowRequests / s.slowRequestsErrorBudget * 100
	}
	if s.failedRequestsErrorBudget > 0 {
		vFailedRequests = s.failedRequests / s.failedRequestsErrorBudget * 100
	}
	if vFailedRequests > vLatency {
		return vFailedRequests
	}
	return vLatency
}

func CalcSummary(w *model.World, app *model.Application, i *model.ApplicationIncident) Summary {
	from := i.OpenedAt
	to := w.Ctx.To
	if i.Resolved() {
		to = i.ResolvedAt
	}
	var summary Summary
	for _, r := range model.AlertRules {
		summary = Summary{
			ActualFrom: from.Add(-r.ShortWindow),
			ActualTo:   to,
			Duration:   to.Sub(from),
		}
		burnRate := float32(0)

		if len(app.AvailabilitySLIs) > 0 {
			sli := app.AvailabilitySLIs[0]
			errorsIter := sli.FailedRequestsRaw.IterFrom(summary.ActualFrom)
			totalIter := sli.TotalRequestsRaw.IterFrom(summary.ActualFrom)
			var total float32
			for totalIter.Next() {
				var e float32
				if errorsIter.Next() {
					_, e = errorsIter.Value()
				}
				ts, t := totalIter.Value()
				if ts > to {
					break
				}
				if t > 0 {
					total += t * float32(w.Ctx.RawStep)
				}
				if e > 0 {
					summary.failedRequests += e * float32(w.Ctx.RawStep)
				}
			}
			if total > 0 {
				summary.failedRequestsPercent = summary.failedRequests / total * 100
				summary.failedRequestsErrorBudget = total * (100 - sli.Config.ObjectivePercentage) / 100
				objective := 1 - sli.Config.ObjectivePercentage/100
				if br := summary.failedRequests / total / objective; br > burnRate {
					burnRate = br
				}
				summary.AvailabilitySLO = &SLODetails{
					Objective:  utils.FormatPercentage(sli.Config.ObjectivePercentage) + " of requests should not fail",
					Compliance: utils.FormatPercentage(100 - summary.failedRequestsPercent),
					Violated:   (100 - summary.failedRequestsPercent) < sli.Config.ObjectivePercentage,
				}
			}
		}
		if len(app.LatencySLIs) > 0 {
			sli := app.LatencySLIs[0]
			totalTs, fastTs := sli.GetTotalAndFast(true)
			slowIter := timeseries.Sub(totalTs, fastTs).IterFrom(summary.ActualFrom)
			totalIter := totalTs.IterFrom(summary.ActualFrom)
			var total float32
			for totalIter.Next() {
				var s float32
				if slowIter.Next() {
					_, s = slowIter.Value()
				}
				ts, t := totalIter.Value()
				if ts > to {
					break
				}
				if t > 0 {
					total += t * float32(w.Ctx.RawStep)
				}
				if s > 0 {
					summary.slowRequests += s * float32(w.Ctx.RawStep)
				}
			}
			if total > 0 {
				summary.slowRequestsPercent = summary.slowRequests / total * 100
				summary.slowRequestsErrorBudget = total * (100 - sli.Config.ObjectivePercentage) / 100
				objective := 1 - sli.Config.ObjectivePercentage/100
				if br := summary.slowRequests / total / objective; br > burnRate {
					burnRate = br
				}
				summary.LatencySLO = &SLODetails{
					Threshold:  sli.Config.ObjectiveBucket,
					Objective:  utils.FormatPercentage(sli.Config.ObjectivePercentage) + " of requests should be served faster than " + utils.FormatLatency(sli.Config.ObjectiveBucket),
					Compliance: utils.FormatPercentage(100 - summary.slowRequestsPercent),
					Violated:   (100 - summary.slowRequestsPercent) < sli.Config.ObjectivePercentage,
				}
			}
		}
		if burnRate > r.BurnRateThreshold {
			break
		}
	}
	summary.ApplicationIncident = *i
	summary.ApplicationCategory = app.Category
	summary.AffectedRequestPercent = max(summary.failedRequestsPercent, summary.slowRequestsPercent)
	summary.ErrorBudgetConsumedPercent = summary.errorBudgetConsumedPercent()
	return summary
}
