package tracing

import (
	"github.com/coroot/coroot/model"
	"strings"
	"time"
)

type Source string

const (
	SourceOtel  = "otel"
	SourceAgent = "agent"
)

type Span struct {
	Timestamp     time.Time
	Name          string
	TraceId       string
	SpanId        string
	ParentSpanId  string
	ServiceName   string
	Duration      time.Duration
	StatusCode    string
	StatusMessage string
	Attributes    map[string]string
	Events        []SpanEvent
}

type SpanEvent struct {
	Timestamp  time.Time
	Name       string
	Attributes map[string]string
}

type LogEntry struct {
	Timestamp          time.Time
	Severity           string
	Body               string
	LogAttributes      map[string]string
	ResourceAttributes map[string]string
}

func GuessService(services []string, appId model.ApplicationId) string {
	appName := appId.Name
	for _, s := range services {
		if s == appName {
			return s
		}
	}
	for _, s := range services {
		if strings.HasSuffix(appName, s) {
			return s
		}
	}
	return ""
}
