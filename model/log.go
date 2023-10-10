package model

import (
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/logparser"
)

type LogLevel string

const (
	LogLevelWarning  LogLevel = "warning"
	LogLevelError    LogLevel = "error"
	LogLevelCritical LogLevel = "critical"
)

func (s LogLevel) IsError() bool {
	return s == LogLevelError || s == LogLevelCritical
}

type LogMessages struct {
	Messages *timeseries.TimeSeries
	Patterns map[string]*LogPattern
}

type LogPattern struct {
	Pattern   *logparser.Pattern
	Level     LogLevel
	Sample    string
	Multiline bool
	Messages  *timeseries.TimeSeries
}
