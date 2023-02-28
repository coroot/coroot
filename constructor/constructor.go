package constructor

import (
	"context"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
	"net"
	"strings"
	"time"
)

type Option int

const (
	OptionLoadPerConnectionHistograms Option = iota
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

type Profile struct {
	Stages  map[string]float32         `json:"stages"`
	Queries map[string]prom.QueryStats `json:"queries"`
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
		queries := map[string]string{}
		for n, q := range QUERIES {
			if !c.options[OptionLoadPerConnectionHistograms] && strings.HasPrefix(n, "container_") && strings.HasSuffix(n, "_histogram") {
				continue
			}
			queries[n] = q
		}
		metrics, err = prom.ParallelQueryRange(ctx, c.prom, from, to, step, queries, prof.Queries)
	})
	if err != nil {
		return nil, err
	}

	// order is important
	prof.stage("load_nodes", func() { loadNodes(w, metrics) })
	prof.stage("load_k8s_metadata", func() { loadKubernetesMetadata(w, metrics) })
	prof.stage("load_rds", func() { loadRds(w, metrics) })
	prof.stage("load_containers", func() { loadContainers(w, metrics) })
	prof.stage("enrich_instances", func() { enrichInstances(w, metrics) })
	prof.stage("join_db_cluster", func() { joinDBClusterComponents(w) })
	prof.stage("calc_app_categories", func() { c.calcApplicationCategories(w) })
	prof.stage("load_sli", func() { c.loadSLIs(ctx, w, from, to, step) })
	prof.stage("load_app_deployments", func() { c.loadApplicationDeployments(w) })
	prof.stage("calc_app_events", func() { calcAppEvents(w) })

	klog.Infof("got %d nodes, %d services, %d applications", len(w.Nodes), len(w.Services), len(w.Applications))
	return w, nil
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

func enrichInstances(w *model.World, metrics map[string][]model.MetricValues) {
	for queryName := range metrics {
		for _, m := range metrics[queryName] {
			switch {
			case strings.HasPrefix(queryName, "pg_"):
				postgres(w, queryName, m)
			case strings.HasPrefix(queryName, "redis_"):
				redis(w, queryName, m)
			}
		}
	}
}

func prometheusJobStatus(metrics map[string][]model.MetricValues, job, instance string) *timeseries.TimeSeries {
	for _, m := range metrics["up"] {
		if m.Labels["job"] == job && m.Labels["instance"] == instance {
			return m.Values
		}
	}
	return nil
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

func findInstance(w *model.World, ls model.Labels, applicationType model.ApplicationType) *model.Instance {
	if rdsId := ls["rds_instance_id"]; rdsId != "" {
		return getOrCreateRdsInstance(w, rdsId)
	}
	if host, port, err := net.SplitHostPort(ls["instance"]); err == nil {
		if ip := net.ParseIP(host); ip != nil && !ip.IsLoopback() {
			return getActualServiceInstance(w.FindInstanceByListen(host, port), applicationType)
		}
	}
	if ns, pod := guessNamespace(ls), guessPod(ls); ns != "" && pod != "" {
		return getActualServiceInstance(w.FindInstanceByPod(ns, pod), applicationType)
	}
	return nil
}

func getActualServiceInstance(instance *model.Instance, applicationType model.ApplicationType) *model.Instance {
	if applicationType == "" {
		return instance
	}
	if instance == nil {
		return nil
	}
	if instance.ApplicationTypes()[applicationType] {
		return instance
	}
	for _, u := range instance.Upstreams {
		if ri := u.RemoteInstance; ri != nil && ri.ApplicationTypes()[applicationType] {
			return ri
		}
	}
	for _, u := range instance.Upstreams {
		if ri := u.RemoteInstance; ri != nil && ri.OwnerId.Kind == model.ApplicationKindExternalService {
			return ri
		}
	}
	klog.Warningf(
		`couldn't find actual instance for "%s", initial instance is "%s" (%+v)`,
		applicationType, instance.Name, instance.ApplicationTypes(),
	)
	return nil
}
