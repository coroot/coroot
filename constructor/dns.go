package constructor

import (
	"strconv"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
)

func (c *Constructor) loadApplicationDNS(w *model.World, metrics map[string][]*model.MetricValues, project *db.Project) {
	for _, mv := range metrics[qRecordingRuleApplicationDNSRequests] {
		appId, err := model.NewApplicationIdFromString(mv.Labels["app"], project.ClusterId())
		if err != nil {
			klog.Warningln(err)
			continue
		}
		app := w.GetApplication(appId)
		if app == nil {
			continue
		}
		r := model.DNSRequest{Type: mv.Labels["request_type"], Domain: mv.Labels["domain"]}
		if r.Type == "" || r.Domain == "" {
			continue
		}
		status := mv.Labels["status"]
		byStatus := app.DNSRequests[r]
		if byStatus == nil {
			byStatus = map[string]*timeseries.TimeSeries{}
			app.DNSRequests[r] = byStatus
		}
		byStatus[status] = merge(byStatus[status], mv.Values, timeseries.NanSum)
	}
	for _, mv := range metrics[qRecordingRuleApplicationDNSLatency] {
		appId, err := model.NewApplicationIdFromString(mv.Labels["app"], project.ClusterId())
		if err != nil {
			klog.Warningln(err)
			continue
		}
		app := w.GetApplication(appId)
		if app == nil {
			continue
		}
		le, err := strconv.ParseFloat(mv.Labels["le"], 32)
		if err != nil {
			klog.Warningln(err)
			continue
		}
		app.DNSRequestsHistogram[float32(le)] = merge(app.DNSRequestsHistogram[float32(le)], mv.Values, timeseries.Any)
	}
}
