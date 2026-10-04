package rum

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/coroot/coroot/clickhouse"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"k8s.io/klog"
)

type View struct {
	Status           model.Status    `json:"status"`
	Message          string          `json:"message"`
	Services         []Service       `json:"services"`
	Percentile       int             `json:"percentile"`
	Filters          Filters         `json:"filters"`
	Summary          *Summary        `json:"summary,omitempty"`
	Heatmap          *model.Heatmap  `json:"heatmap"`
	Charts           []*model.Chart  `json:"charts"`
	TopPages         []PageStat      `json:"top_pages"`
	Browsers         []BreakdownStat `json:"browsers"`
	Devices          []BreakdownStat `json:"devices"`
	OperatingSystems []BreakdownStat `json:"operating_systems,omitempty"`
	Countries        []BreakdownStat `json:"countries,omitempty"`
	Versions         []VersionStat   `json:"versions,omitempty"`
	Ajax             []AjaxStat      `json:"ajax,omitempty"`
	Errors           []ErrorGroup    `json:"errors,omitempty"`
	Sessions         []SessionStat   `json:"sessions,omitempty"`
	VitalRatings     []VitalRating   `json:"vital_ratings,omitempty"`
	VitalHist        []VitalBucket   `json:"vital_hist,omitempty"`
	LinkedBackends   []LinkedBackend `json:"linked_backends,omitempty"`
	PageViews        []PageView      `json:"page_views"`
	SessionInfo      *SessionStat    `json:"session_info,omitempty"`
	Session          []SessionEvent  `json:"session,omitempty"`
	Replay           []ReplaySegment `json:"replay,omitempty"`
	Spans            []Span          `json:"spans,omitempty"`
	Thresholds       Thresholds      `json:"thresholds"`
	ExplorerQuery    string          `json:"explorer_query,omitempty"`
	ExplorerError    string          `json:"explorer_error,omitempty"`
	SessionsLimited  bool            `json:"sessions_limited,omitempty"`
	PageViewsLimited bool            `json:"page_views_limited,omitempty"`
	// RawRetention is the effective raw RUM TTL (e.g. "7d") for UI hints about detailed data.
	RawRetention string `json:"raw_retention,omitempty"`
	// AggregatesRetention is the effective aggregates TTL (e.g. "30d").
	AggregatesRetention string `json:"aggregates_retention,omitempty"`
}

type Filters struct {
	PagePath string `json:"page,omitempty"`
	Browser  string `json:"browser,omitempty"`
	Os       string `json:"os,omitempty"`
	Device   string `json:"device,omitempty"`
	Country  string `json:"country,omitempty"`
	Version  string `json:"version,omitempty"`
}

type Thresholds struct {
	LcpMs  float64 `json:"lcp_ms"`
	InpMs  float64 `json:"inp_ms"`
	Cls    float64 `json:"cls"`
	TtfbMs float64 `json:"ttfb_ms"`
}

