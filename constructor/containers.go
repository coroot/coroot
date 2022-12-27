package constructor

import (
	"fmt"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/logpattern"
	"k8s.io/klog"
	"net"
	"strconv"
	"strings"
)

type metricContext struct {
	ns        string
	pod       string
	node      *model.Node
	container string
}

func getMetricContext(w *model.World, ls model.Labels) *metricContext {
	mc := &metricContext{node: getNode(w, ls)}
	containerId := ls["container_id"]
	parts := strings.Split(containerId, "/")
	if parts[1] == "k8s" && len(parts) == 5 {
		mc.ns = parts[2]
		mc.pod = parts[3]
		mc.container = parts[4]
	} else {
		mc.container = strings.TrimSuffix(parts[len(parts)-1], ".service")
	}
	return mc
}

func getInstanceByPod(w *model.World, ns, pod string) *model.Instance {
	for _, a := range w.Applications {
		if a.Id.Namespace != ns {
			continue
		}
		for _, i := range a.Instances {
			if i.Pod != nil && i.Name == pod {
				return i
			}
		}
	}
	return nil
}

func loadContainers(w *model.World, metrics map[string][]model.MetricValues) {
	rttByInstance := map[model.InstanceId]map[string]timeseries.TimeSeries{}

	for queryName := range metrics {
		if !strings.HasPrefix(queryName, "container_") {
			continue
		}
		for _, m := range metrics[queryName] {
			mc := getMetricContext(w, m.Labels)
			if mc == nil {
				continue
			}
			var instance *model.Instance
			switch {
			case mc.pod != "" && mc.ns != "":
				w.IntegrationStatus.KubeStateMetrics.Required = true
				if instance = getInstanceByPod(w, mc.ns, mc.pod); instance == nil {
					continue
				}
			case mc.container != "" && mc.node != nil:
				appId := model.NewApplicationId("", model.ApplicationKindUnknown, mc.container)
				instanceName := fmt.Sprintf("%s@%s", mc.container, mc.node.Name.Value())
				instance = w.GetOrCreateApplication(appId).GetOrCreateInstance(instanceName)
			}
			if instance == nil {
				continue
			}
			if instance.Node == nil && mc.node != nil {
				instance.Node = mc.node
				mc.node.Instances = append(mc.node.Instances, instance)
			}
			container := instance.GetOrCreateContainer(mc.container)
			promJobStatus := prometheusJobStatus(metrics, m.Labels["job"], m.Labels["instance"])
			switch queryName {
			case "container_info":
				if image := m.Labels["image"]; image != "" {
					container.Image = image
				}
			case "container_net_latency":
				rtts := rttByInstance[instance.InstanceId]
				if rtts == nil {
					rtts = map[string]timeseries.TimeSeries{}
				}
				rtts[m.Labels["destination_ip"]] = timeseries.Merge(rtts[m.Labels["destination_ip"]], m.Values, timeseries.Any)
				rttByInstance[instance.InstanceId] = rtts
			case "container_net_tcp_successful_connects":
				if c := getOrCreateConnection(instance, mc.container, m, w); c != nil {
					c.Connects = timeseries.Merge(c.Connects, m.Values, timeseries.Any)
				}
			case "container_net_tcp_active_connections":
				if c := getOrCreateConnection(instance, mc.container, m, w); c != nil {
					c.Active = timeseries.Merge(c.Active, m.Values, timeseries.Any)
				}
			case "container_net_tcp_listen_info":
				ip, port, err := net.SplitHostPort(m.Labels["listen_addr"])
				if err != nil {
					klog.Warningf("failed to split %s to ip:port pair: %s", m.Labels["listen_addr"], err)
					continue
				}
				isActive := timeseries.Last(m.Values) == 1
				l := model.Listen{IP: ip, Port: port, Proxied: m.Labels["proxy"] != ""}
				if !instance.TcpListens[l] {
					instance.TcpListens[l] = isActive
				}
			case "container_http_requests_count", "container_postgres_queries_count", "container_redis_queries_count",
				"container_memcached_queries_count", "container_mysql_queries_count", "container_mongo_queries_count",
				"container_kafka_requests_count", "container_cassandra_queries_count", "container_rabbitmq_messages":
				if c := getOrCreateConnection(instance, mc.container, m, w); c != nil {
					protocol := model.Protocol(strings.SplitN(queryName, "_", 3)[1])
					status := m.Labels["status"]
					if protocol == "rabbitmq" {
						protocol += model.Protocol("-" + m.Labels["method"])
					}
					if c.RequestsCount[protocol] == nil {
						c.RequestsCount[protocol] = map[string]timeseries.TimeSeries{}
					}
					c.RequestsCount[protocol][status] = timeseries.Merge(c.RequestsCount[protocol][status], m.Values, timeseries.NanSum)
				}
			case "container_http_requests_latency", "container_postgres_queries_latency", "container_redis_queries_latency",
				"container_memcached_queries_latency", "container_mysql_queries_latency", "container_mongo_queries_latency",
				"container_kafka_requests_latency", "container_cassandra_queries_latency":
				if c := getOrCreateConnection(instance, mc.container, m, w); c != nil {
					protocol := model.Protocol(strings.SplitN(queryName, "_", 3)[1])
					c.RequestsLatency[protocol] = timeseries.Merge(c.RequestsLatency[protocol], m.Values, timeseries.Any)
				}
			case "container_http_requests_histogram", "container_postgres_queries_histogram", "container_redis_queries_histogram",
				"container_memcached_queries_histogram", "container_mysql_queries_histogram", "container_mongo_queries_histogram",
				"container_kafka_requests_histogram", "container_cassandra_queries_histogram":
				if c := getOrCreateConnection(instance, mc.container, m, w); c != nil {
					protocol := model.Protocol(strings.SplitN(queryName, "_", 3)[1])
					le, err := strconv.ParseFloat(m.Labels["le"], 64)
					if err != nil {
						klog.Warningln(err)
						continue
					}
					if c.RequestsHistogram[protocol] == nil {
						c.RequestsHistogram[protocol] = map[float64]timeseries.TimeSeries{}
					}
					c.RequestsHistogram[protocol][le] = timeseries.Merge(c.RequestsHistogram[protocol][le], m.Values, timeseries.NanSum)
				}
			case "container_cpu_limit":
				container.CpuLimit = timeseries.Merge(container.CpuLimit, m.Values, timeseries.Any)
			case "container_cpu_usage":
				container.CpuUsage = timeseries.Merge(container.CpuUsage, m.Values, timeseries.Any)
			case "container_cpu_delay":
				container.CpuDelay = timeseries.Merge(container.CpuDelay, m.Values, timeseries.Any)
			case "container_throttled_time":
				container.ThrottledTime = timeseries.Merge(container.ThrottledTime, m.Values, timeseries.Any)
			case "container_memory_rss":
				container.MemoryRss = timeseries.Merge(container.MemoryRss, m.Values, timeseries.Any)
			case "container_memory_cache":
				container.MemoryCache = timeseries.Merge(container.MemoryCache, m.Values, timeseries.Any)
			case "container_memory_limit":
				container.MemoryLimit = timeseries.Merge(container.MemoryLimit, m.Values, timeseries.Any)
			case "container_oom_kills_total":
				container.OOMKills = timeseries.Merge(container.OOMKills, timeseries.Increase(m.Values, promJobStatus), timeseries.Any)
			case "container_restarts":
				container.Restarts = timeseries.Merge(container.Restarts, timeseries.Increase(m.Values, promJobStatus), timeseries.Any)
			case "container_application_type":
				container.ApplicationTypes[model.ApplicationType(m.Labels["application_type"])] = true
			case "container_log_messages":
				logMessage(instance, m.Labels, timeseries.Increase(m.Values, promJobStatus))
			case "container_volume_size":
				v := getOrCreateInstanceVolume(instance, m)
				v.CapacityBytes = timeseries.Merge(v.CapacityBytes, m.Values, timeseries.Any)
			case "container_volume_used":
				v := getOrCreateInstanceVolume(instance, m)
				v.UsedBytes = timeseries.Merge(v.UsedBytes, m.Values, timeseries.Any)
			case "container_jvm_info", "container_jvm_heap_size_bytes", "container_jvm_heap_used_bytes",
				"container_jvm_gc_time_seconds", "container_jvm_safepoint_sync_time_seconds", "container_jvm_safepoint_time_seconds":
				jvm(instance, queryName, m)
			}
		}
	}

	instancesByListen := map[model.Listen]*model.Instance{}
	for _, app := range w.Applications {
		for _, instance := range app.Instances {
			for l := range instance.TcpListens {
				if ip := net.ParseIP(l.IP); ip.IsLoopback() {
					if instance.Node != nil {
						l.IP = instance.Node.Name.Value()
						instancesByListen[l] = instance
					}
				} else {
					instancesByListen[l] = instance
				}
			}
		}
	}

	for _, app := range w.Applications { // lookup remote instance by listen
		for _, instance := range app.Instances {
			for _, u := range instance.Upstreams {
				l := model.Listen{IP: u.ActualRemoteIP, Port: u.ActualRemotePort, Proxied: true}
				if ip := net.ParseIP(u.ActualRemoteIP); ip.IsLoopback() && instance.Node != nil {
					l.IP = instance.Node.Name.Value()
				}
				if u.RemoteInstance = instancesByListen[l]; u.RemoteInstance == nil {
					l.Proxied = false
					u.RemoteInstance = instancesByListen[l]
				}
				if upstreams, ok := rttByInstance[instance.InstanceId]; ok {
					u.Rtt = timeseries.Merge(u.Rtt, upstreams[u.ActualRemoteIP], timeseries.Any)
				}
			}
		}
	}
	for _, app := range w.Applications { // creating ApplicationKindExternalService for unknown remote instances
		for _, instance := range app.Instances {
			for _, u := range instance.Upstreams {
				if u.RemoteInstance != nil {
					continue
				}
				appId := model.NewApplicationId("", model.ApplicationKindExternalService, "")
				svc := w.GetServiceForConnection(u)
				if svc != nil {
					if id, ok := svc.GetDestinationApplicationId(); ok {
						if a := w.GetApplication(id); a != nil {
							a.Downstreams = append(a.Downstreams, u)
						}
						continue
					} else {
						appId.Name = svc.Name
					}
				} else {
					appId.Name = u.ActualRemoteIP + ":" + u.ActualRemotePort
				}
				ri := w.GetOrCreateApplication(appId).GetOrCreateInstance(appId.Name)
				ri.TcpListens[model.Listen{IP: u.ActualRemoteIP, Port: u.ActualRemotePort}] = true
				u.RemoteInstance = ri
			}
		}
	}
	for _, app := range w.Applications {
		for _, instance := range app.Instances {
			for _, u := range instance.Upstreams {
				if u.RemoteInstance == nil {
					continue
				}
				if a := w.GetApplication(u.RemoteInstance.OwnerId); a != nil {
					a.Downstreams = append(a.Downstreams, u)
				}
			}
		}
	}
}

