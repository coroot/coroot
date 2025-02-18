package constructor

import (
	"fmt"
	"strings"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func (c *Constructor) loadFargateNodes(metrics map[string][]*model.MetricValues, nodesById map[model.NodeId]*model.Node) {
	for queryName := range metrics {
		if !strings.HasPrefix(queryName, "fargate_node_") {
			continue
		}
		for _, m := range metrics[queryName] {
			id := model.NewNodeIdFromLabels(m)
			if id.MachineID == "" && id.SystemUUID == "" {
				continue
			}
			node := nodesById[id]
			if node == nil {
				continue
			}
			if m.Labels["eks_amazonaws_com_compute_type"] != "fargate" {
				continue
			}
			node.Fargate = true
			node.Name.Update(m.Values, m.Labels["kubernetes_io_hostname"])
			node.CloudProvider.Update(m.Values, model.CloudProviderAWS)
			if region := m.Labels["topology_kubernetes_io_region"]; region != "" {
				node.Region.Update(m.Values, region)
			}
			if az := m.Labels["topology_kubernetes_io_zone"]; az != "" {
				node.Region.Update(m.Values, az)
			}
			switch queryName {
			case "fargate_node_machine_cpu_cores":
				node.CpuCapacity = merge(node.CpuCapacity, m.Values, timeseries.Any)
			case "fargate_node_machine_memory_bytes":
				node.MemoryTotalBytes = merge(node.MemoryTotalBytes, m.Values, timeseries.Any)
			}
		}
	}
}

func loadFargateContainers(w *model.World, metrics map[string][]*model.MetricValues, pjs promJobStatuses) {
	var instances map[instanceId]*model.Instance
	for queryName := range metrics {
		if !strings.HasPrefix(queryName, "fargate_container_") {
			continue
		}
		for _, m := range metrics[queryName] {
			nodeName := m.Labels["kubernetes_io_hostname"]
			if nodeName == "" {
				continue
			}
			ns := m.Labels["namespace"]
			pod := m.Labels["pod"]
			containerName := m.Labels["container"]

			if ns == "" || pod == "" || containerName == "" {
				continue
			}
			if instances == nil {
				instances = map[instanceId]*model.Instance{}
				for _, a := range w.Applications {
					for _, i := range a.Instances {
						if n := i.NodeName(); n != "" {
							instances[instanceId{ns: a.Id.Namespace, name: i.Name, node: model.NewNodeId(n, n)}] = i
						}
					}
				}
			}
			instance := instances[instanceId{ns: ns, name: pod, node: model.NewNodeId(nodeName, nodeName)}]
			if instance == nil {
				continue
			}
			container := instance.GetOrCreateContainer(fmt.Sprintf("/k8s/%s/%s/%s", ns, pod, containerName), containerName)

			switch queryName {
			case "fargate_container_spec_cpu_limit_cores":
				container.CpuLimit = merge(container.CpuLimit, m.Values, timeseries.Any)
			case "fargate_container_cpu_usage_seconds":
				container.CpuUsage = merge(container.CpuUsage, m.Values, timeseries.Any)
			case "fargate_container_cpu_cfs_throttled_seconds":
				container.ThrottledTime = merge(container.ThrottledTime, m.Values, timeseries.Any)
			case "fargate_container_memory_rss":
				container.MemoryRss = merge(container.MemoryRss, m.Values, timeseries.Any)
			case "fargate_container_memory_rss_for_trend":
				container.MemoryRssForTrend = merge(container.MemoryRssForTrend, m.Values, timeseries.Any)
			case "fargate_container_memory_cache":
				container.MemoryCache = merge(container.MemoryCache, m.Values, timeseries.Any)
			case "fargate_container_spec_memory_limit_bytes":
				container.MemoryLimit = merge(container.MemoryLimit, m.Values, timeseries.Any)
			case "fargate_container_oom_events_total":
				container.OOMKills = merge(container.OOMKills, timeseries.Increase(m.Values, pjs.get(m.Labels)), timeseries.Any)
			}
		}
	}
}
