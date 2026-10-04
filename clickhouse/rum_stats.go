package clickhouse

import (
	"context"

	"github.com/coroot/coroot/model"
	"k8s.io/klog"
)

// LoadRumStatsIntoWorld populates Application.RumStats from ClickHouse for auditor/alerts.
func LoadRumStatsIntoWorld(ctx context.Context, chs Clients, w *model.World) {
	if w == nil || len(chs.Clients) == 0 {
		return
	}
	for _, ch := range chs.Clients {
		services, err := ch.GetRumServices(ctx, w.Ctx.From)
		if err != nil {
			klog.Warningln(err)
			continue
		}
		svcSet := map[string]bool{}
		for _, s := range services {
			svcSet[s] = true
		}

		for _, app := range w.Applications {
			svc := ""
			if app.Settings != nil && app.Settings.Rum != nil {
				svc = app.Settings.Rum.Service
			}
			if svc == "" && app.Id.Kind == model.ApplicationKindRumClient {
				svc = app.Id.Name
			}
			if svc == "" || !svcSet[svc] {
				continue
			}
			stats := &model.RumStats{ServiceName: svc}
			stats.LcpP75, _ = ch.GetRumWebVitalP75(ctx, svc, "lcp", w.Ctx.From, w.Ctx.To, w.Ctx.Step)
			stats.InpP75, _ = ch.GetRumWebVitalP75(ctx, svc, "inp", w.Ctx.From, w.Ctx.To, w.Ctx.Step)
			stats.ClsP75, _ = ch.GetRumWebVitalP75(ctx, svc, "cls", w.Ctx.From, w.Ctx.To, w.Ctx.Step)
			stats.TtfbP75, _ = ch.GetRumWebVitalP75(ctx, svc, "ttfb", w.Ctx.From, w.Ctx.To, w.Ctx.Step)
			stats.PageViewsPs, _ = ch.GetRumPageViewRate(ctx, svc, w.Ctx.From, w.Ctx.To, w.Ctx.Step)
			stats.JsErrorsPs, _ = ch.GetRumErrorRate(ctx, svc, w.Ctx.From, w.Ctx.To, w.Ctx.Step)

			edges, err := ch.GetRumServiceEdges(ctx, w.Ctx.From, w.Ctx.To)
			if err == nil {
				var req, failed float64
				for _, e := range edges {
					if e.ClientService == svc {
						req += e.Requests
						failed += e.Failed
					}
				}
				if req > 0 {
					stats.FetchErrorPct = float32(failed / req * 100)
				}
			}
			app.RumStats = stats
		}
	}
}
