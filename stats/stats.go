package stats

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"runtime/pprof"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/coroot/coroot/auditor"
	"github.com/coroot/coroot/cache"
	cloud_pricing "github.com/coroot/coroot/cloud-pricing"
	"github.com/coroot/coroot/constructor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/grafana/pyroscope-go/godeltaprof"
	"k8s.io/klog"
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
		NodeAgentVersions         *utils.StringSet                     `json:"node_agent_versions"`
		KubeStateMetrics          *bool                                `json:"kube_state_metrics"`
		InspectionOverrides       map[model.CheckId]InspectionOverride `json:"inspection_overrides"`
		ApplicationCategories     int                                  `json:"application_categories"`
		AlertingIntegrations      *utils.StringSet                     `json:"alerting_integrations"`
		CloudCosts                bool                                 `json:"cloud_costs"`
		Clickhouse                bool                                 `json:"clickhouse"`
		Tracing                   bool                                 `json:"tracing"`
		Logs                      bool                                 `json:"logs"`
		Profiles                  bool                                 `json:"profiles"`
	} `json:"integration"`
	Stack struct {
		Clouds               *utils.StringSet `json:"clouds"`
		Services             *utils.StringSet `json:"services"`
		InstrumentedServices *utils.StringSet `json:"instrumented_services"`
	} `json:"stack"`
	Infra struct {
		Projects            int              `json:"projects"`
		Nodes               int              `json:"nodes"`
		CPUCores            int              `json:"cpu_cores"`
		Applications        int              `json:"applications"`
		Instances           int              `json:"instances"`
		Deployments         int              `json:"deployments"`
		DeploymentSummaries map[string]int   `json:"deployment_summaries"`
		KernelVersions      *utils.StringSet `json:"kernel_versions"`
	} `json:"infra"`
	UX struct {
		WorldLoadTimeAvg  float32                    `json:"world_load_time_avg"`
		AuditTimeAvg      float32                    `json:"audit_time_avg"`
		UsersByScreenSize map[string]int             `json:"users_by_screen_size"`
		UsersByTheme      map[string]int             `json:"users_by_theme"`
		Users             *utils.StringSet           `json:"users"`
		UsersByRole       map[string]int             `json:"users_by_role"`
		PageViews         map[string]int             `json:"page_views"`
		SentNotifications map[db.IntegrationType]int `json:"sent_notifications"`
	} `json:"ux"`
	Performance struct {
		Constructor    constructor.Profile                      `json:"constructor"`
		ResourcesUsage map[model.ApplicationId][]ResourcesUsage `json:"resources_usage"`
	} `json:"performance"`
	Profile struct {
		From   int64  `json:"from"`
		To     int64  `json:"to"`
		CPU    string `json:"cpu"`
		Memory string `json:"memory"`
	} `json:"profile"`
}

type ResourcesUsage struct {
	Cpu    float32 `json:"cpu"`
	Memory float32 `json:"memory"`
}

type InspectionOverride struct {
	ProjectLevel     int `json:"project_level"`
	ApplicationLevel int `json:"application_level"`
}

type Collector struct {
	db      *db.DB
	cache   *cache.Cache
	pricing *cloud_pricing.Manager
	client  *http.Client

	instanceUuid    string
	instanceVersion string

	usersByScreenSize map[string]*utils.StringSet
	usersByTheme      map[string]*utils.StringSet
	pageViews         map[string]int
	lock              sync.Mutex

	heapProfiler *godeltaprof.HeapProfiler

	globalClickHouse *db.IntegrationClickhouse
}

