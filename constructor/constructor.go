package constructor

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	pricing "github.com/coroot/coroot/cloud-pricing"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"golang.org/x/exp/maps"
	"k8s.io/klog"
)

var (
	ErrUnknownQuery = errors.New("unknown query")
)

type Option int

const (
	OptionLoadPerConnectionHistograms Option = iota
	OptionDoNotLoadRawSLIs
	OptionLoadContainerLogs
)

type Cache interface {
	QueryRange(ctx context.Context, query string, from, to timeseries.Time, step timeseries.Duration, fillFunc timeseries.FillFunc) ([]*model.MetricValues, error)
	GetStep(from, to timeseries.Time) (timeseries.Duration, error)
}

type Constructor struct {
	db      *db.DB
	project *db.Project
	cache   Cache
	pricing *pricing.Manager
	options map[Option]bool
}

func New(db *db.DB, project *db.Project, cache Cache, pricing *pricing.Manager, options ...Option) *Constructor {
	c := &Constructor{db: db, project: project, cache: cache, pricing: pricing, options: map[Option]bool{}}
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
	start := time.Now()
	rawStep, err := c.cache.GetStep(from, to)
	if err != nil {
		return nil, err
	}
	if rawStep == 0 {
		return model.NewWorld(from, to, step, step), nil
	}
	w := model.NewWorld(from, to, step, rawStep)
	w.CustomApplications = c.project.Settings.CustomApplications
	w.Categories = maps.Keys(c.project.Settings.ApplicationCategories)

	if prof == nil {
		prof = &Profile{}
	}

	prof.stage("get_check_configs", func() {
		w.CheckConfigs, err = c.db.GetCheckConfigs(c.project.Id)
	})
	if err != nil {
		return nil, err
	}

	var metrics map[string][]*model.MetricValues
	prof.stage("query", func() {
		metrics, err = c.queryCache(ctx, from, to, step, rawStep, w.CheckConfigs, prof.Queries)
	})
	if err != nil {
		if !errors.Is(err, ErrUnknownQuery) {
			return nil, err
		}
		klog.Warningln(err)
	}

	pjs := promJobStatuses{}
	nodesByID := map[model.NodeId]*model.Node{}
	rdsInstancesById := map[string]*model.Instance{}
	ecInstancesById := map[string]*model.Instance{}
	servicesByClusterIP := map[string]*model.Service{}
	ip2fqdn := map[string]*utils.StringSet{}
	containers := containerCache{}

	// order is important
	prof.stage("load_job_statuses", func() { loadPromJobStatuses(metrics, pjs) })
	prof.stage("load_nodes", func() { c.loadNodes(w, metrics, nodesByID) })
	prof.stage("load_fqdn", func() { loadFQDNs(metrics, ip2fqdn) })
	prof.stage("load_fargate_nodes", func() { c.loadFargateNodes(metrics, nodesByID) })
	prof.stage("load_k8s_metadata", func() { loadKubernetesMetadata(w, metrics, servicesByClusterIP) })
	prof.stage("load_aws_status", func() { loadAWSStatus(w, metrics) })
	prof.stage("load_rds_metadata", func() { loadRdsMetadata(w, metrics, pjs, rdsInstancesById) })
	prof.stage("load_elasticache_metadata", func() { loadElasticacheMetadata(w, metrics, pjs, ecInstancesById) })
	prof.stage("load_rds", func() { c.loadRds(w, metrics, pjs, rdsInstancesById) })
	prof.stage("load_elasticache", func() { c.loadElasticache(w, metrics, pjs, ecInstancesById) })
	prof.stage("load_fargate_containers", func() { loadFargateContainers(w, metrics, pjs) })
	prof.stage("load_containers", func() { c.loadContainers(w, metrics, pjs, nodesByID, containers, servicesByClusterIP, ip2fqdn) })
	prof.stage("load_jvm", func() { c.loadJVM(metrics, containers) })
	prof.stage("load_dotnet", func() { c.loadDotNet(metrics, containers) })
	prof.stage("load_python", func() { c.loadPython(metrics, containers) })
	prof.stage("enrich_instances", func() { enrichInstances(w, metrics, rdsInstancesById, ecInstancesById) })
	prof.stage("join_db_cluster", func() { joinDBClusterComponents(w) })
	prof.stage("calc_app_categories", func() { c.calcApplicationCategories(w) })
	prof.stage("load_app_settings", func() { c.loadApplicationSettings(w) })
	prof.stage("load_app_sli", func() { c.loadSLIs(w, metrics) })
	prof.stage("load_container_logs", func() { c.loadContainerLogs(metrics, containers, pjs) })
	prof.stage("load_app_logs", func() { c.loadApplicationLogs(w, metrics) })
	prof.stage("load_app_deployments", func() { c.loadApplicationDeployments(w) })
	prof.stage("load_app_incidents", func() { c.loadApplicationIncidents(w) })
	prof.stage("calc_app_events", func() { calcAppEvents(w) })

	klog.Infof("%s: got %d nodes, %d apps in %s", c.project.Id, len(w.Nodes), len(w.Applications), time.Since(start).Truncate(time.Millisecond))
	return w, nil
}

