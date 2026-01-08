package constructor

import (
	"net"

	"github.com/coroot/coroot/model"
)

func mergeWorlds(worlds []*model.World, checkConfigs model.CheckConfigs) *model.World {
	if len(worlds) == 0 {
		return nil
	}
	if len(worlds) == 1 {
		return worlds[0]
	}
	sock2app := map[string]*model.Application{}
	updateSocketToApplicationMapping(worlds[0], sock2app)
	res := worlds[0]
	res.CheckConfigs = checkConfigs
	res.AWS = model.AWS{}
	res.IntegrationStatus = model.IntegrationStatus{}
	for _, w := range worlds[1:] {
		updateSocketToApplicationMapping(w, sock2app)
		res.Nodes = append(res.Nodes, w.Nodes...)
		for appId, app := range w.Applications {
			dest := res.Applications[appId]
			if dest == nil {
				res.Applications[appId] = app
			} else {
				for id, c := range app.Downstreams {
					dest.Downstreams[id] = c
				}
			}
		}
		for appId, checkConfig := range w.CheckConfigs {
			res.CheckConfigs[appId] = checkConfig
		}
		if w.Flux != nil {
			if res.Flux == nil {
				res.Flux.Merge(w.Flux)
			}
		}
		if w.IntegrationStatus.NodeAgent.Installed {
			res.IntegrationStatus.NodeAgent.Installed = true
		}
		if w.IntegrationStatus.KubeStateMetrics.Required {
			res.IntegrationStatus.KubeStateMetrics.Required = true
		}
		if w.IntegrationStatus.KubeStateMetrics.Installed {
			res.IntegrationStatus.KubeStateMetrics.Installed = true
		}
		for id, name := range w.ProjectNamesById {
			res.ProjectNamesById[id] = name
		}
	}
	for _, app := range res.Applications {
		for _, u := range app.Upstreams {
			destApp := u.RemoteApplication
			if destApp == nil || destApp.Id.Kind != model.ApplicationKindExternalService {
				continue
			}
			for _, ep := range u.Endpoints.Items() {
				if newDest := sock2app[ep]; newDest != nil {
					u.RemoteApplication = newDest
					for id, c := range destApp.Downstreams {
						if c == u {
							delete(destApp.Downstreams, id)
							if len(destApp.Downstreams) == 0 {
								delete(res.Applications, destApp.Id)
							}
						}
					}
				}
			}
		}
	}

	return res
}

func updateSocketToApplicationMapping(w *model.World, mapping map[string]*model.Application) {
	var nodeIPs []string
	for _, node := range w.Nodes {
		for _, iface := range node.NetInterfaces {
			nodeIPs = append(nodeIPs, iface.Addresses...)
		}
	}
	for _, app := range w.Applications {
		for _, i := range app.Instances {
			for l := range i.TcpListens {
				if l.Proxied {
					mapping[net.JoinHostPort(l.IP, l.Port)] = app
				}
			}
		}
		for _, s := range app.KubernetesServices {
			for _, port := range s.Ports.Items() {
				if s.ClusterIP != "" {
					mapping[net.JoinHostPort(s.ClusterIP, port)] = app
				}
				for _, lbIP := range s.LoadBalancerIPs.Items() {
					mapping[net.JoinHostPort(lbIP, port)] = app
				}
			}
			for _, np := range s.NodePorts.Items() {
				for _, ip := range nodeIPs {
					mapping[net.JoinHostPort(ip, np)] = app
				}
			}
		}
	}

}
