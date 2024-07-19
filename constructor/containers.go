package constructor

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/coroot/logparser"
	"k8s.io/klog"
)

type instanceId struct {
	ns   string
	name string
	node model.NodeId
}

func (c *Constructor) getInstanceAndContainer(w *model.World, node *model.Node, instances map[instanceId]*model.Instance, containerId string) (*model.Instance, *model.Container) {
	var nodeId model.NodeId
	var nodeName string
	if node != nil {
		nodeId = node.Id
		nodeName = node.GetName()
	}
	if !strings.HasPrefix(containerId, "/") {
		klog.Warningln("invalid container id:", containerId)
		return nil, nil
	}
	parts := strings.Split(containerId, "/")
	var instance *model.Instance
	var containerName string
	if len(parts) == 5 && parts[1] == "k8s" {
		w.IntegrationStatus.KubeStateMetrics.Required = true
		ns, pod := parts[2], parts[3]
		containerName = parts[4]
		instance = instances[instanceId{ns: ns, name: pod, node: nodeId}]
		if instance == nil {
			return nil, nil
		}
		return instance, instance.GetOrCreateContainer(containerId, containerName)
	}

	var (
		id    instanceId
		appId model.ApplicationId
	)
	if len(parts) == 7 && parts[1] == "nomad" {
		ns, job, group, allocId, task := parts[2], parts[3], parts[4], parts[5], parts[6]
		containerName = task
		appId = model.NewApplicationId(ns, model.ApplicationKindNomadJobGroup, job+"."+group)
		id = instanceId{ns: ns, name: group + "-" + allocId, node: nodeId}
	} else {
		if len(parts) == 5 && parts[1] == "swarm" {
			id.ns = parts[2]
			appId = model.NewApplicationId(id.ns, model.ApplicationKindDockerSwarmService, parts[3])
			containerName = parts[3]
			id.name = parts[3] + "." + parts[4]
		} else {
			containerName = strings.TrimSuffix(
				strings.TrimSuffix(parts[len(parts)-1], ".service"),
				".slice")
			id.name = fmt.Sprintf("%s@%s", containerName, nodeName)
			appId = model.NewApplicationId("", model.ApplicationKindUnknown, containerName)
		}
	}
	if id.name == "" {
		return nil, nil
	}
	if id.ns == "" {
		id.ns = "_"
	}
	id.node = nodeId
	instance = instances[id]
	if instance == nil {
		customApp := c.project.GetCustomApplicationName(id.name)
		if customApp != "" {
			appId.Name = customApp
		}
		instance = w.GetOrCreateApplication(appId, customApp != "").GetOrCreateInstance(id.name, node)
		instances[id] = instance
	}
	return instance, instance.GetOrCreateContainer(containerId, containerName)
}

