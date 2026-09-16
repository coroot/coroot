package constructor

import (
	"strings"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func cloudSQLKey(id string) string    { return "cloudsql/" + id }
func memorystoreKey(id string) string { return "memorystore/" + id }

func (c *Constructor) loadGCPMetadata(w *model.World, metrics map[string][]*model.MetricValues, gcpInstancesById map[string]*model.Instance, project *db.Project) {
	for _, m := range metrics["gcp_cloudsql_info"] {
		id := m.Labels["cloudsql_instance_id"]
		if id == "" {
			continue
		}
		instance := gcpInstancesById[cloudSQLKey(id)]
		if instance == nil {
			parts := strings.SplitN(id, "/", 2) // <project>/<instance>
			if len(parts) != 2 {
				continue
			}
			// read replicas are grouped with their primary into one application
			appName := parts[1]
			if primary := m.Labels["primary_instance"]; primary != "" {
				appName = primary
			}
			appId := c.newApplicationId(project.ClusterId(), "", model.ApplicationKindCloudSQL, appName)
			instance = w.GetOrCreateApplication(appId, false).GetOrCreateInstance(parts[1], nil)
			gcpInstancesById[cloudSQLKey(id)] = instance
			instance.CloudSQL = &model.CloudSQL{Id: id}
		}
		if instance.Node == nil {
			name := "cloudsql:" + instance.Name
			instance.Node = model.NewNode(string(project.Id), model.NewNodeId(name, name))
			instance.Node.Name.Update(m.Values, name)
			instance.Node.Instances = append(instance.Node.Instances, instance)
			w.Nodes = append(w.Nodes, instance.Node)
		}
		if len(instance.Volumes) == 0 {
			instance.Volumes = append(instance.Volumes, &model.Volume{MountPoint: "/", EBS: &model.EBS{}})
		}
		instance.Volumes[0].Device.Update(m.Values, "data")
		// the database process is the only workload of the instance: a container carries its CPU and memory usage
		instance.GetOrCreateContainer("cloudsql:"+instance.Name, "db")
		instance.TcpListens[model.Listen{IP: m.Labels["ipv4"], Port: m.Labels["port"]}] = true
		if ip := m.Labels["ipv4"]; ip != "" && len(instance.Node.NetInterfaces) == 0 {
			instance.Node.NetInterfaces = append(instance.Node.NetInterfaces, &model.InterfaceStats{
				Name: "eth0", Addresses: []string{ip}, Up: m.Values.WithNewValue(1),
			})
		}
		instance.CloudSQL.Engine.Update(m.Values, m.Labels["engine"])
		instance.CloudSQL.EngineVersion.Update(m.Values, m.Labels["engine_version"])
		instance.CloudSQL.AvailabilityType.Update(m.Values, m.Labels["availability_type"])
		instance.Node.InstanceType.Update(m.Values, m.Labels["tier"])
		instance.Node.CloudProvider.Update(m.Values, model.CloudProviderGCP)
		instance.Node.Region.Update(m.Values, m.Labels["region"])
		instance.Node.AvailabilityZone.Update(m.Values, m.Labels["zone"])
	}

	for _, m := range metrics["gcp_memorystore_info"] {
		id := m.Labels["memorystore_instance_id"]
		if id == "" {
			continue
		}
		instance := gcpInstancesById[memorystoreKey(id)]
		if instance == nil {
			// <project>/<region>/<instance> for Redis and Valkey, <project>/<region>/<instance>/<node> for Memcached
			parts := strings.Split(id, "/")
			if len(parts) < 3 {
				continue
			}
			appName, instanceName := parts[2], parts[2]
			if len(parts) == 4 {
				instanceName = parts[2] + "-" + parts[3]
			}
			appId := c.newApplicationId(project.ClusterId(), "", model.ApplicationKindMemorystore, appName)
			instance = w.GetOrCreateApplication(appId, false).GetOrCreateInstance(instanceName, nil)
			gcpInstancesById[memorystoreKey(id)] = instance
			instance.Memorystore = &model.Memorystore{Id: id}
		}
		if instance.Node == nil {
			name := "memorystore:" + instance.Name
			instance.Node = model.NewNode(string(project.Id), model.NewNodeId(name, name))
			instance.Node.Name.Update(m.Values, name)
			instance.Node.Instances = append(instance.Node.Instances, instance)
			w.Nodes = append(w.Nodes, instance.Node)
		}
		instance.TcpListens[model.Listen{IP: m.Labels["ipv4"], Port: m.Labels["port"]}] = true
		if ip := m.Labels["ipv4"]; ip != "" && len(instance.Node.NetInterfaces) == 0 {
			instance.Node.NetInterfaces = append(instance.Node.NetInterfaces, &model.InterfaceStats{
				Name: "eth0", Addresses: []string{ip}, Up: m.Values.WithNewValue(1),
			})
		}
		instance.GetOrCreateContainer("memorystore:"+instance.Name, "db")
		instance.Memorystore.Engine.Update(m.Values, m.Labels["engine"])
		instance.Memorystore.EngineVersion.Update(m.Values, m.Labels["engine_version"])
		instance.Node.InstanceType.Update(m.Values, m.Labels["tier"]) // the tier (Redis), the node type (Valkey) or empty (Memcached)
		instance.Node.CloudProvider.Update(m.Values, model.CloudProviderGCP)
		instance.Node.Region.Update(m.Values, m.Labels["region"])
		instance.Node.AvailabilityZone.Update(m.Values, m.Labels["zone"])
	}
}

func (c *Constructor) loadGCP(w *model.World, metrics map[string][]*model.MetricValues, pjs promJobStatuses, gcpInstancesById map[string]*model.Instance) {
	for _, q := range QUERIES { // in the order of QUERIES: the totals come before the metrics derived from them
		switch {
		case strings.HasPrefix(q.Name, "gcp_cloudsql_") && q.Name != "gcp_cloudsql_info":
			for _, m := range metrics[q.Name] {
				instance := gcpInstancesById[cloudSQLKey(m.Labels["cloudsql_instance_id"])]
				if instance == nil {
					continue
				}
				node := instance.Node
				volume := instance.Volumes[0]
				container := instance.Containers["db"]
				switch q.Name {
				case "gcp_cloudsql_cpu_usage_cores":
					container.CpuUsage = merge(container.CpuUsage, m.Values, timeseries.Any)
				case "gcp_cloudsql_memory_components_percent": // percents of the quota
					bytes := timeseries.Mul(m.Values.Map(func(t timeseries.Time, v float32) float32 { return v / 100 }), node.MemoryTotalBytes)
					switch m.Labels["component"] {
					case "free":
						node.MemoryFreeBytes = merge(node.MemoryFreeBytes, bytes, timeseries.Any)
					case "cache":
						node.MemoryCachedBytes = merge(node.MemoryCachedBytes, bytes, timeseries.Any)
					}
					node.MemoryAvailableBytes = timeseries.Sum(node.MemoryFreeBytes, node.MemoryCachedBytes)
				case "gcp_cloudsql_status":
					instance.CloudSQL.LifeSpan = merge(instance.CloudSQL.LifeSpan, m.Values, timeseries.Any)
					instance.CloudSQL.Status.Update(m.Values, m.Labels["status"])
				case "gcp_cloudsql_cpu_usage_percent":
					node.CpuUsagePercent = merge(node.CpuUsagePercent, m.Values, timeseries.Any)
				case "gcp_cloudsql_cpu_cores":
					node.CpuCapacity = merge(node.CpuCapacity, m.Values, timeseries.Any)
				case "gcp_cloudsql_memory_total_bytes":
					node.MemoryTotalBytes = merge(node.MemoryTotalBytes, m.Values, timeseries.Any)
				case "gcp_cloudsql_memory_used_bytes":
					container.MemoryRss = merge(container.MemoryRss, m.Values, timeseries.Any)
				case "gcp_cloudsql_disk_total_bytes":
					volume.CapacityBytes = merge(volume.CapacityBytes, m.Values, timeseries.Any)
				case "gcp_cloudsql_disk_used_bytes":
					volume.UsedBytes = merge(volume.UsedBytes, m.Values, timeseries.Any)
				case "gcp_cloudsql_network_bytes_per_second":
					if len(node.NetInterfaces) == 0 {
						continue
					}
					stat := node.NetInterfaces[0]
					switch m.Labels["direction"] {
					case "rx":
						stat.RxBytes = merge(stat.RxBytes, m.Values, timeseries.Any)
					case "tx":
						stat.TxBytes = merge(stat.TxBytes, m.Values, timeseries.Any)
					}
				case "gcp_cloudsql_io_ops_per_second", "gcp_cloudsql_io_bytes_per_second":
					device := volume.Device.Value()
					stat := node.Disks[device]
					if stat == nil {
						stat = &model.DiskStats{}
						node.Disks[device] = stat
					}
					switch q.Name + "/" + m.Labels["operation"] {
					case "gcp_cloudsql_io_ops_per_second/read":
						stat.ReadOps = merge(stat.ReadOps, m.Values, timeseries.Any)
					case "gcp_cloudsql_io_ops_per_second/write":
						stat.WriteOps = merge(stat.WriteOps, m.Values, timeseries.Any)
					case "gcp_cloudsql_io_bytes_per_second/read":
						stat.ReadBytes = merge(stat.ReadBytes, m.Values, timeseries.Any)
					case "gcp_cloudsql_io_bytes_per_second/write":
						stat.WrittenBytes = merge(stat.WrittenBytes, m.Values, timeseries.Any)
					}
				case "gcp_cloudsql_log_messages_total":
					logMessage(instance, m, pjs)
				}
			}
		case strings.HasPrefix(q.Name, "gcp_memorystore_") && q.Name != "gcp_memorystore_info":
			for _, m := range metrics[q.Name] {
				instance := gcpInstancesById[memorystoreKey(m.Labels["memorystore_instance_id"])]
				if instance == nil {
					continue
				}
				node, container := instance.Node, instance.Containers["db"]
				switch q.Name {
				case "gcp_memorystore_status":
					instance.Memorystore.LifeSpan = merge(instance.Memorystore.LifeSpan, m.Values, timeseries.Any)
					instance.Memorystore.Status.Update(m.Values, m.Labels["status"])
				case "gcp_memorystore_cpu_cores":
					node.CpuCapacity = merge(node.CpuCapacity, m.Values, timeseries.Any)
				case "gcp_memorystore_memory_total_bytes":
					node.MemoryTotalBytes = merge(node.MemoryTotalBytes, m.Values, timeseries.Any)
				case "gcp_memorystore_cpu_usage_percent":
					node.CpuUsagePercent = merge(node.CpuUsagePercent, m.Values, timeseries.Any)
				case "gcp_memorystore_cpu_usage_cores":
					container.CpuUsage = merge(container.CpuUsage, m.Values, timeseries.Any)
					if node.CpuUsagePercent == nil { // Memcached reports the CPU time only
						node.CpuUsagePercent = timeseries.Div(container.CpuUsage, node.CpuCapacity).Map(func(t timeseries.Time, v float32) float32 { return v * 100 })
					}
				case "gcp_memorystore_memory_used_bytes": // the engine is the only memory consumer, no cache is reported
					container.MemoryRss = merge(container.MemoryRss, m.Values, timeseries.Any)
					node.MemoryFreeBytes = timeseries.Sub(node.MemoryTotalBytes, container.MemoryRss)
					node.MemoryAvailableBytes = node.MemoryFreeBytes
					node.MemoryCachedBytes = timeseries.Sub(container.MemoryRss, container.MemoryRss)
				case "gcp_memorystore_network_bytes_per_second":
					if len(node.NetInterfaces) == 0 {
						continue
					}
					stat := node.NetInterfaces[0]
					switch m.Labels["direction"] {
					case "rx":
						stat.RxBytes = merge(stat.RxBytes, m.Values, timeseries.Any)
					case "tx":
						stat.TxBytes = merge(stat.TxBytes, m.Values, timeseries.Any)
					}
				}
			}
		}
	}
}
