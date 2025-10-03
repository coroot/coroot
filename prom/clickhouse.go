package prom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	chgo "github.com/ClickHouse/ch-go"
	"github.com/ClickHouse/ch-go/proto"
	"github.com/coroot/coroot/ch"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	promModel "github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/model/timestamp"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/prometheus/prometheus/storage"
	"golang.org/x/exp/maps"
	"k8s.io/klog"
)

type ClickHouse struct {
	ch     *ch.LowLevelClient
	engine *promql.Engine
	step   timeseries.Duration
}

func newClickHouse(cfg *db.IntegrationClickhouse, step timeseries.Duration) (*ClickHouse, error) {
	if cfg == nil {
		return nil, errors.New("clickhouse config is required")
	}
	client, err := ch.NewLowLevelClient(context.TODO(), cfg)
	if err != nil {
		return nil, err
	}
	c := &ClickHouse{ch: client, step: step}
	c.engine = promql.NewEngine(promql.EngineOpts{
		Timeout:    time.Second * 30,
		MaxSamples: 50000000,
	})
	return c, nil
}

func (c *ClickHouse) Ping(ctx context.Context) error {
	now := timeseries.Now()
	_, err := c.QueryRange(ctx, "up", FilterLabelsDropAll, now.Add(-timeseries.Hour), now, timeseries.Minute)
	return err
}

func (c *ClickHouse) GetStep(from, to timeseries.Time) (timeseries.Duration, error) {
	return c.step, nil
}

func (c *ClickHouse) Querier(mint, maxt int64) (storage.Querier, error) {
	return &clickhouseQuerier{ch: c.ch, mint: mint, maxt: maxt}, nil
}

func (c *ClickHouse) QueryRange(ctx context.Context, query string, filterLabels FilterLabelsF, from, to timeseries.Time, step timeseries.Duration) ([]*model.MetricValues, error) {
	query = strings.ReplaceAll(query, "$RANGE", fmt.Sprintf(`%.0fs`, (step*3).ToStandard().Seconds()))
	opts := promql.NewPrometheusQueryOpts(false, 0)

	from = from.Truncate(step)
	to = to.Truncate(step)

	q, err := c.engine.NewRangeQuery(ctx, c, opts, query, from.ToStandard(), to.ToStandard(), step.ToStandard())
	if err != nil {
		return nil, err
	}
	defer q.Close()
	resp := q.Exec(ctx)
	if resp.Err != nil {
		klog.Infoln(resp.Err.Error(), query)
		return nil, resp.Err
	}
	matrix, err := resp.Matrix()
	if err != nil {
		return nil, err
	}
	res := map[uint64]*model.MetricValues{}
	for _, s := range matrix {
		ls := map[string]string{}
		for _, l := range s.Metric {
			if filterLabels(l.Name) {
				ls[l.Name] = l.Value
			}
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
		for _, f := range s.Floats {
			mv.Values.Set(timeseries.Time(f.T/1000), float32(f.F))
		}
	}
	return maps.Values(res), nil
}

func (c *ClickHouse) MetricMetadata(r *http.Request, w http.ResponseWriter) {
	query := "SELECT DISTINCT lower(MetricFamilyName) as MetricFamilyName, lower(Type) as Type, Help, Unit FROM @@table_metrics_metadata@@"
	if m := r.FormValue("metric"); m != "" {
		query += " WHERE MetricFamilyName='" + "'" + escapeString(m)
	}
	metricFamilyName := (&proto.ColStr{}).LowCardinality()
	typ := (&proto.ColStr{}).LowCardinality()
	help := &proto.ColStr{}
	unit := (&proto.ColStr{}).LowCardinality()
	res := map[string][]metadata.Metadata{}

	err := c.ch.Do(r.Context(), chgo.Query{
		Body: query,
		Result: proto.Results{
			{Name: "MetricFamilyName", Data: metricFamilyName},
			{Name: "Type", Data: typ},
			{Name: "Help", Data: help},
			{Name: "Unit", Data: unit},
		},
		OnResult: func(ctx context.Context, block proto.Block) error {
			for i := 0; i < block.Rows; i++ {
				res[metricFamilyName.Row(i)] = []metadata.Metadata{{
					Type: promModel.MetricType(typ.Row(i)),
					Unit: unit.Row(i),
					Help: help.Row(i)},
				}
			}
			return nil
		},
	})
	if err != nil {
		writePrometheusResponse(w, err, errorInternal, res)
		return
	}
	writePrometheusResponse(w, nil, errorNone, res)
}

func (c *ClickHouse) LabelValues(r *http.Request, w http.ResponseWriter, labelName string) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	now := time.Now()

	if !promModel.LabelNameRE.MatchString(labelName) {
		writePrometheusResponse(w, fmt.Errorf("invalid label name: %q", labelName), errorBadData, nil)
		return
	}

	matcherSets, err := parser.ParseMetricSelectors(r.Form["match[]"])
	if err != nil {
		writePrometheusResponse(w, fmt.Errorf("invalid matchers: %s", r.Form["match[]"]), errorBadData, nil)
		return
	}
	q, _ := c.Querier(timestamp.FromTime(now.Add(-time.Hour)), timestamp.FromTime(now))

	var vals []string
	if len(matcherSets) > 1 {
		labelValuesSet := make(map[string]struct{})
		for _, matchers := range matcherSets {
			vals, _, err = q.LabelValues(ctx, labelName, nil, matchers...)
			if err != nil {
				writePrometheusResponse(w, err, errorExec, nil)
				return
			}
			for _, val := range vals {
				labelValuesSet[val] = struct{}{}
			}
		}

		vals = make([]string, 0, len(labelValuesSet))
		for val := range labelValuesSet {
			vals = append(vals, val)
		}
	} else {
		var matchers []*labels.Matcher
		if len(matcherSets) == 1 {
			matchers = matcherSets[0]
		}
		vals, _, err = q.LabelValues(ctx, labelName, nil, matchers...)
		if err != nil {
			writePrometheusResponse(w, err, errorExec, nil)
			return
		}

		if vals == nil {
			vals = []string{}
		}
	}
	slices.Sort(vals)
	writePrometheusResponse(w, nil, errorNone, vals)
}

