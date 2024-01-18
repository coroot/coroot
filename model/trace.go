package model

import (
	"time"
)

type TraceSource string

const (
	TraceSourceOtel  TraceSource = "otel"
	TraceSourceAgent TraceSource = "agent"
)

type TraceSpan struct {
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
	Events        []TraceSpanEvent
}

type TraceSpanEvent struct {
	Timestamp  time.Time
	Name       string
	Attributes map[string]string
}
