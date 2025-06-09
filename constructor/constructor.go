package constructor

import (
	"context"
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
	"k8s.io/klog"
)

type Option int

const (
	OptionLoadInstanceToInstanceConnections Option = iota
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
	for name := range c.project.Settings.ApplicationCategorySettings {
		if !name.Default() {
			w.Categories = append(w.Categories, name)
		}
	}
	utils.SortSlice(w.Categories)

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
		return nil, err
	}

	pjs := promJobStatuses{}
	nodes := nodeCache{}
	rdsInstancesById := map[string]*model.Instance{}
	ecInstancesById := map[string]*model.Instance{}
	servicesByClusterIP := map[string]*model.Service{}
	ip2fqdn := map[string]*utils.StringSet{}
	fqdn2ip := map[string]*utils.StringSet{}
	containers := containerCache{}

	// order is important
	prof.stage("load_job_statuses", func() { loadPromJobStatuses(metrics, pjs) })
	prof.stage("load_nodes", func() { c.loadNodes(w, metrics, nodes) })
	prof.stage("load_fqdn", func() { loadFQDNs(metrics, ip2fqdn, fqdn2ip) })
	prof.stage("load_fargate_nodes", func() { c.loadFargateNodes(metrics, nodes) })
	prof.stage("load_k8s_metadata", func() { loadKubernetesMetadata(w, metrics, servicesByClusterIP) })
	prof.stage("load_aws_status", func() { loadAWSStatus(w, metrics) })
	prof.stage("load_rds_metadata", func() { loadRdsMetadata(w, metrics, pjs, rdsInstancesById) })
	prof.stage("load_elasticache_metadata", func() { loadElasticacheMetadata(w, metrics, pjs, ecInstancesById) })
	prof.stage("load_rds", func() { c.loadRds(w, metrics, pjs, rdsInstancesById) })
	prof.stage("load_elasticache", func() { c.loadElasticache(w, metrics, pjs, ecInstancesById) })
	prof.stage("load_fargate_containers", func() { loadFargateContainers(w, metrics, pjs) })
	prof.stage("load_containers", func() { c.loadContainers(w, metrics, pjs, nodes, containers, servicesByClusterIP, ip2fqdn) })
	prof.stage("load_app_to_app_connections", func() { c.loadAppToAppConnections(w, metrics, fqdn2ip) })
	prof.stage("load_application_traffic", func() { c.loadApplicationTraffic(w, metrics) })
	prof.stage("load_jvm", func() { c.loadJVM(metrics, containers) })
	prof.stage("load_dotnet", func() { c.loadDotNet(metrics, containers) })
	prof.stage("load_python", func() { c.loadPython(metrics, containers) })
	prof.stage("enrich_instances", func() { enrichInstances(w, metrics, rdsInstancesById, ecInstancesById) })
	prof.stage("calc_app_categories", func() { c.calcApplicationCategories(w) })
	prof.stage("group_custom_applications", func() { c.groupCustomApplications(w) })
	prof.stage("join_db_cluster_components", func() { c.joinDBClusterComponents(w) })
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

	for _, q := range QUERIES {
		if !c.options[OptionLoadInstanceToInstanceConnections] && q.InstanceToInstance {
			continue
		}
		if !c.options[OptionLoadContainerLogs] && q.Name == "container_log_messages" {
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
		addQuery(q.Name, q.Name, q.Query, false)
		if q.Name == "container_memory_rss" || q.Name == "fargate_container_memory_rss" {
			name := q.Name + "_for_trend"
			queries[name] = cacheQuery{
				query:     q.Query,
				from:      to.Add(-timeseries.Hour * 4).Truncate(rawStep),
				to:        to.Truncate(rawStep),
				step:      rawStep,
				statsName: name,
			}
		}
	}
	if !c.options[OptionLoadInstanceToInstanceConnections] {
		for _, query := range qConnectionAggregations {
			queries[query] = cacheQuery{query: query, from: from, to: to, step: step, statsName: query}
		}
		addQuery(qRecordingRuleApplicationL7Requests, qRecordingRuleApplicationL7Requests, qRecordingRuleApplicationL7Requests, true)
		addQuery(qRecordingRuleApplicationL7Histogram, qRecordingRuleApplicationL7Histogram, qRecordingRuleApplicationL7Histogram, true)
	}
	for appId := range checkConfigs {
		qName := fmt.Sprintf("%s/%s/", qApplicationCustomSLI, appId)
		availabilityCfg, _ := checkConfigs.GetAvailability(appId)
		if availabilityCfg.Custom {
			addQuery(qName+"total_requests", qApplicationCustomSLI, availabilityCfg.Total(), true)
			addQuery(qName+"failed_requests", qApplicationCustomSLI, availabilityCfg.Failed(), true)
		}
		latencyCfg, _ := checkConfigs.GetLatency(appId, c.project.CalcApplicationCategory(appId))
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
				s.Cardinality = cardinalityStats(metrics)
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
		if annotation := app.GetAnnotation(model.ApplicationAnnotationCategory); annotation != "" {
			klog.Infoln(app.Id, annotation)
			app.Category = model.ApplicationCategory(annotation)
			continue
		}
		app.Category = c.project.CalcApplicationCategory(app.Id)
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

type appGroup struct {
	app     *model.Application
	members map[model.ApplicationId]*model.Application
}

func (c *Constructor) groupApplications(w *model.World, groups map[model.ApplicationId]*appGroup) {
	for _, group := range groups {
		categories := utils.NewStringSet()
		for _, app := range group.members {
			for _, svc := range app.KubernetesServices {
				found := false
				for _, existingSvc := range group.app.KubernetesServices {
					if svc.Name == existingSvc.Name && svc.Namespace == existingSvc.Namespace {
						found = true
						break
					}
				}
				if !found {
					group.app.KubernetesServices = append(group.app.KubernetesServices, svc)
				}
				svc.DestinationApps[group.app.Id] = group.app
				delete(svc.DestinationApps, app.Id)
			}
			group.app.DesiredInstances = merge(group.app.DesiredInstances, app.DesiredInstances, timeseries.NanSum)
			for _, instance := range app.Instances {
				instance.Owner = group.app
				instance.ClusterComponent = app
				group.app.Instances = append(group.app.Instances, instance)
			}
			categories.Add(string(app.Category))
			delete(w.Applications, app.Id)
		}
		group.app.Category = model.ApplicationCategory(categories.GetFirst())
		if group.app.Category == "" {
			group.app.Category = c.project.CalcApplicationCategory(group.app.Id)
		}
	}
}

func (c *Constructor) groupCustomApplications(w *model.World) {
	customApps := map[model.ApplicationId]*appGroup{}
	for _, app := range w.Applications {
		customName := app.GetAnnotation(model.ApplicationAnnotationCustomName)
		if customName == "" {
			continue
		}
		id := model.NewApplicationId(app.Id.Namespace, model.ApplicationKindCustomApplication, customName)
		group := customApps[id]
		if group == nil {
			group = &appGroup{app: w.GetOrCreateApplication(id, true), members: map[model.ApplicationId]*model.Application{}}
			customApps[id] = group
		}
		group.members[app.Id] = app
	}
	c.groupApplications(w, customApps)
}

func (c *Constructor) joinDBClusterComponents(w *model.World) {
	dbClusters := map[model.ApplicationId]*appGroup{}
	for _, app := range w.Applications {
		for _, instance := range app.Instances {
			if instance.ClusterName.Value() == "" {
				continue
			}
			id := model.NewApplicationId(app.Id.Namespace, model.ApplicationKindDatabaseCluster, instance.ClusterName.Value())
			cluster := dbClusters[id]
			if cluster == nil {
				cluster = &appGroup{app: w.GetOrCreateApplication(id, false), members: map[model.ApplicationId]*model.Application{}}
				dbClusters[id] = cluster
			}
			cluster.members[app.Id] = app
		}
	}
	c.groupApplications(w, dbClusters)
}

func guessPod(ls model.Labels) string {
	for _, l := range possiblePodLabels {
		if pod := ls[l]; pod != "" {
			return pod
		}
	}
	return ""
}

func guessNamespace(ls model.Labels) string {
	for _, l := range possibleNamespaceLabels {
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
	return instance
}

func metricFromInternalExporter(ls model.Labels) bool {
	return ls["job"] == "coroot-cluster-agent"
}
