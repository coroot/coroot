package overview

import (
	"context"
	"strings"

	"github.com/coroot/coroot/clickhouse"
	"github.com/coroot/coroot/model"
	"k8s.io/klog"
)

func sanitizeServerName(s string) string {
	s = strings.TrimPrefix(strings.TrimPrefix(s, "https://"), "http://")
	if i := strings.Index(s, "/"); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return "unknown"
	}
	return s
}

func findOtelAppForHost(w *model.World, serverService string) *model.Application {
	host := clickhouse.NormalizeRumHost(serverService)
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

func renderRumOverview(ctx context.Context, chs clickhouse.Clients, w *model.World) *RumOverview {
	v := &RumOverview{Status: model.OK}
	if len(chs.Clients) == 0 {
		v.Status = model.WARNING
		v.Message = "Clickhouse integration is not configured"
		return v
	}
	seen := map[string]bool{}
	for _, ch := range chs.Clients {
		services, err := ch.GetRumServices(ctx, w.Ctx.From)
		if err != nil {
			klog.Warningln(err)
			v.Status = model.WARNING
			v.Message = err.Error()
			continue
		}
		for _, s := range services {
			if seen[s] {
				continue
			}
			seen[s] = true
			id := clickhouse.NewRumClientApplicationId(s)
			svc := RumService{Name: s, AppId: id}
			if summary, err := ch.GetRumSummary(ctx, s, w.Ctx.From, w.Ctx.To); err != nil {
				klog.Warningln(err)
			} else if summary != nil {
				svc.PageViews = summary.PageViews
				svc.ErrorCount = summary.ErrorCount
				svc.FetchErrorPct = summary.FetchErrorPct
				svc.P75LcpMs = summary.P75LcpMs
				svc.P75LoadMs = summary.P75LoadMs
			}
			v.Services = append(v.Services, svc)
		}
	}
	if len(v.Services) == 0 && v.Status == model.OK {
		v.Status = model.UNKNOWN
		v.Message = "No RUM data yet. Add coroot-rum.js to your web app."
	}
	return v
}
