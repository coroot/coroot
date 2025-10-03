package prom

import (
	"context"
	"fmt"
	"sort"
	"strings"

	chgo "github.com/ClickHouse/ch-go"
	"github.com/ClickHouse/ch-go/proto"
	"github.com/coroot/coroot/ch"
	promModel "github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb/chunkenc"
	"github.com/prometheus/prometheus/tsdb/chunks"
	"github.com/prometheus/prometheus/util/annotations"
)

type clickhouseQuerier struct {
	ch         *ch.LowLevelClient
	mint, maxt int64
}

func (q *clickhouseQuerier) Select(ctx context.Context, _ bool, hints *storage.SelectHints, matchers ...*labels.Matcher) storage.SeriesSet {
	query := fmt.Sprintf(`
		SELECT
		    MetricName,
		    Labels,
		    groupArray(toUnixTimestamp(Timestamp)) AS Timestamps,
			groupArray(Value) AS Values
		FROM @@table_metrics@@
		WHERE
		 Timestamp >= toDateTime(%d) AND Timestamp <= toDateTime(%d) %s
		GROUP BY MetricName, Labels
		ORDER BY MetricName, Labels
	`, q.mint/1000, q.maxt/1000, q.buildWhere(matchers))

	metricName := (&proto.ColStr{}).LowCardinality()
	metricLabels := proto.NewMap[string, string]((&proto.ColStr{}).LowCardinality(), &proto.ColStr{})
	timestamps := proto.NewArray[uint32](&proto.ColUInt32{})
	values := proto.NewArray[float64](&proto.ColFloat64{})

	ss := &seriesSet{}
	ss.err = q.ch.Do(ctx, chgo.Query{
		Body: query,
		Result: proto.Results{
			{Name: "MetricName", Data: metricName},
			{Name: "Labels", Data: metricLabels},
			{Name: "Timestamps", Data: timestamps},
			{Name: "Values", Data: values},
		},
		OnResult: func(ctx context.Context, block proto.Block) error {
			for i := 0; i < block.Rows; i++ {
				lsMap := metricLabels.Row(i)
				ls := make(labels.Labels, 0, len(lsMap)+1)
				ls = append(ls, labels.Label{Name: promModel.MetricNameLabel, Value: metricName.Row(i)})
				for k, v := range lsMap {
					ls = append(ls, labels.Label{Name: k, Value: v})
				}
				sort.Slice(ls, func(i, j int) bool { return ls[i].Name < ls[j].Name })
				tss := timestamps.Row(i)
				vals := values.Row(i)
				samples := make([]chunks.Sample, 0, len(tss))
				for j, ts := range tss {
					samples = append(samples, sample{t: (int64(ts) * 1000) * hints.Step / hints.Step, f: vals[j]})
				}
				ss.series = append(ss.series, storage.NewListSeries(ls, sortedAndDeduplicatedSamples(samples)))
			}
			return nil
		},
	})
	return ss
}

func sortedAndDeduplicatedSamples(samples []chunks.Sample) []chunks.Sample {
	sort.Slice(samples, func(i, j int) bool {
		return samples[i].T() < samples[j].T()
	})
	deduped := samples[:0]
	for i := 0; i < len(samples); {
		t := samples[i].T()
		j := i + 1
		for j < len(samples) && samples[j].T() == t {
			j++
		}
		deduped = append(deduped, samples[j-1])
		i = j
	}
	return deduped
}

func (q *clickhouseQuerier) LabelValues(ctx context.Context, name string, _ *storage.LabelHints, matchers ...*labels.Matcher) ([]string, annotations.Annotations, error) {
	var res []string
	if name == promModel.MetricNameLabel {
		value := (&proto.ColStr{}).LowCardinality()
		err := q.ch.Do(ctx, chgo.Query{
			Body: fmt.Sprintf(`
				SELECT DISTINCT MetricName as LabelValue 
				FROM @@table_metrics@@ 
				WHERE Timestamp >= toDateTime(%d) AND Timestamp <= toDateTime(%d) %s`,
				q.mint/1000, q.maxt/1000, q.buildWhere(matchers)),
			Result: proto.Results{{Name: "LabelValue", Data: value}},
			OnResult: func(ctx context.Context, block proto.Block) error {
				for i := 0; i < block.Rows; i++ {
					res = append(res, value.Row(i))
				}
				return nil
			},
		})
		return res, nil, err
	}

	value := &proto.ColStr{}
	err := q.ch.Do(ctx, chgo.Query{
		Body: fmt.Sprintf(`
			SELECT DISTINCT %s as LabelValue 
			FROM @@table_metrics@@ 
			WHERE Timestamp >= toDateTime(%d) AND Timestamp <= toDateTime(%d) %s`,
			"Labels['"+escapeString(name)+"']", q.mint/1000, q.maxt/1000, q.buildWhere(matchers)),
		Result: proto.Results{{Name: "LabelValue", Data: value}},
		OnResult: func(ctx context.Context, block proto.Block) error {
			for i := 0; i < block.Rows; i++ {
				res = append(res, value.Row(i))
			}
			return nil
		},
	})
	return res, nil, err
}

func (q *clickhouseQuerier) LabelNames(ctx context.Context, _ *storage.LabelHints, _ ...*labels.Matcher) ([]string, annotations.Annotations, error) {
	return nil, nil, fmt.Errorf("not yet implemented")
}

func (q *clickhouseQuerier) Close() error {
	return nil
}

func (q *clickhouseQuerier) buildWhere(ms []*labels.Matcher) string {
	conds := make([]string, 0, len(ms))
	for _, m := range ms {
		if c := q.condition(m); c != "" {
			conds = append(conds, c)
		}
	}
	if len(conds) > 0 {
		return "AND " + strings.Join(conds, " AND ")
	}
	return ""
}

func (q *clickhouseQuerier) condition(matcher *labels.Matcher) string {
	var column string
	if matcher.Name == labels.MetricName {
		column = "MetricName"
	} else {
		column = "Labels['" + escapeString(matcher.Name) + "']"
	}
	switch matcher.Type {
	case labels.MatchEqual:
		return fmt.Sprintf("%s = '%s'", column, escapeString(matcher.Value))
	case labels.MatchNotEqual:
		return fmt.Sprintf("%s != '%s'", column, escapeString(matcher.Value))
	case labels.MatchRegexp:
		return fmt.Sprintf("match(%s, '%s')", column, escapeString(matcher.Value))
	case labels.MatchNotRegexp:
		return fmt.Sprintf("NOT match(%s, '%s')", column, escapeString(matcher.Value))
	}
	return ""
}

type seriesSet struct {
	err    error
	cur    int
	series []storage.Series
}

func (ss *seriesSet) Next() bool {
	ss.cur++
	return ss.cur-1 < len(ss.series)
}

func (ss *seriesSet) At() storage.Series {
	return ss.series[ss.cur-1]
}

func (ss *seriesSet) Err() error {
	return ss.err
}

func (ss *seriesSet) Warnings() annotations.Annotations { return nil }

type sample struct {
	t int64
	f float64
}

func (s sample) T() int64 {
	return s.t
}

func (s sample) F() float64 {
	return s.f
}

func (s sample) H() *histogram.Histogram {
	return nil
}

func (s sample) FH() *histogram.FloatHistogram {
	return nil
}

func (s sample) Type() chunkenc.ValueType {
	return chunkenc.ValFloat
}

func (s sample) Copy() chunks.Sample {
	return sample{t: s.t, f: s.f}
}

func escapeString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
