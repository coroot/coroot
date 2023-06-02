package trace

import (
	"context"
	"fmt"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/tracing"
	"github.com/coroot/coroot/utils"
	"k8s.io/klog"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
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
	Type     tracing.Type `json:"type"`
	Name     string       `json:"name"`
	Selected bool         `json:"selected"`
}

type Service struct {
	Name   string `json:"name"`
	Linked bool   `json:"linked"`
}

type Span struct {
	Service    string            `json:"service"`
	TraceId    string            `json:"trace_id"`
	Id         string            `json:"id"`
	ParentId   string            `json:"parent_id"`
	Name       string            `json:"name"`
	Timestamp  int64             `json:"timestamp"`
	Duration   float64           `json:"duration"`
	Client     string            `json:"client"`
	Status     Status            `json:"status"`
	Details    Details           `json:"details"`
	Attributes map[string]string `json:"attributes"`
	Events     []Event           `json:"events"`
}

type Status struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
}

type Details struct {
	Text string `json:"text"`
	Lang string `json:"lang"`
}

type Event struct {
	Name       string            `json:"name"`
	Timestamp  int64             `json:"timestamp"`
	Attributes map[string]string `json:"attributes"`
}

func Render(ctx context.Context, clickhouse *tracing.ClickhouseClient, app *model.Application, appSettings *db.ApplicationSettings, q url.Values, w *model.World) *View {
	if clickhouse == nil {
		return nil
	}

	parts := strings.Split(q.Get("trace")+"::::", ":")
	typ, traceId, tsRange, durRange := tracing.Type(parts[0]), parts[1], parts[2], parts[3]
	parts = strings.Split(tsRange+"-", "-")
	tsFrom := utils.ParseTime(w.Ctx.To, parts[0], w.Ctx.From)
	tsTo := utils.ParseTime(w.Ctx.To, parts[1], w.Ctx.To)
	parts = strings.Split(durRange+"-", "-")
	durFromStr, durToStr := parts[0], parts[1]
	durFrom := parseDuration(durFromStr)
	durTo := parseDuration(durToStr)
	errors := durFromStr == "inf" || durToStr == "err"

	v := &View{}

	if len(app.LatencySLIs) > 0 {
		sli := app.LatencySLIs[0]
		if len(sli.Histogram) > 0 {
			events := model.EventsToAnnotations(app.Events, w.Ctx)
			incidents := model.IncidentsToAnnotations(app.Incidents, w.Ctx)
			v.Heatmap = model.NewHeatmap(w.Ctx, "Latency & Errors heatmap, requests per second").AddAnnotation(events...).AddAnnotation(incidents...)
			for _, h := range model.HistogramSeries(sli.Histogram, sli.Config.ObjectiveBucket, sli.Config.ObjectivePercentage) {
				v.Heatmap.AddSeries(h.Name, h.Title, h.Data, h.Threshold, h.Value)
			}
		}
	}
	if len(app.AvailabilitySLIs) > 0 && v.Heatmap != nil {
		sli := app.AvailabilitySLIs[0]
		failed := sli.FailedRequests
		if failed.IsEmpty() {
			failed = sli.TotalRequests.WithNewValue(0)
		}
		v.Heatmap.AddSeries("errors", "errors", failed, "", "err")
	}

	services, err := clickhouse.GetServiceNames(ctx)
	if err != nil {
		klog.Errorln(err)
		v.Status = model.WARNING
		v.Message = fmt.Sprintf("Clickhouse error: %s", err)
		return v
	}
	service := app.Id.Name
	if appSettings != nil && appSettings.Tracing != nil {
		service = appSettings.Tracing.Service
	}
	var serviceFound, ebpfSpansFound bool
	for _, s := range services {
		if s == service {
			serviceFound = true
		}
		if s == "coroot-node-agent" {
			ebpfSpansFound = true
			continue
		}
		v.Services = append(v.Services, Service{
			Name:   s,
			Linked: s == service,
		})
	}
	sort.Slice(v.Services, func(i, j int) bool {
		return v.Services[i].Name < v.Services[j].Name
	})

	if serviceFound {
		v.Sources = append(v.Sources, Source{Type: tracing.TypeOtel, Name: "OpenTelemetry"})
	}
	if ebpfSpansFound {
		v.Sources = append(v.Sources, Source{Type: tracing.TypeOtelEbpf, Name: "OpenTelemetry (eBPF)"})
	}

	if !serviceFound && !ebpfSpansFound {
		v.Status = model.UNKNOWN
		v.Message = "No traces found"
		return v
	}

	var spans []*tracing.Span
	if traceId != "" {
		spans, err = clickhouse.GetSpansByTraceId(ctx, traceId)
	} else {
		switch {

		case (typ == "" || typ == tracing.TypeOtel) && serviceFound:
			typ = tracing.TypeOtel
			var monitoringPodIps []string
			for _, a := range w.Applications {
				if a.Category.Monitoring() {
					for _, i := range a.Instances {
						for l := range i.TcpListens {
							if l.Port == "0" {
								monitoringPodIps = append(monitoringPodIps, l.IP)
							}
						}
					}
				}
			}
			spans, err = clickhouse.GetSpansByServiceName(ctx, service, monitoringPodIps, tsFrom, tsTo, durFrom, durTo, errors, limit)

		case (typ == "" || typ == tracing.TypeOtelEbpf) && ebpfSpansFound:
			typ = tracing.TypeOtelEbpf
			var listens []model.Listen
			for _, i := range app.Instances {
				for l := range i.TcpListens {
					listens = append(listens, l)
				}
			}
			var monitoringContainerIds []string
			for _, a := range w.Applications {
				if a.Category.Monitoring() {
					for _, i := range a.Instances {
						for _, c := range i.Containers {
							monitoringContainerIds = append(monitoringContainerIds, c.Id)
						}
					}
				}
			}
			spans, err = clickhouse.GetInboundSpans(ctx, listens, monitoringContainerIds, tsFrom, tsTo, durFrom, durTo, errors, limit)
		}

		if len(spans) == limit {
			v.Limit = limit
		}
	}
	switch typ {
	case tracing.TypeOtel:
		v.Message = fmt.Sprintf("Using traces of <i>%s</i>", service)
	case tracing.TypeOtelEbpf:
		v.Message = "Using data gathered by the eBPF tracer"
	}
	for i := range v.Sources {
		v.Sources[i].Selected = v.Sources[i].Type == typ
	}

	if err != nil {
		klog.Errorln(err)
		v.Status = model.WARNING
		v.Message = fmt.Sprintf("Clickhouse error: %s", err)
		return v
	}

	clients := map[spanKey]string{}
	if traceId == "" {
		clients = getClients(ctx, clickhouse, typ, spans, w)
	}

	v.Status = model.OK
	for _, s := range spans {
		ss := Span{
			Service:    getService(typ, s, app),
			TraceId:    s.TraceId,
			Id:         s.SpanId,
			ParentId:   s.ParentSpanId,
			Name:       s.Name,
			Timestamp:  s.Timestamp.UnixMilli(),
			Duration:   s.Duration.Seconds() * 1000,
			Status:     getStatus(s),
			Attributes: s.Attributes,
			Client:     clients[spanKey{traceId: s.TraceId, spanId: s.SpanId}],
			Details:    getDetails(s),
		}
		for _, e := range s.Events {
			ee := Event{
				Name:       e.Name,
				Timestamp:  e.Timestamp.UnixMilli(),
				Attributes: e.Attributes,
			}
			ss.Events = append(ss.Events, ee)
		}
		v.Spans = append(v.Spans, ss)
	}
	return v
}

