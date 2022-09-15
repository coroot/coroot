package stats

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/coroot/coroot/cache"
	"github.com/coroot/coroot/constructor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/google/uuid"
	"k8s.io/klog"
	"net/http"
	"os"
	"path"
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
		Prometheus       bool  `json:"prometheus"`
		NodeAgent        bool  `json:"node_agent"`
		KubeStateMetrics *bool `json:"kube_state_metrics"`
	} `json:"integration"`
	Stack struct {
		Clouds               []string `json:"clouds"`
		Services             []string `json:"services"`
		InstrumentedServices []string `json:"instrumented_services"`
	} `json:"stack"`
	Infra struct {
		Projects     int `json:"projects"`
		Nodes        int `json:"nodes"`
		Applications int `json:"applications"`
		Instances    int `json:"instances"`
	} `json:"infra"`
	UX struct {
		UsersByScreenSize map[string]int `json:"users_by_screen_size"`
		WorldLoadTimeAvg  float64        `json:"world_load_time_avg"`
	} `json:"ux"`
}

type Collector struct {
	db     *db.DB
	cache  *cache.Cache
	client *http.Client

	instanceUuid    string
	instanceVersion string

	usersByScreenSize map[string]*utils.StringSet
	lock              sync.Mutex
}

func NewCollector(dataDir, version string, db *db.DB, cache *cache.Cache) *Collector {
	instanceUuid := ""
	filePath := path.Join(dataDir, "instance.uuid")
	data, err := os.ReadFile(filePath)
	if err != nil && !os.IsNotExist(err) {
		klog.Errorln("failed to read instance id:", err)
	}
	instanceUuid = strings.TrimSpace(string(data))
	if _, err := uuid.Parse(instanceUuid); err != nil {
		instanceUuid = uuid.NewString()
		if err := os.WriteFile(filePath, []byte(instanceUuid), 0644); err != nil {
			klog.Errorln("failed to write instance id:", err)
			return nil
		}
	}

	c := &Collector{
		db:    db,
		cache: cache,

		client: &http.Client{Timeout: sendTimeout},

		instanceUuid:    instanceUuid,
		instanceVersion: version,

		usersByScreenSize: map[string]*utils.StringSet{},
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

func (c *Collector) RegisterRequest(r *http.Request) {
	if c == nil {
		return
	}
	userUuid := r.Header.Get("x-device-id")
	screenSize := r.Header.Get("x-device-size")
	if userUuid == "" || screenSize == "" {
		return
	}

	c.lock.Lock()
	defer c.lock.Unlock()
	if c.usersByScreenSize[screenSize] == nil {
		c.usersByScreenSize[screenSize] = utils.NewStringSet()
	}
	c.usersByScreenSize[screenSize].Add(userUuid)
}

func (c *Collector) send() {
	stats := c.collect()
	buf := new(bytes.Buffer)
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
	stats.Instance.DatabaseType = c.db.Type()

	c.lock.Lock()
	stats.UX.UsersByScreenSize = map[string]int{}
	for size, users := range c.usersByScreenSize {
		stats.UX.UsersByScreenSize[size] = users.Len()
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
	var loadTime []time.Duration
	now := timeseries.Now()
	for _, p := range projects {
		cc := c.cache.GetCacheClient(p)
		cacheTo, err := cc.GetTo()
		if err != nil {
			klog.Errorln(err)
			continue
		}
		if cacheTo.IsZero() || cacheTo.Before(now.Add(-worldWindow)) {
			continue
		}
		stats.Integration.Prometheus = true

		t := time.Now()
		w, err := constructor.New(cc).LoadWorld(context.Background(), cacheTo.Add(-worldWindow), cacheTo, p.Prometheus.RefreshInterval)
		if err != nil {
			klog.Errorln("failed to load world:", err)
			continue
		}
		loadTime = append(loadTime, time.Since(t))
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
			clouds.Add(n.CloudProvider.Value())
		}

		for _, a := range w.Applications {
			if a.IsStandalone() || a.IsMonitoring() || a.IsControlPlane() {
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
		}
	}
	stats.Stack.Clouds = clouds.Items()
	stats.Stack.Services = services.Items()
	stats.Stack.InstrumentedServices = servicesInstrumented.Items()

	if len(loadTime) > 0 {
		var total time.Duration
		for _, t := range loadTime {
			total += t
		}
		stats.UX.WorldLoadTimeAvg = total.Seconds() / float64(len(loadTime))
	}

	return stats
}
