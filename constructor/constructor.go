package constructor

import (
	"context"
	"github.com/coroot/coroot-focus/model"
	"github.com/coroot/coroot-focus/prometheus"
	"github.com/coroot/coroot-focus/timeseries"
	"k8s.io/klog"
	"time"
)

type Constructor struct {
	prom *prometheus.Client
	step time.Duration
}

func New(prom *prometheus.Client, step time.Duration) *Constructor {
	return &Constructor{prom: prom, step: step}
}

func (c *Constructor) LoadWorld(ctx context.Context, from, to time.Time) (*model.World, error) {
	now := time.Now()
	metrics, err := prometheus.ParallelQueryRange(ctx, c.prom, from, to, QUERIES)
	if err != nil {
		return nil, err
	}
	klog.Infof("got metrics in %s", time.Since(now))
	w := &model.World{
		Ctx: timeseries.Context{
			From: timeseries.Time(from.Unix()),
			To:   timeseries.Time(to.Unix()),
			Step: timeseries.Duration(c.step.Seconds()),
		},
	}

	loadNodes(w, metrics)
	loadKubernetesMetadata(w, metrics)
	loadContainers(w, metrics)
	joinDBClusterComponents(w)
	klog.Infof("got %d nodes, %d services, %d applications", len(w.Nodes), len(w.Services), len(w.Applications))
	//for _, a := range w.Applications {
	//	klog.Infoln(a.ApplicationId)
	//	for _, i := range a.Instances {
	//		klog.Infof("\t%s", i.Name)
	//		for _, c := range i.Containers {
	//			klog.Infof("\t\t%s", c.Name)
	//		}
	//	}
	//}
	return w, nil
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
