package overview

import (
	"sort"
	"strings"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"inet.af/netaddr"
)

const (
	usageTopN = 5
	month     = float32(timeseries.Month)
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
	CrossAzTrafficCosts       float32           `json:"cross_az_traffic_costs"`
	InternetEgressCosts       float32           `json:"internet_egress_costs"`
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

	UsageCosts            float32                 `json:"usage_costs"`
	AllocationCosts       float32                 `json:"allocation_costs"`
	OverProvisioningCosts float32                 `json:"over_provisioning_costs"`
	CrossAzTrafficCosts   float32                 `json:"cross_az_traffic_costs"`
	InternetEgressCosts   float32                 `json:"internet_egress_costs"`
	Components            []*ApplicationComponent `json:"components"`
	Instances             []*ApplicationInstance  `json:"instances"`
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
		if n.Price == nil {
			continue
		}
		nodeApps := map[model.ApplicationId][]*instance{}
		memCached := timeseries.NewAggregate(timeseries.NanSum)
		interZoneIngress := timeseries.NewAggregate(timeseries.NanSum)
		interZoneEgress := timeseries.NewAggregate(timeseries.NanSum)
		internetEgress := timeseries.NewAggregate(timeseries.NanSum)

		for _, i := range n.Instances {
			owner := applicationsIndex[i.Owner.Id]
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
			instanceInterZoneIngress := timeseries.NewAggregate(timeseries.NanSum)
			instanceInterZoneEgress := timeseries.NewAggregate(timeseries.NanSum)
			instanceInternetEgress := timeseries.NewAggregate(timeseries.NanSum)

			for _, c := range i.Containers {
				cpuUsage.Add(c.CpuUsage)
				memUsage.Add(c.MemoryRss)
				memCached.Add(c.MemoryCache)
				cpuRequest.Add(c.CpuRequest)
				memRequest.Add(c.MemoryRequest)
			}
			if i.Rds != nil {
				cpuUsage.Add(
					timeseries.Mul(
						i.Node.CpuUsagePercent.Map(func(t timeseries.Time, v float32) float32 { return v / 100 }),
						i.Node.CpuCapacity,
					),
				)
				memUsage.Add(timeseries.Sub(i.Node.MemoryTotalBytes, i.Node.MemoryFreeBytes))
			}
			for _, u := range i.Upstreams {
				switch trafficType(i, u) {
				case TrafficTypeCrossAZ:
					interZoneEgress.Add(u.BytesSent)
					instanceInterZoneEgress.Add(u.BytesSent)
					interZoneIngress.Add(u.BytesReceived)
					instanceInterZoneIngress.Add(u.BytesReceived)
				case TrafficTypeInternet:
					internetEgress.Add(u.BytesSent)
					instanceInternetEgress.Add(u.BytesSent)
				}
			}
			ii := &instance{
				ownerId:   owner.Id,
				name:      i.Name,
				cpu:       resource{usage: cpuUsage.Get(), request: cpuRequest.Get()},
				memory:    resource{usage: memUsage.Get(), request: memRequest.Get()},
				nodePrice: n.Price,
			}
			if n.DataTransferPrice != nil {
				ii.crossAzTrafficCosts += monthlyTrafficCosts(instanceInterZoneEgress.Get(), n.DataTransferPrice.InterZoneEgressPerGB)
				ii.crossAzTrafficCosts += monthlyTrafficCosts(instanceInterZoneIngress.Get(), n.DataTransferPrice.InterZoneIngressPerGB)
				ii.internetEgressCosts += monthlyTrafficCosts(instanceInternetEgress.Get(), n.DataTransferPrice.GetInternetEgressPrice())
			}

			if _, ok := desiredInstances[owner.Id]; !ok && owner.IsK8s() {
				desiredInstances[owner.Id] = owner.DesiredInstances.Last()
			}
			nodeApps[i.Owner.Id] = append(nodeApps[i.Owner.Id], ii)
			applications[i.Owner.Id] = append(applications[i.Owner.Id], ii)
		}

		nodeCpuUsagePercent := n.CpuUsagePercent.Last()
		if timeseries.IsNaN(nodeCpuUsagePercent) {
			nodeCpuUsagePercent = 0
		}
		nodeMemoryFreeBytes := n.MemoryFreeBytes.Last()
		if timeseries.IsNaN(nodeMemoryFreeBytes) {
			nodeMemoryFreeBytes = 0
		}
		nodeCpuCores := n.CpuCapacity.Last()
		nodeMemoryBytes := n.MemoryTotalBytes.Last()

		nc := &NodeCosts{
			Name:              n.GetName(),
			InstanceLifeCycle: n.InstanceLifeCycle.Value(),
			Description:       strings.Join(getNodeTags(n), " / "),
			Price:             n.Price.Total * month,
		}
		if nodeCpuCores > 0 && nodeMemoryBytes > 0 {
			cpuIdleCost := nodeCpuCores * (1 - nodeCpuUsagePercent/100) * n.Price.PerCPUCore
			memIdleCost := nodeMemoryFreeBytes * n.Price.PerMemoryByte
			nodeAppsCpu := renderNodeApplications(nodeCpuCores, nodeApps, resourceCpu)
			nodeAppsMem := renderNodeApplications(nodeMemoryBytes, nodeApps, resourceMemory)
			nc.CpuUsageApplications = topByUsage(nodeAppsCpu)
			nc.CpuRequestApplications = topByRequest(nodeAppsCpu)
			nc.MemoryUsageApplications = topByUsage(nodeAppsMem)
			nc.MemoryRequestApplications = topByRequest(nodeAppsMem)
			nc.IdleCosts = (cpuIdleCost + memIdleCost) * month
			cached := memCached.Get()
			cachedAvg := cached.Reduce(timeseries.NanSum) / cached.Map(timeseries.Defined).Reduce(timeseries.NanSum)
			if cachedAvg > 0 {
				nc.MemoryUsageApplications = append(nc.MemoryUsageApplications, NodeApplication{
					Name:  "~cached",
					Value: cachedAvg / nodeMemoryBytes * 100,
				})
			}
		}
		if n.DataTransferPrice != nil {
			nc.CrossAzTrafficCosts += monthlyTrafficCosts(interZoneEgress.Get(), n.DataTransferPrice.InterZoneEgressPerGB)
			nc.CrossAzTrafficCosts += monthlyTrafficCosts(interZoneIngress.Get(), n.DataTransferPrice.InterZoneIngressPerGB)
			nc.InternetEgressCosts += monthlyTrafficCosts(internetEgress.Get(), n.DataTransferPrice.GetInternetEgressPrice())
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
				res.UsageCosts += avg * i.nodePrice.PerCPUCore * month
			}
			if u := ai.MemoryUsage.Reduce(timeseries.NanSum); u > 0 {
				up = true
				avg := u / float32(ai.MemoryUsage.Len())
				ai.MemoryUsageAvg = resourceMemory.format(avg)
				memUsageSum += avg
				if avg > memUsageMax {
					memUsageMax = avg
				}
				res.UsageCosts += avg * i.nodePrice.PerMemoryByte * month
			}
			res.CrossAzTrafficCosts += i.crossAzTrafficCosts
			res.InternetEgressCosts += i.internetEgressCosts
			switch i.ownerId.Kind {
			case model.ApplicationKindRds, model.ApplicationKindElasticacheCluster:
				res.UsageCosts += i.nodePrice.Total * month
				res.AllocationCosts += i.nodePrice.Total * month
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
					ac.AllocationCosts += cpuRequest * i.nodePrice.PerCPUCore * month
					ac.AllocationCostsRecommended += cpuRequestRecommended * i.nodePrice.PerCPUCore * month
				}
				if l := i.memory.usage.Last(); l > 0 {
					ac.AllocationCosts += memRequest * i.nodePrice.PerMemoryByte * month
					ac.AllocationCostsRecommended += memRequestRecommended * i.nodePrice.PerMemoryByte * month
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
	if !(total > 0) {
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
				if r := request.Reduce(timeseries.LastNotNaN); r > 0 {
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

func monthlyTrafficCosts(ts *timeseries.TimeSeries, perGBprice float32) float32 {
	if perGBprice > 0 {
		avg := ts.Reduce(timeseries.NanSum) / ts.Map(timeseries.Defined).Reduce(timeseries.NanSum)
		if !timeseries.IsNaN(avg) {
			return avg * month / 1000 / 1000 / 1000 * perGBprice // todo ?
		}
	}
	return 0.
}

type TrafficType uint8

const (
	TrafficTypeUnknown TrafficType = iota
	TrafficTypeCrossAZ
	TrafficTypeSameAZ
	TrafficTypeInternet
)

func trafficType(instance *model.Instance, u *model.Connection) TrafficType {
	if u.RemoteInstance == nil {
		return TrafficTypeUnknown
	}
	if instance.Node == nil {
		return TrafficTypeUnknown
	}
	srcRegion := instance.Node.Region.Value()
	if srcRegion == "" {
		return TrafficTypeUnknown
	}
	if u.RemoteInstance.Node != nil {
		dstRegion := u.RemoteInstance.Node.Region.Value()
		srcAZ := instance.Node.AvailabilityZone.Value()
		dstAZ := u.RemoteInstance.Node.AvailabilityZone.Value()
		if dstRegion != "" && dstRegion == srcRegion && srcAZ != "" && dstAZ != "" {
			if srcAZ == dstAZ {
				return TrafficTypeSameAZ
			} else {
				return TrafficTypeCrossAZ
			}
		}
	}
	if ip, err := netaddr.ParseIP(u.ActualRemoteIP); err == nil && utils.IsIpExternal(ip) {
		return TrafficTypeInternet
	}

	return TrafficTypeUnknown
}
