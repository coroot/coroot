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
	ServiceName        string            `json:"service_name"`
	Timestamp          time.Time         `json:"timestamp"`
	Severity           Severity          `json:"severity"`
	Body               string            `json:"body"`
	TraceId            string            `json:"trace_id,omitempty"`
	LogAttributes      map[string]string `json:"log_attributes,omitempty"`
	ResourceAttributes map[string]string `json:"resource_attributes,omitempty"`
	ClusterId          string            `json:"cluster_id,omitempty"`
	ClusterName        string            `json:"cluster_name,omitempty"`
}

func (e *LogEntry) AllAttributes() map[string]string {
	out := make(map[string]string, len(e.LogAttributes)+len(e.ResourceAttributes))
	for k, v := range e.LogAttributes {
		if k != "" && v != "" {
			out[k] = v
		}
	}
	for k, v := range e.ResourceAttributes {
		if k != "" && v != "" {
			out[k] = v
		}
	}
	return out
}

type LogHistogramBucket struct {
	Severity   Severity
	Timeseries *timeseries.TimeSeries
}
