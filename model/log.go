package model

import (
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/logparser"
)

type LogSeverity string

const (
	LogSeverityUnknown  LogSeverity = "unknown"
	LogSeverityDebug    LogSeverity = "debug"
	LogSeverityInfo     LogSeverity = "info"
	LogSeverityWarning  LogSeverity = "warning"
	LogSeverityError    LogSeverity = "error"
	LogSeverityCritical LogSeverity = "critical"
)

func (s LogSeverity) IsError() bool {
	return s == LogSeverityError || s == LogSeverityCritical
}

type LogMessages struct {
	Messages *timeseries.TimeSeries
	Patterns map[string]*LogPattern
}

type LogPattern struct {
	Pattern   *logparser.Pattern
	Severity  LogSeverity
	Sample    string
	Multiline bool
	Messages  *timeseries.TimeSeries
}
