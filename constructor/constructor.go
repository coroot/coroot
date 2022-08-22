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
}

func New(prom *prometheus.Client) *Constructor {
	return &Constructor{prom: prom}
}

func (c *Constructor) LoadWorld(ctx context.Context, from, to time.Time) (*model.World, error) {
	now := time.Now()
	metrics, err := prometheus.ParallelQueryRange(ctx, c.prom, from, to, QUERIES)
	if err != nil {
		return nil, err
	}
	w := &model.World{}
	klog.Infof("got metrics in %s", time.Since(now))

	loadNodes(w, metrics)
	loadKubernetesMetadata(w, metrics)
	loadContainers(w, metrics)
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
