package watchers

import (
	"context"
	"time"

	"github.com/coroot/coroot/auditor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/notifications"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"k8s.io/klog"
)

type Incidents struct {
	db       *db.DB
	rca      IncidentRCA
	notifier *notifications.IncidentNotifier
}

type IncidentRCA func(ctx context.Context, project *db.Project, world *model.World, incident *model.ApplicationIncident)

func NewIncidents(db *db.DB, rca IncidentRCA) *Incidents {
	return &Incidents{db: db, notifier: notifications.NewIncidentNotifier(db), rca: rca}
}

func (w *Incidents) Check(project *db.Project, world *model.World) {
	start := time.Now()

	auditor.Audit(world, project, nil, false, nil)

	var apps int

	now := timeseries.Now()

	for _, app := range world.Applications {
		var (
			aBadF, aTotalF sumFromFunc
			lBadF, lTotalF sumFromFunc
		)
		details := model.IncidentDetails{}
		details.AvailabilityBurnRates, aBadF, aTotalF = availability(world.Ctx, app)
		details.LatencyBurnRates, lBadF, lTotalF = latency(world.Ctx, app)

		calcImpact := func(openedAt timeseries.Time, badF, totalF sumFromFunc) float32 {
			from := openedAt.Add(-model.MinAlertRuleShortWindow)
			dataFrom := now.Add(-model.MaxAlertRuleWindow)

			if from.Before(dataFrom) {
				from = dataFrom
			}

			if badF == nil || totalF == nil {
				return 0.
			}
			if v := badF(from) / totalF(from); !timeseries.IsNaN(v) {
				return v * 100
			}
			return 0.
		}

		status := model.UNKNOWN
		for _, br := range details.LatencyBurnRates {
			if br.Severity > status {
				status = br.Severity
			}
		}
		for _, br := range details.AvailabilityBurnRates {
			if br.Severity > status {
				status = br.Severity
			}
		}
		if status == model.UNKNOWN {
			continue
		}
		apps++

		incident, err := w.db.GetLastOpenIncident(project.Id, app.Id)
		if err != nil {
			klog.Errorln(err)
			continue
		}
		needNotify := false
		switch {
		case incident == nil && status <= model.OK:
			continue
		case incident == nil:
			incident = &model.ApplicationIncident{
				ApplicationId: app.Id,
				Key:           utils.NanoId(8),
				OpenedAt:      now,
				Severity:      status,
				Details:       details,
			}
			incident.Details.AvailabilityImpact.AffectedRequestPercentage = calcImpact(incident.OpenedAt, aBadF, aTotalF)
			incident.Details.LatencyImpact.AffectedRequestPercentage = calcImpact(incident.OpenedAt, lBadF, lTotalF)
			if err = w.db.CreateIncident(project.Id, app.Id, incident); err != nil {
				klog.Errorln(err)
				continue
			}
			needNotify = true
		default:
			if status == model.OK {
				incident.ResolvedAt = now
				incident.Severity = model.OK
				if err = w.db.ResolveIncident(project.Id, app.Id, incident); err != nil {
					klog.Errorln(err)
					continue
				}
				needNotify = true
			} else {
				incident.Severity = status
				incident.Details.AvailabilityBurnRates = details.AvailabilityBurnRates
				incident.Details.LatencyBurnRates = details.LatencyBurnRates
				incident.Details.AvailabilityImpact.AffectedRequestPercentage = calcImpact(incident.OpenedAt, aBadF, aTotalF)
				incident.Details.LatencyImpact.AffectedRequestPercentage = calcImpact(incident.OpenedAt, lBadF, lTotalF)
				if err = w.db.UpdateIncident(project.Id, app.Id, incident.Key, incident.Severity, incident.Details); err != nil {
					klog.Errorln(err)
					continue
				}
			}
		}
		if w.rca != nil {
			w.rca(context.TODO(), project, world, incident)
		}
		if needNotify {
			w.notifier.Enqueue(project, app, incident, now)
		}
	}
	klog.Infof("%s: checked %d apps in %s", project.Id, apps, time.Since(start).Truncate(time.Millisecond))
}

