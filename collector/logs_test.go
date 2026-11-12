package collector

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ClickHouse/ch-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	collectorv1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	logsv1 "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcev1 "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/proto"
)

func strValue(s string) *commonv1.AnyValue {
	return &commonv1.AnyValue{Value: &commonv1.AnyValue_StringValue{StringValue: s}}
}

func strAttr(k, v string) *commonv1.KeyValue {
	return &commonv1.KeyValue{Key: k, Value: strValue(v)}
}

func testLogsRequest(resources, records int) *collectorv1.ExportLogsServiceRequest {
	req := &collectorv1.ExportLogsServiceRequest{}
	for i := range resources {
		sl := &logsv1.ScopeLogs{}
		for j := range records {
			lr := &logsv1.LogRecord{
				TimeUnixNano:   uint64(1700000000000000000 + j),
				SeverityNumber: logsv1.SeverityNumber_SEVERITY_NUMBER_INFO,
				SeverityText:   "info",
				Body:           strValue(fmt.Sprintf("request %d completed in 12ms", j)),
			}
			for k := range 30 {
				lr.Attributes = append(lr.Attributes, strAttr(fmt.Sprintf("attribute.name.%d", k), fmt.Sprintf("value-%d-%d", j, k)))
			}
			sl.LogRecords = append(sl.LogRecords, lr)
		}
		req.ResourceLogs = append(req.ResourceLogs, &logsv1.ResourceLogs{
			Resource: &resourcev1.Resource{Attributes: []*commonv1.KeyValue{
				strAttr("host.name", fmt.Sprintf("host-%d", i)),
				strAttr("service.name", fmt.Sprintf("service-%d", i)),
			}},
			ScopeLogs: []*logsv1.ScopeLogs{sl},
		})
	}
	return req
}

func TestLogRowsUnmarshal(t *testing.T) {
	req := testLogsRequest(3, 5)
	req.ResourceLogs = append(req.ResourceLogs, &logsv1.ResourceLogs{
		Resource: &resourcev1.Resource{Attributes: []*commonv1.KeyValue{
			strAttr("service.name", "old"),
			strAttr("service.name", "svc"),
			{Key: "replicas", Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_IntValue{IntValue: -3}}},
		}},
		ScopeLogs: []*logsv1.ScopeLogs{
			{
				Scope: &commonv1.InstrumentationScope{Name: "scope", Version: "1.2.3"},
				LogRecords: []*logsv1.LogRecord{
					{
						TimeUnixNano: 1700000000000000001,
						SeverityText: "ERROR",
						Body:         strValue("failed"),
						TraceId:      []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
						SpanId:       []byte{1, 2, 3, 4, 5, 6, 7, 8},
						Flags:        1,
						Attributes: []*commonv1.KeyValue{
							strAttr("a", "1"),
							strAttr("a", "2"),
							strAttr("otel.scope.name", "overridden"),
							strAttr("empty", ""),
							{Key: "bool", Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_BoolValue{BoolValue: true}}},
							{Key: "false", Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_BoolValue{}}},
							{Key: "int", Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_IntValue{IntValue: 42}}},
							{Key: "zero", Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_IntValue{}}},
							{Key: "double", Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_DoubleValue{DoubleValue: 1.5}}},
							{Key: "bytes", Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_BytesValue{BytesValue: []byte("raw")}}},
							{Key: "array", Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_ArrayValue{ArrayValue: &commonv1.ArrayValue{Values: []*commonv1.AnyValue{strValue("x"), strValue("y")}}}}},
							{Key: "kvlist", Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_KvlistValue{KvlistValue: &commonv1.KeyValueList{Values: []*commonv1.KeyValue{strAttr("k", "v")}}}}},
							{Key: "no_value_type", Value: &commonv1.AnyValue{}},
							{Key: "no_value"},
							{Key: "kvlist_no_value", Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_KvlistValue{KvlistValue: &commonv1.KeyValueList{Values: []*commonv1.KeyValue{{Key: "k"}}}}}},
						},
					},
					{TimeUnixNano: 1 << 63, Body: strValue("negative timestamp")},
					{TimeUnixNano: 2, Body: &commonv1.AnyValue{Value: &commonv1.AnyValue_IntValue{IntValue: 1}}},
					{},
				},
			},
			{LogRecords: []*logsv1.LogRecord{{TimeUnixNano: 3, Body: strValue("no scope")}}},
		},
	})
	data, err := proto.Marshal(req)
	require.NoError(t, err)

	actual := &logRows{}
	require.NoError(t, actual.Unmarshal(data))
	require.Len(t, actual.rows, 3*5+4)

	r := actual.rows[0]
	assert.Equal(t, int64(1700000000000000000), r.timestamp)
	assert.Equal(t, "info", r.severityText)
	assert.Equal(t, int32(logsv1.SeverityNumber_SEVERITY_NUMBER_INFO), r.severityNumber)
	assert.Equal(t, "service-0", r.serviceName)
	assert.Equal(t, "request 0 completed in 12ms", r.body)
	assert.Len(t, r.logAttributes, 30)
	assert.Equal(t, "service-2", actual.rows[14].serviceName)

	r = actual.rows[15]
	assert.Equal(t, "svc", r.serviceName)
	assert.Equal(t, []logAttr{{Key: "service.name", Value: "svc"}, {Key: "replicas", Value: "-3"}}, r.resourceAttributes)
	assert.Equal(t, []logAttr{
		{Key: "a", Value: "2"},
		{Key: "otel.scope.name", Value: "scope"},
		{Key: "empty", Value: ""},
		{Key: "bool", Value: "true"},
		{Key: "false", Value: "false"},
		{Key: "int", Value: "42"},
		{Key: "zero", Value: "0"},
		{Key: "double", Value: "1.5"},
		{Key: "bytes", Value: "raw"},
		{Key: "array", Value: `["x","y"]`},
		{Key: "kvlist", Value: `{"k":"v"}`},
		{Key: "no_value_type", Value: "unknown attribute value type: <nil>"},
		{Key: "no_value", Value: "unknown attribute value type: <nil>"},
		{Key: "kvlist_no_value", Value: `{"k":"unknown attribute value type: \u003cnil\u003e"}`},
		{Key: "otel.scope.version", Value: "1.2.3"},
	}, r.logAttributes)
	assert.Empty(t, actual.rows[16].body)

	for i := range data {
		_ = (&logRows{}).Unmarshal(data[:i])
	}
	assert.Error(t, (&logRows{}).Unmarshal(data[:len(data)-1]))
}

