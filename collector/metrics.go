package collector

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/ClickHouse/ch-go"
	chproto "github.com/ClickHouse/ch-go/proto"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	promModel "github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
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

func addLabelsIfNeeded(r *http.Request, body []byte, extraLabels map[string]string) ([]byte, error) {
	if len(extraLabels) == 0 {
		return body, nil
	}
	req, err := parseMetricsRequestBody(r, body)
	if err != nil {
		return nil, err
	}
	for i := range req.Timeseries {
		for k, v := range extraLabels {
			req.Timeseries[i].Labels = append(req.Timeseries[i].Labels, prompb.Label{Name: k, Value: v})
		}
	}
	decompressed, err := proto.Marshal(req)
	if err != nil {
		return nil, err
	}
	return snappy.Encode(nil, decompressed), nil
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
	}
	if cfg.UseClickHouse {
		req, err := parseMetricsRequestBody(r, body)
		if err != nil {
			klog.Errorln(err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}
		c.getMetricsBatch(project).Add(req)
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
	for k, vs := range res.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	if res.StatusCode == http.StatusBadRequest {
		scanner := bufio.NewScanner(io.LimitReader(res.Body, 1024))
		line := ""
		if scanner.Scan() {
			line = scanner.Text()
		}
		klog.Errorf("failed to write: got %d (%s) from prometheus, responding to the agent with 200 (to prevent retry)", res.StatusCode, line)
		w.WriteHeader(http.StatusOK)
		return
	} else if res.StatusCode > 400 {
		klog.Errorf("failed to write: got %d from prometheus", res.StatusCode)
	}
	w.WriteHeader(res.StatusCode)
	_, _ = io.Copy(w, res.Body)
}

func parseMetricsRequestBody(r *http.Request, body []byte) (*prompb.WriteRequest, error) {
	if r.Header.Get("Content-Type") != "application/x-protobuf" {
		return nil, fmt.Errorf("expected application/x-protobuf content-type")
	}
	if r.Header.Get("Content-Encoding") != "snappy" {
		return nil, fmt.Errorf("expected snappy content-encoding")
	}

	decompressed, err := snappy.Decode(nil, body)
	if err != nil {
		return nil, err
	}

	var req prompb.WriteRequest
	if err = proto.Unmarshal(decompressed, &req); err != nil {
		return nil, err
	}

	return &req, nil
}

type MetricsBatch struct {
	limit int
	exec  func(query ch.Query) error

	lock sync.Mutex
	done chan struct{}

	Timestamp  *chproto.ColDateTime64
	MetricHash *chproto.ColUInt64
	Value      *chproto.ColFloat64
	MetricName *chproto.ColLowCardinality[string]
	Labels     *chproto.ColMap[string, string]

	MetricFamilyName *chproto.ColLowCardinality[string]
	Type             *chproto.ColLowCardinality[string]
	Help             *chproto.ColStr
	Unit             *chproto.ColLowCardinality[string]
}

func NewMetricsBatch(limit int, timeout time.Duration, exec func(query ch.Query) error) *MetricsBatch {
	b := &MetricsBatch{
		limit: limit,
		exec:  exec,
		done:  make(chan struct{}),

		Timestamp:  new(chproto.ColDateTime64).WithPrecision(chproto.PrecisionMilli),
		MetricHash: new(chproto.ColUInt64),
		Value:      new(chproto.ColFloat64),
		MetricName: new(chproto.ColStr).LowCardinality(),
		Labels:     chproto.NewMap[string, string](new(chproto.ColStr).LowCardinality(), new(chproto.ColStr)),

		MetricFamilyName: new(chproto.ColStr).LowCardinality(),
		Type:             new(chproto.ColStr).LowCardinality(),
		Help:             new(chproto.ColStr),
		Unit:             new(chproto.ColStr).LowCardinality(),
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

func (b *MetricsBatch) Close() {
	b.done <- struct{}{}
	b.lock.Lock()
	defer b.lock.Unlock()
	b.save()
}

func (b *MetricsBatch) Add(req *prompb.WriteRequest) {
	b.lock.Lock()
	defer b.lock.Unlock()

	for _, md := range req.GetMetadata() {
		b.MetricFamilyName.Append(md.GetMetricFamilyName())
		b.Type.Append(md.GetType().String())
		b.Help.Append(md.GetHelp())
		b.Unit.Append(md.GetUnit())
	}

	for _, ts := range req.GetTimeseries() {
		labels := make(map[string]string, len(ts.Labels))
		sortable := make([]chproto.KV[string, string], 0, len(ts.Labels))
		var metricName string
		for _, label := range ts.Labels {
			if label.Name == promModel.MetricNameLabel {
				metricName = label.Value
			} else {
				sortable = append(sortable, chproto.KV[string, string]{Key: label.Name, Value: label.Value})
			}
			labels[label.Name] = label.Value
		}
		sort.Slice(sortable, func(i, j int) bool {
			return sortable[i].Key < sortable[j].Key
		})
		hash := promModel.LabelsToSignature(labels)
		for _, sample := range ts.Samples {
			b.MetricName.Append(metricName)
			b.Labels.AppendKV(sortable)
			b.Timestamp.Append(time.Unix(sample.Timestamp/1000, 0))
			b.MetricHash.Append(hash)
			b.Value.Append(sample.Value)

		}
		delete(labels, promModel.MetricNameLabel)
	}

	if b.Timestamp.Rows() < b.limit {
		return
	}
	b.save()
}

func (b *MetricsBatch) save() {
	if b.Timestamp.Rows() == 0 {
		return
	}

	labelsInput := chproto.Input{
		chproto.InputColumn{Name: "MetricName", Data: b.MetricName},
		chproto.InputColumn{Name: "Labels", Data: b.Labels},
		chproto.InputColumn{Name: "Timestamp", Data: b.Timestamp},
		chproto.InputColumn{Name: "MetricHash", Data: b.MetricHash},
		chproto.InputColumn{Name: "Value", Data: b.Value},
	}
	if err := b.exec(ch.Query{Body: labelsInput.Into("@@table_metrics@@"), Input: labelsInput}); err != nil {
		klog.Errorln("failed to insert metrics:", err)
	}

	if b.MetricFamilyName.Rows() > 0 {
		labelsInput = chproto.Input{
			chproto.InputColumn{Name: "MetricFamilyName", Data: b.MetricFamilyName},
			chproto.InputColumn{Name: "Type", Data: b.Type},
			chproto.InputColumn{Name: "Help", Data: b.Help},
			chproto.InputColumn{Name: "Unit", Data: b.Unit},
		}
		if err := b.exec(ch.Query{Body: labelsInput.Into("@@table_metrics_metadata@@"), Input: labelsInput}); err != nil {
			klog.Errorln("failed to insert metrics metadata:", err)
		}
		b.MetricFamilyName.Reset()
		b.Type.Reset()
		b.Help.Reset()
		b.Unit.Reset()
	}

	// Reset all columns
	b.MetricName.Reset()
	b.Labels.Reset()
	b.Timestamp.Reset()
	b.MetricHash.Reset()
	b.Value.Reset()
}
