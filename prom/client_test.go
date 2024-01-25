package prom

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryRange(t *testing.T) {
	data := `{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"__name__":"metric","instance":"10.244.0.67:80","job":"job1"},"values":[[1675329024.959,"1"],[1675329040.959,"0.1"],[1675329082.959,"0.04"]]},{"metric":{"__name__":"metric","instance":"10.244.1.135:80","job":"job1"},"values":[[1675329024,"0.02"],[1675329041,"0.2"],[1675329082,"2"]]}]}}`

	from := timeseries.Time(1675329022)
	to := timeseries.Time(1675329082)
	step := timeseries.Duration(15)

	h := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/query_range", r.URL.Path)
		assert.NoError(t, r.ParseForm())
		assert.Equal(t, "metric", r.Form.Get("query"))
		assert.Equal(t, from.Truncate(step).String(), r.Form.Get("start"))
		assert.Equal(t, to.Truncate(step).String(), r.Form.Get("end"))
		assert.Equal(t, strconv.FormatInt(int64(step), 10), r.Form.Get("step"))
		w.Write([]byte(data))
	}
	ts := httptest.NewServer(http.HandlerFunc(h))
	defer ts.Close()

	cfg := NewClientConfig(ts.URL, step)
	cfg.TlsSkipVerify = true
	client, err := NewClient(cfg)
	require.NoError(t, err)

	ctx := context.Background()

	res, err := client.QueryRange(ctx, `metric`, from, to, step)
	assert.NoError(t, err)

	assert.Equal(t, model.Labels{"__name__": "metric", "instance": "10.244.0.67:80", "job": "job1"}, res[0].Labels)
	assert.Equal(t, uint64(4421518228911002942), res[0].LabelsHash)
	assert.Equal(t, "TimeSeries(1675329015, 5, 15, [1 0.100000 . . 0.040000])", res[0].Values.String())

	assert.Equal(t, model.Labels{"__name__": "metric", "instance": "10.244.1.135:80", "job": "job1"}, res[1].Labels)
	assert.Equal(t, uint64(8265455476956637705), res[1].LabelsHash)
	assert.Equal(t, "TimeSeries(1675329015, 5, 15, [0.020000 0.200000 . . 2])", res[1].Values.String())
}

func Test_addExtraSelector(t *testing.T) {
	check := func(src, extraSelector, expected string) {
		actual, err := addExtraSelector(src, extraSelector)
		assert.NoError(t, err)
		assert.Equal(t, expected, actual)
	}
	check(
		`metric`,
		`{cluster="us-west-1", prom_instance=~"us.+"}`,
		`metric{cluster="us-west-1",prom_instance=~"us.+"}`)

	check(
		`rate(metric{label="value"}[1m])`,
		`{cluster="us-west-1", prom_instance=~"us.+"}`,
		`rate(metric{cluster="us-west-1",label="value",prom_instance=~"us.+"}[1m])`)

	check(
		`rate(node_resources_cpu_usage_seconds_total{mode!="idle"}[30s]) / ignoring(mode) group_left sum(rate(node_resources_cpu_usage_seconds_total[30s])) without(mode)*100`,
		`{cluster="cluster1"}`,
		`rate(node_resources_cpu_usage_seconds_total{cluster="cluster1",mode!="idle"}[30s]) / ignoring (mode) group_left () sum without (mode) (rate(node_resources_cpu_usage_seconds_total{cluster="cluster1"}[30s])) * 100`)
}