type Summary struct {
	PageViews      uint64  `json:"page_views"`
	UniqueSessions uint64  `json:"unique_sessions"`
	LoadMs         float64 `json:"load_ms"`
	LcpMs          float64 `json:"lcp_ms"`
	TtfbMs         float64 `json:"ttfb_ms"`
	InpMs          float64 `json:"inp_ms,omitempty"`
	Cls            float64 `json:"cls,omitempty"`
	ErrorCount     uint64  `json:"error_count"`
	FetchErrorPct  float32 `json:"fetch_error_pct"`
	// Deltas vs previous equal-length window (current - previous).
	DeltaPageViews  int64   `json:"delta_page_views,omitempty"`
	DeltaLoadMs     float64 `json:"delta_load_ms,omitempty"`
	DeltaLcpMs      float64 `json:"delta_lcp_ms,omitempty"`
	DeltaInpMs      float64 `json:"delta_inp_ms,omitempty"`
	DeltaCls        float64 `json:"delta_cls,omitempty"`
	DeltaTtfbMs     float64 `json:"delta_ttfb_ms,omitempty"`
	DeltaErrorCount int64   `json:"delta_error_count,omitempty"`
	GoodLcp         uint64  `json:"good_lcp,omitempty"`
	NeedsLcp        uint64  `json:"needs_lcp,omitempty"`
	PoorLcp         uint64  `json:"poor_lcp,omitempty"`
	GoodInp         uint64  `json:"good_inp,omitempty"`
	NeedsInp        uint64  `json:"needs_inp,omitempty"`
	PoorInp         uint64  `json:"poor_inp,omitempty"`
	GoodCls         uint64  `json:"good_cls,omitempty"`
	NeedsCls        uint64  `json:"needs_cls,omitempty"`
	PoorCls         uint64  `json:"poor_cls,omitempty"`
	GoodTtfb        uint64  `json:"good_ttfb,omitempty"`
	NeedsTtfb       uint64  `json:"needs_ttfb,omitempty"`
	PoorTtfb        uint64  `json:"poor_ttfb,omitempty"`
}

type VersionStat struct {
	Version    string  `json:"version"`
	Count      uint64  `json:"count"`
	LatencyMs  float64 `json:"latency_ms"`
	ErrorCount uint64  `json:"error_count"`
}

type LinkedBackend struct {
	Id   model.ApplicationId `json:"id"`
	Name string              `json:"name"`
	Host string              `json:"host,omitempty"`
}

type AjaxStat struct {
	Method      string              `json:"method"`
	URL         string              `json:"url"`
	PeerService string              `json:"peer_service,omitempty"`
	BackendId   model.ApplicationId `json:"backend_id,omitempty"`
	BackendName string              `json:"backend_name,omitempty"`
	Count       uint64              `json:"count"`
	ErrorCount  uint64              `json:"error_count"`
	LatencyMs   float64             `json:"latency_ms"`
}

type ErrorGroup struct {
	Message     string `json:"message"`
	Type        string `json:"type"`
	Count       uint64 `json:"count"`
	Sessions    uint64 `json:"sessions"`
	FirstSeen   int64  `json:"first_seen"`
	LastSeen    int64  `json:"last_seen"`
	TopPage     string `json:"top_page,omitempty"`
	SampleTrace string `json:"sample_trace,omitempty"`
}

type SessionStat struct {
	SessionId  string  `json:"session_id"`
	StartedAt  int64   `json:"started_at"`
	DurationMs float64 `json:"duration_ms"`
	PageViews  uint64  `json:"page_views"`
	Errors     uint64  `json:"errors"`
	Browser    string  `json:"browser,omitempty"`
	Os         string  `json:"os,omitempty"`
	Device     string  `json:"device,omitempty"`
	Country    string  `json:"country,omitempty"`
	Version    string  `json:"version,omitempty"`
}

type VitalRating struct {
	EventType string `json:"event_type"`
	Good      uint64 `json:"good"`
	NeedsImp  uint64 `json:"needs_improvement"`
	Poor      uint64 `json:"poor"`
}

type VitalBucket struct {
	EventType string  `json:"event_type"`
	Bucket    float64 `json:"bucket"`
	Total     uint64  `json:"total"`
}

type SessionEvent struct {
	Timestamp  int64   `json:"timestamp"`
	Kind       string  `json:"kind"`
	Name       string  `json:"name"`
	TraceId    string  `json:"trace_id"`
	SpanId     string  `json:"span_id"`
	PagePath   string  `json:"page_path"`
	DurationMs float64 `json:"duration_ms"`
	Status     string  `json:"status"`
	Version    string  `json:"version,omitempty"`
}

type ReplaySegment struct {
	Timestamp int64  `json:"timestamp"`
	Seq       uint32 `json:"seq"`
	TraceId   string `json:"trace_id"`
	Payload   string `json:"payload"`
}

