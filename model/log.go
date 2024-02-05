package model

import (
	"time"

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

type LogSource string

const (
	LogSourceOtel  LogSource = "otel"
	LogSourceAgent LogSource = "agent"
)

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

type LogEntry struct {
	Timestamp          time.Time
	Severity           string
	Body               string
	TraceId            string
	LogAttributes      map[string]string
	ResourceAttributes map[string]string
}
