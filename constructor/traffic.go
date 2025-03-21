package constructor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
)

func (c *Constructor) loadApplicationTraffic(w *model.World, metrics map[string][]*model.MetricValues) {
	for _, mv := range metrics[qRecordingRuleApplicationTraffic] {
		appId, err := model.NewApplicationIdFromString(mv.Labels["app"])
		if err != nil {
			klog.Warningln(err)
			continue
		}
		app := w.GetApplication(appId)
		if app == nil {
			continue
		}
		switch model.TrafficKind(mv.Labels["kind"]) {
		case model.TrafficKindCrossAZIngress:
			app.TrafficStats.CrossAZIngress = merge(app.TrafficStats.CrossAZIngress, mv.Values, timeseries.Any)
		case model.TrafficKindCrossAZEgress:
			app.TrafficStats.CrossAZEgress = merge(app.TrafficStats.CrossAZEgress, mv.Values, timeseries.Any)
		case model.TrafficKindInternetEgress:
			app.TrafficStats.InternetEgress = merge(app.TrafficStats.InternetEgress, mv.Values, timeseries.Any)
		}
	}
}