func getService(typ tracing.Type, s *tracing.Span, app *model.Application) string {
	switch typ {
	case tracing.TypeOtel:
		return s.ServiceName
	case tracing.TypeOtelEbpf:
		return app.Id.Name
	}
	return ""
}

func getStatus(s *tracing.Span) Status {
	res := Status{Message: "OK"}
	if s.StatusCode == "STATUS_CODE_ERROR" {
		res.Error = true
		res.Message = "ERROR"
		if s.StatusMessage != "" {
			res.Message = s.StatusMessage
		}
	}
	if c := s.Attributes["http.status_code"]; c != "" {
		res.Message = "HTTP-" + c
	}
	return res
}

func getDetails(s *tracing.Span) Details {
	var res Details
	switch {
	case s.Attributes["http.url"] != "":
		res.Text = s.Attributes["http.url"]
	case s.Attributes["db.system"] == "mongodb":
		res.Text = s.Attributes["db.statement"]
		res.Lang = "json"
	case s.Attributes["db.system"] == "redis":
		res.Text = s.Attributes["db.statement"]
	case s.Attributes["db.statement"] != "":
		res.Text = s.Attributes["db.statement"]
		res.Lang = "sql"
	case s.Attributes["db.memcached.item"] != "":
		res.Text = fmt.Sprintf(`%s "%s"`, s.Attributes["db.operation"], s.Attributes["db.memcached.item"])
		res.Lang = "bash"
	}
	return res
}

type spanKey struct {
	traceId, spanId string
}

func getClients(ctx context.Context, cl *tracing.ClickhouseClient, typ tracing.Type, spans []*tracing.Span, w *model.World) map[spanKey]string {
	res := map[spanKey]string{}
	switch typ {
	case tracing.TypeOtel:
		parentSpans, err := cl.GetParentSpans(ctx, spans)
		if err != nil {
			klog.Errorln(err)
			return nil
		}
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
			addr := s.Attributes["net.sock.peer.addr"]
			if res[k] == "" {
				res[k] = appByPodIp[addr].Name
			}
			if res[k] == "" {
				res[k] = addr
			}
		}
	case tracing.TypeOtelEbpf:
		appByContainerId := map[string]string{}
		for _, app := range w.Applications {
			for _, i := range app.Instances {
				for _, c := range i.Containers {
					appByContainerId[c.Id] = app.Id.Name
				}
			}
		}
		for _, s := range spans {
			k := spanKey{traceId: s.TraceId, spanId: s.SpanId}
			res[k] = appByContainerId[s.Attributes["container.id"]]
		}
	}
	return res
}

func parseDuration(s string) time.Duration {
	if s == "" || s == "inf" || s == "err" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		klog.Warningln(err)
		return 0
	}
	return time.Duration(v * float64(time.Second))
}
