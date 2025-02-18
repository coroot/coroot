package constructor

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"k8s.io/klog"
)

var (
	jobSuffixRe = regexp.MustCompile(`-([a-z0-9]{5}|\d{10,})$`)
)

type serviceId struct {
	name, ns string
}

func loadKubernetesMetadata(w *model.World, metrics map[string][]*model.MetricValues, servicesByClusterIP map[string]*model.Service) {
	pods := podInfo(w, metrics["kube_pod_info"])
	podLabels(metrics["kube_pod_labels"], pods)

	appsByPodIP := map[string]*model.Application{}
	for _, pod := range pods {
		if pod.Pod.IP != "" && pod.Owner != nil {
			appsByPodIP[pod.Pod.IP] = pod.Owner
		}
	}
	services := loadServices(metrics)
	for _, s := range services {
		if s.ClusterIP != "" {
			servicesByClusterIP[s.ClusterIP] = s
		}
		for _, ip := range s.EndpointIPs.Items() {
			if app := appsByPodIP[ip]; app != nil {
				s.DestinationApps[app.Id] = app
			}
		}
		for _, app := range s.DestinationApps {
			app.KubernetesServices = append(app.KubernetesServices, s)
		}
	}

	for queryName := range QUERIES {
		switch {
		case strings.HasPrefix(queryName, "kube_pod_status_"):
			podStatus(queryName, metrics[queryName], pods)
		case strings.HasPrefix(queryName, "kube_pod_init_container_"):
			podContainer(queryName, metrics[queryName], pods)
		case strings.HasPrefix(queryName, "kube_pod_container_"):
			podContainer(queryName, metrics[queryName], pods)
		}
	}
	loadApplications(w, metrics)
}

func loadServices(metrics map[string][]*model.MetricValues) map[serviceId]*model.Service {
	services := map[serviceId]*model.Service{}
	for _, m := range metrics["kube_service_info"] {
		name := m.Labels["service"]
		s := &model.Service{
			Name:            name,
			Namespace:       m.Labels["namespace"],
			ClusterIP:       m.Labels["cluster_ip"],
			EndpointIPs:     &utils.StringSet{},
			LoadBalancerIPs: &utils.StringSet{},
			DestinationApps: map[model.ApplicationId]*model.Application{},
		}
		services[serviceId{name: s.Name, ns: s.Namespace}] = s
	}
	for _, m := range metrics["kube_service_spec_type"] {
		if s := services[serviceId{name: m.Labels["service"], ns: m.Labels["namespace"]}]; s != nil {
			s.Type.Update(m.Values, m.Labels["type"])
		}
	}
	for _, m := range metrics["kube_service_spec_type"] {
		if s := services[serviceId{name: m.Labels["service"], ns: m.Labels["namespace"]}]; s != nil {
			s.Type.Update(m.Values, m.Labels["type"])
		}
	}
	for _, m := range metrics["kube_service_status_load_balancer_ingress"] {
		if s := services[serviceId{name: m.Labels["service"], ns: m.Labels["namespace"]}]; s != nil {
			s.LoadBalancerIPs.Add(m.Labels["ip"])
		}
	}
	for _, m := range metrics["kube_endpoint_address"] {
		if s := services[serviceId{name: m.Labels["endpoint"], ns: m.Labels["namespace"]}]; s != nil {
			s.EndpointIPs.Add(m.Labels["ip"])
		}
	}
	for _, s := range services {
		if s.Name == "kubernetes" {
			s.Name = "kube-apiserver"
		}
	}
	return services
}

func loadApplications(w *model.World, metrics map[string][]*model.MetricValues) {
	for queryName := range metrics {
		var (
			kind      model.ApplicationKind
			nameLabel string
		)
		switch {
		case strings.HasPrefix(queryName, "kube_deployment_"):
			kind = model.ApplicationKindDeployment
			nameLabel = "deployment"
		case strings.HasPrefix(queryName, "kube_statefulset_"):
			kind = model.ApplicationKindStatefulSet
			nameLabel = "statefulset"
		case strings.HasPrefix(queryName, "kube_daemonset_"):
			kind = model.ApplicationKindDaemonSet
			nameLabel = "daemonset"
		default:
			continue
		}
		for _, m := range metrics[queryName] {
			app := w.GetApplication(model.NewApplicationId(m.Labels["namespace"], kind, m.Labels[nameLabel]))
			if app == nil {
				continue
			}
			switch queryName {
			case "kube_deployment_spec_replicas", "kube_statefulset_replicas", "kube_daemonset_status_desired_number_scheduled":
				app.DesiredInstances = merge(app.DesiredInstances, m.Values, timeseries.Any)
			}
		}
	}
}

