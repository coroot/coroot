package collector

import (
	"encoding/hex"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/ClickHouse/ch-go"
	chproto "github.com/ClickHouse/ch-go/proto"
	semconv "go.opentelemetry.io/collector/semconv/v1.18.0"
	v1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"
	"k8s.io/klog"
)

func (c *Collector) Traces(w http.ResponseWriter, r *http.Request) {
	project, err := c.getProject(r.Header.Get(ApiKeyHeader))
	if err != nil {
		klog.Errorln(err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	contentType := r.Header.Get("Content-Type")
	switch contentType {
	case "application/x-protobuf":
	default:
		http.Error(w, "unsupported content type: "+contentType, http.StatusBadRequest)
		return
	}

	decoder, err := getDecoder(r.Header.Get("Content-Encoding"), r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	data, err := io.ReadAll(decoder)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	req := &v1.ExportTraceServiceRequest{}
	err = proto.Unmarshal(data, req)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	c.getTracesBatch(project).Add(req)

	resp := &v1.ExportTraceServiceResponse{}
	w.Header().Set("Content-Type", contentType)
	data, err = proto.Marshal(resp)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(data)
}

type TracesBatch struct {
	limit int
	exec  func(query ch.Query) error

	lock sync.Mutex
	done chan struct{}

	Timestamp          *chproto.ColDateTime64
	TraceId            *chproto.ColStr
	SpanId             *chproto.ColStr
	ParentSpanId       *chproto.ColStr
	TraceState         *chproto.ColStr
	SpanName           *chproto.ColLowCardinality[string]
	SpanKind           *chproto.ColLowCardinality[string]
	ServiceName        *chproto.ColLowCardinality[string]
	ResourceAttributes *chproto.ColMap[string, string]
	SpanAttributes     *chproto.ColMap[string, string]
	Duration           *chproto.ColInt64
	StatusCode         *chproto.ColLowCardinality[string]
	StatusMessage      *chproto.ColStr
	EventsTimestamp    *chproto.ColArr[time.Time]
	EventsName         *chproto.ColArr[string]
	EventsAttributes   *chproto.ColArr[map[string]string]
	LinksTraceId       *chproto.ColArr[string]
	LinksSpanId        *chproto.ColArr[string]
	LinksTraceState    *chproto.ColArr[string]
	LinksAttributes    *chproto.ColArr[map[string]string]
}

func NewTracesBatch(limit int, timeout time.Duration, exec func(query ch.Query) error) *TracesBatch {
	b := &TracesBatch{
		limit: limit,
		exec:  exec,
		done:  make(chan struct{}),

		Timestamp:          new(chproto.ColDateTime64).WithPrecision(chproto.PrecisionNano),
		TraceId:            new(chproto.ColStr),
		SpanId:             new(chproto.ColStr),
		ParentSpanId:       new(chproto.ColStr),
		TraceState:         new(chproto.ColStr),
		SpanName:           new(chproto.ColStr).LowCardinality(),
		SpanKind:           new(chproto.ColStr).LowCardinality(),
		ServiceName:        new(chproto.ColStr).LowCardinality(),
		ResourceAttributes: chproto.NewMap[string, string](new(chproto.ColStr).LowCardinality(), new(chproto.ColStr)),
		SpanAttributes:     chproto.NewMap[string, string](new(chproto.ColStr).LowCardinality(), new(chproto.ColStr)),
		Duration:           new(chproto.ColInt64),
		StatusCode:         new(chproto.ColStr).LowCardinality(),
		StatusMessage:      new(chproto.ColStr),
		EventsTimestamp:    new(chproto.ColDateTime64).WithPrecision(chproto.PrecisionNano).Array(),
		EventsName:         new(chproto.ColStr).LowCardinality().Array(),
		EventsAttributes:   chproto.NewArray[map[string]string](chproto.NewMap[string, string](new(chproto.ColStr).LowCardinality(), new(chproto.ColStr))),
		LinksTraceId:       new(chproto.ColStr).Array(),
		LinksSpanId:        new(chproto.ColStr).Array(),
		LinksTraceState:    new(chproto.ColStr).Array(),
		LinksAttributes:    chproto.NewArray[map[string]string](chproto.NewMap[string, string](new(chproto.ColStr).LowCardinality(), new(chproto.ColStr))),
	}

	go func() {
		ticker := time.NewTicker(timeout)
		defer ticker.Stop()
		for {
			select {
			case <-b.done:
				return
			case <-ticker.C:
				b.lock.Lock()
				b.save()
				b.lock.Unlock()
			}
		}
	}()

	return b
}

func (b *TracesBatch) Close() {
	b.done <- struct{}{}
	b.lock.Lock()
	defer b.lock.Unlock()
	b.save()
}

func (b *TracesBatch) Add(req *v1.ExportTraceServiceRequest) {
	b.lock.Lock()
	defer b.lock.Unlock()

	for _, rs := range req.GetResourceSpans() {
		var serviceName string
		resourceAttributes := attributesToMap(rs.GetResource().GetAttributes())
		for k, v := range resourceAttributes {
			if k == semconv.AttributeServiceName {
				serviceName = v
			}
		}
		for _, ss := range rs.GetScopeSpans() {
			scopeName := ss.GetScope().GetName()
			scopeVersion := ss.GetScope().GetVersion()
			for _, s := range ss.GetSpans() {
				spanAttributes := attributesToMap(s.GetAttributes())
				if scopeName != "" {
					spanAttributes[semconv.AttributeOtelScopeName] = scopeName
				}
				if scopeVersion != "" {
					spanAttributes[semconv.AttributeOtelScopeVersion] = scopeVersion
				}
				var eventTimestamps []time.Time
				var eventNames []string
				var eventAttributes []map[string]string
				for _, e := range s.GetEvents() {
					eventTimestamps = append(eventTimestamps, time.Unix(0, int64(e.GetTimeUnixNano())))
					eventNames = append(eventNames, e.GetName())
					eventAttributes = append(eventAttributes, attributesToMap(e.GetAttributes()))
				}
				var linkTraceIds []string
				var linkSpanIds []string
				var linkTraceStates []string
				var linkAttributes []map[string]string
				for _, l := range s.GetLinks() {
					linkTraceIds = append(linkTraceIds, hex.EncodeToString(l.GetTraceId()))
					linkSpanIds = append(linkSpanIds, hex.EncodeToString(l.GetSpanId()))
					linkTraceStates = append(linkTraceStates, l.GetTraceState())
					linkAttributes = append(linkAttributes, attributesToMap(l.GetAttributes()))
				}

				b.Timestamp.Append(time.Unix(0, int64(s.GetStartTimeUnixNano())))
				b.TraceId.Append(hex.EncodeToString(s.GetTraceId()))
				b.SpanId.Append(hex.EncodeToString(s.GetSpanId()))
				b.ParentSpanId.Append(hex.EncodeToString(s.GetParentSpanId()))
				b.TraceState.Append(s.GetTraceState())
				b.SpanName.Append(s.GetName())
				b.SpanKind.Append(s.GetKind().String())
				b.ServiceName.Append(serviceName)
				b.ResourceAttributes.Append(resourceAttributes)
				b.SpanAttributes.Append(spanAttributes)
				b.Duration.Append(int64(s.GetEndTimeUnixNano() - s.GetStartTimeUnixNano()))
				b.StatusCode.Append(s.GetStatus().GetCode().String())
				b.StatusMessage.Append(s.GetStatus().GetMessage())
				b.EventsTimestamp.Append(eventTimestamps)
				b.EventsName.Append(eventNames)
				b.EventsAttributes.Append(eventAttributes)
				b.LinksTraceId.Append(linkTraceIds)
				b.LinksSpanId.Append(linkSpanIds)
				b.LinksTraceState.Append(linkTraceStates)
				b.LinksAttributes.Append(linkAttributes)
			}
		}
	}
	if b.Timestamp.Rows() < b.limit {
		return
	}
	b.save()
}

func (b *TracesBatch) save() {
	if b.Timestamp.Rows() == 0 {
		return
	}

	input := chproto.Input{
		{Name: "Timestamp", Data: b.Timestamp},
		{Name: "TraceId", Data: b.TraceId},
		{Name: "SpanId", Data: b.SpanId},
		{Name: "ParentSpanId", Data: b.ParentSpanId},
		{Name: "TraceState", Data: b.TraceState},
		{Name: "SpanName", Data: b.SpanName},
		{Name: "SpanKind", Data: b.SpanKind},
		{Name: "ServiceName", Data: b.ServiceName},
		{Name: "ResourceAttributes", Data: b.ResourceAttributes},
		{Name: "SpanAttributes", Data: b.SpanAttributes},
		{Name: "Duration", Data: b.Duration},
		{Name: "StatusCode", Data: b.StatusCode},
		{Name: "StatusMessage", Data: b.StatusMessage},
		{Name: "Events.Timestamp", Data: b.EventsTimestamp},
		{Name: "Events.Name", Data: b.EventsName},
		{Name: "Events.Attributes", Data: b.EventsAttributes},
		{Name: "Links.TraceId", Data: b.LinksTraceId},
		{Name: "Links.SpanId", Data: b.LinksSpanId},
		{Name: "Links.TraceState", Data: b.LinksTraceState},
		{Name: "Links.Attributes", Data: b.LinksAttributes},
	}
	err := b.exec(ch.Query{Body: input.Into("@@table_otel_traces@@"), Input: input})
	if err != nil {
		klog.Errorln(err)
	}
	for _, i := range input {
		i.Data.(chproto.Resettable).Reset()
	}
}
