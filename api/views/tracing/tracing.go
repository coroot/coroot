package tracing

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/coroot/coroot/clickhouse"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/utils"
	"golang.org/x/exp/maps"
	"k8s.io/klog"
)

const (
	limit = 100
)

type View struct {
	Status   model.Status   `json:"status"`
	Message  string         `json:"message"`
	Sources  []Source       `json:"sources"`
	Services []Service      `json:"services"`
	Heatmap  *model.Heatmap `json:"heatmap"`
	Spans    []Span         `json:"spans"`
	Limit    int            `json:"limit"`
}

type Source struct {
	Type     model.TraceSource `json:"type"`
	Name     string            `json:"name"`
	Selected bool              `json:"selected"`
}

type Service struct {
	Name   string `json:"name"`
	Linked bool   `json:"linked"`
}

type Span struct {
	Service    string                 `json:"service"`
	TraceId    string                 `json:"trace_id"`
	Id         string                 `json:"id"`
	ParentId   string                 `json:"parent_id"`
	Name       string                 `json:"name"`
	Timestamp  int64                  `json:"timestamp"`
	Duration   float64                `json:"duration"`
	Client     string                 `json:"client"`
	Status     model.TraceSpanStatus  `json:"status"`
	Details    model.TraceSpanDetails `json:"details"`
	Attributes map[string]string      `json:"attributes"`
	Events     []Event                `json:"events"`
}

type Event struct {
	Timestamp  int64             `json:"timestamp"`
	Name       string            `json:"name"`
	Attributes map[string]string `json:"attributes"`
}

