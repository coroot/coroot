package constructor

import (
	"strings"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func ociDBKey(id string) string    { return "ocidb/" + id }
func ociCacheKey(id string) string { return "ocicache/" + id }

func (c *Constructor) loadOCIMetadata(w *model.World, metrics map[string][]*model.MetricValues, cloudInstancesById map[string]*model.Instance, project *db.Project) {
	for _, m := range metrics["oci_db_info"] {
		id, name := m.Labels["oci_db_id"], m.Labels["name"]
		if id == "" || name == "" {
			continue
		}
		instance := cloudInstancesById[ociDBKey(id)]
		if instance == nil {
			// read replicas and standby instances are grouped with their primary into one application
			appName := name
			if primary := m.Labels["primary"]; primary != "" {
				appName = primary
			}
			appId := c.newApplicationId(project.ClusterId(), "", model.ApplicationKindOCIDB, appName)
			instance = w.GetOrCreateApplication(appId, false).GetOrCreateInstance(name, nil)
			cloudInstancesById[ociDBKey(id)] = instance
			instance.OCIDB = &model.OCIDB{Id: id}
		}
		if instance.Node == nil {
			nodeName := "ocidb:" + name
			instance.Node = model.NewNode(string(project.Id), model.NewNodeId(nodeName, nodeName))
			instance.Node.Name.Update(m.Values, nodeName)
			instance.Node.Instances = append(instance.Node.Instances, instance)
			w.Nodes = append(w.Nodes, instance.Node)
		}
		if len(instance.Volumes) == 0 {
			instance.Volumes = append(instance.Volumes, &model.Volume{MountPoint: "/", EBS: &model.EBS{}})
		}
		instance.Volumes[0].Device.Update(m.Values, "data")
		// the database process is the only workload of the instance: a container carries its CPU and memory usage
		instance.GetOrCreateContainer("ocidb:"+name, "db")
		instance.TcpListens[model.Listen{IP: m.Labels["ipv4"], Port: m.Labels["port"]}] = true
		if ip := m.Labels["ipv4"]; ip != "" && len(instance.Node.NetInterfaces) == 0 {
			instance.Node.NetInterfaces = append(instance.Node.NetInterfaces, &model.InterfaceStats{
				Name: "eth0", Addresses: []string{ip}, Up: m.Values.WithNewValue(1),
			})
		}
		instance.OCIDB.Engine.Update(m.Values, m.Labels["engine"])
		instance.OCIDB.EngineVersion.Update(m.Values, m.Labels["engine_version"])
		instance.Node.InstanceType.Update(m.Values, m.Labels["shape"])
		instance.Node.CloudProvider.Update(m.Values, model.CloudProviderOCI)
		instance.Node.Region.Update(m.Values, m.Labels["region"])
		instance.Node.AvailabilityZone.Update(m.Values, m.Labels["availability_domain"])
	}

	for _, m := range metrics["oci_cache_info"] {
		id, name := m.Labels["oci_cache_id"], m.Labels["name"]
		if id == "" || name == "" {
			continue
		}
		instance := cloudInstancesById[ociCacheKey(id)]
		if instance == nil {
			appId := c.newApplicationId(project.ClusterId(), "", model.ApplicationKindOCICache, name)
			instance = w.GetOrCreateApplication(appId, false).GetOrCreateInstance(name, nil)
			cloudInstancesById[ociCacheKey(id)] = instance
			instance.OCICache = &model.OCICache{Id: id}
		}
		if instance.Node == nil {
			nodeName := "ocicache:" + name
			instance.Node = model.NewNode(string(project.Id), model.NewNodeId(nodeName, nodeName))
			instance.Node.Name.Update(m.Values, nodeName)
			instance.Node.Instances = append(instance.Node.Instances, instance)
			w.Nodes = append(w.Nodes, instance.Node)
		}
		instance.TcpListens[model.Listen{IP: m.Labels["ipv4"], Port: m.Labels["port"]}] = true
		if ip := m.Labels["ipv4"]; ip != "" && len(instance.Node.NetInterfaces) == 0 {
			instance.Node.NetInterfaces = append(instance.Node.NetInterfaces, &model.InterfaceStats{
				Name: "eth0", Addresses: []string{ip}, Up: m.Values.WithNewValue(1),
			})
		}
		instance.GetOrCreateContainer("ocicache:"+name, "db")
		instance.OCICache.Engine.Update(m.Values, m.Labels["engine"])
		instance.OCICache.EngineVersion.Update(m.Values, m.Labels["engine_version"])
		instance.Node.CloudProvider.Update(m.Values, model.CloudProviderOCI)
		instance.Node.Region.Update(m.Values, m.Labels["region"])
	}
}

func (c *Constructor) loadOCI(w *model.World, metrics map[string][]*model.MetricValues, pjs promJobStatuses, cloudInstancesById map[string]*model.Instance) {
	for _, q := range QUERIES {
		switch {
		case strings.HasPrefix(q.Name, "oci_db_") && q.Name != "oci_db_info":
			for _, m := range metrics[q.Name] {
				instance := cloudInstancesById[ociDBKey(m.Labels["oci_db_id"])]
				if instance == nil {
					continue
				}
				node, volume, container := instance.Node, instance.Volumes[0], instance.Containers["db"]
				switch q.Name {
				case "oci_db_status":
					instance.OCIDB.LifeSpan = merge(instance.OCIDB.LifeSpan, m.Values, timeseries.Any)
					instance.OCIDB.Status.Update(m.Values, m.Labels["status"])
				case "oci_db_cpu_cores":
					node.CpuCapacity = merge(node.CpuCapacity, m.Values, timeseries.Any)
				case "oci_db_cpu_usage_percent":
					node.CpuUsagePercent = merge(node.CpuUsagePercent, m.Values, timeseries.Any)
				case "oci_db_cpu_usage_cores":
					container.CpuUsage = merge(container.CpuUsage, m.Values, timeseries.Any)
				case "oci_db_memory_total_bytes":
					node.MemoryTotalBytes = merge(node.MemoryTotalBytes, m.Values, timeseries.Any)
				case "oci_db_memory_used_bytes":
					container.MemoryRss = merge(container.MemoryRss, m.Values, timeseries.Any)
				case "oci_db_memory_usage_percent": // PostgreSQL reports the utilization only
					if container.MemoryRss == nil && node.MemoryTotalBytes != nil {
						container.MemoryRss = timeseries.Mul(m.Values.Map(func(t timeseries.Time, v float32) float32 { return v / 100 }), node.MemoryTotalBytes)
					}
				case "oci_db_disk_total_bytes":
					volume.CapacityBytes = merge(volume.CapacityBytes, m.Values, timeseries.Any)
				case "oci_db_disk_used_bytes":
					volume.UsedBytes = merge(volume.UsedBytes, m.Values, timeseries.Any)
				case "oci_db_network_bytes_per_second":
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
				case "oci_db_io_ops_per_second", "oci_db_io_bytes_per_second", "oci_db_io_latency_seconds":
					device := volume.Device.Value()
					stat := node.Disks[device]
					if stat == nil {
						stat = &model.DiskStats{}
						node.Disks[device] = stat
					}
					switch q.Name + "/" + m.Labels["operation"] {
					case "oci_db_io_ops_per_second/read":
						stat.ReadOps = merge(stat.ReadOps, m.Values, timeseries.Any)
					case "oci_db_io_ops_per_second/write":
						stat.WriteOps = merge(stat.WriteOps, m.Values, timeseries.Any)
					case "oci_db_io_bytes_per_second/read":
						stat.ReadBytes = merge(stat.ReadBytes, m.Values, timeseries.Any)
					case "oci_db_io_bytes_per_second/write":
						stat.WrittenBytes = merge(stat.WrittenBytes, m.Values, timeseries.Any)
					case "oci_db_io_latency_seconds/read": // the time spent on I/O, as the node-agent reports it; the ops are loaded before (see QUERIES)
						stat.ReadTime = merge(stat.ReadTime, timeseries.Mul(m.Values, stat.ReadOps), timeseries.Any)
					case "oci_db_io_latency_seconds/write":
						stat.WriteTime = merge(stat.WriteTime, timeseries.Mul(m.Values, stat.WriteOps), timeseries.Any)
					}
				case "oci_db_log_messages_total":
					logMessage(instance, m, pjs)
				}
			}
		case strings.HasPrefix(q.Name, "oci_cache_") && q.Name != "oci_cache_info":
			for _, m := range metrics[q.Name] {
				instance := cloudInstancesById[ociCacheKey(m.Labels["oci_cache_id"])]
				if instance == nil {
					continue
				}
				node, container := instance.Node, instance.Containers["db"]
				switch q.Name {
				case "oci_cache_status":
					instance.OCICache.LifeSpan = merge(instance.OCICache.LifeSpan, m.Values, timeseries.Any)
					instance.OCICache.Status.Update(m.Values, m.Labels["status"])
				case "oci_cache_memory_total_bytes":
					node.MemoryTotalBytes = merge(node.MemoryTotalBytes, m.Values, timeseries.Any)
				case "oci_cache_cpu_usage_percent":
					node.CpuUsagePercent = merge(node.CpuUsagePercent, m.Values, timeseries.Any)
				case "oci_cache_memory_used_bytes": // the engine is the only memory consumer, no cache is reported
					container.MemoryRss = merge(container.MemoryRss, m.Values, timeseries.Any)
					node.MemoryFreeBytes = timeseries.Sub(node.MemoryTotalBytes, container.MemoryRss)
					node.MemoryAvailableBytes = node.MemoryFreeBytes
					node.MemoryCachedBytes = timeseries.Sub(container.MemoryRss, container.MemoryRss)
				case "oci_cache_network_bytes_per_second":
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
				case "oci_cache_log_messages_total":
					logMessage(instance, m, pjs)
				}
			}
		}
	}
	for key, instance := range cloudInstancesById {
		if !strings.HasPrefix(key, "ocidb/") {
			continue
		}
		node, container := instance.Node, instance.Containers["db"]
		if node.MemoryTotalBytes != nil && container.MemoryRss != nil {
			node.MemoryFreeBytes = timeseries.Sub(node.MemoryTotalBytes, container.MemoryRss)
			node.MemoryAvailableBytes = node.MemoryFreeBytes
			node.MemoryCachedBytes = timeseries.Sub(container.MemoryRss, container.MemoryRss)
		}
		if container.CpuUsage == nil && node.CpuUsagePercent != nil && node.CpuCapacity != nil { // PostgreSQL reports the utilization only
			container.CpuUsage = timeseries.Mul(node.CpuUsagePercent.Map(func(t timeseries.Time, v float32) float32 { return v / 100 }), node.CpuCapacity)
		}
	}
}
