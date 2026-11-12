package collector

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/ClickHouse/ch-go"
	chproto "github.com/ClickHouse/ch-go/proto"
	"github.com/VictoriaMetrics/easyproto"
	"github.com/golang/snappy"
	"google.golang.org/protobuf/encoding/protowire"
	"k8s.io/klog"
)

var (
	secureClient = &http.Client{Transport: &http.Transport{
		TLSHandshakeTimeout: 10 * time.Second,
	}}
	insecureClient = &http.Client{Transport: &http.Transport{
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
	}}
)

type metricsBuffers struct {
	scratch   []byte
	rewritten []byte
}

var metricsBuffersPool = sync.Pool{New: func() any { return &metricsBuffers{} }}

func addLabelsIfNeeded(r *http.Request, body []byte, extraLabels map[string]string) ([]byte, error) {
	if len(extraLabels) == 0 {
		return body, nil
	}
	if err := checkMetricsRequestHeaders(r); err != nil {
		return nil, err
	}
	b := metricsBuffersPool.Get().(*metricsBuffers)
	defer metricsBuffersPool.Put(b)
	var err error
	if b.scratch, err = snappy.Decode(b.scratch[:cap(b.scratch)], body); err != nil {
		return nil, err
	}
	if b.rewritten, err = appendLabels(b.rewritten[:0], b.scratch, encodeLabels(extraLabels)); err != nil {
		return nil, err
	}
	b.scratch = snappy.Encode(b.scratch[:cap(b.scratch)], b.rewritten)
	return bytes.Clone(b.scratch), nil
}

func encodeLabels(labels map[string]string) []byte {
	names := make([]string, 0, len(labels))
	for name := range labels {
		names = append(names, name)
	}
	sort.Strings(names)
	var res []byte
	for _, name := range names {
		value := labels[name]
		label := protowire.AppendString(protowire.AppendTag(nil, 1, protowire.BytesType), name)
		label = protowire.AppendString(protowire.AppendTag(label, 2, protowire.BytesType), value)
		res = protowire.AppendBytes(protowire.AppendTag(res, 1, protowire.BytesType), label)
	}
	return res
}

func appendLabels(dst, writeRequest, labels []byte) ([]byte, error) {
	var fc easyproto.FieldContext
	for src := writeRequest; len(src) > 0; {
		rest, err := fc.NextField(src)
		if err != nil {
			return nil, err
		}
		field := src[:len(src)-len(rest)]
		src = rest
		timeseries, ok := fc.MessageData()
		if !ok || fc.FieldNum != 1 {
			dst = append(dst, field...)
			continue
		}
		dst = protowire.AppendTag(dst, 1, protowire.BytesType)
		dst = protowire.AppendVarint(dst, uint64(len(timeseries)+len(labels)))
		dst = append(dst, timeseries...)
		dst = append(dst, labels...)
	}
	return dst, nil
}

