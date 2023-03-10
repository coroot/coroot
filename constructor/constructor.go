package constructor

import (
	"context"
	"errors"
	"fmt"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
	"net"
	"strings"
	"sync"
	"time"
)

var (
	ErrUnknownQuery = errors.New("unknown query")
)

type Option int

const (
	OptionLoadPerConnectionHistograms Option = iota
	OptionDoNotLoadRawSLIs
)

type Constructor struct {
	db      *db.DB
	project *db.Project
	prom    prom.Client
	options map[Option]bool
}

func New(db *db.DB, project *db.Project, prom prom.Client, options ...Option) *Constructor {
	c := &Constructor{db: db, project: project, prom: prom, options: map[Option]bool{}}
	for _, o := range options {
		c.options[o] = true
	}
	return c
}

type QueryStats struct {
	MetricsCount int     `json:"metrics_count"`
	QueryTime    float32 `json:"query_time"`
	Failed       bool    `json:"failed"`
}

type Profile struct {
	Stages  map[string]float32    `json:"stages"`
	Queries map[string]QueryStats `json:"queries"`
}

func (p *Profile) stage(name string, f func()) {
	if p.Stages == nil {
		f()
		return
	}
	t := time.Now()
	f()
	duration := float32(time.Since(t).Seconds())
	if duration > p.Stages[name] {
		p.Stages[name] = duration
	}
}

func (c *Constructor) LoadWorld(ctx context.Context, from, to timeseries.Time, step timeseries.Duration, prof *Profile) (*model.World, error) {
	w := model.NewWorld(from, to, step)

	if prof == nil {
		prof = &Profile{}
	}

	var err error
	prof.stage("get_check_configs", func() {
		w.CheckConfigs, err = c.db.GetCheckConfigs(c.project.Id)
	})
	if err != nil {
		return nil, err
	}

	var metrics map[string][]model.MetricValues
	prof.stage("query", func() {
		metrics, err = c.queryCache(ctx, from, to, step, w.CheckConfigs, prof.Queries)
	})
	if err != nil {
		if !errors.Is(err, ErrUnknownQuery) {
			return nil, err
		}
		klog.Warningln(err)
	}

	pjs := promJobStatuses{}
	nodesByMachineID := map[string]*model.Node{}

	// order is important
	prof.stage("load_job_statuses", func() { loadPromJobStatuses(metrics, pjs) })
	prof.stage("load_nodes", func() { loadNodes(w, metrics, nodesByMachineID) })
	prof.stage("load_k8s_metadata", func() { loadKubernetesMetadata(w, metrics) })
	prof.stage("load_rds", func() { loadRds(w, metrics, pjs) })
	prof.stage("load_containers", func() { loadContainers(w, metrics, pjs, nodesByMachineID) })
	prof.stage("enrich_instances", func() { enrichInstances(w, metrics) })
	prof.stage("join_db_cluster", func() { joinDBClusterComponents(w) })
	prof.stage("calc_app_categories", func() { c.calcApplicationCategories(w) })
	prof.stage("load_sli", func() { c.loadSLIs(w, metrics) })
	prof.stage("load_app_deployments", func() { c.loadApplicationDeployments(w) })
	prof.stage("calc_app_events", func() { calcAppEvents(w) })

	klog.Infof("got %d nodes, %d services, %d applications", len(w.Nodes), len(w.Services), len(w.Applications))
	return w, nil
}

type cacheQuery struct {
	query     string
	from, to  timeseries.Time
	step      timeseries.Duration
	statsName string
}

