package collector

import (
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ClickHouse/ch-go"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	promModel "github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func metricsRequest(t *testing.T) *http.Request {
	r, err := http.NewRequest(http.MethodPost, "/v1/metrics", nil)
	require.NoError(t, err)
	r.Header.Set("Content-Type", "application/x-protobuf")
	r.Header.Set("Content-Encoding", "snappy")
	return r
}

func TestAddLabelsIfNeeded(t *testing.T) {
	req := &prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels:    []prompb.Label{{Name: promModel.MetricNameLabel, Value: "up"}, {Name: "job", Value: "agent"}},
				Samples:   []prompb.Sample{{Value: 1, Timestamp: 1000}, {Value: 0, Timestamp: 2000}},
				Exemplars: []prompb.Exemplar{{Labels: []prompb.Label{{Name: "trace_id", Value: "abc"}}, Value: 1, Timestamp: 1000}},
			},
			{Labels: []prompb.Label{{Name: promModel.MetricNameLabel, Value: "empty"}}},
			{},
		},
		Metadata: []prompb.MetricMetadata{{Type: prompb.MetricMetadata_GAUGE, MetricFamilyName: "up", Help: "help", Unit: "unit"}},
	}
	data, err := proto.Marshal(req)
	require.NoError(t, err)
	body := snappy.Encode(nil, data)
	r := metricsRequest(t)

	res, err := addLabelsIfNeeded(r, body, nil)
	require.NoError(t, err)
	assert.Equal(t, body, res)

	res, err = addLabelsIfNeeded(r, body, map[string]string{"coroot_project_id": "p1", "a": ""})
	require.NoError(t, err)
	decompressed, err := snappy.Decode(nil, res)
	require.NoError(t, err)
	actual := &prompb.WriteRequest{}
	require.NoError(t, proto.Unmarshal(decompressed, actual))
	for i := range req.Timeseries {
		req.Timeseries[i].Labels = append(req.Timeseries[i].Labels, prompb.Label{Name: "a"}, prompb.Label{Name: "coroot_project_id", Value: "p1"})
	}
	assert.Equal(t, req, actual)

	_, err = addLabelsIfNeeded(r, snappy.Encode(nil, data[:len(data)-1]), map[string]string{"a": "b"})
	require.Error(t, err)
	_, err = addLabelsIfNeeded(r, body[:len(body)-1], map[string]string{"a": "b"})
	require.Error(t, err)
	for i := range data {
		_, _ = addLabelsIfNeeded(r, snappy.Encode(nil, data[:i]), map[string]string{"a": "b"})
	}

	r.Header.Set("Content-Encoding", "gzip")
	_, err = addLabelsIfNeeded(r, body, map[string]string{"a": "b"})
	assert.Error(t, err)
}

func TestMetricRowsUnmarshal(t *testing.T) {
	req := &prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels:    []prompb.Label{{Name: "job", Value: "agent"}, {Name: promModel.MetricNameLabel, Value: "up"}, {Name: "Zone", Value: "a"}, {Name: "job", Value: "node-agent"}},
				Samples:   []prompb.Sample{{Value: 1, Timestamp: 1500}, {Value: 0.5, Timestamp: 16500}},
				Exemplars: []prompb.Exemplar{{Labels: []prompb.Label{{Name: "trace_id", Value: "abc"}}, Value: 1, Timestamp: 1000}},
			},
			{Labels: []prompb.Label{{Name: "instance", Value: "i1"}}, Samples: []prompb.Sample{{Value: 2, Timestamp: 2000}}},
			{},
		},
		Metadata: []prompb.MetricMetadata{
			{Type: prompb.MetricMetadata_GAUGE, MetricFamilyName: "up", Help: "help", Unit: "unit"},
			{Type: prompb.MetricMetadata_MetricType(42), MetricFamilyName: "other"},
		},
	}
	data, err := proto.Marshal(req)
	require.NoError(t, err)

	rows := &metricRows{}
	require.NoError(t, unmarshalMetricsRequest(metricsRequest(t), snappy.Encode(nil, data), rows))
	require.Len(t, rows.rows, 3)

	r := rows.rows[0]
	assert.Equal(t, "up", r.name)
	assert.Equal(t, []logAttr{{Key: "Zone", Value: "a"}, {Key: "job", Value: "node-agent"}}, r.labels)
	assert.Equal(t, promModel.LabelsToSignature(map[string]string{promModel.MetricNameLabel: "up", "Zone": "a", "job": "node-agent"}), r.hash)
	assert.Equal(t, []metricSample{{timestamp: 1500, value: 1}, {timestamp: 16500, value: 0.5}}, r.samples)

	r = rows.rows[1]
	assert.Empty(t, r.name)
	assert.Equal(t, []logAttr{{Key: "instance", Value: "i1"}}, r.labels)
	assert.Equal(t, promModel.LabelsToSignature(map[string]string{"instance": "i1"}), r.hash)

	r = rows.rows[2]
	assert.Empty(t, r.labels)
	assert.Empty(t, r.samples)
	assert.Equal(t, promModel.LabelsToSignature(nil), r.hash)

	assert.Equal(t, []metricMetadata{
		{familyName: "up", typ: "GAUGE", help: "help", unit: "unit"},
		{familyName: "other", typ: "42"},
	}, rows.metadata)

	for i := range data {
		_ = (&metricRows{}).Unmarshal(data[:i])
	}
	assert.Error(t, (&metricRows{}).Unmarshal(data[:len(data)-1]))
}

func TestMetricsBatch(t *testing.T) {
	var samples, metadata atomic.Int64
	batch := NewMetricsBatch(100, time.Hour, func(q ch.Query) error {
		rows := q.Input[0].Data.Rows()
		for _, c := range q.Input {
			assert.Equal(t, rows, c.Data.Rows(), c.Name)
		}
		if strings.Contains(q.Body, "metrics_metadata") {
			metadata.Add(int64(rows))
		} else {
			samples.Add(int64(rows))
		}
		return nil
	})
	req := &prompb.WriteRequest{Metadata: []prompb.MetricMetadata{{MetricFamilyName: "up"}}}
	for i := range 7 {
		req.Timeseries = append(req.Timeseries, prompb.TimeSeries{
			Labels:  []prompb.Label{{Name: promModel.MetricNameLabel, Value: "up"}, {Name: "n", Value: strconv.Itoa(i)}},
			Samples: []prompb.Sample{{Value: 1, Timestamp: 1000}, {Value: 1, Timestamp: 2000}},
		})
	}
	data, err := proto.Marshal(req)
	require.NoError(t, err)
	done := make(chan struct{})
	for range 8 {
		go func() {
			defer func() { done <- struct{}{} }()
			for range 100 {
				rows := getMetricRows()
				assert.NoError(t, rows.Unmarshal(data))
				batch.add(rows)
				putMetricRows(rows)
			}
		}()
	}
	for range 8 {
		<-done
	}
	batch.Close()
	assert.Equal(t, int64(8*100*7*2), samples.Load())
	assert.Equal(t, int64(8*100), metadata.Load())
}
