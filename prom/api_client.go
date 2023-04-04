package prom

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/buger/jsonparser"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	promModel "github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/promql/parser"
	"k8s.io/klog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	secureClient   *http.Client
	insecureClient *http.Client

	pool = &sync.Pool{New: func() interface{} {
		return bytes.NewBuffer(nil)
	}}
)

func init() {
	d := net.Dialer{Timeout: 30 * time.Second}
	secureClient = &http.Client{Transport: &http.Transport{
		DialContext:         d.DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
	}}
	insecureClient = &http.Client{Transport: &http.Transport{
		DialContext:         d.DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
	}}
}

type ApiClient struct {
	api           v1.API
	apiClient     api.Client
	httpClient    *http.Client
	extraSelector string
	customHeaders []utils.Header
}

func NewApiClient(address, user, password string, skipTlsVerify bool, extraSelector string, customHeaders []utils.Header) (*ApiClient, error) {
	if user != "" {
		if u, err := url.Parse(address); err != nil {
			klog.Errorln("failed to parse url:", err)
		} else {
			u.User = url.UserPassword(user, password)
			address = u.String()
		}
	}
	cl := secureClient
	if skipTlsVerify {
		cl = insecureClient
	}
	c, err := api.NewClient(api.Config{Address: address, Client: cl})
	if err != nil {
		return nil, err
	}
	return &ApiClient{api: v1.NewAPI(c), apiClient: c, httpClient: cl, extraSelector: extraSelector, customHeaders: customHeaders}, nil
}

func (c *ApiClient) Ping(ctx context.Context) error {
	_, _, err := c.api.Query(ctx, "node_info", time.Now())
	return err
}

func (c *ApiClient) QueryRange(ctx context.Context, query string, from, to timeseries.Time, step timeseries.Duration) ([]model.MetricValues, error) {
	query = strings.ReplaceAll(query, "$RANGE", fmt.Sprintf(`%.0fs`, (step*3).ToStandard().Seconds()))
	var err error
	query, err = addExtraSelector(query, c.extraSelector)
	if err != nil {
		return nil, err
	}
	from = from.Truncate(step)
	to = to.Truncate(step)

	u := c.apiClient.URL("/api/v1/query_range", nil)
	q := u.Query()

	q.Set("query", query)
	q.Set("start", from.String())
	q.Set("end", to.String())
	q.Set("step", strconv.FormatInt(int64(step), 10))

	req, err := http.NewRequest(http.MethodPost, u.String(), strings.NewReader(q.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, h := range c.customHeaders {
		req.Header.Set(h.Key, h.Value)
	}
	req = req.WithContext(ctx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}
	buf := pool.Get().(*bytes.Buffer)
	buf.Reset()
	defer pool.Put(buf)
	if _, err = buf.ReadFrom(resp.Body); err != nil {
		return nil, err
	}

	var res []model.MetricValues
	f := func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		mv := model.MetricValues{
			Labels: map[string]string{},
			Values: timeseries.New(from, int(to.Sub(from)/step)+1, step),
		}
		err = jsonparser.ObjectEach(value, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
			v, err := jsonparser.ParseString(value)
			if err != nil {
				return err
			}
			mv.Labels[string(key)] = v
			return nil
		}, "metric")
		if err != nil {
			return
		}
		mv.LabelsHash = promModel.LabelsToSignature(mv.Labels)

		_, err = jsonparser.ArrayEach(value, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
			var (
				state int
				start int
				t     timeseries.Time
				v     float64
			)
			for i, b := range value {
				switch b {
				case '[':
					state = 1
					start = i + 1
				case '.', ',':
					if state == 1 {
						tInt, err := jsonparser.ParseInt(value[start:i])
						if err != nil {
							return
						}
						t = timeseries.Time(tInt)
						state = 0
						start = 0
					}
				case '"':
					if state == 0 {
						state = 2
						start = i + 1
					} else {
						v, err = jsonparser.ParseFloat(value[start:i])
						if err != nil {
							return
						}
						state = 0
					}
				}
			}
			mv.Values.Set(t, float32(v))
		}, "values")
		if err != nil {
			return
		}
		res = append(res, mv)
	}
	if _, err := jsonparser.ArrayEach(buf.Bytes(), f, "data", "result"); err != nil {
		return nil, err
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
	r.URL = c.apiClient.URL(path, nil)
	r.RequestURI = ""
	resp, data, err := c.apiClient.Do(r.Context(), r)
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

func addExtraSelector(query string, extraSelector string) (string, error) {
	if extraSelector == "" {
		return query, nil
	}
	extra, err := parser.ParseMetricSelector(extraSelector)
	if err != nil {
		return "", err
	}
	expr, err := parser.ParseExpr(query)
	if err != nil {
		return "", err
	}
	parser.Inspect(expr, func(node parser.Node, _ []parser.Node) error {
		vs, ok := node.(*parser.VectorSelector)
		if ok {
			vs.LabelMatchers = append(vs.LabelMatchers, extra...)
		}
		return nil
	})
	return expr.String(), nil
}