type cacheQuery struct {
	query     string
	from, to  timeseries.Time
	step      timeseries.Duration
	statsName string
	fillFunc  timeseries.FillFunc
}

func (c *Constructor) queryCache(ctx context.Context, from, to timeseries.Time, step, rawStep timeseries.Duration, checkConfigs model.CheckConfigs, stats map[string]QueryStats) (map[string][]*model.MetricValues, error) {
	loadRawSLIs := !c.options[OptionDoNotLoadRawSLIs]
	rawFrom := from
	if t := to.Add(-model.MaxAlertRuleWindow); t.Before(rawFrom) {
		rawFrom = t
	}
	rawFrom = rawFrom.Truncate(rawStep)
	rawTo := to.Truncate(rawStep)

	from = from.Truncate(step)
	to = to.Truncate(step)
	queries := map[string]cacheQuery{}
	addQuery := func(name, statsName, query string, sli bool) {
		if sli && loadRawSLIs {
			queries[name+"_raw"] = cacheQuery{query: query, from: rawFrom, to: rawTo, step: rawStep, statsName: statsName + "_raw"}
		}
		if !sli {
			queries[name] = cacheQuery{query: query, from: from, to: to, step: step, statsName: statsName}
		}
	}

	for n, q := range QUERIES {
		if !c.options[OptionLoadPerConnectionHistograms] && strings.HasPrefix(n, "container_") && strings.HasSuffix(n, "_histogram") {
			continue
		}
		if !c.options[OptionLoadContainerLogs] && n == "container_log_messages" {
			queries[qRecordingRuleApplicationLogMessages] = cacheQuery{
				query:     qRecordingRuleApplicationLogMessages,
				from:      from,
				to:        to,
				step:      step,
				statsName: qRecordingRuleApplicationLogMessages,
				fillFunc:  timeseries.FillSum,
			}
			continue
		}
		addQuery(n, n, q, false)
		if n == "container_memory_rss" || n == "fargate_container_memory_rss" {
			name := n + "_for_trend"
			queries[name] = cacheQuery{
				query:     q,
				from:      to.Add(-timeseries.Hour * 4).Truncate(rawStep),
				to:        to.Truncate(rawStep),
				step:      rawStep,
				statsName: name,
			}
		}
	}

	addQuery(qRecordingRuleInboundRequestsTotal, qRecordingRuleInboundRequestsTotal, qRecordingRuleInboundRequestsTotal, true)
	addQuery(qRecordingRuleInboundRequestsHistogram, qRecordingRuleInboundRequestsHistogram, qRecordingRuleInboundRequestsHistogram, true)

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

	res := make(map[string][]*model.MetricValues, len(queries))
	var lock sync.Mutex
	var lastErr error
	wg := sync.WaitGroup{}
	now := time.Now()
	for name, query := range queries {
		wg.Add(1)
		go func(name string, q cacheQuery) {
			defer wg.Done()
			if q.fillFunc == nil {
				q.fillFunc = timeseries.FillAny
			}
			metrics, err := c.cache.QueryRange(ctx, q.query, q.from, q.to, q.step, q.fillFunc)
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

func (c *Constructor) loadApplicationSettings(w *model.World) {
	settings, err := c.db.GetApplicationSettingsByProject(c.project.Id)
	if err != nil {
		klog.Errorln(err)
		return
	}
	for _, app := range w.Applications {
		app.Settings = settings[app.Id]
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

func (c *Constructor) loadApplicationIncidents(w *model.World) {
	byApp, err := c.db.GetApplicationIncidents(c.project.Id, w.Ctx.From, w.Ctx.To)
	if err != nil {
		klog.Errorln(err)
		return
	}
	for id, incidents := range byApp {
		app := w.GetApplication(id)
		if app == nil {
			continue
		}
		app.Incidents = incidents
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

func loadPromJobStatuses(metrics map[string][]*model.MetricValues, statuses promJobStatuses) {
	for _, m := range metrics["up"] {
		statuses[promJob{job: m.Labels["job"], instance: m.Labels["instance"]}] = m.Values
	}
}

type podId struct {
	name, ns string
}

func enrichInstances(w *model.World, metrics map[string][]*model.MetricValues, rdsInstancesById map[string]*model.Instance, ecInstanceById map[string]*model.Instance) {
	instancesByListen := map[model.Listen]*model.Instance{}
	instancesByPod := map[podId]*model.Instance{}
	for _, app := range w.Applications {
		for _, i := range app.Instances {
			if i.Pod != nil {
				instancesByPod[podId{name: i.Name, ns: app.Id.Namespace}] = i
			}
			for l := range i.TcpListens {
				instancesByListen[l] = i
			}
		}
	}

	instancesByListenAddr := map[string]*model.Instance{}
	for l, i := range instancesByListen {
		addr := net.JoinHostPort(l.IP, l.Port)
		if instancesByListenAddr[addr] != nil {
			continue
		}
		if ip := net.ParseIP(l.IP); ip == nil || ip.IsLoopback() {
			continue
		}
		l.Proxied = true
		if ii := instancesByListen[l]; ii != nil {
			instancesByListenAddr[addr] = ii
			continue
		}
		l.Proxied = false
		if ii := instancesByListen[l]; ii != nil {
			instancesByListenAddr[addr] = ii
			continue
		}
		instancesByListenAddr[addr] = i
	}

	for _, queryName := range []string{"pg_up", "redis_up", "mongo_up", "memcached_up"} {
		for _, m := range metrics[queryName] {
			if !metricFromInternalExporter(m.Labels) {
				continue
			}
			address := m.Labels["address"]
			if address == "" {
				continue
			}
			instance := instancesByListenAddr[address]
			if instance == nil {
				continue
			}
			switch queryName {
			case "pg_up":
				instance.Postgres = model.NewPostgres(true)
			case "redis_up":
				instance.Redis = model.NewRedis(true)
			case "mongo_up":
				instance.Mongodb = model.NewMongodb(true)
			case "memcached_up":
				instance.Memcached = model.NewMemcached(true)
			}
		}
	}

	for queryName := range metrics {
		for _, m := range metrics[queryName] {
			switch {
			case strings.HasPrefix(queryName, "pg_"):
				instance := findInstance(instancesByPod, instancesByListenAddr, rdsInstancesById, ecInstanceById, m.Labels, model.ApplicationTypePostgres)
				postgres(instance, queryName, m)
			case strings.HasPrefix(queryName, "redis_"):
				instance := findInstance(instancesByPod, instancesByListenAddr, rdsInstancesById, ecInstanceById, m.Labels, model.ApplicationTypeRedis, model.ApplicationTypeKeyDB)
				redis(instance, queryName, m)
			case strings.HasPrefix(queryName, "mongo_"):
				instance := findInstance(instancesByPod, instancesByListenAddr, rdsInstancesById, ecInstanceById, m.Labels, model.ApplicationTypeMongodb, model.ApplicationTypeMongos)
				mongodb(instance, queryName, m)
			case strings.HasPrefix(queryName, "memcached_"):
				instance := findInstance(instancesByPod, instancesByListenAddr, rdsInstancesById, ecInstanceById, m.Labels, model.ApplicationTypeMemcached)
				memcached(instance, queryName, m)
			case strings.HasPrefix(queryName, "mysql_"):
				instance := findInstance(instancesByPod, instancesByListenAddr, rdsInstancesById, ecInstanceById, m.Labels, model.ApplicationTypeMysql)
				mysql(instance, queryName, m)
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
				w.Applications[id] = cluster
			}
			toDelete[app.Id] = cluster
		}
	}
	if len(toDelete) > 0 {
		for id, app := range w.Applications {
			cluster := toDelete[app.Id]
			if cluster == nil {
				continue
			}
			for _, svc := range app.KubernetesServices {
				found := false
				for _, existingSvc := range cluster.KubernetesServices {
					if svc.Name == existingSvc.Name && svc.Namespace == existingSvc.Namespace {
						found = true
						break
					}
				}
				if !found {
					cluster.KubernetesServices = append(cluster.KubernetesServices, svc)
				}
				svc.DestinationApps[cluster.Id] = cluster
				delete(svc.DestinationApps, id)
			}
			cluster.DesiredInstances = merge(cluster.DesiredInstances, app.DesiredInstances, timeseries.NanSum)
			for _, instance := range app.Instances {
				instance.Owner = cluster
				instance.ClusterComponent = app
			}
			cluster.Instances = append(cluster.Instances, app.Instances...)
			for _, d := range app.Downstreams {
				d.RemoteApplication = cluster
			}
			cluster.Downstreams = append(cluster.Downstreams, app.Downstreams...)
			delete(w.Applications, id)
		}
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

func findInstance(instancesByPod map[podId]*model.Instance, instancesByListen map[string]*model.Instance, rdsInstancesById map[string]*model.Instance, ecInstancesById map[string]*model.Instance, ls model.Labels, applicationTypes ...model.ApplicationType) *model.Instance {
	if rdsId := ls["rds_instance_id"]; rdsId != "" {
		return rdsInstancesById[rdsId]
	}
	if ecId := ls["ec_instance_id"]; ecId != "" {
		return ecInstancesById[ecId]
	}
	address := ls["instance"]
	if ls["address"] != "" {
		address = ls["address"]
	}
	if address != "" {
		instance := instancesByListen[address]
		return getActualServiceInstance(instance, applicationTypes...)
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
		if ri := u.RemoteInstance; ri != nil && ri.Owner.Id.Kind == model.ApplicationKindExternalService {
			return ri
		}
	}
	return instance
}

func metricFromInternalExporter(ls model.Labels) bool {
	return ls["job"] == "coroot-cluster-agent"
}
