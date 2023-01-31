package prom

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	promModel "github.com/prometheus/common/model"
	"k8s.io/klog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type ApiClient struct {
	api    v1.API
	client api.Client
}

func NewApiClient(address, user, password string, skipTlsVerify bool) (*ApiClient, error) {
	if user != "" {
		if u, err := url.Parse(address); err != nil {
			klog.Errorln("failed to parse url:", err)
		} else {
			u.User = url.UserPassword(user, password)
			address = u.String()
		}
	}
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: skipTlsVerify},
	}
	cfg := api.Config{Address: address, RoundTripper: transport}
	c, err := api.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return &ApiClient{api: v1.NewAPI(c), client: c}, nil
}

func (c *ApiClient) Ping(ctx context.Context) error {
	_, _, err := c.api.Query(ctx, "node_info", time.Now())
	return err
}

func (c *ApiClient) QueryRange(ctx context.Context, query string, from, to timeseries.Time, step timeseries.Duration) ([]model.MetricValues, error) {
	query = strings.ReplaceAll(query, "$RANGE", fmt.Sprintf(`%.0fs`, (step*3).ToStandard().Seconds()))
	from = from.Truncate(step)
	to = to.Truncate(step)
	value, _, err := c.api.QueryRange(ctx, query, v1.Range{Start: from.ToStandard(), End: to.ToStandard(), Step: step.ToStandard()})
	if err != nil {
		return nil, err
	}
	if value.Type() != promModel.ValMatrix {
		return nil, fmt.Errorf("result isn't a Matrix")
	}

	matrix := value.(promModel.Matrix)
	if len(matrix) == 0 {
		return nil, nil
	}

	res := make([]model.MetricValues, 0, matrix.Len())
	for _, m := range matrix {
		values := timeseries.New(from, int(to.Sub(from)/step)+1, step)
		mv := model.MetricValues{
			Labels:     make(map[string]string, len(m.Metric)),
			LabelsHash: uint64(m.Metric.Fingerprint()),
			Values:     values,
		}
		for k, v := range m.Metric {
			mv.Labels[string(k)] = string(v)
		}
		values.FillFromSamplePairs(m.Values)
		res = append(res, mv)
	}
	return res, nil
}

func (c *ApiClient) Proxy(r *http.Request, w http.ResponseWriter) {
	reStr, err := mux.CurrentRoute(r).GetPathRegexp()
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	re, err := regexp.Compile(reStr)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	path := re.ReplaceAllString(r.URL.Path, "")
	r.URL = c.client.URL(path, nil)
	r.RequestURI = ""
	resp, data, err := c.client.Do(r.Context(), r)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(data)
}