func TestLogsBatch(t *testing.T) {
	var inserted atomic.Int64
	batch := NewLogsBatch(100, time.Hour, func(q ch.Query) error {
		rows := q.Input[0].Data.Rows()
		for _, c := range q.Input {
			assert.Equal(t, rows, c.Data.Rows())
		}
		time.Sleep(time.Millisecond)
		inserted.Add(int64(rows))
		return nil
	})
	req := testLogsRequest(5, 7)
	data, err := proto.Marshal(req)
	require.NoError(t, err)
	done := make(chan struct{})
	for range 8 {
		go func() {
			defer func() { done <- struct{}{} }()
			for range 100 {
				rows := getLogRows()
				assert.NoError(t, rows.Unmarshal(data))
				batch.add(rows)
				putLogRows(rows)
			}
		}()
	}
	for range 8 {
		<-done
	}
	batch.Close()
	assert.Equal(t, int64(8*100*5*7), inserted.Load())
}

func TestValueToStringDepth(t *testing.T) {
	nested := func(depth int, leaf string) *commonv1.AnyValue {
		v := strValue(leaf)
		for range depth {
			v = &commonv1.AnyValue{Value: &commonv1.AnyValue_ArrayValue{ArrayValue: &commonv1.ArrayValue{Values: []*commonv1.AnyValue{v}}}}
		}
		return v
	}
	assert.Equal(t, `["a"]`, valueToString(nested(1, "a")))
	assert.Equal(t, `["[\"a\"]"]`, valueToString(nested(2, "a")))
	assert.Equal(t, valueToString(nested(maxValueDepth, "...")), valueToString(nested(maxValueDepth+1, "a")))
	assert.Equal(t, valueToString(nested(maxValueDepth, "...")), valueToString(nested(1000, `"`)))
}
