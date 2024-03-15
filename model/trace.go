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
	Timestamp  time.Time         `json:"timestamp"`
	Name       string            `json:"name"`
	Attributes map[string]string `json:"attributes"`
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
	case s.SpanAttributes["db.statement"] != "":
		res.Text = s.SpanAttributes["db.statement"]
		res.Lang = "sql"
	case s.SpanAttributes["db.memcached.item"] != "":
		res.Text = fmt.Sprintf(`%s "%s"`, s.SpanAttributes["db.operation"], s.SpanAttributes["db.memcached.item"])
		res.Lang = "bash"
	}
	return res
}

type TraceSpanAttr struct {
	Source string `json:"source"`
	Name   string `json:"name"`
	Value  string `json:"value"`
}

type TraceSpanAttrStats struct {
	Source string                     `json:"source"`
	Name   string                     `json:"name"`
	Values []*TraceSpanAttrStatsValue `json:"values"`
}

type TraceSpanAttrStatsValue struct {
	Name      string  `json:"name"`
	Selection float32 `json:"selection"`
	Baseline  float32 `json:"baseline"`
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
