package constructor

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
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
	if len(parts) == 5 && parts[1] == "k8s-cronjob" {
		w.IntegrationStatus.KubeStateMetrics.Required = true
		ns, job := parts[2], parts[3]
		containerName = parts[4]
		appId = model.NewApplicationId(ns, model.ApplicationKindCronJob, job)
		id = instanceId{ns: ns, name: fmt.Sprintf("%s@%s", job, nodeName), node: nodeId}
	} else if len(parts) == 7 && parts[1] == "nomad" {
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

type containerCache map[model.NodeContainerId]struct {
	instance  *model.Instance
	container *model.Container
}

func (c *Constructor) loadContainers(w *model.World, metrics map[string][]*model.MetricValues, pjs promJobStatuses, nodesByID map[model.NodeId]*model.Node, containers containerCache, servicesByClusterIP map[string]*model.Service, ip2fqdn map[string]*utils.StringSet) {
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

	rttByInstance := map[instanceId]map[string]*timeseries.TimeSeries{}

	loadContainer := func(queryName string, f func(instance *model.Instance, container *model.Container, metric *model.MetricValues)) {
		ms := metrics[queryName]
		for _, m := range ms {
			v, ok := containers[m.NodeContainerId]
			if !ok {
				nodeId := model.NewNodeIdFromLabels(m)
				v.instance, v.container = c.getInstanceAndContainer(w, nodesByID[nodeId], instances, m.ContainerId)
				containers[m.NodeContainerId] = v
			}
			if v.instance == nil || v.container == nil {
				continue
			}
			f(v.instance, v.container, m)
		}
	}

	loadContainer("container_info", func(instance *model.Instance, container *model.Container, metric *model.MetricValues) {
		if image := metric.Labels["image"]; image != "" {
			container.Image = image
		}
		if strings.HasSuffix(metric.Labels["systemd_triggered_by"], ".timer") {
			container.PeriodicSystemdJob = true
		}
	})
	loadContainer("container_application_type", func(instance *model.Instance, container *model.Container, metric *model.MetricValues) {
		container.ApplicationTypes[model.ApplicationType(metric.Labels["application_type"])] = true
	})

	loadContainer("container_cpu_limit", func(instance *model.Instance, container *model.Container, metric *model.MetricValues) {
		container.CpuLimit = merge(container.CpuLimit, metric.Values, timeseries.Any)
	})
	loadContainer("container_cpu_usage", func(instance *model.Instance, container *model.Container, metric *model.MetricValues) {
		container.CpuUsage = merge(container.CpuUsage, metric.Values, timeseries.Any)
	})
	loadContainer("container_cpu_delay", func(instance *model.Instance, container *model.Container, metric *model.MetricValues) {
		container.CpuDelay = merge(container.CpuDelay, metric.Values, timeseries.Any)
	})
	loadContainer("container_throttled_time", func(instance *model.Instance, container *model.Container, metric *model.MetricValues) {
		container.ThrottledTime = merge(container.ThrottledTime, metric.Values, timeseries.Any)
	})
	loadContainer("container_memory_rss", func(instance *model.Instance, container *model.Container, metric *model.MetricValues) {
		container.MemoryRss = merge(container.MemoryRss, metric.Values, timeseries.Any)
	})
	loadContainer("container_memory_rss_for_trend", func(instance *model.Instance, container *model.Container, metric *model.MetricValues) {
		container.MemoryRssForTrend = merge(container.MemoryRssForTrend, metric.Values, timeseries.Any)
	})
	loadContainer("container_memory_cache", func(instance *model.Instance, container *model.Container, metric *model.MetricValues) {
		container.MemoryCache = merge(container.MemoryCache, metric.Values, timeseries.Any)
	})
	loadContainer("container_memory_limit", func(instance *model.Instance, container *model.Container, metric *model.MetricValues) {
		container.MemoryLimit = merge(container.MemoryLimit, metric.Values, timeseries.Any)
	})
	loadContainer("container_oom_kills_total", func(instance *model.Instance, container *model.Container, metric *model.MetricValues) {
		container.OOMKills = merge(container.OOMKills, timeseries.Increase(metric.Values, pjs.get(metric.Labels)), timeseries.Any)
	})
	loadContainer("container_restarts", func(instance *model.Instance, container *model.Container, metric *model.MetricValues) {
		container.Restarts = merge(container.Restarts, timeseries.Increase(metric.Values, pjs.get(metric.Labels)), timeseries.Any)
	})
	loadContainer("container_net_latency", func(instance *model.Instance, container *model.Container, metric *model.MetricValues) {
		id := instanceId{ns: instance.Owner.Id.Namespace, name: instance.Name, node: instance.NodeId()}
		rtts := rttByInstance[id]
		if rtts == nil {
			rtts = map[string]*timeseries.TimeSeries{}
		}
		rtts[metric.Destination] = merge(rtts[metric.Destination], metric.Values, timeseries.Any)
		rttByInstance[id] = rtts
	})
	loadContainer("container_net_tcp_listen_info", func(instance *model.Instance, container *model.Container, metric *model.MetricValues) {
		ip, port, err := net.SplitHostPort(metric.Labels["listen_addr"])
		if err != nil {
			klog.Warningf("failed to split %s to ip:port pair: %s", metric.Labels["listen_addr"], err)
			return
		}
		isActive := metric.Values.Last() == 1
		l := model.Listen{IP: ip, Port: port, Proxied: metric.Labels["proxy"] != ""}
		if !instance.TcpListens[l] {
			instance.TcpListens[l] = isActive
		}
	})

	loadConnection := func(queryName string, f func(connection *model.Connection, metric *model.MetricValues)) {
		loadContainer(queryName, func(instance *model.Instance, container *model.Container, metric *model.MetricValues) {
			conn := getOrCreateConnection(instance, container.Name, metric)
			if conn != nil {
				f(conn, metric)
			}
		})
	}
	loadConnection("container_net_tcp_successful_connects", func(connection *model.Connection, metric *model.MetricValues) {
		connection.SuccessfulConnections = merge(connection.SuccessfulConnections, metric.Values, timeseries.Any)
	})
	loadConnection("container_net_tcp_connection_time_seconds", func(connection *model.Connection, metric *model.MetricValues) {
		connection.ConnectionTime = merge(connection.ConnectionTime, metric.Values, timeseries.Any)
	})
	loadConnection("container_net_tcp_bytes_sent", func(connection *model.Connection, metric *model.MetricValues) {
		connection.BytesSent = merge(connection.BytesSent, metric.Values, timeseries.Any)
	})
	loadConnection("container_net_tcp_bytes_received", func(connection *model.Connection, metric *model.MetricValues) {
		connection.BytesReceived = merge(connection.BytesReceived, metric.Values, timeseries.Any)
	})
	loadConnection("container_net_tcp_failed_connects", func(connection *model.Connection, metric *model.MetricValues) {
		connection.FailedConnections = merge(connection.FailedConnections, metric.Values, timeseries.Any)
	})
	loadConnection("container_net_tcp_active_connections", func(connection *model.Connection, metric *model.MetricValues) {
		connection.Active = merge(connection.Active, metric.Values, timeseries.Any)
	})
	loadConnection("container_net_tcp_retransmits", func(connection *model.Connection, metric *model.MetricValues) {
		connection.Retransmissions = merge(connection.Retransmissions, metric.Values, timeseries.Any)
	})

	loadL7RequestsCount := func(queryName string, protocol model.Protocol) {
		loadConnection(queryName, func(connection *model.Connection, metric *model.MetricValues) {
			switch protocol {
			case model.ProtocolRabbitmq, model.ProtocolNats:
				protocol += model.Protocol("-" + metric.Labels["method"])
			}
			if connection.RequestsCount[protocol] == nil {
				connection.RequestsCount[protocol] = map[string]*timeseries.TimeSeries{}
			}
			status := metric.Labels["status"]
			connection.RequestsCount[protocol][status] = merge(connection.RequestsCount[protocol][status], metric.Values, timeseries.NanSum)
		})
	}
	loadL7RequestsCount("container_http_requests_count", model.ProtocolHttp)
	loadL7RequestsCount("container_postgres_queries_count", model.ProtocolPostgres)
	loadL7RequestsCount("container_mysql_queries_count", model.ProtocolMysql)
	loadL7RequestsCount("container_mongo_queries_count", model.ProtocolMongodb)
	loadL7RequestsCount("container_redis_queries_count", model.ProtocolRedis)
	loadL7RequestsCount("container_memcached_queries_count", model.ProtocolMemcached)
	loadL7RequestsCount("container_kafka_requests_count", model.ProtocolKafka)
	loadL7RequestsCount("container_cassandra_queries_count", model.ProtocolCassandra)
	loadL7RequestsCount("container_rabbitmq_messages", model.ProtocolRabbitmq)
	loadL7RequestsCount("container_nats_messages", model.ProtocolNats)
	loadL7RequestsCount("container_clickhouse_queries_count", model.ProtocolClickhouse)
	loadL7RequestsCount("container_zookeeper_requests_count", model.ProtocolZookeeper)

	loadL7RequestsLatency := func(queryName string, protocol model.Protocol) {
		loadConnection(queryName, func(connection *model.Connection, metric *model.MetricValues) {
			connection.RequestsLatency[protocol] = merge(connection.RequestsLatency[protocol], metric.Values, timeseries.Any)
		})
	}
	loadL7RequestsLatency("container_http_requests_latency", model.ProtocolHttp)
	loadL7RequestsLatency("container_postgres_queries_latency", model.ProtocolPostgres)
	loadL7RequestsLatency("container_mysql_queries_latency", model.ProtocolMysql)
	loadL7RequestsLatency("container_mongo_queries_latency", model.ProtocolMongodb)
	loadL7RequestsLatency("container_redis_queries_latency", model.ProtocolRedis)
	loadL7RequestsLatency("container_memcached_queries_latency", model.ProtocolMemcached)
	loadL7RequestsLatency("container_kafka_requests_latency", model.ProtocolKafka)
	loadL7RequestsLatency("container_cassandra_queries_latency", model.ProtocolCassandra)
	loadL7RequestsLatency("container_clickhouse_queries_latency", model.ProtocolClickhouse)
	loadL7RequestsLatency("container_zookeeper_requests_latency", model.ProtocolZookeeper)

	loadL7RequestsHistogram := func(queryName string, protocol model.Protocol) {
		loadConnection(queryName, func(connection *model.Connection, metric *model.MetricValues) {
			le, err := strconv.ParseFloat(metric.Labels["le"], 32)
			if err != nil {
				klog.Warningln(err)
				return
			}
			if connection.RequestsHistogram[protocol] == nil {
				connection.RequestsHistogram[protocol] = map[float32]*timeseries.TimeSeries{}
			}
			connection.RequestsHistogram[protocol][float32(le)] = merge(connection.RequestsHistogram[protocol][float32(le)], metric.Values, timeseries.NanSum)
		})
	}
	loadL7RequestsHistogram("container_http_requests_histogram", model.ProtocolHttp)
	loadL7RequestsHistogram("container_postgres_queries_histogram", model.ProtocolPostgres)
	loadL7RequestsHistogram("container_mysql_queries_histogram", model.ProtocolMysql)
	loadL7RequestsHistogram("container_mongo_queries_histogram", model.ProtocolMongodb)
	loadL7RequestsHistogram("container_redis_queries_histogram", model.ProtocolRedis)
	loadL7RequestsHistogram("container_memcached_queries_histogram", model.ProtocolMemcached)
	loadL7RequestsHistogram("container_kafka_requests_histogram", model.ProtocolKafka)
	loadL7RequestsHistogram("container_cassandra_queries_histogram", model.ProtocolCassandra)
	loadL7RequestsHistogram("container_clickhouse_queries_histogram", model.ProtocolClickhouse)
	loadL7RequestsHistogram("container_zookeeper_requests_histogram", model.ProtocolZookeeper)

	loadContainer("container_dns_requests_total", func(instance *model.Instance, container *model.Container, metric *model.MetricValues) {
		r := model.DNSRequest{
			Type:   metric.Labels["request_type"],
			Domain: metric.Labels["domain"],
		}
		if r.Type == "" || r.Domain == "" {
			return
		}
		status := metric.Labels["status"]
		byStatus := container.DNSRequests[r]
		if byStatus == nil {
			byStatus = map[string]*timeseries.TimeSeries{}
			container.DNSRequests[r] = byStatus
		}
		byStatus[status] = merge(byStatus[status], metric.Values, timeseries.Any)
	})
	loadContainer("container_dns_requests_latency", func(instance *model.Instance, container *model.Container, metric *model.MetricValues) {
		le, err := strconv.ParseFloat(metric.Labels["le"], 32)
		if err != nil {
			klog.Warningln(err)
			return
		}
		container.DNSRequestsHistogram[float32(le)] = merge(container.DNSRequestsHistogram[float32(le)], metric.Values, timeseries.Any)
	})

	loadVolume := func(queryName string, f func(volume *model.Volume, metric *model.MetricValues)) {
		loadContainer(queryName, func(instance *model.Instance, container *model.Container, metric *model.MetricValues) {
			v := getOrCreateInstanceVolume(instance, metric)
			f(v, metric)
		})
	}
	loadVolume("container_volume_size", func(volume *model.Volume, metric *model.MetricValues) {
		volume.CapacityBytes = merge(volume.CapacityBytes, metric.Values, timeseries.Any)
	})
	loadVolume("container_volume_used", func(volume *model.Volume, metric *model.MetricValues) {
		volume.UsedBytes = merge(volume.UsedBytes, metric.Values, timeseries.Any)
	})

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
				remoteIP := u.ActualRemoteIP
				remotePort := u.ActualRemotePort
				if remoteIP == "" {
					remoteIP = u.ServiceRemoteIP
					remotePort = u.ServiceRemotePort
				}
				l := model.Listen{IP: remoteIP, Port: remotePort, Proxied: true}
				if ip := net.ParseIP(remoteIP); ip.IsLoopback() && instance.Node != nil {
					l.IP = instance.NodeName()
				}
				if u.RemoteInstance = instancesByListen[l]; u.RemoteInstance == nil {
					l.Proxied = false
					if u.RemoteInstance = instancesByListen[l]; u.RemoteInstance == nil {
						l.Port = "0"
						u.RemoteInstance = instancesByListen[l]
					}
				}
				if upstreams, ok := rttByInstance[instanceId{ns: instance.Owner.Id.Namespace, name: instance.Name, node: instance.NodeId()}]; ok {
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
				appId := model.NewApplicationId("external", model.ApplicationKindExternalService, "")
				svc := servicesByClusterIP[u.ServiceRemoteIP]
				instanceName := u.ServiceRemoteIP + ":" + u.ServiceRemotePort
				if svc != nil {
					u.Service = svc
					if a := svc.GetDestinationApplication(); a != nil {
						a.Downstreams = append(a.Downstreams, u)
						u.RemoteApplication = a
						continue
					} else {
						appId.Name = svc.Name
					}
				} else {
					if u.ActualRemoteIP == "" && net.ParseIP(u.ServiceRemoteIP) == nil {
						appId.Name = u.ServiceRemoteIP
					} else if fqdns := ip2fqdn[u.ServiceRemoteIP]; fqdns != nil && fqdns.Len() > 0 {
						appId.Name = fqdns.Items()[0] + ":" + u.ServiceRemotePort
					} else {
						appId.Name = externalServiceName(u.ServiceRemotePort)
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
					u.RemoteApplication = u.RemoteInstance.Owner
					u.RemoteInstance.Owner.Downstreams = append(u.RemoteInstance.Owner.Downstreams, u)
				} else if u.RemoteApplication != nil {
					u.RemoteApplication.Downstreams = append(u.RemoteApplication.Downstreams, u)
				}
			}
		}
	}
}

