package constructor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"strconv"
	"strings"
)

func getOrCreateRdsInstance(w *model.World, rdsId string) *model.Instance {
	parts := strings.SplitN(rdsId, "/", 2)
	if len(parts) != 2 {
		return nil
	}
	id := model.NewApplicationId("", model.ApplicationKindRds, parts[1])
	return w.GetOrCreateApplication(id).GetOrCreateInstance(parts[1])
}

func loadRds(w *model.World, metrics map[string][]model.MetricValues) {
	for queryName := range QUERIES {
		if !strings.HasPrefix(queryName, "aws_rds_") {
			continue
		}
		for _, m := range metrics[queryName] {
			rdsId := m.Labels["rds_instance_id"]
			if rdsId == "" {
				continue
			}
			instance := getOrCreateRdsInstance(w, rdsId)
			if instance.Rds == nil {
				instance.Rds = &model.Rds{}
			}
			if instance.Node == nil {
				instance.Node = model.NewNode("rds:" + instance.Name)
				instance.Node.Name.Update(m.Values, "rds:"+instance.Name)
				instance.Node.Instances = append(instance.Node.Instances, instance)
				w.Nodes = append(w.Nodes, instance.Node)
			}
			if len(instance.Volumes) == 0 { // the RDS instance should have only one volume
				instance.Volumes = append(instance.Volumes, &model.Volume{
					MountPoint: "/rdsdbdata",
					EBS:        &model.EBS{},
				})
			}
			volume := instance.Volumes[0]
			promJobStatus := prometheusJobStatus(metrics, m.Labels["job"], m.Labels["instance"])
			switch queryName {
			case "aws_rds_info":
				instance.TcpListens[model.Listen{IP: m.Labels["ipv4"], Port: m.Labels["port"]}] = true
				instance.Rds.Engine.Update(m.Values, m.Labels["engine"])
				instance.Rds.EngineVersion.Update(m.Values, m.Labels["engine_version"])
				instance.Node.InstanceType.Update(m.Values, m.Labels["instance_type"])
				volume.EBS.StorageType.Update(m.Values, m.Labels["storage_type"])
				instance.Node.CloudProvider.Update(m.Values, "aws")
				instance.Node.Region.Update(m.Values, m.Labels["region"])
				instance.Node.AvailabilityZone.Update(m.Values, m.Labels["availability_zone"])
				instance.Rds.MultiAz, _ = strconv.ParseBool(m.Labels["multi_az"])
			case "aws_rds_status":
				instance.Rds.LifeSpan = timeseries.Merge(instance.Rds.LifeSpan, m.Values, timeseries.Any)
				instance.Rds.Status.Update(m.Values, m.Labels["status"])
			case "aws_rds_cpu_cores":
				instance.Node.CpuCapacity = timeseries.Merge(instance.Node.CpuCapacity, m.Values, timeseries.Any)
			case "aws_rds_cpu_usage_percent":
				instance.Node.CpuUsagePercent = timeseries.Merge(instance.Node.CpuUsagePercent, m.Values, timeseries.NanSum)
				mode := m.Labels["mode"]
				instance.Node.CpuUsageByMode[mode] = timeseries.Merge(instance.Node.CpuUsageByMode[mode], m.Values, timeseries.Any)
			case "aws_rds_memory_total_bytes":
				instance.Node.MemoryTotalBytes = timeseries.Merge(instance.Node.MemoryTotalBytes, m.Values, timeseries.Any)
			case "aws_rds_memory_cached_bytes":
				instance.Node.MemoryCachedBytes = timeseries.Merge(instance.Node.MemoryCachedBytes, m.Values, timeseries.Any)
				instance.Node.MemoryAvailableBytes = timeseries.Merge(instance.Node.MemoryAvailableBytes, m.Values, timeseries.NanSum)
			case "aws_rds_memory_free_bytes":
				instance.Node.MemoryFreeBytes = timeseries.Merge(instance.Node.MemoryFreeBytes, m.Values, timeseries.Any)
				instance.Node.MemoryAvailableBytes = timeseries.Merge(instance.Node.MemoryAvailableBytes, m.Values, timeseries.NanSum)
			case "aws_rds_storage_provisioned_iops":
				volume.EBS.ProvisionedIOPS = timeseries.Merge(volume.EBS.ProvisionedIOPS, m.Values, timeseries.Any)
			case "aws_rds_allocated_storage_gibibytes":
				volume.EBS.AllocatedGibs = timeseries.Merge(volume.EBS.AllocatedGibs, m.Values, timeseries.Any)
			case "aws_rds_fs_total_bytes":
				volume.CapacityBytes = timeseries.Merge(volume.CapacityBytes, m.Values, timeseries.Any)
			case "aws_rds_fs_used_bytes":
				volume.UsedBytes = timeseries.Merge(volume.UsedBytes, m.Values, timeseries.Any)
			case "aws_rds_io_await_seconds", "aws_rds_io_ops_per_second", "aws_rds_io_util_percent":
				volume.Device.Update(m.Values, m.Labels["device"])
				device := m.Labels["device"]
				stat := instance.Node.Disks[device]
				if stat == nil {
					stat = &model.DiskStats{}
					instance.Node.Disks[device] = stat
				}
				switch queryName {
				case "aws_rds_io_util_percent":
					stat.IOUtilizationPercent = timeseries.Merge(stat.IOUtilizationPercent, m.Values, timeseries.Any)
				case "aws_rds_io_await_seconds":
					stat.Await = timeseries.Merge(stat.Await, m.Values, timeseries.Any)
				case "aws_rds_io_ops_per_second":
					switch m.Labels["operation"] {
					case "read":
						stat.ReadOps = timeseries.Merge(stat.ReadOps, m.Values, timeseries.Any)
					case "write":
						stat.WriteOps = timeseries.Merge(stat.WriteOps, m.Values, timeseries.Any)
					}
				}
			case "aws_rds_log_messages_total":
				logMessage(instance, m.Labels, timeseries.Increase(m.Values, promJobStatus))
			case "aws_rds_net_rx_bytes_per_second", "aws_rds_net_tx_bytes_per_second":
				name := m.Labels["interface"]
				var stat *model.InterfaceStats
				for _, s := range instance.Node.NetInterfaces {
					if s.Name == name {
						stat = s
					}
				}
				if stat == nil {
					stat = &model.InterfaceStats{Name: name}
					instance.Node.NetInterfaces = append(instance.Node.NetInterfaces, stat)
				}
				switch queryName {
				case "aws_rds_net_rx_bytes_per_second":
					stat.RxBytes = timeseries.Merge(stat.RxBytes, m.Values, timeseries.Any)
				case "aws_rds_net_tx_bytes_per_second":
					stat.TxBytes = timeseries.Merge(stat.TxBytes, m.Values, timeseries.Any)
				}
			}
		}
	}

}
