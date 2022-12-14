package constructor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"strings"
)

func getNode(w *model.World, ls model.Labels) *model.Node {
	machineId := ls["machine_id"]
	for _, node := range w.Nodes {
		if node.MachineID == machineId {
			return node
		}
	}
	return nil
}

func initNodesList(w *model.World, nodeInfoMetrics []model.MetricValues) {
	for _, m := range nodeInfoMetrics {
		name := m.Labels["hostname"]
		machineID := m.Labels["machine_id"]
		if machineID == "" {
			continue
		}
		w.IntegrationStatus.NodeAgent.Installed = true
		var node *model.Node
		for _, n := range w.Nodes {
			if n.MachineID == machineID {
				node = n
				break
			}
		}
		if node == nil {
			node = model.NewNode(machineID)
			w.Nodes = append(w.Nodes, node)
		}
		node.Name.Update(m.Values, name)
	}
}

func loadNodes(w *model.World, metrics map[string][]model.MetricValues) {
	initNodesList(w, metrics["node_info"])

	for queryName := range metrics {
		if !strings.HasPrefix(queryName, "node_") {
			continue
		}
		for _, m := range metrics[queryName] {
			node := getNode(w, m.Labels)
			if node == nil {
				continue
			}
			switch queryName {
			case "node_cpu_cores":
				node.CpuCapacity = timeseries.Merge(node.CpuCapacity, m.Values, timeseries.Any)
			case "node_cpu_usage_percent":
				node.CpuUsagePercent = timeseries.Merge(node.CpuUsagePercent, m.Values, timeseries.Any)
			case "node_cpu_usage_by_mode":
				node.CpuUsageByMode[m.Labels["mode"]] = timeseries.Merge(node.CpuUsageByMode[m.Labels["mode"]], m.Values, timeseries.Any)
			case "node_memory_total_bytes":
				node.MemoryTotalBytes = timeseries.Merge(node.MemoryTotalBytes, m.Values, timeseries.Any)
			case "node_memory_available_bytes":
				node.MemoryAvailableBytes = timeseries.Merge(node.MemoryAvailableBytes, m.Values, timeseries.Any)
			case "node_memory_cached_bytes":
				node.MemoryCachedBytes = timeseries.Merge(node.MemoryCachedBytes, m.Values, timeseries.Any)
			case "node_memory_free_bytes":
				node.MemoryFreeBytes = timeseries.Merge(node.MemoryFreeBytes, m.Values, timeseries.Any)
			case "node_cloud_info":
				node.CloudProvider.Update(m.Values, m.Labels["provider"])
				node.Region.Update(m.Values, m.Labels["region"])
				node.AvailabilityZone.Update(m.Values, m.Labels["availability_zone"])
				node.InstanceType.Update(m.Values, m.Labels["instance_type"])
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
			if d.Wait == nil && !timeseries.IsEmpty(d.ReadTime) && !timeseries.IsEmpty(d.WriteTime) {
				d.Wait = timeseries.Aggregate(timeseries.NanSum, d.ReadTime, d.WriteTime)
			}
			switch {
			case d.Await == nil && !timeseries.IsEmpty(d.Wait): // node
				d.Await = timeseries.Aggregate(timeseries.Div,
					d.Wait,
					timeseries.Aggregate(timeseries.NanSum, d.ReadOps, d.WriteOps),
				)
			case d.Wait == nil && !timeseries.IsEmpty(d.Await): // rds
				d.Wait = timeseries.Aggregate(timeseries.Mul,
					d.Await,
					timeseries.Aggregate(timeseries.NanSum, d.ReadOps, d.WriteOps),
				)
			}

		}
	}

}

func nodeDisk(node *model.Node, queryName string, m model.MetricValues) {
	device := m.Labels["device"]
	stat := node.Disks[device]
	if stat == nil {
		stat = &model.DiskStats{}
		node.Disks[device] = stat
	}
	switch queryName {
	case "node_disk_read_time":
		stat.ReadTime = timeseries.Merge(stat.ReadTime, m.Values, timeseries.Any)
	case "node_disk_write_time":
		stat.WriteTime = timeseries.Merge(stat.WriteTime, m.Values, timeseries.Any)
	case "node_disk_reads":
		stat.ReadOps = timeseries.Merge(stat.ReadOps, m.Values, timeseries.Any)
	case "node_disk_writes":
		stat.WriteOps = timeseries.Merge(stat.WriteOps, m.Values, timeseries.Any)
	case "node_disk_read_bytes":
		stat.ReadBytes = timeseries.Merge(stat.ReadBytes, m.Values, timeseries.Any)
	case "node_disk_written_bytes":
		stat.WrittenBytes = timeseries.Merge(stat.WrittenBytes, m.Values, timeseries.Any)
	case "node_disk_io_time":
		stat.IOUtilizationPercent = timeseries.Merge(stat.IOUtilizationPercent, timeseries.Map(func(t timeseries.Time, v float64) float64 {
			return v * 100
		}, m.Values), timeseries.Any)
	}
}

func nodeInterface(node *model.Node, queryName string, m model.MetricValues) {
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
		stat.Up = timeseries.Merge(stat.Up, m.Values, timeseries.Any)
	case "node_net_ip":
		stat.Addresses = append(stat.Addresses, m.Labels["ip"])
	case "node_net_rx_bytes":
		stat.RxBytes = timeseries.Merge(stat.RxBytes, m.Values, timeseries.Any)
	case "node_net_tx_bytes":
		stat.TxBytes = timeseries.Merge(stat.TxBytes, m.Values, timeseries.Any)
	}
}