type Service struct {
	Name   string `json:"name"`
	Linked bool   `json:"linked"`
}

type PageStat struct {
	Path       string  `json:"path"`
	Count      uint64  `json:"count"`
	LatencyMs  float64 `json:"latency_ms"`
	LcpMs      float64 `json:"lcp_ms,omitempty"`
	InpMs      float64 `json:"inp_ms,omitempty"`
	ErrorCount uint64  `json:"error_count"`
}

type BreakdownStat struct {
	Name       string  `json:"name"`
	Count      uint64  `json:"count"`
	LatencyMs  float64 `json:"latency_ms"`
	ErrorCount uint64  `json:"error_count,omitempty"`
}

type PageView struct {
	Timestamp int64   `json:"timestamp"`
	TraceId   string  `json:"trace_id"`
	SpanId    string  `json:"span_id"`
	SessionId string  `json:"session_id,omitempty"`
	PagePath  string  `json:"page_path"`
	Duration  float64 `json:"duration"`
	Status    string  `json:"status"`
	Name      string  `json:"name"`
	LcpMs     float64 `json:"lcp_ms,omitempty"`
	TtfbMs    float64 `json:"ttfb_ms,omitempty"`
	InpMs     float64 `json:"inp_ms,omitempty"`
	Cls       float64 `json:"cls,omitempty"`
	Browser   string  `json:"browser,omitempty"`
	Os        string  `json:"os,omitempty"`
	Device    string  `json:"device,omitempty"`
	Country   string  `json:"country,omitempty"`
	Version   string  `json:"version,omitempty"`
}

type Span struct {
	Service   string                `json:"service"`
	TraceId   string                `json:"trace_id"`
	Id        string                `json:"id"`
	ParentId  string                `json:"parent_id"`
	Name      string                `json:"name"`
	Timestamp int64                 `json:"timestamp"`
	Duration  float64               `json:"duration"`
	Status    model.TraceSpanStatus `json:"status"`
}

func parseFilters(q url.Values) clickhouse.RumFilter {
	return clickhouse.RumFilter{
		PagePath: q.Get("page"),
		Browser:  q.Get("browser"),
		Os:       q.Get("os"),
		Device:   q.Get("device"),
		Country:  q.Get("country"),
		Version:  q.Get("version"),
	}
}

func parsePercentile(q url.Values) (int, float64) {
	p, _ := strconv.Atoi(q.Get("percentile"))
	return clickhouse.NormalizePercentile(p)
}

func parseSelection(q url.Values, w *model.World) *clickhouse.RumSelection {
	raw := q.Get("rum_sel")
	if raw == "" {
		// Also accept tracing-style `trace` brush: ::tsRange:durRange:
		raw = q.Get("trace")
	}
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ":")
	tsRange, durRange := "-", "-"
	if len(parts) >= 4 {
		tsRange = parts[2]
		durRange = parts[3]
	} else if strings.Contains(raw, "-") && !strings.Contains(raw, ":") {
		// rum_sel=tsFrom-tsTo|durFrom-durTo
		halves := strings.SplitN(raw, "|", 2)
		tsRange = halves[0]
		if len(halves) > 1 {
			durRange = halves[1]
		}
	}
	tp := strings.Split(tsRange+"-", "-")
	tsFrom := utils.ParseTime(w.Ctx.To, tp[0], w.Ctx.From)
	tsTo := utils.ParseTime(w.Ctx.To, tp[1], w.Ctx.To)
	dp := strings.Split(durRange+"-", "-")
	durFromStr, durToStr := dp[0], dp[1]
	sel := &clickhouse.RumSelection{
		TsFrom:  tsFrom,
		TsTo:    tsTo,
		DurFrom: utils.ParseHeatmapDuration(durFromStr),
		DurTo:   utils.ParseHeatmapDuration(durToStr),
		Errors:  durFromStr == "inf" || durToStr == "err",
	}
	return sel
}

