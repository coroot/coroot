package constructor

import (
	"fmt"
	"github.com/coroot/coroot-focus/model"
	"github.com/coroot/coroot-focus/timeseries"
	"github.com/coroot/logpattern"
	"k8s.io/klog"
	"net"
	"strings"
)

type metricContext struct {
	ns        string
	pod       string
	node      *model.Node
	container string
}

func getMetricContext(w *model.World, ls model.Labels) *metricContext {
	containerId := ls["container_id"]
	mc := &metricContext{node: getNode(w, ls)}
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
		if a.ApplicationId.Namespace != ns {
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
	instancesByListen := map[model.Listen]*model.Instance{}

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
				if instance = getInstanceByPod(w, mc.ns, mc.pod); instance == nil {
					klog.Warningln("unknown pod: %s/%s", mc.ns, mc.pod)
					continue
				}
			case mc.container != "" && mc.node != nil:
				appId := model.NewApplicationId("", model.ApplicationKindStandaloneContainers, mc.container)
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
			case "container_net_latency":
				rtts := rttByInstance[instance.InstanceId]
				if rtts == nil {
					rtts = map[string]timeseries.TimeSeries{}
				}
				rtts[m.Labels["destination_ip"]] = update(rtts[m.Labels["destination_ip"]], m.Values)
				rttByInstance[instance.InstanceId] = rtts
			case "container_net_tcp_successful_connects":
				if c := getOrCreateConnection(instance, mc.container, m, w); c != nil {
					c.Connects = update(c.Connects, m.Values)
				}
			case "container_net_tcp_active_connections":
				if c := getOrCreateConnection(instance, mc.container, m, w); c != nil {
					c.Active = update(c.Active, m.Values)
				}
			case "container_net_tcp_listen_info":
				ip, port, err := net.SplitHostPort(m.Labels["listen_addr"])
				if err != nil {
					klog.Warningf("failed to split %s to ip:port pair: %s", m.Labels["listen_addr"], err)
					continue
				}
				isActive := m.Values.Last() == 1
				l := model.Listen{IP: ip, Port: port, Proxied: m.Labels["proxy"] != ""}
				instance.TcpListens[l] = isActive
				if ip := net.ParseIP(l.IP); ip.IsLoopback() {
					if instance.Node != nil {
						l.IP = instance.Node.Name.Value()
						instancesByListen[l] = instance
					}
				} else {
					instancesByListen[l] = instance
				}
			case "container_cpu_limit":
				container.CpuLimit = update(container.CpuLimit, m.Values)
			case "container_cpu_usage":
				container.CpuUsage = update(container.CpuUsage, m.Values)
			case "container_cpu_delay":
				container.CpuDelay = update(container.CpuDelay, m.Values)
			case "container_throttled_time":
				container.ThrottledTime = update(container.ThrottledTime, m.Values)
			case "container_memory_rss":
				container.MemoryRss = update(container.MemoryRss, m.Values)
			case "container_memory_cache":
				container.MemoryCache = update(container.MemoryCache, m.Values)
			case "container_memory_limit":
				container.MemoryLimit = update(container.MemoryLimit, m.Values)
			case "container_oom_kills_total":
				container.OOMKills = update(container.OOMKills, timeseries.Increase(m.Values, promJobStatus))
			case "container_restarts":
				container.Restarts = update(container.Restarts, timeseries.Increase(m.Values, promJobStatus))
			case "container_application_type":
				container.ApplicationTypes[model.ApplicationType(m.Labels["application_type"])] = true
			case "container_log_messages":
				logMessage(instance, m)
			case "container_volume_size":
				v := getOrCreateInstanceVolume(instance, m)
				v.CapacityBytes = update(v.CapacityBytes, m.Values)
			case "container_volume_used":
				v := getOrCreateInstanceVolume(instance, m)
				v.UsedBytes = update(v.UsedBytes, m.Values)
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
					u.Rtt = update(u.Rtt, upstreams[u.ActualRemoteIP])
				}
			}
		}
	}
	for _, app := range w.Applications { // creating ApplicationKindExternalService for unknown remote instances
		for _, instance := range app.Instances {
			for _, u := range instance.Upstreams {
				if u.RemoteInstance == nil {
					appId := model.NewApplicationId("", model.ApplicationKindExternalService, "")
					svc := w.GetServiceForConnection(u)
					if svc != nil {
						id, ok := svc.GetDestinationApplicationId()
						if ok {
							appId = id
						} else {
							appId.Name = svc.Name
						}
					} else {
						appId.Name = u.ActualRemoteIP + ":" + u.ActualRemotePort
					}
					klog.Infoln(u.Instance.Name, "->", u.ActualRemoteIP, u.ActualRemotePort)
					ri := w.GetOrCreateApplication(appId).GetOrCreateInstance(appId.Name)
					ri.TcpListens[model.Listen{IP: u.ActualRemoteIP, Port: u.ActualRemotePort}] = true
					u.RemoteInstance = ri
				}
			}
		}
	}
	for _, app := range w.Applications {
		for _, instance := range app.Instances {
			for _, u := range instance.Upstreams {
				if u.RemoteInstance == nil {
					continue
				}
				u.RemoteInstance.Downstreams = append(u.RemoteInstance.Downstreams, u)
			}
		}
	}
}

func getOrCreateConnection(instance *model.Instance, container string, m model.MetricValues, w *model.World) *model.Connection {
	if timeseries.Reduce(timeseries.NanSum, m.Values) < 1 {
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

func logMessage(instance *model.Instance, m model.MetricValues) {
	level := model.LogLevel(m.Labels["level"])
	byLevel, ok := instance.LogMessagesByLevel[level]
	if !ok {
		byLevel = timeseries.Aggregate(timeseries.NanSum)
		instance.LogMessagesByLevel[level] = byLevel
	}
	byLevel.(*timeseries.AggregatedTimeseries).AddInput(m.Values)

	if hash := m.Labels["pattern_hash"]; hash != "" {
		p := instance.LogPatterns[hash]
		if p == nil {
			sample := m.Labels["sample"]
			pattern := logpattern.NewPattern(sample)

			p = &model.LogPattern{
				Level:     level,
				Sample:    sample,
				Multiline: strings.Contains(sample, "\n"),
				Pattern:   pattern,
				Sum:       timeseries.Aggregate(timeseries.NanSum),
			}
			if p.Multiline {
				p.Sample = markMultilineMessage(p.Sample)
			}
			instance.LogPatterns[hash] = p
		}
		p.Sum.(*timeseries.AggregatedTimeseries).AddInput(m.Values)
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
