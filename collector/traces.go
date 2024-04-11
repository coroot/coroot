package collector

import (
	"context"
	"encoding/hex"
	"io"
	"net/http"
	"time"

	"github.com/ClickHouse/ch-go"
	chproto "github.com/ClickHouse/ch-go/proto"
	"github.com/coroot/coroot/db"
	semconv "go.opentelemetry.io/collector/semconv/v1.18.0"
	v1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"
	"k8s.io/klog"
)

func (c *Collector) Traces(w http.ResponseWriter, r *http.Request) {
	projectId := db.ProjectId(r.Header.Get(ApiKeyHeader))
	_, err := c.getClickhouseClient(projectId)
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
	data, err := io.ReadAll(r.Body)
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

	err = c.saveTraces(r.Context(), projectId, req)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

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

func (c *Collector) saveTraces(ctx context.Context, projectId db.ProjectId, req *v1.ExportTraceServiceRequest) error {
	colTimestamp := new(chproto.ColDateTime64).WithPrecision(chproto.PrecisionNano)
	colTraceId := new(chproto.ColStr)
	colSpanId := new(chproto.ColStr)
	colParentSpanId := new(chproto.ColStr)
	colTraceState := new(chproto.ColStr)
	colSpanName := new(chproto.ColStr).LowCardinality()
	colSpanKind := new(chproto.ColStr).LowCardinality()
	colServiceName := new(chproto.ColStr).LowCardinality()
	colResourceAttributes := chproto.NewMap[string, string](new(chproto.ColStr).LowCardinality(), new(chproto.ColStr))
	colSpanAttributes := chproto.NewMap[string, string](new(chproto.ColStr).LowCardinality(), new(chproto.ColStr))
	colDuration := new(chproto.ColInt64)
	colStatusCode := new(chproto.ColStr).LowCardinality()
	colStatusMessage := new(chproto.ColStr)
	colEventsTimestamp := new(chproto.ColDateTime64).WithPrecision(chproto.PrecisionNano).Array()
	colEventsName := new(chproto.ColStr).LowCardinality().Array()
	colEventsAttributes := chproto.NewArray[map[string]string](chproto.NewMap[string, string](new(chproto.ColStr).LowCardinality(), new(chproto.ColStr)))
	colLinksTraceId := new(chproto.ColStr).Array()
	colLinksSpanId := new(chproto.ColStr).Array()
	colLinksTraceState := new(chproto.ColStr).Array()
	colLinksAttributes := chproto.NewArray[map[string]string](chproto.NewMap[string, string](new(chproto.ColStr).LowCardinality(), new(chproto.ColStr)))

	for _, rs := range req.GetResourceSpans() {
		var serviceName string
		resourceAttributes := map[string]string{}
		for _, attr := range rs.GetResource().GetAttributes() {
			if attr.GetKey() == semconv.AttributeServiceName {
				serviceName = attr.GetValue().GetStringValue()
			}
			resourceAttributes[attr.GetKey()] = attr.GetValue().GetStringValue()
		}
		for _, ss := range rs.GetScopeSpans() {
			scopeName := ss.GetScope().GetName()
			scopeVersion := ss.GetScope().GetVersion()
			for _, s := range ss.GetSpans() {
				colTimestamp.Append(time.Unix(0, int64(s.GetStartTimeUnixNano())))
				colTraceId.Append(hex.EncodeToString(s.GetTraceId()))
				colSpanId.Append(hex.EncodeToString(s.GetSpanId()))
				colParentSpanId.Append(hex.EncodeToString(s.GetParentSpanId()))
				colTraceState.Append(s.GetTraceState())
				colSpanName.Append(s.GetName())
				colSpanKind.Append(s.GetKind().String())
				colServiceName.Append(serviceName)
				colDuration.Append(int64(s.GetEndTimeUnixNano() - s.GetStartTimeUnixNano()))
				colStatusCode.Append(s.GetStatus().GetCode().String())
				colStatusMessage.Append(s.GetStatus().GetMessage())

				colResourceAttributes.Append(resourceAttributes)
				spanAttributes := map[string]string{}
				for _, attr := range s.GetAttributes() {
					spanAttributes[attr.GetKey()] = attr.GetValue().GetStringValue()
				}
				if scopeName != "" {
					spanAttributes[semconv.AttributeOtelScopeName] = scopeName
				}
				if scopeVersion != "" {
					spanAttributes[semconv.AttributeOtelScopeVersion] = scopeVersion
				}
				colSpanAttributes.Append(spanAttributes)

				var eventTimestamps []time.Time
				var eventNames []string
				var eventAttributes []map[string]string
				for _, e := range s.GetEvents() {
					eventTimestamps = append(eventTimestamps, time.Unix(0, int64(e.GetTimeUnixNano())))
					eventNames = append(eventNames, e.GetName())
					attrs := map[string]string{}
					for _, a := range e.GetAttributes() {
						attrs[a.GetKey()] = a.GetValue().GetStringValue()
					}
					eventAttributes = append(eventAttributes, attrs)
				}
				colEventsTimestamp.Append(eventTimestamps)
				colEventsName.Append(eventNames)
				colEventsAttributes.Append(eventAttributes)

				var linkTraceIds []string
				var linkSpanIds []string
				var linkTraceStates []string
				var linkAttributes []map[string]string
				for _, l := range s.GetLinks() {
					linkTraceIds = append(linkTraceIds, hex.EncodeToString(l.GetTraceId()))
					linkSpanIds = append(linkSpanIds, hex.EncodeToString(l.GetSpanId()))
					linkTraceStates = append(linkTraceStates, l.GetTraceState())
					attrs := map[string]string{}
					for _, a := range l.GetAttributes() {
						attrs[a.GetKey()] = a.GetValue().GetStringValue()
					}
					linkAttributes = append(linkAttributes, attrs)
				}
				colLinksTraceId.Append(linkTraceIds)
				colLinksSpanId.Append(linkSpanIds)
				colLinksTraceState.Append(linkTraceStates)
				colLinksAttributes.Append(linkAttributes)
			}
		}
	}

	input := chproto.Input{
		{Name: "Timestamp", Data: colTimestamp},
		{Name: "TraceId", Data: colTraceId},
		{Name: "SpanId", Data: colSpanId},
		{Name: "ParentSpanId", Data: colParentSpanId},
		{Name: "TraceState", Data: colTraceState},
		{Name: "SpanName", Data: colSpanName},
		{Name: "SpanKind", Data: colSpanKind},
		{Name: "ServiceName", Data: colServiceName},
		{Name: "ResourceAttributes", Data: colResourceAttributes},
		{Name: "SpanAttributes", Data: colSpanAttributes},
		{Name: "Duration", Data: colDuration},
		{Name: "StatusCode", Data: colStatusCode},
		{Name: "StatusMessage", Data: colStatusMessage},
		{Name: "Events.Timestamp", Data: colEventsTimestamp},
		{Name: "Events.Name", Data: colEventsName},
		{Name: "Events.Attributes", Data: colEventsAttributes},
		{Name: "Links.TraceId", Data: colLinksTraceId},
		{Name: "Links.SpanId", Data: colLinksSpanId},
		{Name: "Links.TraceState", Data: colLinksTraceState},
		{Name: "Links.Attributes", Data: colLinksAttributes},
	}
	return c.clickhouseDo(ctx, projectId, ch.Query{Body: input.Into("otel_traces"), Input: input})
}