func (c *Constructor) queryCache(ctx context.Context, from, to timeseries.Time, step timeseries.Duration, checkConfigs model.CheckConfigs, stats map[string]QueryStats) (map[string][]model.MetricValues, error) {
	queries := map[string]cacheQuery{}
	rawFrom := to.Add(-model.MaxAlertRuleWindow)
	rawStep := c.project.Prometheus.RefreshInterval

	addQuery := func(name, statsName, query string, sli bool) {
		queries[name] = cacheQuery{query: query, from: from, to: to, step: step, statsName: statsName}
		if sli && !c.options[OptionDoNotLoadRawSLIs] {
			queries[name+"_raw"] = cacheQuery{query: query, from: rawFrom, to: to, step: rawStep, statsName: statsName + "_raw"}
		}
	}

	for n, q := range QUERIES {
		if !c.options[OptionLoadPerConnectionHistograms] && strings.HasPrefix(n, "container_") && strings.HasSuffix(n, "_histogram") {
			continue
		}
		addQuery(n, n, q, false)
	}

	for name := range RecordingRules {
		addQuery(name, name, name, true)
	}

	for appId := range checkConfigs {
		qName := fmt.Sprintf("%s/%s/", qApplicationCustomSLI, appId)
		availabilityCfg, _ := checkConfigs.GetAvailability(appId)
		if availabilityCfg.Custom {
			addQuery(qName+"total_requests", qApplicationCustomSLI, availabilityCfg.Total(), true)
			addQuery(qName+"failed_requests", qApplicationCustomSLI, availabilityCfg.Failed(), true)
		}
		latencyCfg, _ := checkConfigs.GetLatency(appId, model.CalcApplicationCategory(appId, c.project.Settings.ApplicationCategories))
		if latencyCfg.Custom {
			addQuery(qName+"requests_histogram", qApplicationCustomSLI, latencyCfg.Histogram(), true)
		}
	}

	res := make(map[string][]model.MetricValues, len(queries))
	var lock sync.Mutex
	var lastErr error
	wg := sync.WaitGroup{}
	now := time.Now()
	for name, query := range queries {
		wg.Add(1)
		go func(name string, q cacheQuery) {
			defer wg.Done()
			metrics, err := c.prom.QueryRange(ctx, q.query, q.from, q.to, q.step)
			if stats != nil {
				queryTime := float32(time.Since(now).Seconds())
				lock.Lock()
				s := stats[q.statsName]
				s.MetricsCount += len(metrics)
				s.QueryTime += queryTime
				s.Failed = s.Failed || err != nil
				stats[q.statsName] = s
				lock.Unlock()
			}
			if err != nil {
				lastErr = err
				return
			}
			lock.Lock()
			res[name] = metrics
			lock.Unlock()
		}(name, query)
	}
	wg.Wait()
	return res, lastErr
}

func (c *Constructor) calcApplicationCategories(w *model.World) {
	for _, app := range w.Applications {
		app.Category = model.CalcApplicationCategory(app.Id, c.project.Settings.ApplicationCategories)
	}
}

func (c *Constructor) loadApplicationDeployments(w *model.World) {
	byApp, err := c.db.GetApplicationDeployments(c.project.Id)
	if err != nil {
		klog.Errorln(err)
		return
	}
	for id, deployments := range byApp {
		app := w.GetApplication(id)
		if app == nil {
			continue
		}
		app.Deployments = deployments
	}
}

type promJob struct {
	job      string
	instance string
}

type promJobStatuses map[promJob]*timeseries.TimeSeries

func (s promJobStatuses) get(ls model.Labels) *timeseries.TimeSeries {
	return s[promJob{job: ls["job"], instance: ls["instance"]}]
}

func loadPromJobStatuses(metrics map[string][]model.MetricValues, statuses promJobStatuses) {
	for _, m := range metrics["up"] {
		statuses[promJob{job: m.Labels["job"], instance: m.Labels["instance"]}] = m.Values
	}
}

type podId struct {
	name, ns string
}

func enrichInstances(w *model.World, metrics map[string][]model.MetricValues) {
	instancesByListen := map[model.Listen]*model.Instance{}
	instancesByPod := map[podId]*model.Instance{}
	rdsInstancesById := map[string]*model.Instance{}

	for _, app := range w.Applications {
		for _, i := range app.Instances {
			if app.Id.Kind == model.ApplicationKindRds {
				rdsId := app.Id.Name + "/" + i.Name
				rdsInstancesById[rdsId] = i
			}
			if i.Pod != nil {
				instancesByPod[podId{name: i.Name, ns: app.Id.Namespace}] = i
			}
			for l := range i.TcpListens {
				instancesByListen[l] = i
			}
		}
	}

	for queryName := range metrics {
		for _, m := range metrics[queryName] {
			switch {
			case strings.HasPrefix(queryName, "pg_"):
				instance := findInstance(instancesByPod, instancesByListen, rdsInstancesById, m.Labels, model.ApplicationTypePostgres)
				postgres(instance, queryName, m)
			case strings.HasPrefix(queryName, "redis_"):
				instance := findInstance(instancesByPod, instancesByListen, rdsInstancesById, m.Labels, model.ApplicationTypeRedis, model.ApplicationTypeKeyDB)
				redis(instance, queryName, m)
			}
		}
	}
}

