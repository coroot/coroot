package constructor

import "github.com/coroot/coroot/model"

func loadFQDNs(metrics map[string][]model.MetricValues, ip2fqdn map[string]*model.LabelLastValue) {
	for _, m := range metrics["ip_to_fqdn"] {
		ip := m.Labels["ip"]
		v := ip2fqdn[ip]
		if v == nil {
			v = &model.LabelLastValue{}
			ip2fqdn[ip] = v
		}
		v.Update(m.Values, m.Labels["fqdn"])
	}
}
