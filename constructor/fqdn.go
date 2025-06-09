package constructor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/utils"
)

func loadFQDNs(metrics map[string][]*model.MetricValues, ip2fqdn, fqdn2ip map[string]*utils.StringSet) {
	var ip, fqdn string
	for _, m := range metrics["ip_to_fqdn"] {
		ip = m.Labels["ip"]
		fqdn = m.Labels["fqdn"]
		v := ip2fqdn[ip]
		if v == nil {
			v = utils.NewStringSet()
			ip2fqdn[ip] = v
		}
		v.Add(fqdn)
		v = fqdn2ip[fqdn]
		if v == nil {
			v = utils.NewStringSet()
			fqdn2ip[fqdn] = v
		}
		v.Add(ip)
	}
}