func getOrCreateConnection(instance *model.Instance, container string, m model.MetricValues, w *model.World) *model.Connection {
	if timeseries.Reduce(timeseries.NanSum, m.Values) == 0 {
		return nil
	}
	if instance.OwnerId.Name == "docker" { // ignore docker-proxy's connections
		return nil
	}
	connection := instance.GetOrCreateUpstreamConnection(m.Labels, container)
	updateServiceEndpoints(w, connection)
	return connection
}

func getOrCreateInstanceVolume(instance *model.Instance, m model.MetricValues) *model.Volume {
	var volume *model.Volume
	for _, v := range instance.Volumes {
		if v.MountPoint == m.Labels["mount_point"] {
			volume = v
			break
		}
	}
	if volume == nil {
		volume = &model.Volume{MountPoint: m.Labels["mount_point"]}
		instance.Volumes = append(instance.Volumes, volume)
	}
	volume.Name.Update(m.Values, m.Labels["volume"])
	volume.Device.Update(m.Values, m.Labels["device"])
	return volume
}

func logMessage(instance *model.Instance, ls model.Labels, values timeseries.TimeSeries) {
	level := model.LogLevel(ls["level"])
	instance.LogMessagesByLevel[level] = timeseries.Merge(instance.LogMessagesByLevel[level], values, timeseries.NanSum)

	if hash := ls["pattern_hash"]; hash != "" {
		p := instance.LogPatterns[hash]
		if p == nil {
			sample := ls["sample"]
			pattern := logpattern.NewPattern(sample)

			p = &model.LogPattern{
				Level:     level,
				Sample:    sample,
				Multiline: strings.Contains(sample, "\n"),
				Pattern:   pattern,
			}
			if p.Multiline {
				p.Sample = markMultilineMessage(p.Sample)
			}
			instance.LogPatterns[hash] = p
		}
		p.Sum = timeseries.Merge(p.Sum, values, timeseries.NanSum)
	}
}

func markMultilineMessage(msg string) string {
	marked := false
	lines := strings.Split(msg, "\n")

	for i, l := range lines {
		if strings.HasPrefix(l, "\tat ") || strings.HasPrefix(l, "\t... ") {
			if i > 0 {
				lines[i-1] = "<mark>" + lines[i-1] + "</mark>"
				marked = true
				break
			}
		}
	}
	if !marked && len(lines) > 1 { //python traceback
		if strings.HasPrefix(lines[len(lines)-2], "    ") {
			lines[len(lines)-1] = "<mark>" + lines[len(lines)-1] + "</mark>"
			marked = true
		}
	}
	return strings.Join(lines, "\n")
}

func updateServiceEndpoints(w *model.World, c *model.Connection) {
	if c.ActualRemoteIP == "" && c.ServiceRemoteIP == "" && c.ServiceRemoteIP == c.ActualRemoteIP {
		return
	}
	for _, s := range w.Services {
		if s.ClusterIP == c.ServiceRemoteIP {
			for _, cc := range s.Connections {
				if c == cc {
					return
				}
			}
			s.Connections = append(s.Connections, c)
		}
	}
}