func Render(ctx context.Context, ch *clickhouse.Client, app *model.Application, q url.Values, w *model.World) *View {
	v := &View{
		Thresholds: Thresholds{
			LcpMs:  float64(model.Checks.RumLcpP75.DefaultThreshold),
			InpMs:  float64(model.Checks.RumInpP75.DefaultThreshold),
			Cls:    float64(model.Checks.RumClsP75.DefaultThreshold),
			TtfbMs: float64(model.Checks.RumTtfbP75.DefaultThreshold),
		},
	}
	pct, qLevel := parsePercentile(q)
	v.Percentile = pct
	filt := parseFilters(q)
	v.Filters = Filters{
		PagePath: filt.PagePath,
		Browser:  filt.Browser,
		Os:       filt.Os,
		Device:   filt.Device,
		Country:  filt.Country,
		Version:  filt.Version,
	}

	if ch == nil {
		v.Status = model.WARNING
		v.Message = "Clickhouse integration is not configured"
		return v
	}

	services, err := ch.GetRumServices(ctx, w.Ctx.From)
	if err != nil {
		klog.Errorln(err)
		v.Status = model.WARNING
		v.Message = fmt.Sprintf("Clickhouse error: %s", err)
		return v
	}

	serviceName := ""
	if app.Settings != nil && app.Settings.Rum != nil && app.Settings.Rum.Service != "" {
		serviceName = app.Settings.Rum.Service
	} else if app.Id.Kind == model.ApplicationKindRumClient {
		serviceName = app.Id.Name
	} else if app.Settings != nil && app.Settings.Tracing != nil {
		serviceName = app.Settings.Tracing.Service
	} else {
		serviceName = model.GuessService(services, w, app)
	}

	for _, s := range services {
		v.Services = append(v.Services, Service{Name: s, Linked: s == serviceName})
	}
	sort.Slice(v.Services, func(i, j int) bool { return v.Services[i].Name < v.Services[j].Name })

	if len(services) == 0 {
		v.Status = model.UNKNOWN
		v.Message = "No RUM data found. Integrate coroot-rum.js to start collecting browser telemetry."
		return v
	}
	if serviceName == "" {
		v.Status = model.UNKNOWN
		v.Message = "Select a RUM service name to link with this application."
		return v
	}

	traceId := q.Get("trace")
	if traceId != "" && !strings.Contains(traceId, ":") {
		spans, e := ch.GetSpansByTraceId(ctx, traceId)
		if e != nil {
			// fallback to rum spans table
			spans, e = ch.GetRumSpansByTraceId(ctx, traceId)
		}
		if e != nil {
			klog.Errorln(e)
			v.Status = model.WARNING
			v.Message = e.Error()
			return v
		}
		for _, s := range spans {
			v.Spans = append(v.Spans, Span{
				Service:   s.ServiceName,
				TraceId:   s.TraceId,
				Id:        s.SpanId,
				ParentId:  s.ParentSpanId,
				Name:      s.Name,
				Timestamp: s.Timestamp.UnixMilli(),
				Duration:  float64(s.Duration) / 1e6,
				Status:    s.Status(),
			})
		}
		v.Status = model.OK
		return v
	}

	// Lightweight session detail for the explorer panel — skip dashboard queries.
	if q.Get("only") == "session" {
		sessionId := q.Get("session")
		if sessionId == "" {
			v.Status = model.WARNING
			v.Message = "session id is required"
			return v
		}
		fillSessionDetail(ctx, ch, v, serviceName, sessionId, w.Ctx.From, w.Ctx.To, q.Get("replay") == "1")
		v.Status = model.OK
		v.Message = fmt.Sprintf("RUM service: <i>%s</i>", serviceName)
		return v
	}

	rumQ := strings.TrimSpace(q.Get("rum_q"))
	v.ExplorerQuery = rumQ
	var explorer *clickhouse.RumExplorerFilter
	if rumQ != "" {
		ex, perr := clickhouse.ParseRumExplorerQuery(rumQ)
		if perr != nil {
			v.ExplorerError = perr.Error()
		} else {
			explorer = &ex
		}
	}

	// Explorer-only reload — skip dashboard queries.
	if q.Get("only") == "explorer" {
		if v.ExplorerError == "" {
			sel := parseSelection(q, w)
			fillExplorerLists(ctx, ch, v, serviceName, w.Ctx.From, w.Ctx.To, filt, sel, explorer)
		}
		v.Status = model.OK
		v.Message = fmt.Sprintf("RUM service: <i>%s</i>", serviceName)
		return v
	}

	// Charts (moved from auditor widgets).
	v.Charts = buildCharts(ctx, ch, app, serviceName, w, qLevel, filt)

	hist, err := ch.GetRumSpansHistogram(ctx, serviceName, w.Ctx.From, w.Ctx.To, w.Ctx.Step)
	if err != nil {
		klog.Errorln(err)
		v.Status = model.WARNING
		v.Message = fmt.Sprintf("Clickhouse error: %s", err)
		return v
	}
	if hist != nil {
		hm := model.NewHeatmap(w.Ctx, "Page load latency & errors")
		for _, s := range model.HistogramSeries(hist, 0, 0) {
			hm.AddSeries(s.Name, s.Title, s.Data, s.Threshold, s.Value)
		}
		v.Heatmap = hm
	}

	if summary, err := ch.GetRumSummaryFiltered(ctx, serviceName, w.Ctx.From, w.Ctx.To, qLevel, filt); err != nil {
		klog.Warningln(err)
	} else if summary != nil {
		v.Summary = &Summary{
			PageViews:      summary.PageViews,
			UniqueSessions: summary.UniqueSessions,
			LoadMs:         summary.P75LoadMs,
			LcpMs:          summary.P75LcpMs,
			TtfbMs:         summary.P75TtfbMs,
			InpMs:          summary.P75InpMs,
			Cls:            summary.P75Cls,
			ErrorCount:     summary.ErrorCount,
			FetchErrorPct:  summary.FetchErrorPct,
		}
		// Previous equal window for deltas.
		window := w.Ctx.To.Sub(w.Ctx.From)
		prevFrom := w.Ctx.From.Add(-window)
		prevTo := w.Ctx.From
		if prev, err := ch.GetRumSummaryFiltered(ctx, serviceName, prevFrom, prevTo, qLevel, filt); err == nil && prev != nil {
			v.Summary.DeltaPageViews = int64(summary.PageViews) - int64(prev.PageViews)
			v.Summary.DeltaLoadMs = summary.P75LoadMs - prev.P75LoadMs
			v.Summary.DeltaLcpMs = summary.P75LcpMs - prev.P75LcpMs
			v.Summary.DeltaInpMs = summary.P75InpMs - prev.P75InpMs
			v.Summary.DeltaCls = summary.P75Cls - prev.P75Cls
			v.Summary.DeltaTtfbMs = summary.P75TtfbMs - prev.P75TtfbMs
			v.Summary.DeltaErrorCount = int64(summary.ErrorCount) - int64(prev.ErrorCount)
		}
		if ratings, err := ch.GetRumWebVitalRatings(ctx, serviceName, w.Ctx.From, w.Ctx.To, filt); err == nil {
			for _, r := range ratings {
				v.VitalRatings = append(v.VitalRatings, VitalRating{
					EventType: r.EventType, Good: r.Good, NeedsImp: r.NeedsImp, Poor: r.Poor,
				})
				switch r.EventType {
				case "lcp":
					v.Summary.GoodLcp, v.Summary.NeedsLcp, v.Summary.PoorLcp = r.Good, r.NeedsImp, r.Poor
				case "inp":
					v.Summary.GoodInp, v.Summary.NeedsInp, v.Summary.PoorInp = r.Good, r.NeedsImp, r.Poor
				case "cls":
					v.Summary.GoodCls, v.Summary.NeedsCls, v.Summary.PoorCls = r.Good, r.NeedsImp, r.Poor
				case "ttfb":
					v.Summary.GoodTtfb, v.Summary.NeedsTtfb, v.Summary.PoorTtfb = r.Good, r.NeedsImp, r.Poor
				}
			}
		}
	}

	if pages, err := ch.GetRumTopPagesFiltered(ctx, serviceName, w.Ctx.From, w.Ctx.To, qLevel, filt, 20); err != nil {
		klog.Warningln(err)
	} else {
		for _, p := range pages {
			v.TopPages = append(v.TopPages, PageStat{
				Path: p.PagePath, Count: p.Count, LatencyMs: p.P75, LcpMs: p.LcpP75, InpMs: p.InpP75, ErrorCount: p.ErrorCount,
			})
		}
	}

	fillBreakdown := func(dim string, dest *[]BreakdownStat) {
		items, err := ch.GetRumBreakdownFiltered(ctx, serviceName, w.Ctx.From, w.Ctx.To, dim, qLevel, filt)
		if err != nil {
			klog.Warningln(err)
			return
		}
		for _, b := range items {
			name := b.BrowserName
			if dim == "os" {
				name = b.OsName
			} else if dim == "device" {
				name = b.DeviceType
			}
			*dest = append(*dest, BreakdownStat{Name: name, Count: b.Count, LatencyMs: b.P75, ErrorCount: b.ErrorCount})
		}
	}
	fillBreakdown("browser", &v.Browsers)
	fillBreakdown("device", &v.Devices)
	fillBreakdown("os", &v.OperatingSystems)

	sel := parseSelection(q, w)
	fillExplorerLists(ctx, ch, v, serviceName, w.Ctx.From, w.Ctx.To, filt, sel, explorer)

	if versions, err := ch.GetRumVersionsFiltered(ctx, serviceName, "lcp", w.Ctx.From, w.Ctx.To, qLevel, filt); err != nil {
		klog.Warningln(err)
	} else {
		for _, ver := range versions {
			v.Versions = append(v.Versions, VersionStat{Version: ver.Version, Count: ver.Count, LatencyMs: ver.P75Ms, ErrorCount: ver.ErrorCount})
		}
	}

	if countries, err := ch.GetRumGeoBreakdownFiltered(ctx, serviceName, w.Ctx.From, w.Ctx.To, qLevel, filt); err != nil {
		klog.Warningln(err)
	} else {
		for _, ctry := range countries {
			v.Countries = append(v.Countries, BreakdownStat{Name: ctry.BrowserName, Count: ctry.Count, LatencyMs: ctry.P75})
		}
	}

	if ajax, err := ch.GetRumAjaxStats(ctx, serviceName, w.Ctx.From, w.Ctx.To, qLevel, filt, 25); err != nil {
		klog.Warningln(err)
	} else {
		for _, a := range ajax {
			item := AjaxStat{
				Method: a.Method, URL: a.URL, PeerService: a.PeerService,
				Count: a.Count, ErrorCount: a.ErrorCount, LatencyMs: a.LatencyMs,
			}
			if a.PeerService != "" && w != nil {
				if backend := clickhouse.GuessBackendAppForRumServer(w, a.PeerService); backend != nil {
					item.BackendId = backend.Id
					item.BackendName = backend.Id.Name
				}
			}
			v.Ajax = append(v.Ajax, item)
		}
	}

	if errs, err := ch.GetRumErrors(ctx, serviceName, w.Ctx.From, w.Ctx.To, filt, 25); err != nil {
		klog.Warningln(err)
	} else {
		for _, e := range errs {
			v.Errors = append(v.Errors, ErrorGroup{
				Message: e.Message, Type: e.Type, Count: e.Count, Sessions: e.Sessions,
				FirstSeen: e.FirstSeen.UnixMilli(), LastSeen: e.LastSeen.UnixMilli(),
				TopPage: e.TopPage, SampleTrace: e.SampleTrace,
			})
		}
	}

	if buckets, err := ch.GetRumWebVitalsHist(ctx, serviceName, w.Ctx.From, w.Ctx.To, filt); err != nil {
		klog.Warningln(err)
	} else {
		for _, b := range buckets {
			v.VitalHist = append(v.VitalHist, VitalBucket{EventType: b.EventType, Bucket: b.Bucket, Total: b.Total})
		}
	}

	if w != nil {
		rumAppId := clickhouse.NewRumClientApplicationId(serviceName)
		if rumApp := w.GetApplication(rumAppId); rumApp != nil {
			for _, u := range rumApp.Upstreams {
				if u.RemoteApplication == nil {
					continue
				}
				v.LinkedBackends = append(v.LinkedBackends, LinkedBackend{
					Id:   u.RemoteApplication.Id,
					Name: u.RemoteApplication.Id.Name,
				})
			}
		}
	}

	v.Status = model.OK
	v.Message = fmt.Sprintf("RUM service: <i>%s</i>", serviceName)
	return v
}

