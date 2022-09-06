package application

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"math"
	"sort"
	"strconv"
)

type EventType int

const (
	EventTypeSwitchover EventType = iota
	EventTypeRollout
	EventTypeInstanceDown
	EventTypeInstanceUp
)

type Event struct {
	Start   timeseries.Time
	End     timeseries.Time
	Type    EventType
	Details string
}

func (e *Event) String() string {
	if e == nil {
		return "-"
	}
	start, end := "", ""
	if !e.Start.IsZero() {
		start = strconv.FormatInt(int64(e.Start), 10)
	}
	if !e.End.IsZero() {
		end = strconv.FormatInt(int64(e.End), 10)
	}
	return start + "-" + end
}

func calcAppEvents(app *model.Application) []*Event {
	var events []*Event
	events = append(events, calcRollouts(app)...)
	events = append(events, calcClusterSwitchovers(app)...)
	events = append(events, calcUpDownEvents(app)...)
	sort.Slice(events, func(i, j int) bool {
		if events[i].Start == events[j].Start {
			return events[i].Details < events[j].Details
		}
		return events[i].Start < events[j].Start
	})
	return events
}

func calcRollouts(app *model.Application) []*Event {
	if app.Id.Kind != model.ApplicationKindDeployment || len(app.Instances) == 0 {
		return nil
	}
	var events []*Event
	var currentRollout *Event
	var lastTs timeseries.Time
	byReplicaSet := map[string]*timeseries.AggregatedTimeseries{}
	for _, instance := range app.Instances {
		if instance.Pod == nil || instance.Pod.ReplicaSet == "" {
			continue
		}
		rs := byReplicaSet[instance.Pod.ReplicaSet]
		if rs == nil {
			rs = timeseries.Aggregate(timeseries.NanSum)
			byReplicaSet[instance.Pod.ReplicaSet] = rs
		}
		rs.AddInput(instance.Pod.LifeSpan)
	}
	if len(byReplicaSet) > 1 {
		activeRss := timeseries.Aggregate(timeseries.NanSum)
		for _, rs := range byReplicaSet {
			activeRss.AddInput(timeseries.Map(func(v float64) float64 {
				if v > 0 {
					return 1
				}
				return 0
			}, rs))
		}
		ctx := activeRss.Range()
		iter := activeRss.Iter()
		t := ctx.From
		for iter.Next() {
			lastTs = t
			_, v := iter.Value()
			if v > 1 {
				if currentRollout == nil {
					currentRollout = &Event{Start: t, Type: EventTypeRollout}
					events = append(events, currentRollout)
				}
			} else {
				if currentRollout != nil {
					currentRollout.End = t
					currentRollout = nil
				}
			}
			t = t.Add(ctx.Step)
		}
	}
	if currentRollout != nil {
		currentRollout.End = lastTs
	}
	return events
}

func calcUpDownEvents(app *model.Application) []*Event {
	var events []*Event
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
		iter := up.Iter()
		status := ""

		for iter.Next() {
			t, v := iter.Value()
			switch {
			case status == "up" && v != 1:
				events = append(events, &Event{Start: t, Type: EventTypeInstanceDown, Details: instance.Name})
			case status == "down" && v == 1:
				events = append(events, &Event{Start: t, Type: EventTypeInstanceUp, Details: instance.Name})
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

func calcClusterSwitchovers(app *model.Application) []*Event {
	names := map[int]string{}
	primaryNum := timeseries.Aggregate(func(accumulator, v float64) float64 {
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
			primaryNum.AddInput(timeseries.Map(func(v float64) float64 {
				if v == float64(model.ClusterRolePrimary) {
					return num
				}
				return timeseries.NaN
			}, role))
		}
	}
	if primaryNum.IsEmpty() {
		return nil
	}
	var events []*Event

	var event *Event
	iter := primaryNum.Iter()
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
			event = &Event{Start: t, Details: names[int(prev)] + " &rarr; ", Type: EventTypeSwitchover}
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
