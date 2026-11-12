package collector

import (
	"errors"
	"slices"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/easyproto"
	promModel "github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
)

var errMetricsWireType = errors.New("unexpected wire type in a remote write request")

type metricSample struct {
	timestamp int64
	value     float64
}

type metricRow struct {
	name    string
	hash    uint64
	labels  []logAttr
	samples []metricSample
}

type metricMetadata struct {
	familyName string
	typ        string
	help       string
	unit       string
}

type metricRows struct {
	attrList
	rows     []metricRow
	samples  []metricSample
	metadata []metricMetadata
}

var metricRowsPool = sync.Pool{New: func() any { return &metricRows{} }}

func getMetricRows() *metricRows {
	return metricRowsPool.Get().(*metricRows)
}

func putMetricRows(l *metricRows) {
	clear(l.rows)
	clear(l.attrs)
	clear(l.metadata)
	l.rows = l.rows[:0]
	l.attrs = l.attrs[:0]
	l.samples = l.samples[:0]
	l.metadata = l.metadata[:0]
	metricRowsPool.Put(l)
}

func (l *metricRows) Unmarshal(data []byte) error {
	var fc easyproto.FieldContext
	for len(data) > 0 {
		var err error
		if data, err = nextField(&fc, data); err != nil {
			return err
		}
		switch fc.FieldNum {
		case 1:
			b, ok := fc.MessageData()
			if !ok {
				return errMetricsWireType
			}
			if err = l.unmarshalTimeSeries(b); err != nil {
				return err
			}
		case 3:
			b, ok := fc.MessageData()
			if !ok {
				return errMetricsWireType
			}
			if err = l.unmarshalMetadata(b); err != nil {
				return err
			}
		}
	}
	return nil
}

func (l *metricRows) unmarshalTimeSeries(data []byte) error {
	var fc easyproto.FieldContext
	var err error
	from, samplesFrom := len(l.attrs), len(l.samples)
	for len(data) > 0 {
		if data, err = nextField(&fc, data); err != nil {
			return err
		}
		switch fc.FieldNum {
		case 1:
			b, ok := fc.MessageData()
			if !ok {
				return errMetricsWireType
			}
			if err = l.unmarshalLabel(from, b); err != nil {
				return err
			}
		case 2:
			b, ok := fc.MessageData()
			if !ok {
				return errMetricsWireType
			}
			if err = l.unmarshalSample(b); err != nil {
				return err
			}
		}
	}
	labels := l.attrs[from:len(l.attrs):len(l.attrs)]
	slices.SortFunc(labels, func(a, b logAttr) int { return strings.Compare(a.Key, b.Key) })
	row := metricRow{hash: labelsHash(labels), samples: l.samples[samplesFrom:len(l.samples):len(l.samples)]}
	if i, ok := slices.BinarySearchFunc(labels, promModel.MetricNameLabel, func(a logAttr, name string) int { return strings.Compare(a.Key, name) }); ok {
		row.name = labels[i].Value
		labels = slices.Delete(labels, i, i+1)
		l.attrs = l.attrs[:len(l.attrs)-1]
	}
	row.labels = labels
	l.rows = append(l.rows, row)
	return nil
}

func (l *metricRows) unmarshalLabel(from int, data []byte) error {
	var fc easyproto.FieldContext
	var err error
	var name, value string
	for len(data) > 0 {
		if data, err = nextField(&fc, data); err != nil {
			return err
		}
		switch fc.FieldNum {
		case 1:
			v, ok := fc.String()
			if !ok {
				return errMetricsWireType
			}
			name = v
		case 2:
			v, ok := fc.String()
			if !ok {
				return errMetricsWireType
			}
			value = v
		}
	}
	l.setAttr(from, name, value)
	return nil
}

func (l *metricRows) unmarshalSample(data []byte) error {
	var fc easyproto.FieldContext
	var err error
	var sample metricSample
	for len(data) > 0 {
		if data, err = nextField(&fc, data); err != nil {
			return err
		}
		switch fc.FieldNum {
		case 1:
			v, ok := fc.Double()
			if !ok {
				return errMetricsWireType
			}
			sample.value = v
		case 2:
			v, ok := fc.Int64()
			if !ok {
				return errMetricsWireType
			}
			sample.timestamp = v
		}
	}
	l.samples = append(l.samples, sample)
	return nil
}

func (l *metricRows) unmarshalMetadata(data []byte) error {
	var fc easyproto.FieldContext
	var err error
	var typ int32
	var md metricMetadata
	for len(data) > 0 {
		if data, err = nextField(&fc, data); err != nil {
			return err
		}
		switch fc.FieldNum {
		case 1:
			v, ok := fc.Uint64()
			if !ok {
				return errMetricsWireType
			}
			typ = int32(v) //nolint:gosec // same truncation as protobuf enums
		case 2:
			v, ok := fc.String()
			if !ok {
				return errMetricsWireType
			}
			md.familyName = v
		case 4:
			v, ok := fc.String()
			if !ok {
				return errMetricsWireType
			}
			md.help = v
		case 5:
			v, ok := fc.String()
			if !ok {
				return errMetricsWireType
			}
			md.unit = v
		}
	}
	md.typ = prompb.MetricMetadata_MetricType(typ).String()
	l.metadata = append(l.metadata, md)
	return nil
}

const (
	fnvOffset64 = 14695981039346656037
	fnvPrime64  = 1099511628211
)

func labelsHash(labels []logAttr) uint64 {
	h := uint64(fnvOffset64)
	for _, l := range labels {
		h = hashAdd(h, l.Key)
		h = hashAdd(h, l.Value)
	}
	return h
}

func hashAdd(h uint64, s string) uint64 {
	for i := range len(s) {
		h ^= uint64(s[i])
		h *= fnvPrime64
	}
	h ^= uint64(promModel.SeparatorByte)
	h *= fnvPrime64
	return h
}
