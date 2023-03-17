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

type LogPattern struct {
	Pattern   *logparser.Pattern
	Level     LogLevel
	Sample    string
	Multiline bool
	Sum       *timeseries.TimeSeries
}
