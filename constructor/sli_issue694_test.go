package constructor

import (
	"fmt"
	"testing"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type issue694RequestSource struct {
	status string
	values []float32
}

func issue694MissingValues(points int) []float32 {
	values := make([]float32, points)
	for i := range values {
		values[i] = timeseries.NaN
	}
	return values
}

func issue694TimeSeries(values []float32) *timeseries.TimeSeries {
	return timeseries.NewWithData(0, timeseries.Minute, append([]float32(nil), values...))
}

func issue694LoadAvailabilitySLI(t *testing.T, builtin bool, sources []issue694RequestSource) *model.AvailabilitySLI {
	t.Helper()

	project := &db.Project{Id: db.ProjectId("issue-694")}
	world := model.NewWorld(0, timeseries.Time(0).Add(3*timeseries.Minute), timeseries.Minute, timeseries.Minute)
	targetID := model.NewApplicationId(project.ClusterId(), "default", model.ApplicationKindDeployment, "target")
	target := world.GetOrCreateApplication(targetID, false)
	target.Category = model.ApplicationCategoryApplication

	queryName := qRecordingRuleApplicationL7InboundRequests
	labels := func(status string) model.Labels {
		return model.Labels{"app": targetID.String(), "status": status}
	}
	if builtin {
		queryName = qRecordingRuleApplicationL7Requests
		sourceID := model.NewApplicationId(project.ClusterId(), "default", model.ApplicationKindDeployment, "source")
		source := world.GetOrCreateApplication(sourceID, false)
		source.Category = model.ApplicationCategoryApplication
		labels = func(status string) model.Labels {
			return model.Labels{"app": sourceID.String(), "dest": targetID.String(), "status": status}
		}
	}

	metricValues := func() []*model.MetricValues {
		values := make([]*model.MetricValues, 0, len(sources))
		for _, source := range sources {
			values = append(values, &model.MetricValues{
				Labels: labels(source.status),
				Values: issue694TimeSeries(source.values),
			})
		}
		return values
	}
	metrics := map[string][]*model.MetricValues{
		queryName + "_avg": metricValues(),
		queryName + "_raw": metricValues(),
	}

	(&Constructor{}).loadSLIs(world, metrics, project)
	require.Len(t, target.AvailabilitySLIs, 1)
	return target.AvailabilitySLIs[0]
}

func TestIssue694AvailabilityAggregation(t *testing.T) {
	tests := []struct {
		name          string
		sources       []issue694RequestSource
		want          string
		wantTailEmpty bool
	}{
		{
			name: "one_missing_source",
			sources: []issue694RequestSource{
				{status: "200", values: issue694MissingValues(3)},
			},
			want:          "TimeSeries(0, 3, 60, [. . .])",
			wantTailEmpty: true,
		},
		{
			name: "multiple_missing_sources",
			sources: []issue694RequestSource{
				{status: "200", values: issue694MissingValues(3)},
				{status: "200", values: issue694MissingValues(3)},
			},
			want:          "TimeSeries(0, 3, 60, [. . .])",
			wantTailEmpty: true,
		},
		{
			name: "missing_and_genuine_zero",
			sources: []issue694RequestSource{
				{status: "200", values: issue694MissingValues(3)},
				{status: "200", values: []float32{0, 0, 0}},
			},
			want:          "TimeSeries(0, 3, 60, [0 0 0])",
			wantTailEmpty: false,
		},
		{
			name: "missing_and_positive",
			sources: []issue694RequestSource{
				{status: "200", values: issue694MissingValues(3)},
				{status: "200", values: []float32{1, 2, 3}},
			},
			want:          "TimeSeries(0, 3, 60, [1 2 3])",
			wantTailEmpty: false,
		},
		{
			name: "multiple_genuine_zero_sources",
			sources: []issue694RequestSource{
				{status: "200", values: []float32{0, 0, 0}},
				{status: "200", values: []float32{0, 0, 0}},
			},
			want:          "TimeSeries(0, 3, 60, [0 0 0])",
			wantTailEmpty: false,
		},
		{
			name: "mixed_missing_and_defined_samples",
			sources: []issue694RequestSource{
				{status: "200", values: []float32{timeseries.NaN, 0, timeseries.NaN, 2}},
				{status: "200", values: []float32{1, timeseries.NaN, timeseries.NaN, 3}},
			},
			want:          "TimeSeries(0, 4, 60, [1 0 . 5])",
			wantTailEmpty: false,
		},
	}

	for _, builtin := range []bool{false, true} {
		path := "inbound"
		if builtin {
			path = "builtin"
		}
		t.Run(path, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					sli := issue694LoadAvailabilitySLI(t, builtin, tt.sources)
					require.NotNil(t, sli.TotalRequests)
					require.NotNil(t, sli.TotalRequestsRaw)
					assert.Equal(t, tt.want, sli.TotalRequests.String())
					assert.Equal(t, tt.want, sli.TotalRequestsRaw.String())
					assert.Equal(t, tt.wantTailEmpty, sli.TotalRequests.TailIsEmpty())
					assert.Equal(t, tt.wantTailEmpty, sli.TotalRequestsRaw.TailIsEmpty())
				})
			}
		})
	}
}