func Render(ctx context.Context, ch *clickhouse.Client, app *model.Application, q url.Values, w *model.World) *View {
	if ch == nil {
		return nil
	}

	parts := strings.Split(q.Get("trace")+"::::", ":")
	source, traceId, tsRange, durRange := model.TraceSource(parts[0]), parts[1], parts[2], parts[3]
	parts = strings.Split(tsRange+"-", "-")
	tsFrom := utils.ParseTime(w.Ctx.To, parts[0], w.Ctx.From)
	tsTo := utils.ParseTime(w.Ctx.To, parts[1], w.Ctx.To)
	parts = strings.Split(durRange+"-", "-")
	durFromStr, durToStr := parts[0], parts[1]
	durFrom := utils.ParseHeatmapDuration(durFromStr)
	durTo := utils.ParseHeatmapDuration(durToStr)
	errors := durFromStr == "inf" || durToStr == "err"

	v := &View{}

	services, err := ch.GetServicesFromTraces(ctx, w.Ctx.From)
	if err != nil {
		klog.Errorln(err)
		v.Status = model.WARNING
		v.Message = fmt.Sprintf("Clickhouse error: %s", err)
		return v
	}

	var ebpfSpansFound bool
	var otelServices []string
	for _, s := range services {
		if strings.HasPrefix(s, "/") {
			ebpfSpansFound = true
		} else {
			otelServices = append(otelServices, s)
		}
	}

	var otelService string
	if app.Settings != nil && app.Settings.Tracing != nil {
		otelService = app.Settings.Tracing.Service
	} else {
		otelService = model.GuessService(otelServices, app.Id)
	}
	for _, s := range otelServices {
		v.Services = append(v.Services, Service{Name: s, Linked: s == otelService})
	}
	sort.Slice(v.Services, func(i, j int) bool {
		return v.Services[i].Name < v.Services[j].Name
	})

	if len(otelServices) > 0 {
		v.Sources = append(v.Sources, Source{Type: model.TraceSourceOtel, Name: "OpenTelemetry"})
	}
	if ebpfSpansFound {
		v.Sources = append(v.Sources, Source{Type: model.TraceSourceAgent, Name: "OpenTelemetry (eBPF)"})
	}

	if len(v.Sources) == 0 {
		v.Status = model.UNKNOWN
		v.Message = "No traces found"
		return v
	}

	var histogram []model.HistogramBucket
	var spans []*model.TraceSpan
	clients := map[spanKey]string{}
	switch {
	case traceId != "":
		spans, err = ch.GetSpansByTraceId(ctx, traceId)

	case (source == "" || source == model.TraceSourceOtel) && otelService != "":
		source = model.TraceSourceOtel
		var ignoredPeerAddrs []string
		if !app.Category.Monitoring() {
			ignoredPeerAddrs = getMonitoringPodIps(w)
		}
		wg := sync.WaitGroup{}
		wg.Add(2)
		go func() {
			defer wg.Done()
			var e error
			sq := clickhouse.SpanQuery{
				Ctx:              w.Ctx,
				ExcludePeerAddrs: ignoredPeerAddrs,
			}
			sq.AddFilter("ServiceName", "=", otelService)
			histogram, e = ch.GetSpansByServiceNameHistogram(ctx, sq)
			if e != nil {
				err = e
			}
		}()
		var parentSpans []*model.TraceSpan
		go func() {
			defer wg.Done()
			var e error
			sq := clickhouse.SpanQuery{
				Ctx:              w.Ctx,
				TsFrom:           tsFrom,
				TsTo:             tsTo,
				DurFrom:          durFrom,
				DurTo:            durTo,
				Errors:           errors,
				Limit:            limit,
				ExcludePeerAddrs: ignoredPeerAddrs,
			}
			sq.AddFilter("ServiceName", "=", otelService)
			spans, e = ch.GetSpansByServiceName(ctx, sq)
			if e != nil {
				err = e
				return
			}
			parentSpans, e = ch.GetParentSpans(ctx, spans)
			if e != nil {
				err = e
			}
		}()
		wg.Wait()
		clients = getClientsByParentSpans(spans, parentSpans, w)

	case (source == "" || source == model.TraceSourceAgent) && ebpfSpansFound:
		source = model.TraceSourceAgent
		listens := getAppListens(app)
		appClients := getAppClients(app)
		wg := sync.WaitGroup{}
		wg.Add(2)
		go func() {
			defer wg.Done()
			var e error
			sq := clickhouse.SpanQuery{
				Ctx: w.Ctx,
			}
			histogram, e = ch.GetInboundSpansHistogram(ctx, sq, maps.Keys(appClients), listens)
			if e != nil {
				err = e
			}
		}()
		go func() {
			defer wg.Done()
			var e error
			sq := clickhouse.SpanQuery{
				Ctx:     w.Ctx,
				TsFrom:  tsFrom,
				TsTo:    tsTo,
				DurFrom: durFrom,
				DurTo:   durTo,
				Errors:  errors,
				Limit:   limit,
			}
			spans, e = ch.GetInboundSpans(ctx, sq, maps.Keys(appClients), listens)
			if e != nil {
				err = e
			}
		}()
		wg.Wait()
		clients = getClientsByAppClients(spans, appClients)
	}

	if len(spans) == limit {
		v.Limit = limit
	}

	switch source {
	case model.TraceSourceOtel:
		v.Message = fmt.Sprintf("Using traces of <i>%s</i>", otelService)
	case model.TraceSourceAgent:
		v.Message = "Using data gathered by the eBPF tracer"
	}
	for i := range v.Sources {
		v.Sources[i].Selected = v.Sources[i].Type == source
	}

	if err != nil {
		klog.Errorln(err)
		v.Status = model.WARNING
		v.Message = fmt.Sprintf("Clickhouse error: %s", err)
		return v
	}

	if len(histogram) > 1 {
		var sli model.LatencySLI
		if len(app.LatencySLIs) > 0 {
			sli = *app.LatencySLIs[0]
		}
		events := model.EventsToAnnotations(app.Events, w.Ctx)
		v.Heatmap = model.NewHeatmap(w.Ctx, "Latency & Errors heatmap, requests per second").AddAnnotation(events...)
		for _, h := range model.HistogramSeries(histogram[1:], sli.Config.ObjectiveBucket, sli.Config.ObjectiveBucket) {
			v.Heatmap.AddSeries(h.Name, h.Title, h.Data, h.Threshold, h.Value)
		}
		v.Heatmap.AddSeries("errors", "errors", histogram[0].TimeSeries, "", "err")
	}

	v.Status = model.OK
	for _, s := range spans {
		ss := Span{
			Service:    getService(source, s, app),
			TraceId:    s.TraceId,
			Id:         s.SpanId,
			ParentId:   s.ParentSpanId,
			Name:       s.Name,
			Timestamp:  s.Timestamp.UnixMilli(),
			Duration:   s.Duration.Seconds() * 1000,
			Status:     s.Status(),
			Attributes: map[string]string{},
			Client:     clients[spanKey{traceId: s.TraceId, spanId: s.SpanId}],
			Details:    s.Details(),
		}
		for name, value := range s.ResourceAttributes {
			ss.Attributes[name] = value
		}
		for name, value := range s.SpanAttributes {
			ss.Attributes[name] = value
		}
		for _, e := range s.Events {
			ss.Events = append(ss.Events, Event{
				Timestamp:  e.Timestamp.UnixMilli(),
				Name:       e.Name,
				Attributes: e.Attributes,
			})
		}
		v.Spans = append(v.Spans, ss)
	}
	return v
}

