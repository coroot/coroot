package constructor

import (
	"strconv"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
)

func (c *Constructor) loadAppDNS(w *model.World, metrics map[string][]*model.MetricValues) {
	for _, mv := range metrics[qRecordingRuleDNSRequests] {
		appId, err := model.NewApplicationIdFromString(mv.Labels["app"])
		if err != nil {
			klog.Warningln(err)
			continue
		}
		app := w.GetOrCreateApplication(appId, false)
		r := model.DNSRequest{
			Type:   mv.Labels["request_type"],
			Domain: mv.Labels["domain"],
		}
		if r.Type == "" || r.Domain == "" {
			return
		}
		status := mv.Labels["status"]
		byStatus := app.DNSRequests[r]
		if byStatus == nil {
			byStatus = map[string]*timeseries.TimeSeries{}
			app.DNSRequests[r] = byStatus
		}
		byStatus[status] = merge(byStatus[status], mv.Values, timeseries.Any)
	}
	for _, mv := range metrics[qRecordingRuleDNSLatency] {
		appId, err := model.NewApplicationIdFromString(mv.Labels["app"])
		if err != nil {
			klog.Warningln(err)
			continue
		}
		app := w.GetOrCreateApplication(appId, false)
		le, err := strconv.ParseFloat(mv.Labels["le"], 32)
		if err != nil {
			klog.Warningln(err)
			return
		}
		app.DNSRequestsHistogram[float32(le)] = merge(app.DNSRequestsHistogram[float32(le)], mv.Values, timeseries.Any)
	}
}
