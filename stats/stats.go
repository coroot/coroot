package stats

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/coroot/coroot/auditor"
	"github.com/coroot/coroot/cache"
	"github.com/coroot/coroot/constructor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/pyroscope-io/godeltaprof"
	"io/ioutil"
	"k8s.io/klog"
	"net/http"
	"runtime/pprof"
	"strings"
	"sync"
	"time"
)

const (
	collectUrl      = "https://coroot.com/ce/usage-statistics"
	collectInterval = time.Hour
	sendTimeout     = time.Minute
	worldWindow     = timeseries.Hour
)

type Stats struct {
	Instance struct {
		Uuid         string `json:"uuid"`
		Version      string `json:"version"`
		DatabaseType string `json:"database_type"`
	} `json:"instance"`
	Integration struct {
		Prometheus                bool                                 `json:"prometheus"`
		PrometheusRefreshInterval int                                  `json:"prometheus_refresh_interval"`
		NodeAgent                 bool                                 `json:"node_agent"`
		KubeStateMetrics          *bool                                `json:"kube_state_metrics"`
		InspectionOverrides       map[model.CheckId]InspectionOverride `json:"inspection_overrides"`
		ApplicationCategories     int                                  `json:"application_categories"`
		AlertingIntegrations      []string                             `json:"alerting_integrations"`
		Pyroscope                 bool                                 `json:"pyroscope"`
	} `json:"integration"`
	Stack struct {
		Clouds               []string `json:"clouds"`
		Services             []string `json:"services"`
		InstrumentedServices []string `json:"instrumented_services"`
	} `json:"stack"`
	Infra struct {
		Projects            int            `json:"projects"`
		Nodes               int            `json:"nodes"`
		Applications        int            `json:"applications"`
		Instances           int            `json:"instances"`
		Deployments         int            `json:"deployments"`
		DeploymentSummaries map[string]int `json:"deployment_summaries"`
	} `json:"infra"`
	UX struct {
		WorldLoadTimeAvg  float32                    `json:"world_load_time_avg"`
		AuditTimeAvg      float32                    `json:"audit_time_avg"`
		UsersByScreenSize map[string]int             `json:"users_by_screen_size"`
		Users             *utils.StringSet           `json:"users"`
		PageViews         map[string]int             `json:"page_views"`
		SentNotifications map[db.IntegrationType]int `json:"sent_notifications"`
	} `json:"ux"`
	Performance struct {
		Constructor constructor.Profile `json:"constructor"`
	} `json:"performance"`
	Profile struct {
		From   int64  `json:"from"`
		To     int64  `json:"to"`
		CPU    string `json:"cpu"`
		Memory string `json:"memory"`
	} `json:"profile"`
}

type InspectionOverride struct {
	ProjectLevel     int `json:"project_level"`
	ApplicationLevel int `json:"application_level"`
}

type Collector struct {
	db     *db.DB
	cache  *cache.Cache
	client *http.Client

	instanceUuid    string
	instanceVersion string

	usersByScreenSize map[string]*utils.StringSet
	pageViews         map[string]int
	lock              sync.Mutex

	heapProfiler *godeltaprof.HeapProfiler
}

func NewCollector(instanceUuid, version string, db *db.DB, cache *cache.Cache) *Collector {
	c := &Collector{
		db:    db,
		cache: cache,

		client: &http.Client{Timeout: sendTimeout},

		instanceUuid:    instanceUuid,
		instanceVersion: version,

		usersByScreenSize: map[string]*utils.StringSet{},
		pageViews:         map[string]int{},

		heapProfiler: godeltaprof.NewHeapProfiler(),
	}

	if err := c.heapProfiler.Profile(ioutil.Discard); err != nil {
		klog.Warningln(err)
	}

	go func() {
		c.send()
		ticker := time.NewTicker(collectInterval)
		for range ticker.C {
			c.send()
		}
	}()

	return c
}

