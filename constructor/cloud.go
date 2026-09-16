package constructor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func loadCloudStatus(w *model.World, metrics map[string][]*model.MetricValues) {
	for _, m := range metrics["aws_discovery_error"] {
		if timeseries.IsNaN(m.Values.Last()) {
			continue
		}
		w.AWS.Configured = true
		if e := m.Labels["error"]; e != "" && m.Values.Last() > 0 {
			w.AWS.DiscoveryErrors[e] = true
		}
	}
	for _, m := range metrics["gcp_discovery_error"] {
		if timeseries.IsNaN(m.Values.Last()) {
			continue
		}
		w.GCP.Configured = true
		if e := m.Labels["error"]; e != "" && m.Values.Last() > 0 {
			w.GCP.DiscoveryErrors[e] = true
		}
	}
}
