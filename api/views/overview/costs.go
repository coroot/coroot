package overview

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"sort"
	"strings"
)

const (
	month     = 24 * 30
	usageTopN = 5
)

type Costs struct {
	Nodes        []*NodeCosts        `json:"nodes"`
	Applications []*ApplicationCosts `json:"applications"`
}

type NodeCosts struct {
	Name                      string            `json:"name"`
	InstanceLifeCycle         string            `json:"instance_life_cycle"`
	Description               string            `json:"description"`
	CpuUsage                  float32           `json:"cpu_usage"`
	CpuUsageApplications      []NodeApplication `json:"cpu_usage_applications"`
	CpuRequestApplications    []NodeApplication `json:"cpu_request_applications"`
	MemoryUsage               float32           `json:"memory_usage"`
	MemoryUsageApplications   []NodeApplication `json:"memory_usage_applications"`
	MemoryRequestApplications []NodeApplication `json:"memory_request_applications"`
	Price                     float32           `json:"price"`
	IdleCosts                 float32           `json:"idle_costs"`
}

type NodeApplication struct {
	Name      string                    `json:"name"`
	Value     float32                   `json:"value"`
	Instances []NodeApplicationInstance `json:"instances"`
}

type NodeApplicationInstance struct {
	Name    string                 `json:"name"`
	Usage   string                 `json:"usage"`
	Request string                 `json:"request"`
	Chart   *timeseries.TimeSeries `json:"chart"`
}

type ApplicationCosts struct {
	Id       model.ApplicationId       `json:"id"`
	Category model.ApplicationCategory `json:"category"`

	UsageCosts            float32 `json:"usage_costs"`
	AllocationCosts       float32 `json:"allocation_costs"`
	OverProvisioningCosts float32 `json:"over_provisioning_costs"`

	Applications []*ApplicationComponent `json:"applications"`
	Instances    []*ApplicationInstance  `json:"instances"`
}

type ApplicationComponent struct {
	Name string                `json:"name"`
	Kind model.ApplicationKind `json:"kind"`

	CpuRequest               string `json:"cpu_request"`
	CpuRequestRecommended    string `json:"cpu_request_recommended"`
	MemoryRequest            string `json:"memory_request"`
	MemoryRequestRecommended string `json:"memory_request_recommended"`

	AllocationCosts            float32 `json:"allocation_costs"`
	AllocationCostsRecommended float32 `json:"allocation_costs_recommended"`
}

type ApplicationInstance struct {
	Name           string                 `json:"name"`
	CpuUsage       *timeseries.TimeSeries `json:"cpu_usage"`
	CpuUsageAvg    string                 `json:"cpu_usage_avg"`
	MemoryUsage    *timeseries.TimeSeries `json:"memory_usage"`
	MemoryUsageAvg string                 `json:"memory_usage_avg"`
}

