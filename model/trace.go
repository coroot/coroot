package model

import (
	"fmt"
	"time"
)

type TraceSource string

const (
	TraceSourceOtel  TraceSource = "otel"
	TraceSourceAgent TraceSource = "agent"
)

type Trace struct {
	Spans []*TraceSpan
}

type TraceSpan struct {
	Timestamp          time.Time
	Name               string
	TraceId            string
	SpanId             string
	ParentSpanId       string
	ServiceName        string
	Duration           time.Duration
	StatusCode         string
	StatusMessage      string
	ResourceAttributes map[string]string
	SpanAttributes     map[string]string
	Events             []TraceSpanEvent
}

type TraceSpanEvent struct {
	Timestamp  time.Time
	Name       string
	Attributes map[string]string
}

type TraceSpanStatus struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
}

func (s *TraceSpan) Status() TraceSpanStatus {
	res := TraceSpanStatus{Message: "OK"}
	if s.StatusCode == "STATUS_CODE_ERROR" {
		res.Error = true
		res.Message = "ERROR"
		if s.StatusMessage != "" {
			res.Message = s.StatusMessage
		}
	}
	if c := s.SpanAttributes["http.status_code"]; c != "" {
		res.Message = "HTTP-" + c
	}
	return res
}

func (s *TraceSpan) Labels() Labels {
	res := map[string]string{}
	for name, value := range s.SpanAttributes {
		switch name {
		case "net.peer.name":
		case "server.address":
		case "net.host.name":
		case "http.route":
		case "http.host.name":
		case "db.system":
		case "db.operation":
		case "messaging.system":
		case "messaging.operation":
		default:
			continue
		}
		res[name] = value
	}
	return res
}
func (s *TraceSpan) ErrorMessage() string {
	if s.StatusCode != "STATUS_CODE_ERROR" {
		return ""
	}
	if s.StatusMessage != "" {
		return s.StatusMessage
	}
	if a := s.SpanAttributes["grpc.error_message"]; a != "" {
		return a
	}
	for _, e := range s.Events {
		if e.Name == "exception" {
			for name, value := range e.Attributes {
				if name == "exception.message" {
					return value
				}
			}
		}
	}
	if a := s.SpanAttributes["http.status_code"]; a != "" {
		return "HTTP-" + a
	}
	return ""
}

type TraceSpanDetails struct {
	Text string `json:"text"`
	Lang string `json:"lang"`
}

func (s *TraceSpan) Details() TraceSpanDetails {
	var res TraceSpanDetails
	switch {
	case s.SpanAttributes["http.url"] != "":
		res.Text = s.SpanAttributes["http.url"]
	case s.SpanAttributes["db.system"] == "mongodb":
		res.Text = s.SpanAttributes["db.statement"]
		res.Lang = "json"
	case s.SpanAttributes["db.system"] == "redis":
		res.Text = s.SpanAttributes["db.statement"]
	case s.SpanAttributes["db.system"] == "zookeeper":
		res.Text = s.SpanAttributes["db.statement"]
	case s.SpanAttributes["db.statement"] != "":
		res.Text = s.SpanAttributes["db.statement"]
		res.Lang = "sql"
	case s.SpanAttributes["db.memcached.item"] != "":
		res.Text = fmt.Sprintf(`%s "%s"`, s.SpanAttributes["db.operation"], s.SpanAttributes["db.memcached.item"])
		res.Lang = "bash"
	}
	return res
}

type TraceErrorsStat struct {
	ServiceName   string  `json:"service_name"`
	SpanName      string  `json:"span_name"`
	Labels        Labels  `json:"labels"`
	SampleTraceId string  `json:"sample_trace_id"`
	SampleError   string  `json:"sample_error"`
	Count         float32 `json:"count"`
}

type TraceSpanAttrStats struct {
	Name   string                     `json:"name"`
	Values []*TraceSpanAttrStatsValue `json:"values"`
}

type TraceSpanAttrStatsValue struct {
	Name          string  `json:"name"`
	Selection     float32 `json:"selection"`
	Baseline      float32 `json:"baseline"`
	SampleTraceId string  `json:"sample_trace_id"`
}

type TraceSpanSummary struct {
	Stats   []TraceSpanStats `json:"stats"`
	Overall TraceSpanStats   `json:"overall"`
}

type TraceSpanStats struct {
	ServiceName       string    `json:"service_name"`
	SpanName          string    `json:"span_name"`
	Total             float32   `json:"total"`
	Failed            float32   `json:"failed"`
	DurationQuantiles []float32 `json:"duration_quantiles"`
}
