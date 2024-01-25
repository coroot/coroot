package model

import (
	"strconv"

	"github.com/coroot/coroot/timeseries"
)

type ApplicationEventType int

const (
	ApplicationEventTypeSwitchover ApplicationEventType = iota
	ApplicationEventTypeRollout
	ApplicationEventTypeInstanceDown
	ApplicationEventTypeInstanceUp
)

type ApplicationEvent struct {
	Start   timeseries.Time
	End     timeseries.Time
	Type    ApplicationEventType
	Details string
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
