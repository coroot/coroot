package constructor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"math"
	"sort"
)

func calcAppEvents(w *model.World) {
	for _, app := range w.Applications {
		var events []*model.ApplicationEvent
		events = append(events, calcClusterSwitchovers(app)...)
		events = append(events, calcUpDownEvents(app)...)
		for _, d := range app.Deployments {
			if d.StartedAt.Before(w.Ctx.From) || d.StartedAt.After(w.Ctx.To) {
				continue
			}
			events = append(events, &model.ApplicationEvent{
				Start:   d.StartedAt,
				End:     d.StartedAt,
				Type:    model.ApplicationEventTypeRollout,
				Details: d.Version(),
			})
		}
		sort.Slice(events, func(i, j int) bool {
			if events[i].Start == events[j].Start {
				return events[i].Details < events[j].Details
			}
			return events[i].Start < events[j].Start
		})
		app.Events = events
	}
}

func calcUpDownEvents(app *model.Application) []*model.ApplicationEvent {
	var events []*model.ApplicationEvent
	for _, instance := range app.Instances {
		var up timeseries.TimeSeries
		switch {
		case instance.Postgres != nil && instance.Postgres.Up != nil:
			up = instance.Postgres.Up
		case instance.Redis != nil && instance.Redis.Up != nil:
			up = instance.Redis.Up
		default:
			continue
		}

		iter := timeseries.Iter(up)
		status := ""
		for iter.Next() {
			t, v := iter.Value()
			switch {
			case status == "up" && v != 1:
				events = append(events, &model.ApplicationEvent{Start: t, Type: model.ApplicationEventTypeInstanceDown, Details: instance.Name})
			case status == "down" && v == 1:
				events = append(events, &model.ApplicationEvent{Start: t, Type: model.ApplicationEventTypeInstanceUp, Details: instance.Name})
			}
			if v == 1 {
				status = "up"
			} else {
				status = "down"
			}
		}
	}
	return events
}

func calcClusterSwitchovers(app *model.Application) []*model.ApplicationEvent {
	names := map[int]string{}
	primaryNum := timeseries.Aggregate(func(t timeseries.Time, accumulator, v float64) float64 {
		if accumulator < 0 {
			return -1
		}
		if accumulator >= 0 && v >= 0 {
			return -1
		}
		if v >= 0 {
			return v
		}
		return accumulator
	})
	for i, instance := range app.Instances {
		names[i] = instance.Name
		if role := instance.ClusterRole(); role != nil {
			num := float64(i)
			primaryNum.AddInput(timeseries.Map(func(t timeseries.Time, v float64) float64 {
				if v == float64(model.ClusterRolePrimary) {
					return num
				}
				return timeseries.NaN
			}, role))
		}
	}
	if timeseries.IsEmpty(primaryNum) {
		return nil
	}

	var events []*model.ApplicationEvent
	var event *model.ApplicationEvent
	iter := timeseries.Iter(primaryNum)
	prev := float64(-1)
	for iter.Next() {
		t, curr := iter.Value()
		if prev == -1 {
			if !math.IsNaN(curr) {
				prev = curr
			}
			continue
		}
		if curr != prev && event == nil {
			event = &model.ApplicationEvent{Start: t, Details: names[int(prev)] + " &rarr; ", Type: model.ApplicationEventTypeSwitchover}
		}
		if curr != prev && event != nil && !math.IsNaN(curr) && curr >= 0 {
			event.End = t
			event.Details += names[int(curr)]
			events = append(events, event)
			event = nil
		}
		if !math.IsNaN(curr) && curr >= 0 {
			prev = curr
		}
	}
	return events
}
