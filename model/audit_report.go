package model

import (
	"strings"

	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

type AuditReportName string

const (
	AuditReportSLO         AuditReportName = "SLO"
	AuditReportInstances   AuditReportName = "Instances"
	AuditReportCPU         AuditReportName = "CPU"
	AuditReportMemory      AuditReportName = "Memory"
	AuditReportStorage     AuditReportName = "Storage"
	AuditReportNetwork     AuditReportName = "Net"
	AuditReportDNS         AuditReportName = "DNS"
	AuditReportLogs        AuditReportName = "Logs"
	AuditReportPostgres    AuditReportName = "Postgres"
	AuditReportRedis       AuditReportName = "Redis"
	AuditReportMongodb     AuditReportName = "Mongodb"
	AuditReportMemcached   AuditReportName = "Memcached"
	AuditReportMysql       AuditReportName = "Mysql"
	AuditReportJvm         AuditReportName = "JVM"
	AuditReportDotNet      AuditReportName = ".NET"
	AuditReportPython      AuditReportName = "Python"
	AuditReportNode        AuditReportName = "Node"
	AuditReportDeployments AuditReportName = "Deployments"
	AuditReportProfiling   AuditReportName = "Profiling"
	AuditReportTracing     AuditReportName = "Tracing"
)

type ConfigurationHint struct {
	Message      string `json:"message"`
	ReadMoreLink string `json:"read_more_link"`
}

type AuditReport struct {
	app          *Application
	ctx          timeseries.Context
	checkConfigs CheckConfigs
	detailed     bool

	Name            AuditReportName `json:"name"`
	Status          Status          `json:"status"`
	Widgets         []*Widget       `json:"widgets"`
	Checks          []*Check        `json:"checks"`
	Custom          bool            `json:"custom"`
	Instrumentation ApplicationType `json:"instrumentation"`
}

func NewAuditReport(app *Application, ctx timeseries.Context, checkConfigs CheckConfigs, name AuditReportName, detailed bool) *AuditReport {
	return &AuditReport{app: app, Name: name, ctx: ctx, checkConfigs: checkConfigs, detailed: detailed}
}

func (r *AuditReport) AddWidget(w *Widget) *Widget {
	r.Widgets = append(r.Widgets, w)
	return w
}

func (r *AuditReport) GetOrCreateChartGroup(title string, doc *DocLink) *ChartGroup {
	if !r.detailed {
		return nil
	}
	for _, w := range r.Widgets {
		if cg := w.ChartGroup; cg != nil {
			if cg.Title == title {
				return cg
			}
		}
	}
	cg := NewChartGroup(r.ctx, title)
	r.Widgets = append(r.Widgets, &Widget{ChartGroup: cg, DocLink: doc})
	return cg
}

func (r *AuditReport) GetOrCreateChartInGroup(title string, chartTitle string, doc *DocLink) *Chart {
	if !r.detailed {
		return nil
	}
	return r.GetOrCreateChartGroup(title, doc).GetOrCreateChart(chartTitle)
}

func (r *AuditReport) GetOrCreateChart(title string, doc *DocLink) *Chart {
	if !r.detailed {
		return nil
	}
	for _, w := range r.Widgets {
		if ch := w.Chart; ch != nil {
			if ch.Title == title {
				return ch
			}
		}
	}
	ch := NewChart(r.ctx, title)
	r.Widgets = append(r.Widgets, &Widget{Chart: ch, DocLink: doc})
	return ch
}

func (r *AuditReport) GetOrCreateHeatmap(title string) *Heatmap {
	if !r.detailed {
		return nil
	}
	for _, w := range r.Widgets {
		if h := w.Heatmap; h != nil {
			if h.Title == title {
				return h
			}
		}
	}
	h := NewHeatmap(r.ctx, title)
	r.Widgets = append(r.Widgets, &Widget{Heatmap: h, Width: "100%"})
	return h
}

func (r *AuditReport) GetOrCreateDependencyMap() *DependencyMap {
	if !r.detailed {
		return nil
	}
	for _, w := range r.Widgets {
		if w.DependencyMap != nil {
			return w.DependencyMap
		}
	}
	dm := &DependencyMap{}
	r.Widgets = append(r.Widgets, &Widget{DependencyMap: dm, Width: "100%"})
	return dm
}

func (r *AuditReport) GetOrCreateTable(header ...string) *Table {
	if !r.detailed {
		return nil
	}
	for _, w := range r.Widgets {
		if t := w.Table; t != nil {
			return t
		}
	}
	t := NewTable(header...)
	r.Widgets = append(r.Widgets, &Widget{Table: t, Width: "100%"})
	return t
}

func (r *AuditReport) CreateCheck(cfg CheckConfig) *Check {
	ch := &Check{
		Id:                      cfg.Id,
		Title:                   cfg.Title,
		Status:                  OK,
		Unit:                    cfg.Unit,
		ConditionFormatTemplate: cfg.ConditionFormatTemplate,

		typ:             cfg.Type,
		messageTemplate: cfg.MessageTemplate,
		items:           utils.NewStringSet(),
	}
	switch cfg.Id {
	case Checks.SLOAvailability.Id:
		availabilityCfg, _ := r.checkConfigs.GetAvailability(r.app.Id)
		ch.Threshold = availabilityCfg.ObjectivePercentage
	case Checks.SLOLatency.Id:
		latencyCfg, _ := r.checkConfigs.GetLatency(r.app.Id, r.app.Category)
		ch.Threshold = latencyCfg.ObjectivePercentage
		ch.ConditionFormatTemplate = strings.Replace(
			ch.ConditionFormatTemplate,
			"<bucket>",
			utils.FormatLatency(latencyCfg.ObjectiveBucket),
			1,
		)
	default:
		ch.Threshold = r.checkConfigs.GetSimple(cfg.Id, r.app.Id).Threshold
	}
	r.Checks = append(r.Checks, ch)
	return ch
}
