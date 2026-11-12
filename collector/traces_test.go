package collector

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/ClickHouse/ch-go"
	chproto "github.com/ClickHouse/ch-go/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	collectorv1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	resourcev1 "go.opentelemetry.io/proto/otlp/resource/v1"
	tracev1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

func testTracesRequest() *collectorv1.ExportTraceServiceRequest {
	traceId := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	return &collectorv1.ExportTraceServiceRequest{ResourceSpans: []*tracev1.ResourceSpans{
		{
			Resource: &resourcev1.Resource{Attributes: []*commonv1.KeyValue{
				strAttr("service.name", "old"),
				strAttr("service.name", "svc"),
				strAttr("host.name", "host"),
			}},
			ScopeSpans: []*tracev1.ScopeSpans{
				{
					Scope: &commonv1.InstrumentationScope{Name: "lib", Version: "2.0.1"},
					Spans: []*tracev1.Span{
						{
							TraceId:           traceId,
							SpanId:            []byte{1, 2, 3, 4, 5, 6, 7, 8},
							ParentSpanId:      []byte{8, 7, 6, 5, 4, 3, 2, 1},
							TraceState:        "k=v",
							Name:              "GET /",
							Kind:              tracev1.Span_SPAN_KIND_SERVER,
							StartTimeUnixNano: 1700000000000000000,
							EndTimeUnixNano:   1700000000000005000,
							Attributes: []*commonv1.KeyValue{
								strAttr("a", "1"),
								strAttr("a", "2"),
								{Key: "int", Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_IntValue{IntValue: 42}}},
								{Key: "arr", Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_ArrayValue{ArrayValue: &commonv1.ArrayValue{Values: []*commonv1.AnyValue{strValue("x")}}}}},
								{Key: "none"},
							},
							Events: []*tracev1.Span_Event{
								{TimeUnixNano: 1700000000000001000, Name: "exception", Attributes: []*commonv1.KeyValue{strAttr("exception.type", "E"), strAttr("exception.type", "Error")}},
								{TimeUnixNano: 1700000000000002000, Name: "retry"},
							},
							Links: []*tracev1.Span_Link{
								{TraceId: traceId, SpanId: []byte{9, 9, 9, 9, 9, 9, 9, 9}, TraceState: "l=1", Attributes: []*commonv1.KeyValue{strAttr("link", "1")}},
							},
							Status: &tracev1.Status{Code: tracev1.Status_STATUS_CODE_ERROR, Message: "failed"},
						},
						{TraceId: traceId, SpanId: []byte{2}, Name: "child", Kind: tracev1.Span_SpanKind(42), StartTimeUnixNano: 10, EndTimeUnixNano: 5},
						{},
					},
				},
				{Spans: []*tracev1.Span{{Name: "no scope", StartTimeUnixNano: 3, EndTimeUnixNano: 4, Status: &tracev1.Status{Code: tracev1.Status_STATUS_CODE_OK}}}},
			},
		},
		{ScopeSpans: []*tracev1.ScopeSpans{{Spans: []*tracev1.Span{{Name: "no resource"}}}}},
	}}
}