func (c *Collector) Metrics(w http.ResponseWriter, r *http.Request) {
	project, err := c.getProject(r.Header.Get(ApiKeyHeader))
	if err != nil {
		klog.Errorln(err)
		if errors.Is(err, ErrProjectNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	cfg := project.PrometheusConfig(c.globalPrometheus)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	if cfg.UseClickHouse {
		rows := getMetricRows()
		defer putMetricRows(rows)
		if err = unmarshalMetricsRequest(r, body, rows); err != nil {
			klog.Errorln(err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}
		c.getMetricsBatch(project).add(rows)
		return
	}

	var u *url.URL
	if cfg.RemoteWriteUrl == "" {
		u, err = url.Parse(cfg.Url)
		if err != nil {
			klog.Errorln(err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		u = u.JoinPath("/api/v1/write")
	} else {
		u, err = url.Parse(cfg.RemoteWriteUrl)
		if err != nil {
			klog.Errorln(err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	}

	if cfg.BasicAuth != nil {
		u.User = url.UserPassword(cfg.BasicAuth.User, cfg.BasicAuth.Password)
	}
	body, err = addLabelsIfNeeded(r, body, cfg.ExtraLabels)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), r.Method, u.String(), bytes.NewReader(body))
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	for _, h := range cfg.CustomHeaders {
		req.Header.Add(h.Key, h.Value)
	}
	for k, vs := range r.Header {
		if k == ApiKeyHeader {
			continue
		}
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	httpClient := secureClient
	if cfg.TlsSkipVerify {
		httpClient = insecureClient
	}
	res, err := httpClient.Do(req)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	defer func() {
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
	}()
	if res.StatusCode == http.StatusBadRequest {
		scanner := bufio.NewScanner(io.LimitReader(res.Body, 1024))
		line := ""
		if scanner.Scan() {
			line = scanner.Text()
		}
		klog.Errorf("failed to write: got %d (%s) from prometheus, responding to the agent with 200 (to prevent retry)", res.StatusCode, line)
		w.WriteHeader(http.StatusOK)
		return
	}
	if res.StatusCode > 400 {
		klog.Errorf("failed to write: got %d from prometheus", res.StatusCode)
	}
	for k, vs := range res.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(res.StatusCode)
	_, _ = io.Copy(w, res.Body)
}

func checkMetricsRequestHeaders(r *http.Request) error {
	if r.Header.Get("Content-Type") != "application/x-protobuf" {
		return errors.New("expected application/x-protobuf content-type")
	}
	if r.Header.Get("Content-Encoding") != "snappy" {
		return errors.New("expected snappy content-encoding")
	}
	return nil
}

func unmarshalMetricsRequest(r *http.Request, body []byte, rows *metricRows) error {
	if err := checkMetricsRequestHeaders(r); err != nil {
		return err
	}
	decompressed, err := snappy.Decode(nil, body)
	if err != nil {
		return err
	}
	return rows.Unmarshal(decompressed)
}

type metricsColumns struct {
	Timestamp  *chproto.ColDateTime64
	MetricHash *chproto.ColUInt64
	Value      *chproto.ColFloat64
	MetricName *chproto.ColLowCardinality[string]
	Labels     *chproto.ColMap[string, string]
}

func newMetricsColumns() *metricsColumns {
	return &metricsColumns{
		Timestamp:  new(chproto.ColDateTime64).WithPrecision(chproto.PrecisionMilli),
		MetricHash: new(chproto.ColUInt64),
		Value:      new(chproto.ColFloat64),
		MetricName: new(chproto.ColStr).LowCardinality(),
		Labels:     newAttributesColumn(),
	}
}

func (c *metricsColumns) append(rows []metricRow) {
	for i := range rows {
		r := &rows[i]
		for _, sample := range r.samples {
			c.MetricName.Append(r.name)
			c.Labels.AppendKV(r.labels)
			c.Timestamp.Append(time.Unix(sample.timestamp/1000, 0))
			c.MetricHash.Append(r.hash)
			c.Value.Append(sample.value)
		}
	}
}

func (c *metricsColumns) rows() int {
	return c.Timestamp.Rows()
}

func (c *metricsColumns) input() chproto.Input {
	return chproto.Input{
		{Name: "MetricName", Data: c.MetricName},
		{Name: "Labels", Data: c.Labels},
		{Name: "Timestamp", Data: c.Timestamp},
		{Name: "MetricHash", Data: c.MetricHash},
		{Name: "Value", Data: c.Value},
	}
}

type metricsMetadataColumns struct {
	MetricFamilyName *chproto.ColLowCardinality[string]
	Type             *chproto.ColLowCardinality[string]
	Help             *chproto.ColStr
	Unit             *chproto.ColLowCardinality[string]
}

func newMetricsMetadataColumns() *metricsMetadataColumns {
	return &metricsMetadataColumns{
		MetricFamilyName: new(chproto.ColStr).LowCardinality(),
		Type:             new(chproto.ColStr).LowCardinality(),
		Help:             new(chproto.ColStr),
		Unit:             new(chproto.ColStr).LowCardinality(),
	}
}

func (c *metricsMetadataColumns) append(metadata []metricMetadata) {
	for _, md := range metadata {
		c.MetricFamilyName.Append(md.familyName)
		c.Type.Append(md.typ)
		c.Help.Append(md.help)
		c.Unit.Append(md.unit)
	}
}

func (c *metricsMetadataColumns) rows() int {
	return c.Help.Rows()
}

func (c *metricsMetadataColumns) input() chproto.Input {
	return chproto.Input{
		{Name: "MetricFamilyName", Data: c.MetricFamilyName},
		{Name: "Type", Data: c.Type},
		{Name: "Help", Data: c.Help},
		{Name: "Unit", Data: c.Unit},
	}
}

type MetricsBatch struct {
	samples  batch[*metricsColumns]
	metadata batch[*metricsMetadataColumns]
}

func NewMetricsBatch(limit int, timeout time.Duration, exec func(query ch.Query) error) *MetricsBatch {
	b := &MetricsBatch{}
	b.samples.start(limit, timeout, "@@table_metrics@@", newMetricsColumns, exec)
	b.metadata.start(limit, timeout, "@@table_metrics_metadata@@", newMetricsMetadataColumns, exec)
	return b
}

func (b *MetricsBatch) Close() {
	b.samples.Close()
	b.metadata.Close()
}

func (b *MetricsBatch) add(rows *metricRows) {
	b.samples.write(func(c *metricsColumns) { c.append(rows.rows) })
	if len(rows.metadata) > 0 {
		b.metadata.write(func(c *metricsMetadataColumns) { c.append(rows.metadata) })
	}
}