type Event struct {
	Type       string `json:"type"`
	DeviceId   string `json:"device_id"`
	DeviceSize string `json:"device_size"`
	Path       string `json:"path"`
}

func (c *Collector) RegisterRequest(r *http.Request) {
	if c == nil {
		return
	}
	var e Event
	if err := utils.ReadJson(r, &e); err != nil {
		klog.Warningln(err)
		return
	}
	if e.DeviceId == "" || e.DeviceSize == "" {
		return
	}

	c.lock.Lock()
	defer c.lock.Unlock()
	if e.Type == "route-open" {
		c.pageViews[e.Path]++
	}
	if c.usersByScreenSize[e.DeviceSize] == nil {
		c.usersByScreenSize[e.DeviceSize] = utils.NewStringSet()
	}
	c.usersByScreenSize[e.DeviceSize].Add(e.DeviceId)
}

func (c *Collector) send() {
	buf := new(bytes.Buffer)
	if err := pprof.StartCPUProfile(buf); err != nil {
		klog.Warningln(err)
	}
	from := time.Now()

	stats := c.collect()

	stats.Profile.From = from.Unix()
	stats.Profile.To = time.Now().Unix()
	pprof.StopCPUProfile()
	stats.Profile.CPU = base64.StdEncoding.EncodeToString(buf.Bytes())
	buf.Reset()
	if err := c.heapProfiler.Profile(buf); err != nil {
		klog.Warningln(err)
	}

	stats.Profile.Memory = base64.StdEncoding.EncodeToString(buf.Bytes())

	buf.Reset()
	if err := json.NewEncoder(buf).Encode(stats); err != nil {
		klog.Errorln("failed to encode stats:", err)
		return
	}
	res, err := c.client.Post(collectUrl, "application/json", buf)
	if err != nil {
		klog.Errorln("failed to send stats:", err)
		return
	}
	_ = res.Body.Close()
}

