package overview

import (
	"fmt"
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
	Availability        *Availability             `json:"availability,omitempty"`
}

type Exposure struct {
	IPs                  []string `json:"ips"`
	Ports                []string `json:"ports"`
	NodePortServices     []string `json:"node_port_services"`
	LoadBalancerServices []string `json:"load_balancer_services"`
}

type Availability struct {
	Description string `json:"description"`
}

func renderRisks(w *model.World) []*Risk {
	res := dbPortExposures(w)
	res = append(res, availabilityRisks(w)...)

	sort.Slice(res, func(i, j int) bool {
		if res[i].Severity == res[j].Severity {
			return res[i].ApplicationId.Name < res[j].ApplicationId.Name
		}
		return res[i].Severity > res[j].Severity
	})
	return res
}

func availabilityRisks(w *model.World) []*Risk {
	var res []*Risk
	zones := utils.NewStringSet()
	seenOnDemandNodes := false
	for _, n := range w.Nodes {
		if az := n.AvailabilityZone.Value(); az != "" {
			zones.Add(az)
		}
		if lc := n.InstanceLifeCycle.Value(); lc == "on-demand" {
			seenOnDemandNodes = true
		}
	}

	for _, app := range w.Applications {
		switch app.Id.Kind {
		case model.ApplicationKindExternalService, model.ApplicationKindRds, model.ApplicationKindElasticacheCluster,
			model.ApplicationKindJob, model.ApplicationKindCronJob:
			continue
		}
		dismissals := map[model.RiskKey]*model.RiskDismissal{}
		if app.Settings != nil {
			for _, ro := range app.Settings.RiskOverrides {
				dismissals[ro.Key] = ro.Dismissal
			}
		}
		appZones := utils.NewStringSet()
		appNodes := utils.NewStringSet()
		instanceLifeCycles := utils.NewStringSet()
		availableInstances := 0
		for _, i := range app.Instances {
			if !i.IsObsolete() && i.IsUp() {
				availableInstances++
				if i.Node != nil {
					if z := i.Node.AvailabilityZone.Value(); z != "" {
						appZones.Add(z)
					}
					appNodes.Add(i.NodeName())
					lc := i.Node.InstanceLifeCycle.Value()
					if lc == "preemptible" {
						lc = "spot"
					}
					instanceLifeCycles.Add(lc)
				}
			}
		}
		switch {
		case app.IsStandalone():
		case availableInstances == 1 && len(w.Nodes) > 1:
			res = append(res, availabilityRisk(
				app,
				dismissals,
				model.WARNING,
				model.RiskTypeSingleInstanceApp,
				"Single instance - not resilient to node failure",
			))
		case appNodes.Len() == 1 && len(w.Nodes) > 1:
			res = append(res, availabilityRisk(
				app,
				dismissals,
				model.WARNING,
				model.RiskTypeSingleNodeApp,
				"All instances on one node - not resilient to node failure",
			))
		case appZones.Len() == 1 && zones.Len() > 1:
			res = append(res, availabilityRisk(
				app,
				dismissals,
				model.WARNING,
				model.RiskTypeSingleAzApp,
				"All instances in one Availability Zone - failure causes downtime",
			))
		case seenOnDemandNodes && instanceLifeCycles.Len() == 1 && instanceLifeCycles.Items()[0] == "spot":
			res = append(res, availabilityRisk(
				app,
				dismissals,
				model.WARNING,
				model.RiskTypeSpotOnlyApp,
				"All instances on Spot nodes - risk of sudden termination. Add On-Demand",
			))
		}
		appTypes := app.ApplicationTypes()
		for _, t := range []model.ApplicationType{
			model.ApplicationTypeMysql, model.ApplicationTypePostgres,
			model.ApplicationTypeRedis, model.ApplicationTypeDragonfly, model.ApplicationTypeKeyDB, model.ApplicationTypeValkey,
			model.ApplicationTypeMongodb,
			model.ApplicationTypeElasticsearch, model.ApplicationTypeOpensearch,
		} {
			if !appTypes[t] {
				continue
			}
			replicated := false
			for _, u := range app.Upstreams {
				if u.RemoteApplication.ApplicationTypes()[t] {
					replicated = true
				}
			}
			if !replicated {
				for _, u := range app.Downstreams {
					if u.Application.ApplicationTypes()[t] {
						replicated = true
					}
				}
			}
			if availableInstances < 2 && !replicated {
				res = append(res, availabilityRisk(
					app,
					dismissals,
					model.CRITICAL,
					model.RiskTypeUnreplicatedDatabase,
					"%s isn’t replicated - data loss possible",
					utils.Capitalize(string(t)),
				))
			}
		}
	}
	return res
}

func availabilityRisk(app *model.Application, dismissals map[model.RiskKey]*model.RiskDismissal, status model.Status, typ model.RiskType, format string, args ...any) *Risk {
	key := model.RiskKey{
		Category: model.RiskCategoryAvailability,
		Type:     typ,
	}
	dismissal := dismissals[key]
	if dismissal != nil {
		status = model.OK
	}
	return &Risk{
		Key:                 key,
		ApplicationId:       app.Id,
		ApplicationCategory: app.Category,
		ApplicationType:     getApplicationType(app),
		Severity:            status,
		Dismissal:           dismissal,
		Availability: &Availability{
			Description: fmt.Sprintf(format, args...),
		},
	}
}

func dbPortExposures(w *model.World) []*Risk {
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
	return res
}
