package clickhouse

import (
	"testing"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func rumTestWorld(t *testing.T) *model.World {
	t.Helper()
	return model.NewWorld(1_700_000_000, 1_700_000_600, timeseries.Minute, timeseries.Minute)
}

func TestApplyRumTopology(t *testing.T) {
	t.Run("creates RumClient with frontend category", func(t *testing.T) {
		w := rumTestWorld(t)
		applyRumTopology(w, nil, []string{"demo-web"}, nil)

		id := NewRumClientApplicationId("demo-web")
		app := w.GetApplication(id)
		require.NotNil(t, app)
		assert.Equal(t, model.ApplicationCategoryFrontend, app.Category)
		require.NotNil(t, app.Settings)
		require.NotNil(t, app.Settings.Rum)
		assert.Equal(t, "demo-web", app.Settings.Rum.Service)
	})

	t.Run("host mapping links backend", func(t *testing.T) {
		w := rumTestWorld(t)
		backendId := model.NewApplicationId("cluster1", "shop", model.ApplicationKindDeployment, "api")
		w.GetOrCreateApplication(backendId, false)

		project := &db.Project{Settings: db.ProjectSettings{
			Rum: &db.RumProjectSettings{
				HostMappings: []db.RumHostMapping{
					{Pattern: "api.example.com", AppId: backendId.String()},
				},
			},
		}}
		edges := []RumServiceEdge{{
			ClientService: "shop-web",
			ServerService: "api.example.com",
			Requests:      10,
			Failed:        0,
			AvgLatencyMs:  120,
		}}
		applyRumTopology(w, project, []string{"shop-web"}, edges)

		client := w.GetApplication(NewRumClientApplicationId("shop-web"))
		require.NotNil(t, client)
		conn := client.Upstreams[backendId]
		require.NotNil(t, conn)
		assert.True(t, conn.RumObserved)
		assert.Equal(t, backendId, conn.RemoteApplication.Id)
	})

	t.Run("unknown server becomes ExternalService placeholder", func(t *testing.T) {
		w := rumTestWorld(t)
		edges := []RumServiceEdge{{
			ClientService: "portal",
			ServerService: "https://unknown.vendor.com/path",
			Requests:      4,
			Failed:        0,
			AvgLatencyMs:  50,
		}}
		applyRumTopology(w, nil, []string{"portal"}, edges)

		client := w.GetApplication(NewRumClientApplicationId("portal"))
		require.NotNil(t, client)
		require.Len(t, client.Upstreams, 1)
		var server *model.Application
		for _, conn := range client.Upstreams {
			server = conn.RemoteApplication
			break
		}
		require.NotNil(t, server)
		assert.Equal(t, model.ApplicationKindExternalService, server.Id.Kind)
		assert.Equal(t, "unknown.vendor.com", server.Id.Name)
	})

	t.Run("failed requests use 5xx status bucket", func(t *testing.T) {
		w := rumTestWorld(t)
		edges := []RumServiceEdge{
			{
				ClientService: "a",
				ServerService: "fail.example.com",
				Requests:      10,
				Failed:        5,
				AvgLatencyMs:  200,
			},
			{
				ClientService: "b",
				ServerService: "ok.example.com",
				Requests:      10,
				Failed:        4,
				AvgLatencyMs:  100,
			},
		}
		applyRumTopology(w, nil, []string{"a", "b"}, edges)

		conn5xx := w.GetApplication(NewRumClientApplicationId("a")).Upstreams
		require.Len(t, conn5xx, 1)
		for _, c := range conn5xx {
			require.NotNil(t, c.RequestsCount[model.ProtocolHttp]["5xx"])
			assert.Nil(t, c.RequestsCount[model.ProtocolHttp]["2xx"])
		}

		conn2xx := w.GetApplication(NewRumClientApplicationId("b")).Upstreams
		require.Len(t, conn2xx, 1)
		for _, c := range conn2xx {
			require.NotNil(t, c.RequestsCount[model.ProtocolHttp]["2xx"])
			assert.Nil(t, c.RequestsCount[model.ProtocolHttp]["5xx"])
		}
	})
}
