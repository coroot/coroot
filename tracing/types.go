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
	Timestamp    time.Time
	Name         string
	TraceId      string
	SpanId       string
	ParentSpanId string
	ServiceName  string
	Duration     time.Duration
	Status       string
	Attributes   map[string]string
}
