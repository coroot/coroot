package collector

import (
	"errors"
	"strconv"
	"sync"

	chproto "github.com/ClickHouse/ch-go/proto"
	"github.com/VictoriaMetrics/easyproto"
	semconv "go.opentelemetry.io/collector/semconv/v1.18.0"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	resourcev1 "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/proto"
)

type logAttr = chproto.KV[string, string]

type logRow struct {
	timestamp          int64
	traceId            []byte
	spanId             []byte
	traceFlags         uint32
	severityText       string
	severityNumber     int32
	serviceName        string
	body               string
	resourceAttributes []logAttr
	logAttributes      []logAttr
}

type logRows struct {
	attrList
	rows []logRow
}

var logRowsPool = sync.Pool{New: func() any { return &logRows{} }}

func getLogRows() *logRows {
	return logRowsPool.Get().(*logRows)
}

func putLogRows(l *logRows) {
	clear(l.rows)
	clear(l.attrs)
	l.rows = l.rows[:0]
	l.attrs = l.attrs[:0]
	logRowsPool.Put(l)
}

type attrList struct {
	attrs []logAttr
}

func (l *attrList) setAttr(from int, key, value string) {
	for i := from; i < len(l.attrs); i++ {
		if l.attrs[i].Key == key {
			l.attrs[i].Value = value
			return
		}
	}
	l.attrs = append(l.attrs, logAttr{Key: key, Value: value})
}

func (l *logRows) Unmarshal(data []byte) error {
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
			if err = l.unmarshalResourceLogs(d); err != nil {
				return err
			}
		}
	}
	return nil
}

func (l *logRows) unmarshalResourceLogs(data []byte) error {
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
			if err = l.unmarshalScopeLogs(b, serviceName, resourceAttributes); err != nil {
				return err
			}
		}
	}
	return nil
}

func (l *logRows) unmarshalScopeLogs(data []byte, serviceName string, resourceAttributes []logAttr) error {
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
			if err = l.unmarshalLogRecord(b, serviceName, scopeName, scopeVersion, resourceAttributes); err != nil {
				return err
			}
		}
	}
	return nil
}

func (l *logRows) unmarshalLogRecord(data []byte, serviceName, scopeName, scopeVersion string, resourceAttributes []logAttr) error {
	var fc easyproto.FieldContext
	var err error
	row := logRow{serviceName: serviceName, resourceAttributes: resourceAttributes}
	from := len(l.attrs)
	for len(data) > 0 {
		if data, err = nextField(&fc, data); err != nil {
			return err
		}
		switch fc.FieldNum {
		case 1:
			if v, ok := fc.Fixed64(); ok {
				row.timestamp = int64(v) //nolint:gosec // negative is skipped below
			}
		case 2:
			if v, ok := fc.Uint64(); ok {
				row.severityNumber = int32(v) //nolint:gosec // same truncation as protobuf enums
			}
		case 3:
			if v, ok := fc.String(); ok {
				row.severityText = v
			}
		case 5:
			if b, ok := fc.MessageData(); ok {
				if row.body, err = unmarshalBody(b, row.body); err != nil {
					return err
				}
			}
		case 6:
			if b, ok := fc.MessageData(); ok {
				if err = l.unmarshalAttr(from, b); err != nil {
					return err
				}
			}
		case 8:
			if v, ok := fc.Fixed32(); ok {
				row.traceFlags = v
			}
		case 9:
			if b, ok := fc.Bytes(); ok {
				row.traceId = b
			}
		case 10:
			if b, ok := fc.Bytes(); ok {
				row.spanId = b
			}
		}
	}
	if row.timestamp < 0 {
		l.attrs = l.attrs[:from]
		return nil
	}
	if scopeName != "" {
		l.setAttr(from, semconv.AttributeOtelScopeName, scopeName)
	}
	if scopeVersion != "" {
		l.setAttr(from, semconv.AttributeOtelScopeVersion, scopeVersion)
	}
	row.logAttributes = l.attrs[from:len(l.attrs):len(l.attrs)]
	l.rows = append(l.rows, row)
	return nil
}