func TestSpanRowsUnmarshal(t *testing.T) {
	req := testTracesRequest()
	data, err := proto.Marshal(req)
	require.NoError(t, err)

	actual := &spanRows{}
	require.NoError(t, actual.Unmarshal(data))
	require.Len(t, actual.rows, 5)

	r := actual.rows[0]
	assert.Equal(t, int64(1700000000000000000), r.timestamp)
	assert.Equal(t, []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, r.traceId)
	assert.Equal(t, []byte{1, 2, 3, 4, 5, 6, 7, 8}, r.spanId)
	assert.Equal(t, []byte{8, 7, 6, 5, 4, 3, 2, 1}, r.parentSpanId)
	assert.Equal(t, "k=v", r.traceState)
	assert.Equal(t, "GET /", r.name)
	assert.Equal(t, "svc", r.serviceName)
	assert.Equal(t, []logAttr{{Key: "service.name", Value: "svc"}, {Key: "host.name", Value: "host"}}, r.resourceAttributes)
	assert.Equal(t, []logAttr{
		{Key: "a", Value: "2"},
		{Key: "int", Value: "42"},
		{Key: "arr", Value: `["x"]`},
		{Key: "none", Value: "unknown attribute value type: <nil>"},
		{Key: "otel.scope.name", Value: "lib"},
		{Key: "otel.scope.version", Value: "2.0.1"},
	}, r.spanAttributes)
	assert.Equal(t, int64(5000), r.duration)
	assert.Equal(t, "SPAN_KIND_SERVER", spanKinds(r.kind))
	assert.Equal(t, "STATUS_CODE_ERROR", statusCodes(r.statusCode))
	assert.Equal(t, "failed", r.statusMessage)
	require.Len(t, r.events, 2)
	assert.Equal(t, int64(1700000000000001000), r.events[0].timestamp)
	assert.Equal(t, "exception", r.events[0].name)
	assert.Equal(t, "retry", r.events[1].name)
	assert.Equal(t, []logAttr{{Key: "exception.type", Value: "Error"}}, r.events[0].attributes)
	assert.Empty(t, r.events[1].attributes)
	require.Len(t, r.links, 1)
	assert.Equal(t, []byte{9, 9, 9, 9, 9, 9, 9, 9}, r.links[0].spanId)
	assert.Equal(t, "l=1", r.links[0].traceState)
	assert.Equal(t, []logAttr{{Key: "link", Value: "1"}}, r.links[0].attributes)
	assert.Equal(t, "42", spanKinds(actual.rows[1].kind))
	assert.Equal(t, int64(-5), actual.rows[1].duration)
	assert.Equal(t, "STATUS_CODE_UNSET", statusCodes(actual.rows[2].statusCode))
	assert.Equal(t, "no scope", actual.rows[3].name)
	assert.Equal(t, "STATUS_CODE_OK", statusCodes(actual.rows[3].statusCode))
	assert.Empty(t, actual.rows[3].spanAttributes)
	assert.Equal(t, "svc", actual.rows[3].serviceName)
	assert.Empty(t, actual.rows[4].serviceName)
	assert.Empty(t, actual.rows[4].resourceAttributes)

	for i := range data {
		_ = (&spanRows{}).Unmarshal(data[:i])
	}
	assert.Error(t, (&spanRows{}).Unmarshal(data[:len(data)-1]))
}

func TestTracesBatch(t *testing.T) {
	var inserted, events, links atomic.Int64
	batch := NewTracesBatch(100, time.Hour, func(q ch.Query) error {
		rows := q.Input[0].Data.Rows()
		for _, c := range q.Input {
			assert.Equal(t, rows, c.Data.Rows(), c.Name)
			switch c.Name {
			case "Events.Name":
				events.Add(int64(c.Data.(*chproto.ColArr[string]).Data.Rows()))
			case "Links.Attributes":
				links.Add(int64(c.Data.(*chproto.ColArr[map[string]string]).Data.Rows()))
			}
		}
		time.Sleep(time.Millisecond)
		inserted.Add(int64(rows))
		return nil
	})
	req := testTracesRequest()
	data, err := proto.Marshal(req)
	require.NoError(t, err)
	done := make(chan struct{})
	for range 8 {
		go func() {
			defer func() { done <- struct{}{} }()
			for range 100 {
				rows := getSpanRows()
				assert.NoError(t, rows.Unmarshal(data))
				batch.add(rows)
				putSpanRows(rows)
			}
		}()
	}
	for range 8 {
		<-done
	}
	batch.Close()
	assert.Equal(t, int64(8*100*5), inserted.Load())
	assert.Equal(t, int64(8*100*2), events.Load())
	assert.Equal(t, int64(8*100), links.Load())
}
