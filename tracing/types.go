package tracing

import (
	"time"
)

type Type string

const (
	TypeOtel     = "otel"
	TypeOtelEbpf = "otel_ebpf"
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
	Events        []Event
}

type Event struct {
	Timestamp  time.Time
	Name       string
	Attributes map[string]string
}
