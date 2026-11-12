package collector

import (
	"sync"

	"github.com/VictoriaMetrics/easyproto"
	semconv "go.opentelemetry.io/collector/semconv/v1.18.0"
	tracev1 "go.opentelemetry.io/proto/otlp/trace/v1"
)

type spanEvent struct {
	timestamp  int64
	name       string
	attributes []logAttr
}

type spanLink struct {
	traceId    []byte
	spanId     []byte
	traceState string
	attributes []logAttr
}

type spanRow struct {
	timestamp          int64
	traceId            []byte
	spanId             []byte
	parentSpanId       []byte
	traceState         string
	name               string
	kind               int32
	serviceName        string
	duration           int64
	statusCode         int32
	statusMessage      string
	resourceAttributes []logAttr
	spanAttributes     []logAttr
	events             []spanEvent
	links              []spanLink
}

type spanRows struct {
	attrList
	nested attrList
	rows   []spanRow
	events []spanEvent
	links  []spanLink
}

var spanRowsPool = sync.Pool{New: func() any { return &spanRows{} }}

func getSpanRows() *spanRows {
	return spanRowsPool.Get().(*spanRows)
}

func putSpanRows(l *spanRows) {
	clear(l.rows)
	clear(l.attrs)
	clear(l.nested.attrs)
	clear(l.events)
	clear(l.links)
	l.rows = l.rows[:0]
	l.attrs = l.attrs[:0]
	l.nested.attrs = l.nested.attrs[:0]
	l.events = l.events[:0]
	l.links = l.links[:0]
	spanRowsPool.Put(l)
}

func (l *spanRows) Unmarshal(data []byte) error {
	var fc easyproto.FieldContext
	for len(data) > 0 {
		var err error
		if data, err = nextField(&fc, data); err != nil {
			return err
		}
		if fc.FieldNum != 1 {
			continue
		}
		if d, ok := fc.MessageData(); ok {
			if err = l.unmarshalResourceSpans(d); err != nil {
				return err
			}
		}
	}
	return nil
}

func (l *spanRows) unmarshalResourceSpans(data []byte) error {
	var fc easyproto.FieldContext
	var err error
	from := len(l.attrs)
	for d := data; len(d) > 0; {
		if d, err = nextField(&fc, d); err != nil {
			return err
		}
		if fc.FieldNum != 1 {
			continue
		}
		resource, ok := fc.MessageData()
		if !ok {
			continue
		}
		if err = l.unmarshalResource(from, resource); err != nil {
			return err
		}
	}
	resourceAttributes := l.attrs[from:len(l.attrs):len(l.attrs)]
	serviceName := getServiceName(resourceAttributes)
	for d := data; len(d) > 0; {
		if d, err = nextField(&fc, d); err != nil {
			return err
		}
		if fc.FieldNum != 2 {
			continue
		}
		if b, ok := fc.MessageData(); ok {
			if err = l.unmarshalScopeSpans(b, serviceName, resourceAttributes); err != nil {
				return err
			}
		}
	}
	return nil
}

func (l *spanRows) unmarshalScopeSpans(data []byte, serviceName string, resourceAttributes []logAttr) error {
	var fc easyproto.FieldContext
	var err error
	var scopeName, scopeVersion string
	for d := data; len(d) > 0; {
		if d, err = nextField(&fc, d); err != nil {
			return err
		}
		if fc.FieldNum != 1 {
			continue
		}
		scope, ok := fc.MessageData()
		if !ok {
			continue
		}
		if err = unmarshalScope(scope, &scopeName, &scopeVersion); err != nil {
			return err
		}
	}
	for d := data; len(d) > 0; {
		if d, err = nextField(&fc, d); err != nil {
			return err
		}
		if fc.FieldNum != 2 {
			continue
		}
		if b, ok := fc.MessageData(); ok {
			if err = l.unmarshalSpan(b, serviceName, scopeName, scopeVersion, resourceAttributes); err != nil {
				return err
			}
		}
	}
	return nil
}

