package constructor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/utils"
)

func loadFQDNs(metrics map[string][]*model.MetricValues, ip2fqdn map[string]*utils.StringSet) {
	for _, m := range metrics["ip_to_fqdn"] {
		ip := m.Labels["ip"]
		v := ip2fqdn[ip]
		if v == nil {
			v = utils.NewStringSet()
			ip2fqdn[ip] = v
		}
		v.Add(m.Labels["fqdn"])
	}
}
