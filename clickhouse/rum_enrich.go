package clickhouse

import (
	"context"
	"strings"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
)

// EnrichWorldWithRum adds RumClient applications, RUM-derived service map edges, and RumStats.
func EnrichWorldWithRum(ctx context.Context, chs Clients, w *model.World, project *db.Project) {
	if w == nil || len(chs.Clients) == 0 {
		return
	}
	for _, ch := range chs.Clients {
		services, err := ch.GetRumServices(ctx, w.Ctx.From)
		if err != nil {
			klog.Warningln(err)
			continue
		}
		var edges []RumServiceEdge
		edges, err = ch.GetRumServiceEdges(ctx, w.Ctx.From, w.Ctx.To)
		if err != nil {
			klog.Warningln(err)
			edges = nil
		}
		applyRumTopology(w, project, services, edges)
	}
	LoadRumStatsIntoWorld(ctx, chs, w)
}

// applyRumTopology creates RumClient apps and HTTP upstream edges from RUM service/edge lists.
func applyRumTopology(w *model.World, project *db.Project, services []string, edges []RumServiceEdge) {
	if w == nil {
		return
	}
	for _, svc := range services {
		id := NewRumClientApplicationId(svc)
		app := w.GetOrCreateApplication(id, true)
		app.Category = model.ApplicationCategoryFrontend
		if app.Settings == nil {
			app.Settings = &model.ApplicationSettings{}
		}
		if app.Settings.Rum == nil {
			app.Settings.Rum = &model.ApplicationSettingsRum{Service: svc}
		}
		if app.Settings.Tracing == nil {
			app.Settings.Tracing = &model.ApplicationSettingsTracing{Service: svc}
		}
	}

	for _, e := range edges {
		if e.ClientService == "" || e.ServerService == "" {
			continue
		}
		clientId := NewRumClientApplicationId(e.ClientService)
		client := w.GetOrCreateApplication(clientId, true)
		if client.Settings == nil {
			client.Settings = &model.ApplicationSettings{Rum: &model.ApplicationSettingsRum{Service: e.ClientService}}
		}

		server := GuessBackendAppForRumServerWithProject(w, project, e.ServerService)
		if server == nil {
			server = findOtelAppForRumHost(w, e.ServerService)
		}
		if server == nil {
			name := sanitizeRumServerName(e.ServerService)
			server = w.GetOrCreateApplication(model.NewApplicationId(model.ClusterIdExternal, "external", model.ApplicationKindExternalService, name), false)
		}

		conn := client.Upstreams[server.Id]
		if conn == nil {
			empty := timeseries.New(w.Ctx.From, int(w.Ctx.To.Sub(w.Ctx.From)/w.Ctx.Step)+1, w.Ctx.Step)
			conn = &model.AppToAppConnection{
				Application:       client,
				RemoteApplication: server,
				RequestsCount:     map[model.Protocol]map[string]*timeseries.TimeSeries{},
				RequestsLatency:   map[model.Protocol]*timeseries.TimeSeries{},
				BytesSent:         empty,
				BytesReceived:     empty,
			}
			client.Upstreams[server.Id] = conn
			if server.Downstreams == nil {
				server.Downstreams = map[model.ApplicationId]*model.AppToAppConnection{}
			}
			server.Downstreams[client.Id] = conn
		}
		status := "2xx"
		if e.Failed > 0 && e.Failed >= e.Requests/2 {
			status = "5xx"
		}
		if conn.RequestsCount[model.ProtocolHttp] == nil {
			conn.RequestsCount[model.ProtocolHttp] = map[string]*timeseries.TimeSeries{}
		}
		rate := float32(e.Requests) / float32(w.Ctx.To.Sub(w.Ctx.From))
		if rate <= 0 {
			rate = float32(e.Requests)
		}
		existing := conn.RequestsCount[model.ProtocolHttp][status]
		if existing != nil {
			if prev := existing.Last(); !timeseries.IsNaN(prev) && prev > rate {
				rate = prev
			}
		}
		points := int(w.Ctx.To.Sub(w.Ctx.From)/w.Ctx.Step) + 1
		if points < 1 {
			points = 1
		}
		from := w.Ctx.From.Truncate(w.Ctx.Step)
		ts := timeseries.New(from, points, w.Ctx.Step)
		for i := 0; i < points; i++ {
			ts.Set(from.Add(w.Ctx.Step*timeseries.Duration(i)), rate)
		}
		conn.RequestsCount[model.ProtocolHttp][status] = ts
		conn.RumObserved = true

		lat := timeseries.New(from, points, w.Ctx.Step)
		for i := 0; i < points; i++ {
			lat.Set(from.Add(w.Ctx.Step*timeseries.Duration(i)), float32(e.AvgLatencyMs)/1000)
		}
		conn.RequestsLatency[model.ProtocolHttp] = lat
	}
}

func sanitizeRumServerName(s string) string {
	s = strings.TrimPrefix(strings.TrimPrefix(s, "https://"), "http://")
	if i := strings.Index(s, "/"); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return "unknown"
	}
	return s
}

func findOtelAppForRumHost(w *model.World, serverService string) *model.Application {
	host := NormalizeRumHost(serverService)
	if host == "" {
		host = serverService
	}
	for _, app := range w.Applications {
		if app.Settings == nil || app.Settings.Tracing == nil || app.Settings.Tracing.Service == "" {
			continue
		}
		if app.Id.Kind == model.ApplicationKindRumClient {
			continue
		}
		svc := app.Settings.Tracing.Service
		if strings.EqualFold(svc, host) || strings.EqualFold(svc, serverService) || strings.EqualFold(app.Id.Name, host) || strings.EqualFold(app.Id.Name, serverService) {
			return app
		}
	}
	return nil
}
