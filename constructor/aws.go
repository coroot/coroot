package constructor

import (
	"github.com/coroot/coroot/model"
)

func loadAWSStatus(w *model.World, metrics map[string][]*model.MetricValues) {
	for _, m := range metrics["aws_discovery_error"] {
		if e := m.Labels["error"]; e != "" && m.Values.Last() > 0 {
			w.AWS.DiscoveryErrors[e] = true
		}
	}
}