func calcCosts(w *model.World) *Costs {
	res := &Costs{}

	applications := map[model.ApplicationId]*model.Application{}
	for _, app := range w.Applications {
		applications[app.Id] = app
	}
	desiredInstances := map[model.ApplicationId]float32{}
	worldInstances := map[model.ApplicationId][]*instance{}
	for _, n := range w.Nodes {
		price := n.GetPriceBreakdown()
		if price == nil {
			continue
		}

		nodeApps := map[model.ApplicationId][]*instance{}
		for _, i := range n.Instances {
			owner := applications[i.OwnerId]
			if owner == nil {
				continue
			}
			if i.ClusterComponent != nil {
				owner = i.ClusterComponent
			}
			cpuUsage := timeseries.NewAggregate(timeseries.NanSum)
			memUsage := timeseries.NewAggregate(timeseries.NanSum)
			cpuRequest := timeseries.NewAggregate(timeseries.NanSum)
			memRequest := timeseries.NewAggregate(timeseries.NanSum)
			for _, c := range i.Containers {
				cpuUsage.Add(c.CpuUsage)
				memUsage.Add(c.MemoryRss, c.MemoryCache)
				cpuRequest.Add(c.CpuRequest)
				memRequest.Add(c.MemoryRequest)
			}
			ii := &instance{
				ownerId: owner.Id,
				name:    i.Name,
				cpu:     resource{usage: cpuUsage.Get(), request: cpuRequest.Get()},
				memory:  resource{usage: memUsage.Get(), request: memRequest.Get()},
				price:   price,
			}
			if _, ok := desiredInstances[owner.Id]; !ok && owner.IsK8s() {
				desiredInstances[owner.Id] = owner.DesiredInstances.Last()
			}
			nodeApps[i.OwnerId] = append(nodeApps[i.OwnerId], ii)
			worldInstances[i.OwnerId] = append(worldInstances[i.OwnerId], ii)
		}

		cpuIdleCost := n.CpuCapacity.Last() * (1 - n.CpuUsagePercent.Last()/100) * price.CPUPerCore
		memIdleCost := n.MemoryFreeBytes.Last() * price.MemoryPerByte
		nc := &NodeCosts{
			Name:                      n.Name.Value(),
			InstanceLifeCycle:         n.InstanceLifeCycle.Value(),
			Description:               strings.Join(getNodeTags(n), " / "),
			CpuUsageApplications:      renderNodeApplications(n.CpuCapacity.Last(), nodeApps, resourceCpu, "usage"),
			CpuRequestApplications:    renderNodeApplications(n.CpuCapacity.Last(), nodeApps, resourceCpu, "request"),
			MemoryUsageApplications:   renderNodeApplications(n.MemoryTotalBytes.Last(), nodeApps, resourceMemory, "usage"),
			MemoryRequestApplications: renderNodeApplications(n.MemoryTotalBytes.Last(), nodeApps, resourceMemory, "request"),
			Price:                     n.PricePerHour * month,
			IdleCosts:                 (cpuIdleCost + memIdleCost) * month,
		}
		for _, a := range nc.CpuUsageApplications {
			nc.CpuUsage += a.Value
		}
		for _, a := range nc.MemoryUsageApplications {
			nc.MemoryUsage += a.Value
		}
		res.Nodes = append(res.Nodes, nc)
	}

	for appId, appInstances := range worldInstances {
		a := &ApplicationCosts{
			Id:       appId,
			Category: applications[appId].Category,
		}

		byComponent := map[model.ApplicationId][]*instance{}
		for _, i := range appInstances {
			byComponent[i.ownerId] = append(byComponent[i.ownerId], i)
		}
		for ownerId, instances := range byComponent {
			component := &ApplicationComponent{
				Name: ownerId.Name,
				Kind: ownerId.Kind,
			}
			var cpuUsageSum, memUsageSum float32
			var cpuUsageMax, memUsageMax float32
			var cpuRequest, memRequest float32
			for _, i := range instances {
				ii := &ApplicationInstance{
					Name:        i.name,
					CpuUsage:    i.cpu.usage,
					MemoryUsage: i.memory.usage,
				}
				var up bool
				if u := ii.CpuUsage.Reduce(timeseries.NanSum); u > 0 {
					up = true
					avg := u / float32(ii.CpuUsage.Len())
					ii.CpuUsageAvg = resourceCpu.format(avg)
					cpuUsageSum += avg
					if avg > cpuUsageMax {
						cpuUsageMax = avg
					}
					a.UsageCosts += avg * i.price.CPUPerCore * month
				}
				if u := ii.MemoryUsage.Reduce(timeseries.NanSum); u > 0 {
					up = true
					avg := u / float32(ii.MemoryUsage.Len())
					ii.MemoryUsageAvg = resourceMemory.format(avg)
					memUsageSum += avg
					if avg > memUsageMax {
						memUsageMax = avg
					}
					a.UsageCosts += avg * i.price.MemoryPerByte * month
				}
				if !up {
					continue
				}
				if v := i.cpu.request.Last(); v > cpuRequest {
					cpuRequest = v
				}
				if v := i.memory.request.Last(); v > memRequest {
					memRequest = v
				}
				a.Instances = append(a.Instances, ii)
			}
			if cpuRequest > 0 {
				component.CpuRequest = resourceCpu.format(cpuRequest)
			}
			if memRequest > 0 {
				component.MemoryRequest = resourceMemory.format(memRequest)
			}
			if desired := desiredInstances[ownerId]; desired > 0 {
				cpuUsage := cpuUsageSum / desired
				if cpuUsageMax > cpuUsage {
					cpuUsage = cpuUsageMax
				}
				memUsage := memUsageSum / desired
				if memUsageMax > memUsage {
					memUsage = memUsageMax
				}
				cpuRequestRecommended := resourceCpu.suggestRequest(cpuUsage)
				component.CpuRequestRecommended = resourceCpu.format(cpuRequestRecommended)
				memRequestRecommended := resourceMemory.suggestRequest(memUsage)
				component.MemoryRequestRecommended = resourceMemory.format(memRequestRecommended)
				for _, i := range instances {
					if l := i.cpu.usage.Last(); l > 0 {
						component.AllocationCosts += cpuRequest * i.price.CPUPerCore * month
						component.AllocationCostsRecommended += cpuRequestRecommended * i.price.CPUPerCore * month
					}
					if l := i.memory.usage.Last(); l > 0 {
						component.AllocationCosts += memRequest * i.price.MemoryPerByte * month
						component.AllocationCostsRecommended += memRequestRecommended * i.price.MemoryPerByte * month
					}
				}
			}
			a.Applications = append(a.Applications, component)
			a.AllocationCosts += component.AllocationCosts
			a.OverProvisioningCosts += component.AllocationCosts - component.AllocationCostsRecommended
		}
		sort.Slice(a.Applications, func(i, j int) bool { return a.Applications[i].Name < a.Applications[j].Name })
		sort.Slice(a.Instances, func(i, j int) bool { return a.Instances[i].Name < a.Instances[j].Name })
		res.Applications = append(res.Applications, a)
	}

	return res
}