func unmarshalBody(data []byte, body string) (string, error) {
	var fc easyproto.FieldContext
	var err error
	all := data
	nested := false
	for len(data) > 0 {
		if data, err = nextField(&fc, data); err != nil {
			return "", err
		}
		switch fc.FieldNum {
		case 1:
			if v, ok := fc.String(); ok {
				body = v
			}
		case 2, 3:
			if _, ok := fc.Uint64(); ok {
				body = ""
			}
		case 4:
			if _, ok := fc.Fixed64(); ok {
				body = ""
			}
		case 5, 6:
			if _, ok := fc.MessageData(); ok {
				body = ""
				nested = true
			}
		case 7:
			if _, ok := fc.Bytes(); ok {
				body = ""
			}
		}
	}
	if nested {
		if err = proto.Unmarshal(all, &commonv1.AnyValue{}); err != nil {
			return "", err
		}
	}
	return body, nil
}

func (l *attrList) unmarshalAttr(from int, data []byte) error {
	var fc easyproto.FieldContext
	var err error
	all := data
	var key string
	var value []byte
	var hasValue bool
	for len(data) > 0 {
		if data, err = nextField(&fc, data); err != nil {
			return err
		}
		b, ok := fc.MessageData()
		if !ok {
			continue
		}
		switch fc.FieldNum {
		case 1:
			key, _ = fc.String()
		case 2:
			if hasValue {
				var kv commonv1.KeyValue
				if err = proto.Unmarshal(all, &kv); err != nil {
					return err
				}
				l.setAttr(from, kv.GetKey(), valueToString(kv.GetValue()))
				return nil
			}
			value, hasValue = b, true
		}
	}
	if !hasValue {
		l.setAttr(from, key, valueToString(&commonv1.AnyValue{}))
		return nil
	}
	v, err := unmarshalValue(value)
	if err != nil {
		return err
	}
	l.setAttr(from, key, v)
	return nil
}

func (l *attrList) unmarshalResource(from int, data []byte) error {
	var fc easyproto.FieldContext
	var err error
	for d := data; len(d) > 0; {
		if d, err = nextField(&fc, d); err != nil {
			return err
		}
		b, ok := fc.MessageData()
		if !ok {
			continue
		}
		if fc.FieldNum == 1 {
			err = l.unmarshalAttr(from, b)
		} else {
			err = proto.Unmarshal(data, &resourcev1.Resource{})
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func unmarshalScope(data []byte, name, version *string) error {
	var fc easyproto.FieldContext
	var err error
	for d := data; len(d) > 0; {
		if d, err = nextField(&fc, d); err != nil {
			return err
		}
		v, ok := fc.String()
		if !ok {
			continue
		}
		switch fc.FieldNum {
		case 1:
			*name = v
		case 2:
			*version = v
		default:
			if err = proto.Unmarshal(data, &commonv1.InstrumentationScope{}); err != nil {
				return err
			}
		}
	}
	return nil
}

func getServiceName(resourceAttributes []logAttr) string {
	var serviceName string
	for _, a := range resourceAttributes {
		if a.Key == semconv.AttributeServiceName {
			serviceName = a.Value
		}
	}
	return serviceName
}

func unmarshalValue(data []byte) (string, error) {
	var fc easyproto.FieldContext
	rest, err := nextField(&fc, data)
	if err == nil && len(rest) == 0 {
		switch fc.FieldNum {
		case 1:
			if v, ok := fc.String(); ok {
				return v, nil
			}
		case 2:
			if v, ok := fc.Uint64(); ok {
				return strconv.FormatBool(v != 0), nil
			}
		case 3:
			if v, ok := fc.Int64(); ok {
				return strconv.FormatInt(v, 10), nil
			}
		}
	}
	var value commonv1.AnyValue
	if err = proto.Unmarshal(data, &value); err != nil {
		return "", err
	}
	return valueToString(&value), nil
}

const maxFieldNumber = 1<<29 - 1

var errFieldNumber = errors.New("invalid field number")

func nextField(fc *easyproto.FieldContext, data []byte) ([]byte, error) {
	data, err := fc.NextField(data)
	if err != nil {
		return nil, err
	}
	if fc.FieldNum == 0 || fc.FieldNum > maxFieldNumber {
		return nil, errFieldNumber
	}
	return data, nil
}
