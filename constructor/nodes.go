package constructor

import (
	"strconv"
	"strings"

	"k8s.io/klog"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func initNodesList(w *model.World, metrics map[string][]*model.MetricValues, nodesByID map[model.NodeId]*model.Node) {
	nodesBySystemUUID := map[string]*model.Node{}
	for _, m := range metrics["node_info"] {
		name := m.Labels["hostname"]
		id := model.NewNodeId(m.MachineID, m.SystemUUID)
		if id.MachineID == "" && id.SystemUUID == "" {
			klog.Infoln("invalid `node_info` metric: missing `machine_id` and `system_uuid` labels")
			continue
		}
		w.IntegrationStatus.NodeAgent.Installed = true
		node := nodesByID[id]
		if node == nil {
			node = model.NewNode(id)
			w.Nodes = append(w.Nodes, node)
			nodesByID[node.Id] = node
			nodesBySystemUUID[node.Id.SystemUUID] = node
		}
		node.Name.Update(m.Values[0], name)
		node.KernelVersion.Update(m.Values[0], m.Labels["kernel_version"])
	}
	for _, m := range metrics["kube_node_info"] {
		name := m.Labels["node"]
		id := model.NewNodeId(m.MachineID, m.SystemUUID)
		if id.MachineID == "" && id.SystemUUID == "" {
			klog.Infoln("invalid `kube_node_info` metric: missing `system_uuid` label")
			continue
		}
		node := nodesBySystemUUID[id.SystemUUID]
		if node == nil {
			node = model.NewNode(id)
			w.Nodes = append(w.Nodes, node)
			nodesByID[node.Id] = node
			nodesBySystemUUID[node.Id.SystemUUID] = node
		}
		node.K8sName.Update(m.Values[0], name)
		if node.KernelVersion.Value() == "" {
			node.KernelVersion.Update(m.Values[0], m.Labels["kernel_version"])
		}
	}
}

func (c *Constructor) loadNodes(w *model.World, metrics map[string][]*model.MetricValues, nodesByID map[model.NodeId]*model.Node) {
	initNodesList(w, metrics, nodesByID)

	for queryName := range metrics {
		if !strings.HasPrefix(queryName, "node_") {
			continue
		}
		for _, m := range metrics[queryName] {
			node := nodesByID[model.NewNodeId(m.MachineID, m.SystemUUID)]
			if node == nil {
				continue
			}
			switch queryName {
			case "node_agent_info":
				node.AgentVersion.Update(m.Values[0], m.Labels["version"])
			case "node_cpu_cores":
				node.CpuCapacity = merge(node.CpuCapacity, m.Values[0], timeseries.Any)
			case "node_cpu_usage_percent":
				node.CpuUsagePercent = merge(node.CpuUsagePercent, m.Values[0], timeseries.Any)
			case "node_cpu_usage_by_mode":
				node.CpuUsageByMode[m.Labels["mode"]] = merge(node.CpuUsageByMode[m.Labels["mode"]], m.Values[0], timeseries.Any)
			case "node_memory_total_bytes":
				node.MemoryTotalBytes = merge(node.MemoryTotalBytes, m.Values[0], timeseries.Any)
			case "node_memory_available_bytes":
				node.MemoryAvailableBytes = merge(node.MemoryAvailableBytes, m.Values[0], timeseries.Any)
			case "node_memory_cached_bytes":
				node.MemoryCachedBytes = merge(node.MemoryCachedBytes, m.Values[0], timeseries.Any)
			case "node_memory_free_bytes":
				node.MemoryFreeBytes = merge(node.MemoryFreeBytes, m.Values[0], timeseries.Any)
			case "node_cloud_info":
				provider := strings.ToLower(m.Labels["provider"])
				region := m.Labels["region"]
				az := m.Labels["availability_zone"]
				node.CloudProvider.Update(m.Values[0], provider)
				node.Region.Update(m.Values[0], region)
				if _, err := strconv.ParseInt(az, 10, 8); err == nil && provider == model.CloudProviderAzure {
					az = region + "-" + az
				}
				node.AvailabilityZone.Update(m.Values[0], az)
				node.InstanceType.Update(m.Values[0], m.Labels["instance_type"])
				node.InstanceLifeCycle.Update(m.Values[0], m.Labels["instance_life_cycle"])
			case "node_uptime_seconds":
				node.Uptime = merge(node.Uptime, m.Values[0], timeseries.Any)
			default:
				if strings.HasPrefix(queryName, "node_disk_") {
					nodeDisk(node, queryName, m)
				} else if strings.HasPrefix(queryName, "node_net_") {
					nodeInterface(node, queryName, m)
				}
			}
		}
	}
	for _, n := range w.Nodes {
		for _, d := range n.Disks {
			if d.Wait == nil && !d.ReadTime.IsEmpty() && !d.WriteTime.IsEmpty() {
				d.Wait = timeseries.NewAggregate(timeseries.NanSum).Add(d.ReadTime, d.WriteTime).Get()
			}
			switch {
			case d.Await == nil && !d.Wait.IsEmpty(): // node
				d.Await = timeseries.Div(d.Wait, timeseries.NewAggregate(timeseries.NanSum).Add(d.ReadOps, d.WriteOps).Get())
			case d.Wait == nil && !d.Await.IsEmpty(): // rds
				d.Wait = timeseries.Mul(d.Await, timeseries.NewAggregate(timeseries.NanSum).Add(d.ReadOps, d.WriteOps).Get())
			}
		}
	}
	if c.pricing != nil {
		for _, n := range w.Nodes {
			n.Price = c.pricing.GetNodePrice(n)
			n.DataTransferPrice = c.pricing.GetDataTransferPrice(n)
		}
	}
}

func nodeDisk(node *model.Node, queryName string, m *model.MetricValues) {
	device := m.Labels["device"]
	stat := node.Disks[device]
	if stat == nil {
		stat = &model.DiskStats{}
		node.Disks[device] = stat
	}
	switch queryName {
	case "node_disk_read_time":
		stat.ReadTime = merge(stat.ReadTime, m.Values[0], timeseries.Any)
	case "node_disk_write_time":
		stat.WriteTime = merge(stat.WriteTime, m.Values[0], timeseries.Any)
	case "node_disk_reads":
		stat.ReadOps = merge(stat.ReadOps, m.Values[0], timeseries.Any)
	case "node_disk_writes":
		stat.WriteOps = merge(stat.WriteOps, m.Values[0], timeseries.Any)
	case "node_disk_read_bytes":
		stat.ReadBytes = merge(stat.ReadBytes, m.Values[0], timeseries.Any)
	case "node_disk_written_bytes":
		stat.WrittenBytes = merge(stat.WrittenBytes, m.Values[0], timeseries.Any)
	case "node_disk_io_time":
		stat.IOUtilizationPercent = merge(stat.IOUtilizationPercent, m.Values[0].Map(func(t timeseries.Time, v float32) float32 {
			return v * 100
		}), timeseries.Any)
	}
}

func nodeInterface(node *model.Node, queryName string, m *model.MetricValues) {
	name := m.Labels["interface"]
	var stat *model.InterfaceStats
	for _, s := range node.NetInterfaces {
		if s.Name == name {
			stat = s
		}
	}
	if stat == nil {
		stat = &model.InterfaceStats{Name: name}
		node.NetInterfaces = append(node.NetInterfaces, stat)
	}
	switch queryName {
	case "node_net_up":
		stat.Up = merge(stat.Up, m.Values[0], timeseries.Any)
	case "node_net_ip":
		stat.Addresses = append(stat.Addresses, m.Labels["ip"])
	case "node_net_rx_bytes":
		stat.RxBytes = merge(stat.RxBytes, m.Values[0], timeseries.Any)
	case "node_net_tx_bytes":
		stat.TxBytes = merge(stat.TxBytes, m.Values[0], timeseries.Any)
	}
}
