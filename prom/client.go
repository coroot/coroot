package prom

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/buger/jsonparser"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/gorilla/mux"
	promModel "github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/promql/parser"
	"k8s.io/klog"
)

var (
	secureTransport   *http.Transport
	insecureTransport *http.Transport

	pool = &sync.Pool{New: func() interface{} {
		return bytes.NewBuffer(nil)
	}}
)

func init() {
	d := net.Dialer{Timeout: 30 * time.Second}
	secureTransport = &http.Transport{
		DialContext:         d.DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	insecureTransport = &http.Transport{
		DialContext:         d.DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
	}
}

type ClientConfig struct {
	Url           string
	BasicAuth     *utils.BasicAuth
	TlsSkipVerify bool
	ExtraSelector string
	CustomHeaders []utils.Header
	Step          timeseries.Duration
	Transport     *http.Transport
}

func NewClientConfig(url string, step timeseries.Duration) ClientConfig {
	return ClientConfig{
		Url:  url,
		Step: step,
	}
}

type Client struct {
	config     ClientConfig
	url        url.URL
	httpClient *http.Client
}

func NewClient(config ClientConfig) (*Client, error) {
	u, err := url.Parse(config.Url)
	if err != nil {
		return nil, err
	}
	if config.BasicAuth != nil {
		u.User = url.UserPassword(config.BasicAuth.User, config.BasicAuth.Password)
	}
	tr := config.Transport
	if tr == nil {
		tr = secureTransport
		if config.TlsSkipVerify {
			tr = insecureTransport
		}
	}
	c := &Client{
		config:     config,
		url:        *u,
		httpClient: &http.Client{Transport: tr},
	}
	return c, nil
}

func (c *Client) Ping(ctx context.Context) error {
	now := timeseries.Now()
	_, err := c.QueryRange(ctx, "up", now.Add(-timeseries.Hour), now, timeseries.Minute)
	return err
}

func (c *Client) GetStep(from, to timeseries.Time) (timeseries.Duration, error) {
	return c.config.Step, nil
}

func (c *Client) QueryRange(ctx context.Context, query string, from, to timeseries.Time, step timeseries.Duration) ([]*model.MetricValues, error) {
	query = strings.ReplaceAll(query, "$RANGE", fmt.Sprintf(`%.0fs`, (step*3).ToStandard().Seconds()))
	var err error
	query, err = addExtraSelector(query, c.config.ExtraSelector)
	if err != nil {
		return nil, err
	}
	from = from.Truncate(step)
	to = to.Truncate(step)

	u := c.url
	u.Path = path.Join(u.Path, "/api/v1/query_range")
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
	for _, h := range c.config.CustomHeaders {
		req.Header.Add(h.Key, h.Value)
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

	var res []*model.MetricValues
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
		res = append(res, &mv)
	}
	if _, err := jsonparser.ArrayEach(buf.Bytes(), f, "data", "result"); err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Client) Proxy(r *http.Request, w http.ResponseWriter) {
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
	p := re.ReplaceAllString(r.URL.Path, "")
	u := c.url
	u.Path = path.Join(u.Path, p)
	r.URL = &u
	r.RequestURI = ""
	resp, err := c.httpClient.Do(r)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	if _, err = buf.ReadFrom(resp.Body); err != nil {
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
	_, _ = w.Write(buf.Bytes())
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
