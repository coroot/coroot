package collector

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
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

func addLabelsIfNeeded(r *http.Request, extraLabels map[string]string) (io.Reader, error) {
	if len(extraLabels) == 0 {
		return r.Body, nil
	}
	if ct := r.Header.Get("Content-Type"); ct != "" && ct != "application/x-protobuf" {
		return nil, fmt.Errorf("expected a protobuf request, got %s content-type", ct)
	}
	if enc := r.Header.Get("Content-Encoding"); enc != "" && enc != "snappy" {
		return nil, fmt.Errorf("only snappy encoding is supported, got %s content-encoding", enc)
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	decompressed, err := snappy.Decode(nil, body)
	if err != nil {
		return nil, err
	}
	var req prompb.WriteRequest
	if err = proto.Unmarshal(decompressed, &req); err != nil {
		return nil, err
	}
	for i := range req.Timeseries {
		for k, v := range extraLabels {
			req.Timeseries[i].Labels = append(req.Timeseries[i].Labels, prompb.Label{Name: k, Value: v})
		}
	}
	if decompressed, err = proto.Marshal(&req); err != nil {
		return nil, err
	}
	return bytes.NewBuffer(snappy.Encode(nil, decompressed)), nil
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
	body, err := addLabelsIfNeeded(r, cfg.ExtraLabels)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), r.Method, u.String(), body)
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
	for k, vs := range res.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	if res.StatusCode >= 400 {
		klog.Errorf("failed to write: got %d from prometheus", res.StatusCode)
	}
	w.WriteHeader(res.StatusCode)
	_, _ = io.Copy(w, r.Body)
	_ = res.Body.Close()
}
