package overview

import (
	"sort"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/utils"
	"inet.af/netaddr"
)

type Risk struct {
	Key                 model.RiskKey             `json:"key"`
	ApplicationId       model.ApplicationId       `json:"application_id"`
	ApplicationCategory model.ApplicationCategory `json:"application_category"`
	ApplicationType     *ApplicationType          `json:"application_type"`
	Severity            model.Status              `json:"severity"`
	Type                string                    `json:"type"`
	Dismissal           *model.RiskDismissal      `json:"dismissal,omitempty"`
	Exposure            *Exposure                 `json:"exposure,omitempty"`
}

type Exposure struct {
	IPs                  []string `json:"ips"`
	Ports                []string `json:"ports"`
	NodePortServices     []string `json:"node_port_services"`
	LoadBalancerServices []string `json:"load_balancer_services"`
}

func renderRisks(w *model.World) []*Risk {
	var res []*Risk

	nodePublicIPs := utils.StringSet{}
	for _, n := range w.Nodes {
		for _, iface := range n.NetInterfaces {
			for _, addr := range iface.Addresses {
				if ip, err := netaddr.ParseIP(addr); err == nil && utils.IsIpExternal(ip) {
					nodePublicIPs.Add(addr)
				}
			}
		}
	}

	for _, app := range w.Applications {
		if !app.IsDatabase() && !app.IsQueue() {
			continue
		}
		dismissals := map[model.RiskKey]*model.RiskDismissal{}
		if app.Settings != nil {
			for _, ro := range app.Settings.RiskOverrides {
				dismissals[ro.Key] = ro.Dismissal
			}
		}

		exposedPorts := utils.StringSet{}
		publicIPs := utils.StringSet{}
		nodePortServices := utils.StringSet{}
		lbServices := utils.StringSet{}

		for _, s := range app.KubernetesServices {
			switch s.Type.Value() {
			case model.ServiceTypeNodePort:
				nodePortServices.Add(s.Name)
				publicIPs.Add(nodePublicIPs.Items()...)
			case model.ServiceTypeLoadBalancer:
				ips := &utils.StringSet{}
				for _, lbIp := range s.LoadBalancerIPs.Items() {
					if ip, err := netaddr.ParseIP(lbIp); err == nil && utils.IsIpExternal(ip) {
						ips.Add(lbIp)
					}
				}
				if ips.Len() > 0 {
					lbServices.Add(s.Name)
					publicIPs.Add(ips.Items()...)
				}
			}
		}
		for _, i := range app.Instances {
			for l, active := range i.TcpListens {
				if !active || l.Port == "0" {
					continue
				}
				if ip, err := netaddr.ParseIP(l.IP); err == nil && utils.IsIpExternal(ip) {
					publicIPs.Add(l.IP)
					exposedPorts.Add(l.Port)
				}
			}
		}
		if exposedPorts.Len() > 0 || (nodePortServices.Len() > 0 && publicIPs.Len() > 0) || lbServices.Len() > 0 {
			key := model.RiskKey{
				Category: model.RiskCategorySecurity,
				Type:     model.RiskTypeDbInternetExposure,
			}
			dismissal := dismissals[key]
			severity := model.CRITICAL
			if dismissal != nil {
				severity = model.OK
			}
			res = append(res, &Risk{
				Key:                 key,
				ApplicationId:       app.Id,
				ApplicationCategory: app.Category,
				ApplicationType:     getApplicationType(app),
				Severity:            severity,
				Dismissal:           dismissal,
				Exposure: &Exposure{
					IPs:                  publicIPs.Items(),
					Ports:                exposedPorts.Items(),
					NodePortServices:     nodePortServices.Items(),
					LoadBalancerServices: lbServices.Items(),
				},
			})
		}
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].ApplicationId.Name < res[j].ApplicationId.Name
	})
	return res
}