func (c *ClickHouse) Series(r *http.Request, w http.ResponseWriter) {
	ctx := r.Context()

	now := time.Now()
	if err := r.ParseForm(); err != nil {
		writePrometheusResponse(w, fmt.Errorf("error parsing form values: %w", err), errorBadData, nil)
		return
	}
	if len(r.Form["match[]"]) == 0 {
		writePrometheusResponse(w, errors.New("no match[] parameter provided"), errorBadData, nil)
		return
	}
	matcherSets, err := parser.ParseMetricSelectors(r.Form["match[]"])
	if err != nil {
		writePrometheusResponse(w, fmt.Errorf("invalid matchers: %s", r.Form["match[]"]), errorBadData, nil)
		return
	}
	q, _ := c.Querier(timestamp.FromTime(now.Add(-time.Hour)), timestamp.FromTime(now))

	hints := &storage.SelectHints{
		Start: timestamp.FromTime(now.Add(-time.Hour)),
		End:   timestamp.FromTime(now),
		Func:  "series",
		Step:  int64(c.step * 1000),
	}
	var set storage.SeriesSet

	if len(matcherSets) > 1 {
		var sets []storage.SeriesSet
		for _, mset := range matcherSets {
			s := q.Select(ctx, true, hints, mset...)
			sets = append(sets, s)
		}
		set = storage.NewMergeSeriesSet(sets, storage.ChainedSeriesMerge)
	} else {
		set = q.Select(ctx, false, hints, matcherSets[0]...)
	}

	var metrics []labels.Labels

	for set.Next() {
		metrics = append(metrics, set.At().Labels())
	}
	if set.Err() != nil {
		writePrometheusResponse(w, err, errorExec, nil)
		return
	}
	writePrometheusResponse(w, nil, errorNone, metrics)
}

func (c *ClickHouse) Close() {
	c.ch.Close()
}

type errorType string

const (
	errorNone          errorType = ""
	errorTimeout       errorType = "timeout"
	errorCanceled      errorType = "canceled"
	errorExec          errorType = "execution"
	errorBadData       errorType = "bad_data"
	errorInternal      errorType = "internal"
	errorUnavailable   errorType = "unavailable"
	errorNotFound      errorType = "not_found"
	errorNotAcceptable errorType = "not_acceptable"
)

type PrometheusResponse struct {
	Status    string      `json:"status"`
	Data      interface{} `json:"data,omitempty"`
	ErrorType errorType   `json:"errorType,omitempty"`
	Error     string      `json:"error,omitempty"`
	Warnings  []string    `json:"warnings,omitempty"`
	Infos     []string    `json:"infos,omitempty"`
}

func writePrometheusResponse(w http.ResponseWriter, err error, errorType errorType, data interface{}) {
	resp := &PrometheusResponse{
		Status: "success",
		Data:   data,
	}
	if err != nil {
		resp.Status = "error"
		resp.Error = err.Error()
		resp.ErrorType = errorType
	}
	_ = json.NewEncoder(w).Encode(resp)
}
