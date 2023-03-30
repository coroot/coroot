package overview

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"sort"
	"strings"
)

const (
	month     = 24 * 30
	gb        = 1e9
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
	usage     float32
	request   float32
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

	Components []*ApplicationComponent `json:"components"`
	Instances  []*ApplicationInstance  `json:"instances"`
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

func renderCosts(w *model.World) *Costs {
	res := &Costs{}

	applicationsIndex := map[model.ApplicationId]*model.Application{}
	for _, app := range w.Applications {
		applicationsIndex[app.Id] = app
	}
	applications := map[model.ApplicationId][]*instance{}
	desiredInstances := map[model.ApplicationId]float32{}
	for _, n := range w.Nodes {
		nodeCpuCores := n.CpuCapacity.Last()
		nodeMemoryBytes := n.MemoryTotalBytes.Last()
		if n.PricePerHour == 0 || timeseries.IsNaN(nodeCpuCores) || timeseries.IsNaN(nodeMemoryBytes) {
			continue
		}
		// assume that 1Gb of memory costs the same as 1 vCPU
		pricePerUnit := n.PricePerHour / (nodeCpuCores + nodeMemoryBytes/gb)
		cpuPricePerCore := pricePerUnit
		memPricePerByte := pricePerUnit / gb

		nodeApps := map[model.ApplicationId][]*instance{}
		for _, i := range n.Instances {
			owner := applicationsIndex[i.OwnerId]
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
				ownerId:         owner.Id,
				name:            i.Name,
				cpu:             resource{usage: cpuUsage.Get(), request: cpuRequest.Get()},
				memory:          resource{usage: memUsage.Get(), request: memRequest.Get()},
				cpuPricePerCore: cpuPricePerCore,
				memPricePerByte: memPricePerByte,
			}
			if _, ok := desiredInstances[owner.Id]; !ok && owner.IsK8s() {
				desiredInstances[owner.Id] = owner.DesiredInstances.Last()
			}
			nodeApps[i.OwnerId] = append(nodeApps[i.OwnerId], ii)
			applications[i.OwnerId] = append(applications[i.OwnerId], ii)
		}

		cpuIdleCost := nodeCpuCores * (1 - n.CpuUsagePercent.Last()/100) * cpuPricePerCore
		memIdleCost := n.MemoryFreeBytes.Last() * memPricePerByte
		nodeAppsCpu := renderNodeApplications(nodeCpuCores, nodeApps, resourceCpu)
		nodeAppsMem := renderNodeApplications(nodeMemoryBytes, nodeApps, resourceMemory)
		nc := &NodeCosts{
			Name:                      n.Name.Value(),
			InstanceLifeCycle:         n.InstanceLifeCycle.Value(),
			Description:               strings.Join(getNodeTags(n), " / "),
			CpuUsageApplications:      topByUsage(nodeAppsCpu),
			CpuRequestApplications:    topByRequest(nodeAppsCpu),
			MemoryUsageApplications:   topByUsage(nodeAppsMem),
			MemoryRequestApplications: topByRequest(nodeAppsMem),
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

	for appId, appInstances := range applications {
		ac := renderApplicationCosts(appInstances, desiredInstances)
		ac.Id = appId
		ac.Category = applicationsIndex[appId].Category
		res.Applications = append(res.Applications, ac)
	}

	return res
}

func renderApplicationCosts(appInstances []*instance, desiredInstances map[model.ApplicationId]float32) *ApplicationCosts {
	res := &ApplicationCosts{}
	byComponent := map[model.ApplicationId][]*instance{}
	for _, i := range appInstances {
		byComponent[i.ownerId] = append(byComponent[i.ownerId], i)
	}
	for componentId, componentInstances := range byComponent {
		var cpuUsageSum, memUsageSum float32
		var cpuUsageMax, memUsageMax float32
		var cpuRequest, memRequest float32
		for _, i := range componentInstances {
			ai := &ApplicationInstance{
				Name:        i.name,
				CpuUsage:    i.cpu.usage,
				MemoryUsage: i.memory.usage,
			}
			var up bool
			if u := ai.CpuUsage.Reduce(timeseries.NanSum); u > 0 {
				up = true
				avg := u / float32(ai.CpuUsage.Len())
				ai.CpuUsageAvg = resourceCpu.format(avg)
				cpuUsageSum += avg
				if avg > cpuUsageMax {
					cpuUsageMax = avg
				}
				res.UsageCosts += avg * i.cpuPricePerCore * month
			}
			if u := ai.MemoryUsage.Reduce(timeseries.NanSum); u > 0 {
				up = true
				avg := u / float32(ai.MemoryUsage.Len())
				ai.MemoryUsageAvg = resourceMemory.format(avg)
				memUsageSum += avg
				if avg > memUsageMax {
					memUsageMax = avg
				}
				res.UsageCosts += avg * i.memPricePerByte * month
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
			res.Instances = append(res.Instances, ai)
		}

		ac := &ApplicationComponent{
			Name:          componentId.Name,
			Kind:          componentId.Kind,
			CpuRequest:    resourceCpu.format(cpuRequest),
			MemoryRequest: resourceMemory.format(memRequest),
		}
		if desired := desiredInstances[componentId]; desired > 0 {
			cpuUsage := cpuUsageSum / desired
			if cpuUsageMax > cpuUsage {
				cpuUsage = cpuUsageMax
			}
			memUsage := memUsageSum / desired
			if memUsageMax > memUsage {
				memUsage = memUsageMax
			}
			cpuRequestRecommended := resourceCpu.suggestRequest(cpuUsage)
			ac.CpuRequestRecommended = resourceCpu.format(cpuRequestRecommended)
			memRequestRecommended := resourceMemory.suggestRequest(memUsage)
			ac.MemoryRequestRecommended = resourceMemory.format(memRequestRecommended)
			for _, i := range componentInstances {
				if l := i.cpu.usage.Last(); l > 0 {
					ac.AllocationCosts += cpuRequest * i.cpuPricePerCore * month
					ac.AllocationCostsRecommended += cpuRequestRecommended * i.cpuPricePerCore * month
				}
				if l := i.memory.usage.Last(); l > 0 {
					ac.AllocationCosts += memRequest * i.memPricePerByte * month
					ac.AllocationCostsRecommended += memRequestRecommended * i.memPricePerByte * month
				}
			}
		}
		res.Components = append(res.Components, ac)
		res.AllocationCosts += ac.AllocationCosts
		res.OverProvisioningCosts += ac.AllocationCosts - ac.AllocationCostsRecommended
	}
	sort.Slice(res.Components, func(i, j int) bool { return res.Components[i].Name < res.Components[j].Name })
	sort.Slice(res.Instances, func(i, j int) bool { return res.Instances[i].Name < res.Instances[j].Name })
	return res
}

func renderNodeApplications(total float32, apps map[model.ApplicationId][]*instance, rt resourceType) []NodeApplication {
	if timeseries.IsNaN(total) {
		return nil
	}
	var res []NodeApplication
	for app, instances := range apps {
		a := NodeApplication{Name: app.Name}
		for _, i := range instances {
			usage := i.getResource(rt).usage
			request := i.getResource(rt).request
			if u := usage.Reduce(timeseries.NanSum); u > 0 {
				ai := NodeApplicationInstance{Name: i.name}
				avg := u / usage.Map(timeseries.Defined).Reduce(timeseries.NanSum)
				ai.Usage = rt.format(avg)
				ai.Chart = usage
				if r := request.Reduce(timeseries.LastNotNaN); !timeseries.IsNaN(r) {
					ai.Request = rt.format(r)
				}
				a.Instances = append(a.Instances, ai)
				a.usage += u * 100 / float32(usage.Len()) / total
				if r := request.Reduce(timeseries.NanSum); r > 0 {
					a.request += r * 100 / float32(request.Len()) / total
				}
			}
		}
		sort.Slice(a.Instances, func(i, j int) bool {
			return a.Instances[i].Name < a.Instances[j].Name
		})
		res = append(res, a)
	}
	return res
}

func topBy(apps []NodeApplication, by func(a NodeApplication) float32) []NodeApplication {
	sort.Slice(apps, func(i, j int) bool {
		return by(apps[i]) > by(apps[j])
	})
	res := make([]NodeApplication, 0, usageTopN+1)
	var s float32
	for i, a := range apps {
		v := by(a)
		if i < usageTopN {
			a.Value = v
			res = append(res, a)
		} else {
			s += v
		}
	}
	if s > 0 {
		res = append(res, NodeApplication{Name: "~other", Value: s})
	}
	return res
}

func topByUsage(apps []NodeApplication) []NodeApplication {
	return topBy(apps, func(a NodeApplication) float32 { return a.usage })
}

func topByRequest(apps []NodeApplication) []NodeApplication {
	return topBy(apps, func(a NodeApplication) float32 { return a.request })
}