func (l *spanRows) unmarshalSpan(data []byte, serviceName, scopeName, scopeVersion string, resourceAttributes []logAttr) error {
	var fc easyproto.FieldContext
	var err error
	var start, end uint64
	row := spanRow{serviceName: serviceName, resourceAttributes: resourceAttributes}
	from, eventsFrom, linksFrom := len(l.attrs), len(l.events), len(l.links)
	for len(data) > 0 {
		if data, err = nextField(&fc, data); err != nil {
			return err
		}
		switch fc.FieldNum {
		case 1:
			if b, ok := fc.Bytes(); ok {
				row.traceId = b
			}
		case 2:
			if b, ok := fc.Bytes(); ok {
				row.spanId = b
			}
		case 3:
			if v, ok := fc.String(); ok {
				row.traceState = v
			}
		case 4:
			if b, ok := fc.Bytes(); ok {
				row.parentSpanId = b
			}
		case 5:
			if v, ok := fc.String(); ok {
				row.name = v
			}
		case 6:
			if v, ok := fc.Uint64(); ok {
				row.kind = int32(v) //nolint:gosec // same truncation as protobuf enums
			}
		case 7:
			if v, ok := fc.Fixed64(); ok {
				start = v
			}
		case 8:
			if v, ok := fc.Fixed64(); ok {
				end = v
			}
		case 9:
			if b, ok := fc.MessageData(); ok {
				if err = l.unmarshalAttr(from, b); err != nil {
					return err
				}
			}
		case 11:
			if b, ok := fc.MessageData(); ok {
				if err = l.unmarshalEvent(b); err != nil {
					return err
				}
			}
		case 13:
			if b, ok := fc.MessageData(); ok {
				if err = l.unmarshalLink(b); err != nil {
					return err
				}
			}
		case 15:
			if b, ok := fc.MessageData(); ok {
				if err = unmarshalStatus(b, &row); err != nil {
					return err
				}
			}
		}
	}
	if scopeName != "" {
		l.setAttr(from, semconv.AttributeOtelScopeName, scopeName)
	}
	if scopeVersion != "" {
		l.setAttr(from, semconv.AttributeOtelScopeVersion, scopeVersion)
	}
	row.timestamp = int64(start)
	row.duration = int64(end - start)
	row.spanAttributes = l.attrs[from:len(l.attrs):len(l.attrs)]
	row.events = l.events[eventsFrom:len(l.events):len(l.events)]
	row.links = l.links[linksFrom:len(l.links):len(l.links)]
	l.rows = append(l.rows, row)
	return nil
}

func (l *spanRows) unmarshalEvent(data []byte) error {
	var fc easyproto.FieldContext
	var err error
	var event spanEvent
	from := len(l.nested.attrs)
	for len(data) > 0 {
		if data, err = nextField(&fc, data); err != nil {
			return err
		}
		switch fc.FieldNum {
		case 1:
			if v, ok := fc.Fixed64(); ok {
				event.timestamp = int64(v) //nolint:gosec // stored as is
			}
		case 2:
			if v, ok := fc.String(); ok {
				event.name = v
			}
		case 3:
			if b, ok := fc.MessageData(); ok {
				if err = l.nested.unmarshalAttr(from, b); err != nil {
					return err
				}
			}
		}
	}
	event.attributes = l.nested.attrs[from:len(l.nested.attrs):len(l.nested.attrs)]
	l.events = append(l.events, event)
	return nil
}

func (l *spanRows) unmarshalLink(data []byte) error {
	var fc easyproto.FieldContext
	var err error
	var link spanLink
	from := len(l.nested.attrs)
	for len(data) > 0 {
		if data, err = nextField(&fc, data); err != nil {
			return err
		}
		switch fc.FieldNum {
		case 1:
			if b, ok := fc.Bytes(); ok {
				link.traceId = b
			}
		case 2:
			if b, ok := fc.Bytes(); ok {
				link.spanId = b
			}
		case 3:
			if v, ok := fc.String(); ok {
				link.traceState = v
			}
		case 4:
			if b, ok := fc.MessageData(); ok {
				if err = l.nested.unmarshalAttr(from, b); err != nil {
					return err
				}
			}
		}
	}
	link.attributes = l.nested.attrs[from:len(l.nested.attrs):len(l.nested.attrs)]
	l.links = append(l.links, link)
	return nil
}

func unmarshalStatus(data []byte, row *spanRow) error {
	var fc easyproto.FieldContext
	var err error
	for len(data) > 0 {
		if data, err = nextField(&fc, data); err != nil {
			return err
		}
		switch fc.FieldNum {
		case 2:
			if v, ok := fc.String(); ok {
				row.statusMessage = v
			}
		case 3:
			if v, ok := fc.Uint64(); ok {
				row.statusCode = int32(v) //nolint:gosec // same truncation as protobuf enums
			}
		}
	}
	return nil
}

var (
	spanKinds   = enumNames(tracev1.Span_SpanKind_name, func(i int32) string { return tracev1.Span_SpanKind(i).String() })
	statusCodes = enumNames(tracev1.Status_StatusCode_name, func(i int32) string { return tracev1.Status_StatusCode(i).String() })
)

func enumNames(names map[int32]string, toString func(int32) string) func(int32) string {
	known := make([]string, len(names))
	for i := range known {
		known[i] = toString(int32(i))
	}
	return func(i int32) string {
		if i >= 0 && int(i) < len(known) {
			return known[i]
		}
		return toString(i)
	}
}