func getOrCreateConnection(instance *model.Instance, container string, m *model.MetricValues) *model.Connection {
	if instance.Owner.Id.Name == "docker" { // ignore docker-proxy's connections
		return nil
	}

	connection := instance.Upstreams[m.ConnectionKey]
	if connection == nil {
		var actualIP, actualPort, serviceIP, servicePort string
		var err error
		serviceIP, servicePort, err = net.SplitHostPort(m.Destination)
		if err != nil {
			klog.Warningf("failed to split %s to ip:port pair: %s", m.Destination, err)
			return nil
		}
		if m.ActualDestination != "" {
			actualIP, actualPort, err = net.SplitHostPort(m.ActualDestination)
			if err != nil {
				klog.Warningf("failed to split %s to ip:port pair: %s", m.ActualDestination, err)
				return nil
			}
		}
		connection = &model.Connection{
			Instance:          instance,
			ActualRemoteIP:    actualIP,
			ActualRemotePort:  actualPort,
			ServiceRemoteIP:   serviceIP,
			ServiceRemotePort: servicePort,
			Container:         container,

			RequestsCount:     map[model.Protocol]map[string]*timeseries.TimeSeries{},
			RequestsLatency:   map[model.Protocol]*timeseries.TimeSeries{},
			RequestsHistogram: map[model.Protocol]map[float32]*timeseries.TimeSeries{},
		}
		instance.Upstreams[m.ConnectionKey] = connection
	}
	return connection
}

func getOrCreateInstanceVolume(instance *model.Instance, m *model.MetricValues) *model.Volume {
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
		return "external:" + port
	}
	return "external-" + service
}