const explorerListLimit = 200

func fillExplorerLists(ctx context.Context, ch *clickhouse.Client, v *View, serviceName string, from, to timeseries.Time, filt clickhouse.RumFilter, sel *clickhouse.RumSelection, explorer *clickhouse.RumExplorerFilter) {
	// On parse errors keep previous lists empty here; caller sets ExplorerError and full render still
	// returns other dashboard data. For only=explorer the frontend should not reset lists on error.
	ex := explorer
	if v.ExplorerError != "" {
		ex = nil
	}
	if views, err := ch.GetRumPageViewsFiltered(ctx, serviceName, from, to, filt, sel, ex, explorerListLimit); err != nil {
		klog.Warningln(err)
	} else {
		v.PageViewsLimited = len(views) >= explorerListLimit
		for _, pv := range views {
			v.PageViews = append(v.PageViews, PageView{
				Timestamp: pv.Timestamp.UnixMilli(),
				TraceId:   pv.TraceId,
				SpanId:    pv.SpanId,
				SessionId: pv.SessionId,
				PagePath:  pv.PagePath,
				Duration:  pv.Duration,
				Status:    pv.Status,
				Name:      pv.Name,
				LcpMs:     pv.Lcp,
				TtfbMs:    pv.Ttfb,
				InpMs:     pv.Inp,
				Cls:       pv.Cls,
				Browser:   pv.Browser,
				Os:        pv.Os,
				Device:    pv.Device,
				Country:   pv.Country,
				Version:   pv.Version,
			})
		}
	}
	if sessions, err := ch.GetRumSessions(ctx, serviceName, from, to, filt, ex, explorerListLimit); err != nil {
		klog.Warningln(err)
	} else {
		v.SessionsLimited = len(sessions) >= explorerListLimit
		for _, s := range sessions {
			v.Sessions = append(v.Sessions, sessionStatFromCH(s))
		}
	}
}

