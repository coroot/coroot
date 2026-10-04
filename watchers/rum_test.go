package watchers

import (
	"testing"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tsConst(v float32) *timeseries.TimeSeries {
	return timeseries.NewWithData(0, timeseries.Minute, []float32{v})
}

func TestCollectRumSignals(t *testing.T) {
	world := model.NewWorld(0, 600, timeseries.Minute, timeseries.Minute)

	rumID := model.NewApplicationId(model.ClusterIdExternal, "frontend", model.ApplicationKindRumClient, "shop-web")
	rumApp := world.GetOrCreateApplication(rumID, true)
	rumApp.RumStats = &model.RumStats{
		LcpP75:        tsConst(model.Checks.RumLcpP75.DefaultThreshold + 100),
		FetchErrorPct: model.Checks.RumFetchErrors.DefaultThreshold + 1,
	}

	backendID := model.NewApplicationId("c1", "shop", model.ApplicationKindDeployment, "api")
	backend := world.GetOrCreateApplication(backendID, false)
	conn := &model.AppToAppConnection{
		Application:       rumApp,
		RemoteApplication: backend,
	}
	rumApp.Upstreams[backendID] = conn
	backend.Downstreams = map[model.ApplicationId]*model.AppToAppConnection{rumID: conn}

	otherRumID := model.NewApplicationId(model.ClusterIdExternal, "frontend", model.ApplicationKindRumClient, "other-web")
	otherRum := world.GetOrCreateApplication(otherRumID, true)
	otherRum.RumStats = &model.RumStats{LcpP75: tsConst(99999)}

	t.Run("RumClient itself", func(t *testing.T) {
		signals := collectRumSignals(rumApp, world)
		require.NotEmpty(t, signals)
		checks := map[string]bool{}
		for _, s := range signals {
			checks[s.Check] = true
		}
		assert.True(t, checks[string(model.Checks.RumLcpP75.Id)])
		assert.True(t, checks[string(model.Checks.RumFetchErrors.Id)])
	})

	t.Run("backend linked via upstreams", func(t *testing.T) {
		signals := collectRumSignals(backend, world)
		require.NotEmpty(t, signals)
		assert.Equal(t, "shop-web", signals[0].Service)
	})

	t.Run("unrelated RumClient ignored for backend", func(t *testing.T) {
		signals := collectRumSignals(backend, world)
		for _, s := range signals {
			assert.NotEqual(t, "other-web", s.Service)
		}
	})

	t.Run("FetchErrorPct threshold", func(t *testing.T) {
		quiet := world.GetOrCreateApplication(
			model.NewApplicationId(model.ClusterIdExternal, "frontend", model.ApplicationKindRumClient, "quiet"),
			true,
		)
		quiet.RumStats = &model.RumStats{FetchErrorPct: model.Checks.RumFetchErrors.DefaultThreshold}
		assert.Empty(t, collectRumSignals(quiet, world))
	})
}
