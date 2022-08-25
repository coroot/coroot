package constructor

import (
	"context"
	"fmt"
	"github.com/coroot/coroot-focus/model"
	"github.com/coroot/coroot-focus/prom"
	"github.com/coroot/coroot-focus/timeseries"
	"github.com/coroot/coroot-focus/utils"
	"k8s.io/klog"
	"net"
	"strings"
	"time"
)

type Constructor struct {
	prom prom.Client
	step time.Duration
}

func New(prom prom.Client, step time.Duration) *Constructor {
	return &Constructor{prom: prom, step: step}
}

func (c *Constructor) LoadWorld(ctx context.Context, from, to time.Time) (*model.World, error) {
	now := time.Now()

	actualQueries := utils.NewStringSet()
	for _, q := range QUERIES {
		actualQueries.Add(q)
	}

	step := c.step
	duration := to.Sub(from)
	switch {
	case duration > 5*24*time.Hour:
		step = maxDuration(step, 60*time.Minute)
	case duration > 24*time.Hour:
		step = maxDuration(step, 15*time.Minute)
	case duration > 12*time.Hour:
		step = maxDuration(step, 10*time.Minute)
	case duration > 6*time.Hour:
		step = maxDuration(step, 5*time.Minute)
	case duration > 4*time.Hour:
		step = maxDuration(step, time.Minute)
	}

	lastUpdateTs := c.prom.LastUpdateTime(actualQueries)
	if !lastUpdateTs.IsZero() && lastUpdateTs.Before(to) {
		if lastUpdateTs.Before(from) {
			return nil, fmt.Errorf("out of cache time range")
		}
		lastUpdateTs = lastUpdateTs.Add(-c.step)
		from = from.Add(lastUpdateTs.Sub(to))
		to = lastUpdateTs
	}
	from = from.Truncate(step)
	to = to.Truncate(step)

	metrics, err := prom.ParallelQueryRange(ctx, c.prom, from, to, step, QUERIES)
	if err != nil {
		return nil, err
	}
	klog.Infof("got metrics in %s", time.Since(now))
	w := &model.World{
		Ctx: timeseries.Context{
			From: timeseries.Time(from.Unix()),
			To:   timeseries.Time(to.Unix()),
			Step: timeseries.Duration(step.Seconds()),
		},
	}

	loadNodes(w, metrics)
	loadKubernetesMetadata(w, metrics)
	loadContainers(w, metrics)
	enrichInstances(w, metrics)
	joinDBClusterComponents(w)
	klog.Infof("got %d nodes, %d services, %d applications", len(w.Nodes), len(w.Services), len(w.Applications))
	return w, nil
}

func enrichInstances(w *model.World, metrics map[string][]model.MetricValues) {
	for queryName := range metrics {
		for _, m := range metrics[queryName] {
			switch {
			case strings.HasPrefix(queryName, "pg_"):
				postgres(w, queryName, m)
			}
		}
	}
}

func prometheusJobStatus(metrics map[string][]model.MetricValues, job, instance string) timeseries.TimeSeries {
	for _, m := range metrics["up"] {
		if m.Labels["job"] == job && m.Labels["instance"] == instance {
			return m.Values
		}
	}
	return nil
}

func update(dest, v timeseries.TimeSeries) timeseries.TimeSeries {
	if dest == nil {
		dest = timeseries.Aggregate(timeseries.Any)
	}
	dest.(*timeseries.AggregatedTimeseries).AddInput(v)
	return dest
}

func joinDBClusterComponents(w *model.World) {
	clusters := map[model.ApplicationId]*model.Application{}
	toDelete := map[model.ApplicationId]bool{}
	for _, app := range w.Applications {
		for _, instance := range app.Instances {
			if instance.ClusterName.Value() == "" {
				continue
			}
			id := model.NewApplicationId(app.Id.Namespace, model.ApplicationKindDatabaseCluster, instance.ClusterName.Value())
			a := clusters[id]
			if a == nil {
				a = model.NewApplication(id)
				clusters[id] = a
				w.Applications = append(w.Applications, a)
			}
			a.Instances = append(a.Instances, instance)
			instance.OwnerId = id
			toDelete[app.Id] = true
		}
	}
	if len(toDelete) > 0 {
		var apps []*model.Application
		for _, app := range w.Applications {
			if !toDelete[app.Id] {
				apps = append(apps, app)
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
	if instance.ApplicationTypes()[applicationType] {
		return instance
	}
	for _, u := range instance.Upstreams {
		if ri := u.RemoteInstance; ri != nil && ri.ApplicationTypes()[applicationType] {
			return ri
		}
	}
	klog.Warningf(
		`couldn't find actual instance for "%s", initial instance is "%s" (%+v)`,
		applicationType, instance.Name, instance.ApplicationTypes(),
	)
	return nil
}

func maxDuration(d1, d2 time.Duration) time.Duration {
	if d1 >= d2 {
		return d1
	}
	return d2
}