func NewCollector(instanceUuid, version string, db *db.DB, cache *cache.Cache, pricing *cloud_pricing.Manager, globalClickHouse *db.IntegrationClickhouse) *Collector {
	c := &Collector{
		db:      db,
		cache:   cache,
		pricing: pricing,

		client: &http.Client{Timeout: sendTimeout},

		instanceUuid:    instanceUuid,
		instanceVersion: version,

		usersByScreenSize: map[string]*utils.StringSet{},
		usersByTheme:      map[string]*utils.StringSet{},
		pageViews:         map[string]int{},

		heapProfiler: godeltaprof.NewHeapProfiler(),

		globalClickHouse: globalClickHouse,
	}

	if err := c.heapProfiler.Profile(io.Discard); err != nil {
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
	Theme      string `json:"theme"`
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
	if c.usersByTheme[e.Theme] == nil {
		c.usersByTheme[e.Theme] = utils.NewStringSet()
	}
	c.usersByTheme[e.Theme].Add(e.DeviceId)
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

	stats.UX.UsersByTheme = map[string]int{}
	for theme, us := range c.usersByTheme {
		stats.UX.UsersByTheme[theme] = us.Len()
	}
	c.usersByTheme = map[string]*utils.StringSet{}

	c.lock.Unlock()

	users, err := c.db.GetUsers()
	if err != nil {
		klog.Errorln("failed to get users:", err)
	}
	stats.UX.UsersByRole = map[string]int{}
	for _, u := range users {
		for _, r := range u.Roles {
			stats.UX.UsersByRole[string(r)]++
		}
	}

	projects, err := c.db.GetProjects()
	if err != nil {
		klog.Errorln("failed to get projects:", err)
		return stats
	}
	stats.Infra.Projects = len(projects)

	applicationCategories := utils.NewStringSet()
	stats.Integration.NodeAgentVersions = utils.NewStringSet()
	stats.Infra.KernelVersions = utils.NewStringSet()
	stats.Integration.InspectionOverrides = map[model.CheckId]InspectionOverride{}
	stats.Integration.AlertingIntegrations = utils.NewStringSet()
	stats.Stack.Clouds = utils.NewStringSet()
	stats.Stack.Services = utils.NewStringSet()
	stats.Stack.InstrumentedServices = utils.NewStringSet()
	stats.Performance.Constructor.Stages = map[string]float32{}
	stats.Performance.Constructor.Queries = map[string]constructor.QueryStats{}
	stats.Performance.ResourcesUsage = map[model.ApplicationId][]ResourcesUsage{}
	stats.Infra.DeploymentSummaries = map[string]int{}
	var loadTime, auditTime []time.Duration
	now := timeseries.Now()
	for _, p := range projects {
		if p.Prometheus.Url != "" {
			stats.Integration.Prometheus = true
		}
		if stats.Integration.PrometheusRefreshInterval == 0 || int(p.Prometheus.RefreshInterval) < stats.Integration.PrometheusRefreshInterval {
			stats.Integration.PrometheusRefreshInterval = int(p.Prometheus.RefreshInterval)
		}
		if cfg := p.ClickHouseConfig(c.globalClickHouse); cfg != nil && cfg.Addr != "" {
			stats.Integration.Clickhouse = true
			stats.Integration.Tracing = true
			stats.Integration.Logs = true
			stats.Integration.Profiles = true
		}

		for category := range p.Settings.ApplicationCategories {
			applicationCategories.Add(string(category))
		}

		for _, i := range p.Settings.Integrations.GetInfo() {
			if i.Configured {
				stats.Integration.AlertingIntegrations.Add(string(i.Type))
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

		cacheClient := c.cache.GetCacheClient(p.Id)
		cacheTo, err := cacheClient.GetTo()
		if err != nil {
			klog.Errorln(err)
			continue
		}
		if cacheTo.IsZero() || cacheTo.Before(now.Add(-worldWindow)) {
			continue
		}
		t := time.Now()
		to := cacheTo
		from := to.Add(-worldWindow)
		step, err := cacheClient.GetStep(from, to)
		if err != nil {
			klog.Errorln(err)
			continue
		}
		ctr := constructor.New(c.db, p, cacheClient, c.pricing)
		w, err := ctr.LoadWorld(context.Background(), from, to, step, &stats.Performance.Constructor)
		if err != nil {
			klog.Errorln("failed to load world:", err)
			continue
		}
		loadTime = append(loadTime, time.Since(t))

		t = time.Now()
		auditor.Audit(w, p, nil, p.ClickHouseConfig(c.globalClickHouse) != nil)
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
			if cores := n.CpuCapacity.Last(); cores > 0 {
				stats.Infra.CPUCores += int(cores)
			}
			stats.Integration.NodeAgentVersions.Add(n.AgentVersion.Value())
			stats.Infra.KernelVersions.Add(n.KernelVersion.Value())
			stats.Stack.Clouds.Add(strings.ToLower(n.CloudProvider.Value()))
			if n.Price != nil {
				stats.Integration.CloudCosts = true
			}
		}

		corootComponents := map[model.ApplicationId]*model.Application{}
		for _, a := range w.Applications {
			switch {
			case strings.HasSuffix(a.Id.Name, "node-agent"):
			case strings.HasSuffix(a.Id.Name, "cluster-agent"):
			case strings.HasSuffix(a.Id.Name, "operator") && strings.Contains(a.Id.Name, "coroot"):
			case strings.HasSuffix(a.Id.Name, "coroot"):
				for _, i := range a.Instances {
					for _, u := range i.Upstreams { // prometheus and clickhouse
						if u.RemoteInstance != nil {
							corootComponents[u.RemoteInstance.Owner.Id] = u.RemoteInstance.Owner
						}
					}
				}
			default:
				continue
			}
			corootComponents[a.Id] = a

			if a.IsStandalone() || a.Category.Auxiliary() {
				continue
			}
			stats.Infra.Applications++
			stats.Infra.Instances += len(a.Instances)
			for _, i := range a.Instances {
				for t := range i.ApplicationTypes() {
					stats.Stack.Services.Add(string(t))
				}
				stats.Stack.InstrumentedServices.Add(string(i.InstrumentedType()))
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
		for _, a := range corootComponents {
			if usage := lastUsage(a); len(usage) > 0 {
				stats.Performance.ResourcesUsage[a.Id] = append(stats.Performance.ResourcesUsage[a.Id], usage...)
			}
		}
	}
	stats.Integration.ApplicationCategories = applicationCategories.Len()

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

func lastUsage(app *model.Application) []ResourcesUsage {
	var res []ResourcesUsage
	for _, i := range app.Instances {
		if len(i.Containers) == 0 {
			continue
		}
		var cpuSum, memSum float32
		for _, c := range i.Containers {
			if v := c.CpuUsage.Last(); !timeseries.IsNaN(v) {
				cpuSum += v
			}
			if v := c.MemoryRss.Last(); !timeseries.IsNaN(v) {
				memSum += v
			}
		}
		if cpuSum > 0 {
			res = append(res, ResourcesUsage{Cpu: cpuSum, Memory: memSum / 1024 / 1024})
		}
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].Cpu > res[j].Cpu
	})
	return res
}
