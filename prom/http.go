package prom

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/buger/jsonparser"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	promModel "github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/promql/parser"
	"golang.org/x/exp/maps"
	"k8s.io/klog"
)

type HttpClient struct {
	config     httpClientConfig
	url        url.URL
	httpClient *http.Client
}

func (c *HttpClient) Ping(ctx context.Context) error {
	now := timeseries.Now()
	_, err := c.QueryRange(ctx, "up", FilterLabelsDropAll, now.Add(-timeseries.Hour), now, timeseries.Minute)
	return err
}

func (c *HttpClient) GetStep(from, to timeseries.Time) (timeseries.Duration, error) {
	return c.config.Step, nil
}

func (c *HttpClient) QueryRange(ctx context.Context, query string, filterLabels FilterLabelsF, from, to timeseries.Time, step timeseries.Duration) ([]*model.MetricValues, error) {
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
		d, _ := io.ReadAll(resp.Body)
		var j struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(d, &j)
		if j.Error != "" {
			return nil, fmt.Errorf(j.Error)
		}
		return nil, errors.New(resp.Status)
	}
	buf := pool.Get().(*bytes.Buffer)
	buf.Reset()
	defer pool.Put(buf)
	if _, err = buf.ReadFrom(resp.Body); err != nil {
		return nil, err
	}

	res := map[uint64]*model.MetricValues{}
	f := func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		ls := map[string]string{}
		err = jsonparser.ObjectEach(value, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
			v, err := jsonparser.ParseString(value)
			if err != nil {
				return err
			}
			k := string(key)
			if filterLabels(k) {
				ls[string(key)] = v
			}
			return nil
		}, "metric")
		if err != nil {
			return
		}

		lsHash := promModel.LabelsToSignature(ls)
		mv := res[lsHash]
		if mv == nil {
			mv = &model.MetricValues{
				Labels:     ls,
				LabelsHash: lsHash,
				Values:     timeseries.New(from, int(to.Sub(from)/step)+1, step),
			}
			res[lsHash] = mv
		}

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
	}
	if _, err := jsonparser.ArrayEach(buf.Bytes(), f, "data", "result"); err != nil {
		return nil, err
	}
	return maps.Values(res), nil
}

func (c *HttpClient) MetricMetadata(r *http.Request, w http.ResponseWriter) {
	c.proxy(r, w, "/api/v1/metadata")
}

func (c *HttpClient) LabelValues(r *http.Request, w http.ResponseWriter, labelName string) {
	c.proxy(r, w, "/api/v1/label/"+labelName+"/values")
}

func (c *HttpClient) Series(r *http.Request, w http.ResponseWriter) {
	c.proxy(r, w, "/api/v1/series")
}

func (c *HttpClient) QueryRangeHandler(r *http.Request, w http.ResponseWriter) {
	c.proxy(r, w, "/api/v1/query_range")
}

func (c *HttpClient) proxy(r *http.Request, w http.ResponseWriter, promUrlPath string) {
	u := c.url
	u.Path = path.Join(u.Path, promUrlPath)
	req := r.Clone(r.Context())
	req.URL = &u
	req.RequestURI = ""
	for _, h := range c.config.CustomHeaders {
		req.Header.Add(h.Key, h.Value)
	}
	resp, err := c.httpClient.Do(req)
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

func (c *HttpClient) Close() {}

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