func podInfo(w *model.World, metrics []*model.MetricValues) map[string]*model.Instance {
	pods := map[string]*model.Instance{}
	podOwners := map[podId]model.ApplicationId{}
	var podsOwnedByPods []*model.Instance
	for _, m := range metrics {
		w.IntegrationStatus.KubeStateMetrics.Installed = true
		pod := m.Labels["pod"]
		ns := m.Labels["namespace"]
		ownerName := m.Labels["created_by_name"]
		ownerKind := model.ApplicationKind(m.Labels["created_by_kind"])
		nodeName := m.Labels["node"]
		uid := m.Labels["uid"]
		if uid == "" {
			klog.Errorln("invalid 'kube_pod_info' metric: 'uid' label is empty")
			continue
		}
		node := w.GetNode(nodeName)
		var appId model.ApplicationId

		switch {
		case ownerKind == "" || ownerKind == "<none>" || ownerKind == "Node":
			appId = model.NewApplicationId(ns, model.ApplicationKindStaticPods, strings.TrimSuffix(pod, "-"+nodeName))
		case ownerKind == model.ApplicationKindSparkApplication || ownerKind == model.ApplicationKindArgoWorkflow:
			appId = model.NewApplicationId(ns, ownerKind, jobSuffixRe.ReplaceAllString(ownerName, ""))
		case ownerName != "":
			appId = model.NewApplicationId(ns, ownerKind, ownerName)
		default:
			continue
		}
		podOwners[podId{name: pod, ns: ns}] = appId
		instance := pods[uid]
		if instance == nil {
			app := w.GetOrCreateApplication(appId, false)
			if appId.Kind == model.ApplicationKindCronJob {
				continue
			}
			instance = app.GetOrCreateInstance(pod, node)
			if instance.Pod == nil {
				instance.Pod = &model.Pod{}
			}
			if ownerKind == model.ApplicationKindReplicaSet {
				instance.Pod.ReplicaSet = ownerName
			}
			pods[uid] = instance
		}
		if node != nil && instance.Node == nil {
			instance.Node = node
			node.Instances = append(node.Instances, instance)
		}

		podIp := m.Labels["pod_ip"]
		hostIp := m.Labels["host_ip"]

		if podIp != "" && (podIp != hostIp || (node != nil && node.Fargate)) {
			if ip := net.ParseIP(podIp); ip != nil {
				isActive := m.Values.Last() == 1
				instance.TcpListens[model.Listen{IP: podIp, Port: "0", Proxied: false}] = isActive
				instance.Pod.IP = podIp
			}
		}
		if appId.Kind == model.ApplicationKindPod {
			podsOwnedByPods = append(podsOwnedByPods, instance)
		}
	}
	for _, instance := range podsOwnedByPods {
		id := podId{name: instance.Owner.Id.Name, ns: instance.Owner.Id.Namespace}
		if ownerOfOwner, ok := podOwners[id]; ok {
			if app := w.GetApplication(ownerOfOwner); app != nil {
				delete(w.Applications, instance.Owner.Id)
				instance.Owner = app
				app.Instances = append(app.Instances, instance)
			}
		}
	}
	return pods
}

func podLabels(metrics []*model.MetricValues, pods map[string]*model.Instance) {
	for _, m := range metrics {
		uid := m.Labels["uid"]
		if uid == "" {
			continue
		}
		instance := pods[uid]
		if instance == nil {
			//klog.Warningln("unknown pod:", uid, m.Labels["pod"], m.Labels["namespace"])
			continue
		}
		cluster, role := "", ""
		switch {
		case m.Labels["label_postgres_operator_crunchydata_com_cluster"] != "":
			cluster = m.Labels["label_postgres_operator_crunchydata_com_cluster"]
			role = m.Labels["label_postgres_operator_crunchydata_com_role"]
		case m.Labels["label_cluster_name"] != "" && m.Labels["label_team"] != "": // zalando pg operator
			cluster = m.Labels["label_cluster_name"]
			if m.Labels["label_application"] == "spilo" { // not a pooler (pgbouncer)
				role = m.Labels["label_spilo_role"]
			}
		case m.Labels["label_k8s_enterprisedb_io_cluster"] != "":
			cluster = m.Labels["label_k8s_enterprisedb_io_cluster"]
			role = m.Labels["label_role"]
		case m.Labels["label_cnpg_io_cluster"] != "":
			cluster = m.Labels["label_cnpg_io_cluster"]
			role = m.Labels["label_role"]
		case m.Labels["label_stackgres_io_cluster_name"] != "":
			cluster = m.Labels["label_stackgres_io_cluster_name"]
			role = m.Labels["label_role"]
		case m.Labels["label_app_kubernetes_io_managed_by"] == "percona-server-mongodb-operator":
			cluster = m.Labels["label_app_kubernetes_io_instance"]
		case strings.HasPrefix(m.Labels["label_helm_sh_chart"], "mongodb"):
			if m.Labels["label_app_kubernetes_io_name"] != "" && m.Labels["label_app_kubernetes_io_instance"] != "" {
				cluster = m.Labels["label_app_kubernetes_io_instance"] + "-" + m.Labels["label_app_kubernetes_io_name"]
			}
		case strings.HasPrefix(m.Labels["label_helm_sh_chart"], "redis") || strings.HasPrefix(m.Labels["label_helm_sh_chart"], "valkey"):
			if m.Labels["label_app_kubernetes_io_name"] != "" && m.Labels["label_app_kubernetes_io_instance"] != "" {
				cluster = m.Labels["label_app_kubernetes_io_instance"] + "-" + m.Labels["label_app_kubernetes_io_name"]
			}
		case strings.HasPrefix(m.Labels["label_helm_sh_chart"], "mysql"):
			if m.Labels["label_app_kubernetes_io_name"] != "" && m.Labels["label_app_kubernetes_io_instance"] != "" {
				cluster = m.Labels["label_app_kubernetes_io_instance"] + "-" + m.Labels["label_app_kubernetes_io_name"]
			}
		case strings.HasPrefix(m.Labels["label_helm_sh_chart"], "mariadb"):
			if m.Labels["label_app_kubernetes_io_name"] != "" && m.Labels["label_app_kubernetes_io_instance"] != "" {
				cluster = m.Labels["label_app_kubernetes_io_instance"] + "-" + m.Labels["label_app_kubernetes_io_name"]
			}
		case strings.HasPrefix(m.Labels["label_helm_sh_chart"], "clickhouse"):
			if m.Labels["label_app_kubernetes_io_name"] != "" && m.Labels["label_app_kubernetes_io_instance"] != "" {
				cluster = m.Labels["label_app_kubernetes_io_instance"] + "-" + m.Labels["label_app_kubernetes_io_name"]
			}
		case m.Labels["label_app_kubernetes_io_managed_by"] == "coroot-operator" && m.Labels["label_app_kubernetes_io_component"] == "clickhouse":
			cluster = m.Labels["label_app_kubernetes_io_part_of"] + "-" + m.Labels["label_app_kubernetes_io_component"]
		default:
			continue
		}
		if cluster != "" {
			instance.ClusterName.Update(m.Values, cluster)
		}
		if role == "master" {
			role = "primary"
		}
		instance.UpdateClusterRole(role, m.Values)
	}
}

