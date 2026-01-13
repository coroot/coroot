package collector

import (
	"errors"
	"fmt"
	"net/http"
	"sort"

	"github.com/coroot/coroot/constructor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"golang.org/x/exp/maps"
	"inet.af/netaddr"
	"k8s.io/klog"
)

type ApplicationInstrumentation struct {
	Type        model.ApplicationType `json:"type"`
	Host        string                `json:"host"`
	Port        string                `json:"port"`
	Credentials model.Credentials     `json:"credentials"`
	Params      map[string]string     `json:"params"`
	Instance    string                `json:"instance"`
}

type ConfigData struct {
	ApplicationInstrumentation []ApplicationInstrumentation `json:"application_instrumentation"`

	AWSConfig *db.IntegrationAWS `json:"aws_config"`
}

func (c *Collector) Config(w http.ResponseWriter, r *http.Request) {
	project, err := c.getProject(r.Header.Get(ApiKeyHeader))
	if err != nil {
		klog.Errorln(err)
		if errors.Is(err, ErrProjectNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if project.Multicluster() {
		klog.Warningf("an attempt to get config for the multicluster project %s", project.Id)
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	cacheClient := c.cache.GetCacheClient(project.Id)
	cacheTo, err := cacheClient.GetTo()
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if cacheTo.IsZero() {
		return
	}
	to := cacheTo
	from := to.Add(-timeseries.Hour)
	step, err := cacheClient.GetStep(from, to)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	ctr := constructor.New(c.db, project, map[db.ProjectId]constructor.Cache{project.Id: cacheClient}, nil)
	world, err := ctr.LoadWorld(r.Context(), from, to, step, nil)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var res ConfigData

	res.AWSConfig = project.Settings.Integrations.AWS

	for _, app := range world.Applications {
		instancesByType := map[model.ApplicationType]map[*model.Instance]bool{}
		if app.Id.Kind == model.ApplicationKindExternalService {
			appTypes := app.ApplicationTypes()
			for t := range appTypes {
				if instancesByType[t] == nil {
					instancesByType[t] = map[*model.Instance]bool{}
				}
				for _, i := range app.Instances {
					instancesByType[t][i] = true
				}
			}
		} else {
			for _, instance := range app.Instances {
				if instance.IsObsolete() {
					continue
				}
				for t := range instance.ApplicationTypes() {
					if instancesByType[t] == nil {
						instancesByType[t] = map[*model.Instance]bool{}
					}
					instancesByType[t][instance] = true
				}
			}
		}

		for t := range app.ApplicationTypes() {
			it := t.InstrumentationType()
			var instrumentation *model.ApplicationInstrumentation
			if app.Settings != nil && app.Settings.Instrumentation != nil && app.Settings.Instrumentation[it] != nil {
				instrumentation = app.Settings.Instrumentation[it]
			}
			if instrumentation == nil {
				continue
			}
			if instrumentation.Enabled != nil && !*instrumentation.Enabled {
				continue
			}

			for instance := range instancesByType[t] {
				ips := map[string]netaddr.IP{}
				for listen := range instance.TcpListens {
					if listen.Port == instrumentation.Port {
						if ip, err := netaddr.ParseIP(listen.IP); err == nil {
							ips[listen.IP] = ip
						}
					}
				}
				if ip := SelectIP(maps.Values(ips)); ip != nil {
					i := ApplicationInstrumentation{
						Type:        instrumentation.Type,
						Host:        ip.String(),
						Port:        instrumentation.Port,
						Credentials: instrumentation.Credentials,
						Params:      instrumentation.Params,
					}
					owner := instance.Owner
					i.Instance = fmt.Sprintf("app=%s instance=%s node=%s", owner.Id.Name, instance.Name, instance.NodeName())
					if owner.Id.Namespace != "_" {
						i.Instance = fmt.Sprintf("ns=%s %s", owner.Id.Namespace, i.Instance)
					}
					res.ApplicationInstrumentation = append(res.ApplicationInstrumentation, i)
				}
			}
		}
	}

	utils.WriteJson(w, res)
}

func SelectIP(ips []netaddr.IP) *netaddr.IP {
	if len(ips) == 0 {
		return nil
	}

	if len(ips) == 1 {
		return &ips[0]
	}

	type weightedIp struct {
		ip     netaddr.IP
		weight int
	}

	weightedIps := make([]weightedIp, 0, len(ips))
	for _, ip := range ips {
		rank := 5
		switch {
		case ip.IsLoopback():
			rank = 10
		case utils.IsIpDocker(ip):
			rank = 9
		case utils.IsIpPrivate(ip):
			rank = 1
			if ip.Is6() {
				rank = 2
			}
		case ip.Is6():
			rank = 6
		}
		weightedIps = append(weightedIps, weightedIp{ip, rank})
	}

	sort.Slice(weightedIps, func(i, j int) bool {
		ip1, ip2 := weightedIps[i], weightedIps[j]
		if ip1.weight == ip2.weight {
			return ip1.ip.String() < ip2.ip.String()
		}
		return ip1.weight < ip2.weight
	})

	return &weightedIps[0].ip
}
