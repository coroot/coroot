package model

import (
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/logpattern"
)

type LogLevel string

const (
	LogLevelWarning  LogLevel = "warning"
	LogLevelError    LogLevel = "error"
	LogLevelCritical LogLevel = "critical"
)

type LogPattern struct {
	Pattern   *logpattern.Pattern
	Level     LogLevel
	Sample    string
	Multiline bool
	Sum       *timeseries.TimeSeries
}