func (c *Constructor) loadContainers(w *model.World, metrics map[string][]model.MetricValues, pjs promJobStatuses, nodesByID map[model.NodeId]*model.Node, servicesByClusterIP map[string]*model.Service, ip2fqdn map[string]*utils.StringSet) {
	instances := map[instanceId]*model.Instance{}
	for _, a := range w.Applications {
		for _, i := range a.Instances {
			var nodeId model.NodeId
			if i.Node != nil {
				nodeId = i.Node.Id
			}
			instances[instanceId{ns: a.Id.Namespace, name: i.Name, node: nodeId}] = i
		}
	}

	servicesByActualDestIP := map[string]*model.Service{}

	connectionCache := map[connectionKey]*model.Connection{}
	rttByInstance := map[instanceId]map[string]*timeseries.TimeSeries{}
	for queryName := range metrics {
		if !strings.HasPrefix(queryName, "container_") {
			continue
		}
		for _, m := range metrics[queryName] {
			nodeId := model.NewNodeIdFromLabels(m.Labels)
			instance, container := c.getInstanceAndContainer(w, nodesByID[nodeId], instances, m.Labels["container_id"])
			if instance == nil || container == nil {
				continue
			}
			switch queryName {
			case "container_info":
				if image := m.Labels["image"]; image != "" {
					container.Image = image
				}
				if strings.HasSuffix(m.Labels["systemd_triggered_by"], ".timer") {
					container.PeriodicSystemdJob = true
				}
			case "container_net_latency":
				id := instanceId{ns: instance.OwnerId.Namespace, name: instance.Name, node: instance.NodeId()}
				rtts := rttByInstance[id]
				if rtts == nil {
					rtts = map[string]*timeseries.TimeSeries{}
				}
				rtts[m.Labels["destination_ip"]] = merge(rtts[m.Labels["destination_ip"]], m.Values, timeseries.Any)
				rttByInstance[id] = rtts
			case "container_net_tcp_successful_connects":
				if c := getOrCreateConnection(instance, container.Name, m, connectionCache, servicesByClusterIP, servicesByActualDestIP); c != nil {
					c.SuccessfulConnections = merge(c.SuccessfulConnections, m.Values, timeseries.Any)
				}
			case "container_net_tcp_connection_time_seconds":
				if c := getOrCreateConnection(instance, container.Name, m, connectionCache, servicesByClusterIP, servicesByActualDestIP); c != nil {
					c.ConnectionTime = merge(c.ConnectionTime, m.Values, timeseries.Any)
				}
			case "container_net_tcp_failed_connects":
				if c := getOrCreateConnection(instance, container.Name, m, connectionCache, servicesByClusterIP, servicesByActualDestIP); c != nil {
					c.FailedConnections = merge(c.FailedConnections, m.Values, timeseries.Any)
				}
			case "container_net_tcp_active_connections":
				if c := getOrCreateConnection(instance, container.Name, m, connectionCache, servicesByClusterIP, servicesByActualDestIP); c != nil {
					c.Active = merge(c.Active, m.Values, timeseries.Any)
				}
			case "container_net_tcp_listen_info":
				ip, port, err := net.SplitHostPort(m.Labels["listen_addr"])
				if err != nil {
					klog.Warningf("failed to split %s to ip:port pair: %s", m.Labels["listen_addr"], err)
					continue
				}
				isActive := m.Values.Last() == 1
				l := model.Listen{IP: ip, Port: port, Proxied: m.Labels["proxy"] != ""}
				if !instance.TcpListens[l] {
					instance.TcpListens[l] = isActive
				}
			case "container_net_tcp_retransmits":
				if c := getOrCreateConnection(instance, container.Name, m, connectionCache, servicesByClusterIP, servicesByActualDestIP); c != nil {
					c.Retransmissions = merge(c.Retransmissions, m.Values, timeseries.Any)
				}
			case "container_http_requests_count", "container_postgres_queries_count", "container_redis_queries_count",
				"container_memcached_queries_count", "container_mysql_queries_count", "container_mongo_queries_count",
				"container_kafka_requests_count", "container_cassandra_queries_count",
				"container_rabbitmq_messages", "container_nats_messages":
				if c := getOrCreateConnection(instance, container.Name, m, connectionCache, servicesByClusterIP, servicesByActualDestIP); c != nil {
					protocol := model.Protocol(strings.SplitN(queryName, "_", 3)[1])
					status := m.Labels["status"]
					if protocol == "rabbitmq" || protocol == "nats" {
						protocol += model.Protocol("-" + m.Labels["method"])
					}
					if c.RequestsCount[protocol] == nil {
						c.RequestsCount[protocol] = map[string]*timeseries.TimeSeries{}
					}
					c.RequestsCount[protocol][status] = merge(c.RequestsCount[protocol][status], m.Values, timeseries.NanSum)
				}
			case "container_http_requests_latency", "container_postgres_queries_latency", "container_redis_queries_latency",
				"container_memcached_queries_latency", "container_mysql_queries_latency", "container_mongo_queries_latency",
				"container_kafka_requests_latency", "container_cassandra_queries_latency":
				if c := getOrCreateConnection(instance, container.Name, m, connectionCache, servicesByClusterIP, servicesByActualDestIP); c != nil {
					protocol := model.Protocol(strings.SplitN(queryName, "_", 3)[1])
					c.RequestsLatency[protocol] = merge(c.RequestsLatency[protocol], m.Values, timeseries.Any)
				}
			case "container_http_requests_histogram", "container_postgres_queries_histogram", "container_redis_queries_histogram",
				"container_memcached_queries_histogram", "container_mysql_queries_histogram", "container_mongo_queries_histogram",
				"container_kafka_requests_histogram", "container_cassandra_queries_histogram":
				if c := getOrCreateConnection(instance, container.Name, m, connectionCache, servicesByClusterIP, servicesByActualDestIP); c != nil {
					protocol := model.Protocol(strings.SplitN(queryName, "_", 3)[1])
					le, err := strconv.ParseFloat(m.Labels["le"], 32)
					if err != nil {
						klog.Warningln(err)
						continue
					}
					if c.RequestsHistogram[protocol] == nil {
						c.RequestsHistogram[protocol] = map[float32]*timeseries.TimeSeries{}
					}
					c.RequestsHistogram[protocol][float32(le)] = merge(c.RequestsHistogram[protocol][float32(le)], m.Values, timeseries.NanSum)
				}
			case "container_dns_requests_total":
				r := model.DNSRequest{
					Type:   m.Labels["request_type"],
					Domain: m.Labels["domain"],
				}
				if r.Type == "" || r.Domain == "" {
					continue
				}
				status := m.Labels["status"]
				byStatus := container.DNSRequests[r]
				if byStatus == nil {
					byStatus = map[string]*timeseries.TimeSeries{}
					container.DNSRequests[r] = byStatus
				}
				byStatus[status] = merge(byStatus[status], m.Values, timeseries.Any)
			case "container_dns_requests_latency":
				le, err := strconv.ParseFloat(m.Labels["le"], 32)
				if err != nil {
					klog.Warningln(err)
					continue
				}
				container.DNSRequestsHistogram[float32(le)] = merge(container.DNSRequestsHistogram[float32(le)], m.Values, timeseries.Any)
			case "container_cpu_limit":
				container.CpuLimit = merge(container.CpuLimit, m.Values, timeseries.Any)
			case "container_cpu_usage":
				container.CpuUsage = merge(container.CpuUsage, m.Values, timeseries.Any)
			case "container_cpu_delay":
				container.CpuDelay = merge(container.CpuDelay, m.Values, timeseries.Any)
			case "container_throttled_time":
				container.ThrottledTime = merge(container.ThrottledTime, m.Values, timeseries.Any)
			case "container_memory_rss":
				container.MemoryRss = merge(container.MemoryRss, m.Values, timeseries.Any)
			case "container_memory_rss_for_trend":
				container.MemoryRssForTrend = merge(container.MemoryRssForTrend, m.Values, timeseries.Any)
			case "container_memory_cache":
				container.MemoryCache = merge(container.MemoryCache, m.Values, timeseries.Any)
			case "container_memory_limit":
				container.MemoryLimit = merge(container.MemoryLimit, m.Values, timeseries.Any)
			case "container_oom_kills_total":
				container.OOMKills = merge(container.OOMKills, timeseries.Increase(m.Values, pjs.get(m.Labels)), timeseries.Any)
			case "container_restarts":
				container.Restarts = merge(container.Restarts, timeseries.Increase(m.Values, pjs.get(m.Labels)), timeseries.Any)
			case "container_application_type":
				container.ApplicationTypes[model.ApplicationType(m.Labels["application_type"])] = true
			case "container_log_messages":
				logMessage(instance, m.Labels, timeseries.Increase(m.Values, pjs.get(m.Labels)))
			case "container_volume_size":
				v := getOrCreateInstanceVolume(instance, m)
				v.CapacityBytes = merge(v.CapacityBytes, m.Values, timeseries.Any)
			case "container_volume_used":
				v := getOrCreateInstanceVolume(instance, m)
				v.UsedBytes = merge(v.UsedBytes, m.Values, timeseries.Any)
			case "container_jvm_info", "container_jvm_heap_size_bytes", "container_jvm_heap_used_bytes",
				"container_jvm_gc_time_seconds", "container_jvm_safepoint_sync_time_seconds", "container_jvm_safepoint_time_seconds":
				jvm(instance, queryName, m)
			case "container_dotnet_info", "container_dotnet_memory_allocated_bytes_total", "container_dotnet_exceptions_total",
				"container_dotnet_memory_heap_size_bytes", "container_dotnet_gc_count_total", "container_dotnet_heap_fragmentation_percent",
				"container_dotnet_monitor_lock_contentions_total", "container_dotnet_thread_pool_completed_items_total",
				"container_dotnet_thread_pool_queue_length", "container_dotnet_thread_pool_size":
				dotnet(instance, queryName, m)
			case "container_python_thread_lock_wait_time_seconds":
				python(instance, queryName, m)
			}
		}
	}

	instancesByListen := map[model.Listen]*model.Instance{}
	for _, app := range w.Applications {
		for _, instance := range app.Instances {
			for l := range instance.TcpListens {
				if ip := net.ParseIP(l.IP); ip.IsLoopback() {
					if instance.Node != nil {
						l.IP = instance.NodeName()
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
					l.IP = instance.NodeName()
				}
				if u.RemoteInstance = instancesByListen[l]; u.RemoteInstance == nil {
					l.Proxied = false
					if u.RemoteInstance = instancesByListen[l]; u.RemoteInstance == nil {
						l.Port = "0"
						u.RemoteInstance = instancesByListen[l]
					}
				}
				if upstreams, ok := rttByInstance[instanceId{ns: instance.OwnerId.Namespace, name: instance.Name, node: instance.NodeId()}]; ok {
					u.Rtt = merge(u.Rtt, upstreams[u.ActualRemoteIP], timeseries.Any)
				}
				if svc := servicesByClusterIP[u.ServiceRemoteIP]; svc != nil {
					u.Service = svc
					if u.RemoteInstance == nil {
						if a := w.GetApplicationByNsAndName(svc.Namespace, svc.Name); a != nil {
							u.RemoteApplication = a
						}
					}
				}
			}
		}
	}

	for _, app := range w.Applications { // creating ApplicationKindExternalService for unknown remote instances
		for _, instance := range app.Instances {
			for _, u := range instance.Upstreams {
				if u.RemoteInstance != nil || u.RemoteApplication != nil {
					continue
				}
				appId := model.NewApplicationId("", model.ApplicationKindExternalService, "")
				svc := getServiceForConnection(u, servicesByClusterIP, servicesByActualDestIP)
				instanceName := u.ActualRemoteIP + ":" + u.ActualRemotePort
				if svc != nil {
					u.Service = svc
					if id, ok := svc.GetDestinationApplicationId(); ok {
						if a := w.GetApplication(id); a != nil {
							a.Downstreams = append(a.Downstreams, u)
						}
						continue
					} else {
						appId.Name = svc.Name
					}
				} else {
					if fqdns := ip2fqdn[u.ActualRemoteIP]; fqdns != nil && fqdns.Len() > 0 {
						appId.Name = fqdns.Items()[0] + ":" + u.ActualRemotePort
					} else {
						appId.Name = externalServiceName(u.ActualRemotePort)
					}
				}
				customApp := c.project.GetCustomApplicationName(instanceName)
				if customApp != "" {
					appId.Name = customApp
				}
				ri := w.GetOrCreateApplication(appId, customApp != "").GetOrCreateInstance(instanceName, nil)
				ri.TcpListens[model.Listen{IP: u.ActualRemoteIP, Port: u.ActualRemotePort}] = true
				u.RemoteInstance = ri
			}
		}
	}
	for _, app := range w.Applications {
		for _, instance := range app.Instances {
			for _, u := range instance.Upstreams {
				if u.RemoteInstance != nil {
					if a := w.GetApplication(u.RemoteInstance.OwnerId); a != nil {
						u.RemoteApplication = a
						a.Downstreams = append(a.Downstreams, u)
					}
				} else if u.RemoteApplication != nil {
					u.RemoteApplication.Downstreams = append(u.RemoteApplication.Downstreams, u)
				}
			}
		}
	}
}

func getServiceForConnection(c *model.Connection, byClusterIP map[string]*model.Service, byActualDestIP map[string]*model.Service) *model.Service {
	if s := byClusterIP[c.ServiceRemoteIP]; s != nil {
		return s
	}
	return byActualDestIP[c.ActualRemoteIP]
}

type connectionKey struct {
	instanceId
	destination, actualDestination string
}

func getOrCreateConnection(instance *model.Instance, container string, m model.MetricValues, cache map[connectionKey]*model.Connection, servicesByClusterIP, servicesByActualDestIP map[string]*model.Service) *model.Connection {
	if instance.OwnerId.Name == "docker" { // ignore docker-proxy's connections
		return nil
	}

	dest := m.Labels["destination"]
	actualDest := m.Labels["actual_destination"]
	if actualDest == "" {
		actualDest = dest
	}
	connKey := connectionKey{
		instanceId: instanceId{
			ns:   instance.OwnerId.Namespace,
			name: instance.Name,
			node: instance.NodeId(),
		},
		destination:       dest,
		actualDestination: actualDest,
	}
	connection := cache[connKey]
	if connection == nil {
		var actualIP, actualPort, serviceIP, servicePort string
		var err error
		serviceIP, servicePort, err = net.SplitHostPort(dest)
		if err != nil {
			klog.Warningf("failed to split %s to ip:port pair: %s", dest, err)
			return nil
		}
		if actualDest != "" {
			actualIP, actualPort, err = net.SplitHostPort(actualDest)
			if err != nil {
				klog.Warningf("failed to split %s to ip:port pair: %s", actualDest, err)
				return nil
			}
		}
		connection = instance.AddUpstreamConnection(actualIP, actualPort, serviceIP, servicePort, container)
		cache[connKey] = connection
		updateServiceEndpoints(connection, servicesByClusterIP, servicesByActualDestIP)
	}

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

func logMessage(instance *model.Instance, ls model.Labels, values *timeseries.TimeSeries) {
	level := model.LogLevel(ls["level"])
	msgs := instance.LogMessages[level]
	if msgs == nil {
		msgs = &model.LogMessages{}
		instance.LogMessages[level] = msgs
	}
	msgs.Messages = merge(msgs.Messages, values, timeseries.NanSum)

	if hash := ls["pattern_hash"]; hash != "" {
		if msgs.Patterns == nil {
			msgs.Patterns = map[string]*model.LogPattern{}
		}
		p := msgs.Patterns[hash]
		if p == nil {
			sample := ls["sample"]
			p = &model.LogPattern{
				Level:     level,
				Sample:    sample,
				Multiline: strings.Contains(sample, "\n"),
				Pattern:   logparser.NewPattern(sample),
			}
			msgs.Patterns[hash] = p
		}
		p.Messages = merge(p.Messages, values, timeseries.NanSum)
	}
}

func updateServiceEndpoints(c *model.Connection, servicesByClusterIP, servicesByActualDestIP map[string]*model.Service) {
	if c.ActualRemoteIP == "" && c.ServiceRemoteIP == "" {
		return
	}
	if s := servicesByClusterIP[c.ServiceRemoteIP]; s != nil {
		s.Connections = append(s.Connections, c)
		servicesByActualDestIP[c.ActualRemoteIP] = s
	}
}

func externalServiceName(port string) string {
	service := ""
	switch port {
	case "5432":
		service = "postgres"
	case "3306":
		service = "mysql"
	case "11211":
		service = "memcached"
	case "2181":
		service = "zookeeper"
	case "9092", "9093", "9094":
		service = "kafka"
	case "6379":
		service = "redis"
	case "9042", "9160", "9142", "7000", "7001", "7199":
		service = "cassandra"
	case "27017", "27018":
		service = "mongodb"
	case "9200", "9300":
		service = "elasticsearch"
	case "80", "443", "8080":
		service = "http"
	default:
		service = ":" + port
	}
	return "external " + service
}