func sessionStatFromCH(s clickhouse.RumSessionStat) SessionStat {
	return SessionStat{
		SessionId:  s.SessionId,
		StartedAt:  s.StartedAt.UnixMilli(),
		DurationMs: s.DurationMs,
		PageViews:  s.PageViews,
		Errors:     s.Errors,
		Browser:    s.Browser,
		Os:         s.Os,
		Device:     s.Device,
		Country:    s.Country,
		Version:    s.Version,
	}
}

func fillSessionDetail(ctx context.Context, ch *clickhouse.Client, v *View, serviceName, sessionId string, from, to timeseries.Time, replay bool) {
	if summary, err := ch.GetRumSessionSummary(ctx, serviceName, sessionId, from, to); err != nil {
		klog.Warningln(err)
	} else if summary != nil {
		info := sessionStatFromCH(*summary)
		v.SessionInfo = &info
	}
	events, e := ch.GetRumSessionTimeline(ctx, serviceName, sessionId, from, to, 200)
	if e != nil {
		klog.Warningln(e)
	} else {
		for _, ev := range events {
			v.Session = append(v.Session, SessionEvent{
				Timestamp:  ev.Timestamp.UnixMilli(),
				Kind:       ev.Kind,
				Name:       ev.Name,
				TraceId:    ev.TraceId,
				SpanId:     ev.SpanId,
				PagePath:   ev.PagePath,
				DurationMs: ev.DurationMs,
				Status:     ev.Status,
				Version:    ev.Version,
			})
		}
	}
	if !replay {
		return
	}
	segs, se := ch.GetRumReplaySegments(ctx, serviceName, sessionId, from, to, 500)
	if se != nil {
		klog.Warningln(se)
		return
	}
	for _, s := range segs {
		v.Replay = append(v.Replay, ReplaySegment{
			Timestamp: s.Timestamp.UnixMilli(),
			Seq:       s.Seq,
			TraceId:   s.TraceId,
			Payload:   s.Payload,
		})
	}
}

