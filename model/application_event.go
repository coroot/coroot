package model

import (
	"net"
	"strconv"

	"github.com/coroot/coroot/timeseries"
)

type ApplicationEventType int

const (
	ApplicationEventTypeSwitchover ApplicationEventType = iota
	ApplicationEventTypeRollout
	ApplicationEventTypeInstanceDown
	ApplicationEventTypeInstanceUp
	ApplicationEventTypeDbChange
)

type ApplicationEvent struct {
	Start   timeseries.Time
	End     timeseries.Time
	Type    ApplicationEventType
	Details string
	Link    *RouterLink
}

func (app *Application) SetDbChangeEvents(entries []*LogEntry, step timeseries.Duration, link *RouterLink) {
	type eventKey struct {
		ts      timeseries.Time
		details string
	}
	seen := map[eventKey]bool{}
	for _, e := range entries {
		ts := timeseries.Time(e.Timestamp.Unix()).Truncate(3 * step)
		object := e.LogAttributes["db_change.object"]
		dbName := e.LogAttributes["db.name"]
		schemaName := e.LogAttributes["db_change.schema"]
		var details string
		switch {
		case dbName != "" && schemaName != "":
			details = dbName + "." + schemaName + "." + object
		case dbName != "":
			details = dbName + "." + object
		default:
			details = object
		}
		k := eventKey{ts: ts, details: details}
		if seen[k] {
			continue
		}
		seen[k] = true
		app.Events = append(app.Events, &ApplicationEvent{
			Start:   ts,
			End:     ts,
			Type:    ApplicationEventTypeDbChange,
			Details: details,
			Link:    link,
		})
	}
}

func (app *Application) ListenTargets() []string {
	var targets []string
	for _, instance := range app.Instances {
		for listen := range instance.TcpListens {
			if listen.Port == "0" {
				continue
			}
			targets = append(targets, net.JoinHostPort(listen.IP, listen.Port))
		}
	}
	return targets
}

func (e *ApplicationEvent) String() string {
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