func TestIssue694PositiveTotalsPreserveMissingFailures(t *testing.T) {
	for _, builtin := range []bool{false, true} {
		path := "inbound"
		if builtin {
			path = "builtin"
		}
		t.Run(path, func(t *testing.T) {
			sli := issue694LoadAvailabilitySLI(t, builtin, []issue694RequestSource{
				{status: "200", values: []float32{10, 20, 30}},
				{status: "500", values: issue694MissingValues(3)},
			})
			assert.Equal(t, "TimeSeries(0, 3, 60, [10 20 30])", sli.TotalRequests.String())
			assert.Equal(t, "TimeSeries(0, 3, 60, [. . .])", sli.FailedRequests.String())
			assert.Equal(t, "TimeSeries(0, 3, 60, [10 20 30])", sli.TotalRequestsRaw.String())
			assert.Equal(t, "TimeSeries(0, 3, 60, [. . .])", sli.FailedRequestsRaw.String())
		})
	}
}

func TestIssue694GroupedApplicationsPreserveMissingAvailability(t *testing.T) {
	project := &db.Project{Id: db.ProjectId("issue-694")}
	world := model.NewWorld(0, timeseries.Time(0).Add(3*timeseries.Minute), timeseries.Minute, timeseries.Minute)
	groupID := model.NewApplicationId(project.ClusterId(), "default", model.ApplicationKindCustomApplication, "group")
	group := world.GetOrCreateApplication(groupID, true)
	members := map[model.ApplicationId]*model.Application{}
	for i := 0; i < 2; i++ {
		id := model.NewApplicationId(project.ClusterId(), "default", model.ApplicationKindDeployment, fmt.Sprintf("member-%d", i))
		member := world.GetOrCreateApplication(id, false)
		member.Category = model.ApplicationCategoryApplication
		member.InboundRequestsCount["200"] = issue694TimeSeries(issue694MissingValues(3))
		member.InboundRequestsCount["201"] = issue694TimeSeries(issue694MissingValues(3))
		members[id] = member
	}

	(&Constructor{}).groupApplications(world, map[model.ApplicationId]*appGroup{
		groupID: {app: group, members: members},
	}, project)
	assert.Equal(t, "TimeSeries(0, 3, 60, [. . .])", group.InboundRequestsCount["200"].String())
	assert.Equal(t, "TimeSeries(0, 3, 60, [. . .])", group.InboundRequestsCount["201"].String())

	recorded := RecordingRules[qRecordingRuleApplicationL7InboundRequests](nil, project, world)
	require.Len(t, recorded, 1)
	assert.Equal(t, "TimeSeries(0, 3, 60, [. . .])", recorded[0].Values.String())

	(&Constructor{}).loadSLIs(world, map[string][]*model.MetricValues{
		qRecordingRuleApplicationL7InboundRequests + "_avg": recorded,
		qRecordingRuleApplicationL7InboundRequests + "_raw": recorded,
	}, project)
	require.Len(t, group.AvailabilitySLIs, 1)
	assert.Equal(t, "TimeSeries(0, 3, 60, [. . .])", group.AvailabilitySLIs[0].TotalRequests.String())
	assert.Equal(t, "TimeSeries(0, 3, 60, [. . .])", group.AvailabilitySLIs[0].TotalRequestsRaw.String())
}

func TestIssue694OutgoingRequestRecordingRule(t *testing.T) {
	tests := []struct {
		name          string
		sources       [][]float32
		want          string
		wantTailEmpty bool
	}{
		{
			name: "multiple_missing_sources",
			sources: [][]float32{
				issue694MissingValues(3),
				issue694MissingValues(3),
				issue694MissingValues(3),
			},
			want:          "TimeSeries(0, 3, 60, [. . .])",
			wantTailEmpty: true,
		},
		{
			name: "missing_and_genuine_zero",
			sources: [][]float32{
				issue694MissingValues(3),
				{0, 0, 0},
			},
			want:          "TimeSeries(0, 3, 60, [0 0 0])",
			wantTailEmpty: false,
		},
		{
			name: "missing_and_positive",
			sources: [][]float32{
				issue694MissingValues(3),
				{1, 2, 3},
			},
			want:          "TimeSeries(0, 3, 60, [1 2 3])",
			wantTailEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := &db.Project{Id: db.ProjectId("issue-694")}
			world := model.NewWorld(0, timeseries.Time(0).Add(3*timeseries.Minute), timeseries.Minute, timeseries.Minute)
			sourceID := model.NewApplicationId(project.ClusterId(), "default", model.ApplicationKindDeployment, "source")
			targetID := model.NewApplicationId(project.ClusterId(), "default", model.ApplicationKindDeployment, "target")
			source := world.GetOrCreateApplication(sourceID, false)
			target := world.GetOrCreateApplication(targetID, false)
			targetInstance := target.GetOrCreateInstance("target", nil)

			for i, values := range tt.sources {
				instance := source.GetOrCreateInstance(fmt.Sprintf("source-%d", i), nil)
				instance.Upstreams[model.ConnectionKey{Destination: fmt.Sprintf("target-%d", i)}] = &model.Connection{
					RemoteInstance: targetInstance,
					RequestsCount: map[model.Protocol]map[string]*timeseries.TimeSeries{
						model.ProtocolHttp: {
							"200": issue694TimeSeries(values),
						},
					},
				}
			}

			recorded := RecordingRules[qRecordingRuleApplicationL7Requests](nil, project, world)
			require.Len(t, recorded, 1)
			assert.Equal(t, sourceID.String(), recorded[0].Labels["app"])
			assert.Equal(t, targetID.String(), recorded[0].Labels["dest"])
			assert.Equal(t, "ok", recorded[0].Labels["status"])
			assert.Equal(t, string(model.ProtocolHttp), recorded[0].Labels["proto"])
			assert.Equal(t, tt.want, recorded[0].Values.String())
			assert.Equal(t, tt.wantTailEmpty, recorded[0].Values.TailIsEmpty())
		})
	}
}