func buildCharts(ctx context.Context, ch *clickhouse.Client, app *model.Application, service string, w *model.World, qLevel float64, f clickhouse.RumFilter) []*model.Chart {
	var charts []*model.Chart
	useStats := f.Empty() && int(qLevel*100+0.5) == 75 && app != nil && app.RumStats != nil
	stats := app.RumStats

	rateChart := model.NewChart(w.Ctx, "Page views, per second")
	if useStats && stats.PageViewsPs != nil {
		rateChart.AddSeries("views", stats.PageViewsPs)
	} else if ts, err := ch.GetRumPageViewRateFiltered(ctx, service, w.Ctx.From, w.Ctx.To, w.Ctx.Step, f); err == nil {
		rateChart.AddSeries("views", ts)
	}
	charts = append(charts, rateChart)

	errChart := model.NewChart(w.Ctx, "Browser errors, per second")
	if useStats && stats.JsErrorsPs != nil {
		errChart.AddSeries("errors", stats.JsErrorsPs)
	} else if ts, err := ch.GetRumErrorRateFiltered(ctx, service, w.Ctx.From, w.Ctx.To, w.Ctx.Step, f); err == nil {
		errChart.AddSeries("errors", ts)
	}
	charts = append(charts, errChart)

	pctLabel := fmt.Sprintf("p%d", int(qLevel*100+0.5))
	cwvChart := model.NewChart(w.Ctx, fmt.Sprintf("Core Web Vitals %s (ms)", pctLabel))
	addVital := func(name string, fromStats *timeseries.TimeSeries) {
		if useStats && fromStats != nil {
			cwvChart.AddSeries(name, fromStats)
			return
		}
		if ts, err := ch.GetRumWebVitalQuantile(ctx, service, name, w.Ctx.From, w.Ctx.To, w.Ctx.Step, qLevel, f); err == nil {
			cwvChart.AddSeries(name, ts)
		}
	}
	if useStats {
		addVital("lcp", stats.LcpP75)
		addVital("inp", stats.InpP75)
		addVital("ttfb", stats.TtfbP75)
	} else {
		addVital("lcp", nil)
		addVital("inp", nil)
		addVital("ttfb", nil)
	}
	charts = append(charts, cwvChart)

	clsChart := model.NewChart(w.Ctx, fmt.Sprintf("CLS %s", pctLabel))
	if useStats && stats.ClsP75 != nil {
		clsChart.AddSeries("cls", stats.ClsP75)
	} else if ts, err := ch.GetRumWebVitalQuantile(ctx, service, "cls", w.Ctx.From, w.Ctx.To, w.Ctx.Step, qLevel, f); err == nil {
		clsChart.AddSeries("cls", ts)
	}
	charts = append(charts, clsChart)
	return charts
}