func podStatus(queryName string, metrics []*model.MetricValues, pods map[string]*model.Instance) {
	for _, m := range metrics {
		uid := m.Labels["uid"]
		if uid == "" {
			continue
		}
		instance := pods[uid]
		if instance == nil {
			//klog.Warningln("unknown pod:", uid, m.Labels["pod"], m.Labels["namespace"])
			continue
		}
		switch queryName {
		case "kube_pod_status_phase":
			instance.Pod.LifeSpan = merge(instance.Pod.LifeSpan, m.Values, timeseries.NanSum)
			if m.Values.Last() > 0 {
				instance.Pod.Phase = m.Labels["phase"]
			}
			if m.Labels["phase"] == "Running" {
				instance.Pod.Running = merge(instance.Pod.Running, m.Values, timeseries.Any)
			}
		case "kube_pod_status_ready":
			if m.Labels["condition"] == "true" {
				instance.Pod.Ready = merge(instance.Pod.Ready, m.Values, timeseries.Any)
			}
		case "kube_pod_status_scheduled":
			if m.Values.Last() > 0 && m.Labels["condition"] == "true" {
				instance.Pod.Scheduled = true
			}
		}
	}
}

func podContainer(queryName string, metrics []*model.MetricValues, pods map[string]*model.Instance) {
	for _, m := range metrics {
		uid := m.Labels["uid"]
		if uid == "" {
			continue
		}
		instance := pods[uid]
		if instance == nil {
			//klog.Warningln("unknown pod:", uid, m.Labels["pod"], m.Labels["namespace"])
			continue
		}
		containerId := fmt.Sprintf("/k8s/%s/%s/%s", m.Labels["namespace"], m.Labels["pod"], m.Labels["container"])
		container := instance.GetOrCreateContainer(containerId, m.Labels["container"])

		switch queryName {
		case "kube_pod_init_container_info":
			container.InitContainer = true
		case "kube_pod_container_resource_requests":
			switch m.Labels["resource"] {
			case "cpu":
				container.CpuRequest = merge(container.CpuRequest, m.Values, timeseries.Max)
			case "memory":
				container.MemoryRequest = merge(container.MemoryRequest, m.Values, timeseries.Max)
			}
		case "kube_pod_container_status_ready":
			container.Ready = m.Values.Last() > 0
		case "kube_pod_container_status_waiting":
			if m.Values.Last() > 0 {
				container.Status = model.ContainerStatusWaiting
			}
		case "kube_pod_container_status_running":
			if m.Values.Last() > 0 {
				container.Status = model.ContainerStatusRunning
				container.Reason = ""
			}
		case "kube_pod_container_status_terminated":
			if m.Values.Last() > 0 {
				container.Status = model.ContainerStatusTerminated
			}
		case "kube_pod_container_status_waiting_reason":
			if m.Values.Last() > 0 {
				container.Status = model.ContainerStatusWaiting
				container.Reason = m.Labels["reason"]
			}
		case "kube_pod_container_status_terminated_reason":
			if m.Values.Last() > 0 {
				container.Status = model.ContainerStatusTerminated
				container.Reason = m.Labels["reason"]
			}
		case "kube_pod_container_status_last_terminated_reason":
			if m.Values.Last() > 0 {
				container.LastTerminatedReason = m.Labels["reason"]
			}
		}
	}
}
