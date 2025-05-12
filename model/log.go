package model

import (
	"time"

	"github.com/coroot/coroot/utils"

	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/logparser"
)

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
	Pattern              *logparser.Pattern
	Sample               string
	Multiline            bool
	Messages             *timeseries.TimeSeries
	SimilarPatternHashes *utils.StringSet
}

type LogEntry struct {
	ServiceName        string
	Timestamp          time.Time
	Severity           Severity
	Body               string
	TraceId            string
	LogAttributes      map[string]string
	ResourceAttributes map[string]string
}

type LogHistogramBucket struct {
	Severity   Severity
	Timeseries *timeseries.TimeSeries
}
