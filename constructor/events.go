package constructor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"math"
	"math/bits"
	"sort"
)

func calcAppEvents(w *model.World) {
	for _, app := range w.Applications {
		var events []*model.ApplicationEvent
		events = append(events, calcRollouts(app)...)
		events = append(events, calcClusterSwitchovers(app)...)
		events = append(events, calcUpDownEvents(app)...)
		sort.Slice(events, func(i, j int) bool {
			if events[i].Start == events[j].Start {
				return events[i].Details < events[j].Details
			}
			return events[i].Start < events[j].Start
		})
		app.Events = events
	}
}

func calcRollouts(app *model.Application) []*model.ApplicationEvent {
	if app.Id.Kind != model.ApplicationKindDeployment || len(app.Instances) == 0 {
		return nil
	}

	byReplicaSet := map[string]*timeseries.AggregatedTimeseries{}
	for _, instance := range app.Instances {
		if instance.Pod == nil || instance.Pod.ReplicaSet == "" {
			continue
		}
		byReplicaSet[instance.Pod.ReplicaSet] = timeseries.Merge(byReplicaSet[instance.Pod.ReplicaSet], instance.Pod.LifeSpan, timeseries.NanSum)
	}
	if len(byReplicaSet) == 0 {
		return nil
	}

	var rss []timeseries.TimeSeries
	rsNum := 1
	for _, rs := range byReplicaSet {
		n := float64(rsNum)
		rss = append(rss, timeseries.Map(func(t timeseries.Time, v float64) float64 {
			if v > 0 {
				return n
			}
			return 0
		}, rs))
		rsNum++
	}

	activeRss := timeseries.Aggregate(func(t timeseries.Time, accumulator, v float64) float64 {
		if v == 0 {
			return accumulator
		}
		return float64(int64(accumulator) | 1<<int64(v-1))
	}, append([]timeseries.TimeSeries{timeseries.Replace(rss[0], 0)}, rss...)...)

	var events []*model.ApplicationEvent
	var event *model.ApplicationEvent
	iter := timeseries.Iter(activeRss)
	prev := 0
	i := 0
	for iter.Next() {
		i++
		t, v := iter.Value()
		rssBits := uint64(v)
		rssCount := bits.OnesCount64(rssBits)
		switch rssCount {
		case 0:
		case 1:
			curr := bits.TrailingZeros64(rssBits) + 1
			if i == 1 {
				prev = curr
				continue
			}
			if prev == curr {
				continue
			}
			prev = curr
			if event == nil {
				event = &model.ApplicationEvent{Type: model.ApplicationEventTypeRollout, Start: t}
			}
			event.End = t
			events = append(events, event)
			event = nil
		default:
			if event == nil {
				event = &model.ApplicationEvent{Type: model.ApplicationEventTypeRollout, Start: t}
			}
		}
	}
	if event != nil {
		events = append(events, event)
	}
	return events
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