func (c *Collector) collect() Stats {
	stats := Stats{}

	stats.Instance.Uuid = c.instanceUuid
	stats.Instance.Version = c.instanceVersion
	stats.Instance.DatabaseType = string(c.db.Type())

	stats.UX.UsersByScreenSize = map[string]int{}
	stats.UX.Users = utils.NewStringSet()
	c.lock.Lock()
	stats.UX.PageViews = c.pageViews
	c.pageViews = map[string]int{}
	for size, us := range c.usersByScreenSize {
		stats.UX.UsersByScreenSize[size] = us.Len()
		stats.UX.Users.Add(us.Items()...)
	}
	c.usersByScreenSize = map[string]*utils.StringSet{}
	c.lock.Unlock()

	projects, err := c.db.GetProjects()
	if err != nil {
		klog.Errorln("failed to get projects:", err)
		return stats
	}
	stats.Infra.Projects = len(projects)

	clouds := utils.NewStringSet()
	services := utils.NewStringSet()
	servicesInstrumented := utils.NewStringSet()
	applicationCategories := utils.NewStringSet()
	stats.Integration.InspectionOverrides = map[model.CheckId]InspectionOverride{}
	stats.Performance.Constructor.Stages = map[string]float32{}
	stats.Performance.Constructor.Queries = map[string]prom.QueryStats{}
	stats.Infra.DeploymentSummaries = map[string]int{}
	alertingIntegrations := utils.NewStringSet()
	var loadTime, auditTime []time.Duration
	now := timeseries.Now()
	for _, p := range projects {
		if p.Prometheus.Url != "" {
			stats.Integration.Prometheus = true
		}
		if stats.Integration.PrometheusRefreshInterval == 0 || int(p.Prometheus.RefreshInterval) < stats.Integration.PrometheusRefreshInterval {
			stats.Integration.PrometheusRefreshInterval = int(p.Prometheus.RefreshInterval)
		}
		if cfg := p.Settings.Integrations.Pyroscope; cfg != nil && cfg.Url != "" {
			stats.Integration.Pyroscope = true
		}

		for category := range p.Settings.ApplicationCategories {
			applicationCategories.Add(string(category))
		}

		for _, i := range p.Settings.Integrations.GetInfo() {
			if i.Configured {
				alertingIntegrations.Add(string(i.Type))
			}
		}

		checkConfigs, err := c.db.GetCheckConfigs(p.Id)
		if err != nil {
			klog.Errorln(err)
			continue
		}
		for appId, configs := range checkConfigs {
			for checkId := range configs {
				s := stats.Integration.InspectionOverrides[checkId]
				if appId.IsZero() {
					s.ProjectLevel++
				} else {
					s.ApplicationLevel++
				}
				stats.Integration.InspectionOverrides[checkId] = s
			}
		}

		cc := c.cache.GetCacheClient(p)
		cacheTo, err := cc.GetTo()
		if err != nil {
			klog.Errorln(err)
			continue
		}
		if cacheTo.IsZero() || cacheTo.Before(now.Add(-worldWindow)) {
			continue
		}
		t := time.Now()
		step := p.Prometheus.RefreshInterval
		cr := constructor.New(c.db, p, cc, constructor.OptionLoadPerConnectionHistograms)
		w, err := cr.LoadWorld(context.Background(), cacheTo.Add(-worldWindow), cacheTo, step, &stats.Performance.Constructor)
		if err != nil {
			klog.Errorln("failed to load world:", err)
			continue
		}
		loadTime = append(loadTime, time.Since(t))

		t = time.Now()
		auditor.Audit(w, p)
		auditTime = append(auditTime, time.Since(t))

		stats.Integration.NodeAgent = stats.Integration.NodeAgent || w.IntegrationStatus.NodeAgent.Installed
		if w.IntegrationStatus.KubeStateMetrics.Required {
			installed := w.IntegrationStatus.KubeStateMetrics.Installed
			if stats.Integration.KubeStateMetrics != nil {
				installed = *stats.Integration.KubeStateMetrics || installed
			}
			stats.Integration.KubeStateMetrics = &installed
		}

		stats.Infra.Nodes += len(w.Nodes)
		for _, n := range w.Nodes {
			clouds.Add(strings.ToLower(n.CloudProvider.Value()))
		}

		for _, a := range w.Applications {
			if a.IsStandalone() || a.Category.Auxiliary() {
				continue
			}
			stats.Infra.Applications++
			stats.Infra.Instances += len(a.Instances)
			for _, i := range a.Instances {
				for t := range i.ApplicationTypes() {
					services.Add(string(t))
				}
				servicesInstrumented.Add(string(i.InstrumentedType()))
			}
			for _, ds := range model.CalcApplicationDeploymentStatuses(a, w.CheckConfigs, now) {
				if now.Sub(ds.Deployment.StartedAt) > timeseries.Hour {
					continue
				}
				stats.Infra.Deployments++
				for _, s := range ds.Summary {
					sign := "-"
					if s.Ok {
						sign = "+"
					}
					stats.Infra.DeploymentSummaries[sign+string(s.Report)]++
				}
			}
		}
	}
	stats.Integration.ApplicationCategories = applicationCategories.Len()
	stats.Integration.AlertingIntegrations = alertingIntegrations.Items()
	stats.Stack.Clouds = clouds.Items()
	stats.Stack.Services = services.Items()
	stats.Stack.InstrumentedServices = servicesInstrumented.Items()

	stats.UX.WorldLoadTimeAvg = avgDuration(loadTime)
	stats.UX.AuditTimeAvg = avgDuration(auditTime)

	stats.UX.SentNotifications = c.db.GetSentIncidentNotificationsStat(now.Add(-timeseries.Duration(collectInterval.Seconds())))

	return stats
}

func avgDuration(durations []time.Duration) float32 {
	if len(durations) == 0 {
		return 0
	}
	var total time.Duration
	for _, d := range durations {
		total += d
	}
	return float32(total.Seconds() / float64(len(durations)))
}
