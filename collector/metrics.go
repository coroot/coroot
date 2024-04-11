package collector

import (
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/coroot/coroot/db"
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

func (c *Collector) Metrics(w http.ResponseWriter, r *http.Request) {
	projectId := db.ProjectId(r.Header.Get(ApiKeyHeader))
	project, err := c.getProject(projectId)
	if err != nil {
		klog.Errorln(err)
		if errors.Is(err, ErrProjectNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	cfg := project.Prometheus

	u, err := url.Parse(cfg.Url)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	if cfg.BasicAuth != nil {
		u.User = url.UserPassword(cfg.BasicAuth.User, cfg.BasicAuth.Password)
	}

	u = u.JoinPath("/api/v1/write")

	req, err := http.NewRequestWithContext(r.Context(), r.Method, u.String(), r.Body)
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
	w.WriteHeader(res.StatusCode)
	_, _ = io.Copy(w, r.Body)
	_ = res.Body.Close()
}