func joinDBClusterComponents(w *model.World) {
	clusters := map[model.ApplicationId]*model.Application{}
	toDelete := map[model.ApplicationId]*model.Application{}
	for _, app := range w.Applications {
		for _, instance := range app.Instances {
			if instance.ClusterName.Value() == "" {
				continue
			}
			id := model.NewApplicationId(app.Id.Namespace, model.ApplicationKindDatabaseCluster, instance.ClusterName.Value())
			cluster := clusters[id]
			if cluster == nil {
				cluster = model.NewApplication(id)
				clusters[id] = cluster
				w.Applications = append(w.Applications, cluster)
			}
			toDelete[app.Id] = cluster
		}
	}
	if len(toDelete) > 0 {
		var apps []*model.Application
		for _, app := range w.Applications {
			if cluster := toDelete[app.Id]; cluster == nil {
				apps = append(apps, app)
			} else {
				for _, instance := range app.Instances {
					instance.OwnerId = cluster.Id
				}
				cluster.Instances = append(cluster.Instances, app.Instances...)
				cluster.Downstreams = append(cluster.Downstreams, app.Downstreams...)
			}
		}
		w.Applications = apps
	}
}

func guessPod(ls model.Labels) string {
	for _, l := range []string{"pod", "pod_name", "kubernetes_pod", "k8s_pod"} {
		if pod := ls[l]; pod != "" {
			return pod
		}
	}
	return ""
}

func guessNamespace(ls model.Labels) string {
	for _, l := range []string{"namespace", "ns", "kubernetes_namespace", "kubernetes_ns", "k8s_namespace", "k8s_ns"} {
		if ns := ls[l]; ns != "" {
			return ns
		}
	}
	return ""
}

func findInstance(instancesByPod map[podId]*model.Instance, instancesByListen map[model.Listen]*model.Instance, rdsInstancesById map[string]*model.Instance, ls model.Labels, applicationTypes ...model.ApplicationType) *model.Instance {
	if rdsId := ls["rds_instance_id"]; rdsId != "" {
		return rdsInstancesById[rdsId]
	}
	if host, port, err := net.SplitHostPort(ls["instance"]); err == nil {
		if ip := net.ParseIP(host); ip != nil && !ip.IsLoopback() {
			var instance *model.Instance
			for _, l := range []model.Listen{{IP: host, Port: port, Proxied: true}, {IP: host, Port: port, Proxied: false}, {IP: host, Port: "0", Proxied: false}} {
				if instance = instancesByListen[l]; instance != nil {
					break
				}
			}
			return getActualServiceInstance(instance, applicationTypes...)
		}
	}
	if ns, pod := guessNamespace(ls), guessPod(ls); ns != "" && pod != "" {
		return getActualServiceInstance(instancesByPod[podId{name: pod, ns: ns}], applicationTypes...)
	}
	return nil
}

func getActualServiceInstance(instance *model.Instance, applicationTypes ...model.ApplicationType) *model.Instance {
	if len(applicationTypes) == 0 {
		return instance
	}
	if instance == nil {
		return nil
	}
	for _, t := range applicationTypes {
		if instance.ApplicationTypes()[t] {
			return instance
		}
	}
	for _, u := range instance.Upstreams {
		if ri := u.RemoteInstance; ri != nil {
			for _, t := range applicationTypes {
				if ri.ApplicationTypes()[t] {
					return ri
				}
			}
		}
	}
	for _, u := range instance.Upstreams {
		if ri := u.RemoteInstance; ri != nil && ri.OwnerId.Kind == model.ApplicationKindExternalService {
			return ri
		}
	}
	return instance
}