func getService(typ model.TraceSource, s *model.TraceSpan, app *model.Application) string {
	switch typ {
	case model.TraceSourceOtel:
		return s.ServiceName
	case model.TraceSourceAgent:
		return app.Id.Name
	}
	return ""
}

type spanKey struct {
	traceId, spanId string
}

func getClientsByParentSpans(spans []*model.TraceSpan, parentSpans []*model.TraceSpan, w *model.World) map[spanKey]string {
	res := map[spanKey]string{}
	var serviceNames = map[spanKey]string{}
	for _, s := range parentSpans {
		k := spanKey{traceId: s.TraceId, spanId: s.SpanId}
		serviceNames[k] = s.ServiceName
	}
	appByPodIp := map[string]model.ApplicationId{}
	for _, a := range w.Applications {
		for _, i := range a.Instances {
			for l := range i.TcpListens {
				if l.Port == "0" {
					appByPodIp[l.IP] = a.Id
				}
			}
		}
	}
	for _, s := range spans {
		k := spanKey{traceId: s.TraceId, spanId: s.SpanId}
		res[k] = serviceNames[spanKey{traceId: s.TraceId, spanId: s.ParentSpanId}]
		addr := s.SpanAttributes["net.sock.peer.addr"]
		if res[k] == "" {
			res[k] = appByPodIp[addr].Name
		}
		if res[k] == "" {
			res[k] = addr
		}
	}
	return res
}

func getClientsByAppClients(spans []*model.TraceSpan, appClients map[string]*model.Application) map[spanKey]string {
	res := map[spanKey]string{}
	for _, s := range spans {
		k := spanKey{traceId: s.TraceId, spanId: s.SpanId}
		res[k] = appClients[s.ResourceAttributes["service.name"]].Id.Name
	}
	return res
}

func getAppClients(app *model.Application) map[string]*model.Application {
	res := map[string]*model.Application{}
	for _, d := range app.Downstreams {
		client := d.Instance.Owner
		if client == nil || client == app {
			continue
		}
		if !app.Category.Monitoring() && client.Category.Monitoring() {
			continue
		}
		for _, i := range client.Instances {
			for _, c := range i.Containers {
				res[model.ContainerIdToServiceName(c.Id)] = client
			}
		}
	}
	return res
}

func getAppListens(app *model.Application) []model.Listen {
	var res []model.Listen
	for _, i := range app.Instances {
		for l := range i.TcpListens {
			res = append(res, l)
		}
	}
	return res
}

func getMonitoringPodIps(w *model.World) []string {
	var res []string
	for _, a := range w.Applications {
		if a.Category.Monitoring() {
			for _, i := range a.Instances {
				for l := range i.TcpListens {
					if l.Port == "0" {
						res = append(res, l.IP)
					}
				}
			}
		}
	}
	return res
}
