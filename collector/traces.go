package collector

import (
	"encoding/hex"
	"io"
	"net/http"
	"time"

	"github.com/ClickHouse/ch-go"
	chproto "github.com/ClickHouse/ch-go/proto"
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
	rows := getSpanRows()
	defer putSpanRows(rows)
	if err = rows.Unmarshal(data); err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	c.getTracesBatch(project).add(rows)

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

type tracesColumns struct {
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

	eventsTimestamp  *chproto.ColDateTime64
	eventsName       *chproto.ColLowCardinality[string]
	eventsAttributes *chproto.ColMap[string, string]
	linksTraceId     *chproto.ColStr
	linksSpanId      *chproto.ColStr
	linksTraceState  *chproto.ColStr
	linksAttributes  *chproto.ColMap[string, string]

	hexBuf []byte
}

func newAttributesColumn() *chproto.ColMap[string, string] {
	return chproto.NewMap[string, string](new(chproto.ColStr).LowCardinality(), new(chproto.ColStr))
}

func newTracesColumns() *tracesColumns {
	c := &tracesColumns{
		Timestamp:          new(chproto.ColDateTime64).WithPrecision(chproto.PrecisionNano),
		TraceId:            new(chproto.ColStr),
		SpanId:             new(chproto.ColStr),
		ParentSpanId:       new(chproto.ColStr),
		TraceState:         new(chproto.ColStr),
		SpanName:           new(chproto.ColStr).LowCardinality(),
		SpanKind:           new(chproto.ColStr).LowCardinality(),
		ServiceName:        new(chproto.ColStr).LowCardinality(),
		ResourceAttributes: newAttributesColumn(),
		SpanAttributes:     newAttributesColumn(),
		Duration:           new(chproto.ColInt64),
		StatusCode:         new(chproto.ColStr).LowCardinality(),
		StatusMessage:      new(chproto.ColStr),

		eventsTimestamp:  new(chproto.ColDateTime64).WithPrecision(chproto.PrecisionNano),
		eventsName:       new(chproto.ColStr).LowCardinality(),
		eventsAttributes: newAttributesColumn(),
		linksTraceId:     new(chproto.ColStr),
		linksSpanId:      new(chproto.ColStr),
		linksTraceState:  new(chproto.ColStr),
		linksAttributes:  newAttributesColumn(),
	}
	c.EventsTimestamp = c.eventsTimestamp.Array()
	c.EventsName = c.eventsName.Array()
	c.EventsAttributes = chproto.NewArray[map[string]string](c.eventsAttributes)
	c.LinksTraceId = c.linksTraceId.Array()
	c.LinksSpanId = c.linksSpanId.Array()
	c.LinksTraceState = c.linksTraceState.Array()
	c.LinksAttributes = chproto.NewArray[map[string]string](c.linksAttributes)
	return c
}

func (c *tracesColumns) appendHex(col *chproto.ColStr, id []byte) {
	c.hexBuf = hex.AppendEncode(c.hexBuf[:0], id)
	col.AppendBytes(c.hexBuf)
}

func (c *tracesColumns) append(rows []spanRow) {
	for i := range rows {
		r := &rows[i]
		c.Timestamp.Append(time.Unix(0, r.timestamp))
		c.appendHex(c.TraceId, r.traceId)
		c.appendHex(c.SpanId, r.spanId)
		c.appendHex(c.ParentSpanId, r.parentSpanId)
		c.TraceState.Append(r.traceState)
		c.SpanName.Append(r.name)
		c.SpanKind.Append(spanKinds(r.kind))
		c.ServiceName.Append(r.serviceName)
		c.ResourceAttributes.AppendKV(r.resourceAttributes)
		c.SpanAttributes.AppendKV(r.spanAttributes)
		c.Duration.Append(r.duration)
		c.StatusCode.Append(statusCodes(r.statusCode))
		c.StatusMessage.Append(r.statusMessage)
		for _, e := range r.events {
			c.eventsTimestamp.Append(time.Unix(0, e.timestamp))
			c.eventsName.Append(e.name)
			c.eventsAttributes.AppendKV(e.attributes)
		}
		for _, l := range r.links {
			c.appendHex(c.linksTraceId, l.traceId)
			c.appendHex(c.linksSpanId, l.spanId)
			c.linksTraceState.Append(l.traceState)
			c.linksAttributes.AppendKV(l.attributes)
		}
		events := uint64(c.eventsTimestamp.Rows()) //nolint:gosec // row count
		links := uint64(c.linksTraceId.Rows())     //nolint:gosec // row count
		c.EventsTimestamp.Offsets.Append(events)
		c.EventsName.Offsets.Append(events)
		c.EventsAttributes.Offsets.Append(events)
		c.LinksTraceId.Offsets.Append(links)
		c.LinksSpanId.Offsets.Append(links)
		c.LinksTraceState.Offsets.Append(links)
		c.LinksAttributes.Offsets.Append(links)
	}
}

func (c *tracesColumns) rows() int {
	return c.Timestamp.Rows()
}

func (c *tracesColumns) input() chproto.Input {
	return chproto.Input{
		{Name: "Timestamp", Data: c.Timestamp},
		{Name: "TraceId", Data: c.TraceId},
		{Name: "SpanId", Data: c.SpanId},
		{Name: "ParentSpanId", Data: c.ParentSpanId},
		{Name: "TraceState", Data: c.TraceState},
		{Name: "SpanName", Data: c.SpanName},
		{Name: "SpanKind", Data: c.SpanKind},
		{Name: "ServiceName", Data: c.ServiceName}, //nolint:goconst // column name
		{Name: "ResourceAttributes", Data: c.ResourceAttributes},
		{Name: "SpanAttributes", Data: c.SpanAttributes},
		{Name: "Duration", Data: c.Duration},
		{Name: "StatusCode", Data: c.StatusCode},
		{Name: "StatusMessage", Data: c.StatusMessage},
		{Name: "Events.Timestamp", Data: c.EventsTimestamp},
		{Name: "Events.Name", Data: c.EventsName},
		{Name: "Events.Attributes", Data: c.EventsAttributes},
		{Name: "Links.TraceId", Data: c.LinksTraceId},
		{Name: "Links.SpanId", Data: c.LinksSpanId},
		{Name: "Links.TraceState", Data: c.LinksTraceState},
		{Name: "Links.Attributes", Data: c.LinksAttributes},
	}
}

type TracesBatch struct {
	batch[*tracesColumns]
}

func NewTracesBatch(limit int, timeout time.Duration, exec func(query ch.Query) error) *TracesBatch {
	b := &TracesBatch{}
	b.start(limit, timeout, "@@table_otel_traces@@", newTracesColumns, exec)
	return b
}

func (b *TracesBatch) add(rows *spanRows) {
	b.write(func(c *tracesColumns) { c.append(rows.rows) })
}
