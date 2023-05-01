package tracing

import (
	"context"
	"fmt"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
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
	Status       model.Status   `json:"status"`
	Message      string         `json:"message"`
	Sources      []Source       `json:"sources"`
	Applications []Application  `json:"applications"`
	Heatmap      *model.Heatmap `json:"heatmap"`
	Spans        []Span         `json:"spans"`
	Limit        int            `json:"limit"`
}

type Source struct {
	Type     tracing.Type `json:"type"`
	Name     string       `json:"name"`
	Selected bool         `json:"selected"`
}

type Application struct {
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
}

type Status struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
}

type Details struct {
	Text string `json:"text"`
	Lang string `json:"lang"`
}

func Render(ctx context.Context, project *db.Project, app *model.Application, appSettings *db.ApplicationSettings, q url.Values, w *model.World) *View {
	cfg := project.Settings.Integrations.Clickhouse
	if cfg == nil {
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
	errors := durFromStr == "err" || durToStr == "err"

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

	cl, err := tracing.NewClickhouseClient(cfg.Addr, cfg.Auth)
	if err != nil {
		klog.Errorln(err)
		v.Status = model.WARNING
		v.Message = fmt.Sprintf("clickhouse error: %s", err)
		return v
	}

	applications, err := cl.GetApplications(ctx, w.Ctx.From, w.Ctx.To)
	if err != nil {
		klog.Errorln(err)
		v.Status = model.WARNING
		v.Message = fmt.Sprintf("clickhouse error: %s", err)
		return v
	}
	application := app.Id.Name
	if appSettings != nil && appSettings.Tracing != nil {
		application = appSettings.Tracing.Application
	}
	var applicationFound bool
	for _, a := range applications {
		if a == application {
			applicationFound = true
		}
		v.Applications = append(v.Applications, Application{
			Name:   a,
			Linked: a == application,
		})
	}
	sort.Slice(v.Applications, func(i, j int) bool {
		return v.Applications[i].Name < v.Applications[j].Name
	})

	if applicationFound {
		v.Sources = append(v.Sources, Source{Type: tracing.TypeOtel, Name: "OpenTelemetry"})
	}
	v.Sources = append(v.Sources, Source{Type: tracing.TypeOtelEbpf, Name: "OpenTelemetry (eBPF)"})

	var spans []*tracing.Span
	if traceId != "" {
		spans, err = cl.GetSpansByTraceId(ctx, traceId)
	} else {
		switch {
		case (typ == "" || typ == tracing.TypeOtel) && applicationFound:
			typ = tracing.TypeOtel
			spans, err = cl.GetSpansByApplicationName(ctx, application, tsFrom, tsTo, durFrom, durTo, errors, limit)
		case typ == "" || typ == tracing.TypeOtelEbpf:
			typ = tracing.TypeOtelEbpf
			var listens []model.Listen
			for _, i := range app.Instances {
				for l := range i.TcpListens {
					listens = append(listens, l)
				}
			}
			spans, err = cl.GetSpansByListens(ctx, listens, tsFrom, tsTo, durFrom, durTo, errors, limit)
		}
		if len(spans) == limit {
			v.Limit = limit
		}
	}
	switch typ {
	case tracing.TypeOtel:
		v.Message = fmt.Sprintf("Using traces of <i>%s</i>", application)
	case tracing.TypeOtelEbpf:
		v.Message = "Using data gathered by the eBPF tracer"
	}
	for i := range v.Sources {
		v.Sources[i].Selected = v.Sources[i].Type == typ
	}

	if err != nil {
		klog.Errorln(err)
		v.Status = model.WARNING
		v.Message = fmt.Sprintf("clickhouse error: %s", err)
		return v
	}

	if traceId == "" && !applicationFound && len(spans) == 0 {
		v.Status = model.UNKNOWN
		v.Message = "No traces found"
		return v
	}

	clients := getClients(ctx, cl, typ, spans, w, tsFrom, tsTo)

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
	if s.Status == "STATUS_CODE_ERROR" {
		res.Error = true
		res.Message = "ERROR"
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

func getClients(ctx context.Context, cl *tracing.ClickhouseClient, typ tracing.Type, spans []*tracing.Span, w *model.World, from, to timeseries.Time) map[spanKey]string {
	res := map[spanKey]string{}
	switch typ {
	case tracing.TypeOtel:
		parentSpans, err := cl.GetParentSpans(ctx, spans, from, to)
		if err != nil {
			klog.Errorln(err)
			return nil
		}
		var serviceNames = map[spanKey]string{}
		for _, s := range parentSpans {
			k := spanKey{traceId: s.TraceId, spanId: s.SpanId}
			serviceNames[k] = s.ServiceName
		}
		for _, s := range spans {
			k := spanKey{traceId: s.TraceId, spanId: s.SpanId}
			res[k] = serviceNames[spanKey{traceId: s.TraceId, spanId: s.ParentSpanId}]
			if res[k] == "" {
				res[k] = s.Attributes["net.sock.peer.addr"]
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
			if res[k] == "" {
				res[k] = s.Attributes["net.peer.name"]
			}
		}
	}
	return res
}

func parseDuration(s string) time.Duration {
	if s == "inf" || s == "err" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		klog.Warningln(err)
		return 0
	}
	return time.Duration(v * float64(time.Second))
}