func renderNodeApplications(total float32, apps map[model.ApplicationId][]*instance, rt resourceType, usageOrRequest string) []NodeApplication {
	if timeseries.IsNaN(total) {
		return nil
	}

	var res []NodeApplication
	for app, instances := range apps {
		nodeApplication := NodeApplication{Name: app.Name}
		for _, i := range instances {
			usage := i.getResource(rt).usage
			request := i.getResource(rt).request
			if u := usage.Reduce(timeseries.NanSum); u > 0 {
				applicationInstance := NodeApplicationInstance{Name: i.name}
				avg := u / usage.Map(timeseries.Defined).Reduce(timeseries.NanSum)
				applicationInstance.Usage = rt.format(avg)
				applicationInstance.Chart = usage
				if r := request.Reduce(timeseries.LastNotNaN); !timeseries.IsNaN(r) {
					applicationInstance.Request = rt.format(r)
				}
				switch usageOrRequest {
				case "usage":
					nodeApplication.Value += u * 100 / float32(usage.Len()) / total
				case "request":
					if r := request.Reduce(timeseries.NanSum); r > 0 {
						nodeApplication.Value += r * 100 / float32(request.Len()) / total
					}
				}
				nodeApplication.Instances = append(nodeApplication.Instances, applicationInstance)
			}
		}
		sort.Slice(nodeApplication.Instances, func(i, j int) bool {
			return nodeApplication.Instances[i].Name < nodeApplication.Instances[j].Name
		})
		res = append(res, nodeApplication)
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].Value > res[j].Value
	})
	if len(res) > usageTopN {
		var s float32
		for _, r := range res[usageTopN:] {
			s += r.Value
		}
		res = append(res[:usageTopN], NodeApplication{Name: "~other", Value: s})
	}
	return res
}