type sumFromFunc func(from timeseries.Time) float32

func availability(ctx timeseries.Context, app *model.Application) ([]model.BurnRate, sumFromFunc, sumFromFunc) {
	if len(app.AvailabilitySLIs) == 0 {
		return nil, nil, nil
	}
	sli := app.AvailabilitySLIs[0]
	if sli.TotalRequestsRaw.TailIsEmpty() {
		return nil, nil, nil
	}

	totalF := totalSum(sli.TotalRequestsRaw)
	failedF := func(from timeseries.Time) float32 {
		return 0
	}
	if !sli.FailedRequestsRaw.IsEmpty() {
		failedF = func(from timeseries.Time) float32 {
			iter := sli.FailedRequestsRaw.IterFrom(from)
			var sum float32
			for iter.Next() {
				_, v := iter.Value()
				if timeseries.IsNaN(v) {
					continue
				}
				sum += v
			}
			return sum
		}
	}
	return calcBurnRates(ctx.To, failedF, totalF, sli.Config.ObjectivePercentage), failedF, totalF
}

func latency(ctx timeseries.Context, app *model.Application) ([]model.BurnRate, sumFromFunc, sumFromFunc) {
	if len(app.LatencySLIs) == 0 {
		return nil, nil, nil
	}
	sli := app.LatencySLIs[0]
	totalRaw, fastRaw := sli.GetTotalAndFast(true)

	totalF := totalSum(totalRaw)
	var slowF sumFromFunc
	if !fastRaw.IsEmpty() {
		slowF = func(from timeseries.Time) float32 {
			totalIter := totalRaw.IterFrom(from)
			fastIter := fastRaw.IterFrom(from)
			var sum float32
			for totalIter.Next() && fastIter.Next() {
				_, total := totalIter.Value()
				if timeseries.IsNaN(total) {
					continue
				}
				_, fast := fastIter.Value()
				if timeseries.IsNaN(fast) {
					sum += total
				} else {
					sum += total - fast
				}
			}
			return sum
		}
	}
	if slowF == nil {
		return nil, nil, nil
	}
	return calcBurnRates(ctx.To, slowF, totalF, sli.Config.ObjectivePercentage), slowF, totalF
}

func calcBurnRates(now timeseries.Time, badSum, totalSum sumFromFunc, objectivePercentage float32) []model.BurnRate {
	objective := 1 - objectivePercentage/100
	var res []model.BurnRate

	for _, r := range model.AlertRules {
		from := now.Add(-r.LongWindow)
		total := totalSum(from)
		bad := badSum(from)
		br := model.BurnRate{
			LongWindow:  r.LongWindow,
			ShortWindow: r.ShortWindow,
			Threshold:   r.BurnRateThreshold,
			Severity:    model.OK,
		}
		if v := bad / total; !timeseries.IsNaN(v) {
			br.LongWindowPercentage = v * 100
			br.LongWindowBurnRate = v / objective
		} else {
			continue
		}
		from = now.Add(-r.ShortWindow)
		if v := badSum(from) / totalSum(from); !timeseries.IsNaN(v) {
			br.ShortWindowPercentage = v * 100
			br.ShortWindowBurnRate = v / objective
		} else {
			continue
		}
		if br.LongWindowBurnRate > r.BurnRateThreshold && br.ShortWindowBurnRate > r.BurnRateThreshold {
			br.Severity = r.Severity
		}
		res = append(res, br)
	}
	return res
}

func totalSum(ts *timeseries.TimeSeries) sumFromFunc {
	return func(from timeseries.Time) float32 {
		iter := ts.IterFrom(from)
		var sum float32
		var count, countDefined int
		for iter.Next() {
			_, v := iter.Value()
			count++
			if timeseries.IsNaN(v) {
				continue
			}
			sum += v
			countDefined++
		}
		if float32(countDefined)/float32(count) < 0.5 {
			return timeseries.NaN
		}
		return sum
	}
}
