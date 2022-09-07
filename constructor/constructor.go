package constructor

import (
	"context"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
	"net"
	"strings"
	"time"
)

type Constructor struct {
	prom prom.Client
}

func New(prom prom.Client) *Constructor {
	return &Constructor{prom: prom}
}

func (c *Constructor) LoadWorld(ctx context.Context, from, to timeseries.Time, step timeseries.Duration) (*model.World, error) {
	w := model.NewWorld(from, to, step)

	t := time.Now()
	metrics, err := prom.ParallelQueryRange(ctx, c.prom, from, to, step, QUERIES)
	if err != nil {
		return nil, err
	}
	klog.Infof("got metrics in %s", time.Since(t))

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
			case strings.HasPrefix(queryName, "redis_"):
				redis(w, queryName, m)
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
	klog.Warningf(
		`couldn't find actual instance for "%s", initial instance is "%s" (%+v)`,
		applicationType, instance.Name, instance.ApplicationTypes(),
	)
	return nil
}
