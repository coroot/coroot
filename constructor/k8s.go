package constructor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
	"strings"
)

type podId struct {
	pod string
	ns  string
}

func loadKubernetesMetadata(w *model.World, metrics map[string][]model.MetricValues) {
	loadServices(w, metrics["kube_service_info"])
	pods := podInfo(w, metrics["kube_pod_info"])
	podLabels(metrics["kube_pod_labels"], pods)

	for queryName := range QUERIES {
		switch {
		case strings.HasPrefix(queryName, "kube_pod_status_"):
			podStatus(queryName, metrics[queryName], pods)
		case strings.HasPrefix(queryName, "kube_pod_init_container_"):
			podContainerStatus(queryName, metrics[queryName], pods)
		case strings.HasPrefix(queryName, "kube_pod_container_status_"):
			podContainerStatus(queryName, metrics[queryName], pods)
		}
	}
	loadApplications(w, metrics)
}

func loadServices(w *model.World, metrics []model.MetricValues) {
	for _, m := range metrics {
		name := m.Labels["service"]
		if name == "kubernetes" {
			name = "kube-apiserver"
		}
		if clusterIP := m.Labels["cluster_ip"]; clusterIP != "" {
			w.Services = append(w.Services, &model.Service{
				Name:      name,
				Namespace: m.Labels["namespace"],
				ClusterIP: clusterIP,
			})
		}
	}
}

func loadApplications(w *model.World, metrics map[string][]model.MetricValues) {
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

func podInfo(w *model.World, metrics []model.MetricValues) map[podId]*model.Instance {
	pods := map[podId]*model.Instance{}
	for _, m := range metrics {
		w.IntegrationStatus.KubeStateMetrics.Installed = true
		pod := m.Labels["pod"]
		ns := m.Labels["namespace"]
		ownerName := m.Labels["created_by_name"]
		ownerKind := m.Labels["created_by_kind"]
		var appId model.ApplicationId

		switch {
		case ownerKind == "" || ownerKind == "<none>" || ownerKind == "Node":
			appId = model.NewApplicationId(ns, model.ApplicationKindStaticPods, strings.TrimSuffix(pod, "-"+m.Labels["node"]))
		case ownerName != "" && ownerKind != "":
			appId = model.NewApplicationId(ns, model.ApplicationKind(ownerKind), ownerName)
		default:
			continue
		}
		instance := w.GetOrCreateApplication(appId).GetOrCreateInstance(pod)
		instance.Pod = &model.Pod{}
		if model.ApplicationKind(ownerKind) == model.ApplicationKindReplicaSet {
			instance.Pod.ReplicaSet = ownerName
		}
		pods[podId{
			pod: instance.Name,
			ns:  instance.InstanceId.OwnerId.Namespace,
		}] = instance
	}
	return pods
}

func podLabels(metrics []model.MetricValues, pods map[podId]*model.Instance) {
	for _, m := range metrics {
		id := podId{pod: m.Labels["pod"], ns: m.Labels["namespace"]}
		instance := pods[id]
		if instance == nil {
			klog.Warningln("unknown pod:", id)
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

func podStatus(queryName string, metrics []model.MetricValues, pods map[podId]*model.Instance) {
	for _, m := range metrics {
		id := podId{pod: m.Labels["pod"], ns: m.Labels["namespace"]}
		instance := pods[id]
		if instance == nil {
			klog.Warningln("unknown pod:", id)
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

func podContainerStatus(queryName string, metrics []model.MetricValues, pods map[podId]*model.Instance) {
	for _, m := range metrics {
		id := podId{pod: m.Labels["pod"], ns: m.Labels["namespace"]}
		instance := pods[id]
		if instance == nil {
			klog.Warningln("unknown pod:", id)
			continue
		}
		container := instance.GetOrCreateContainer(m.Labels["container"])

		switch queryName {
		case "kube_pod_init_container_info":
			container.InitContainer = true
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
		case "kube_pod_container_status_last_terminated_reason":
			if m.Values.Last() > 0 {
				container.LastTerminatedReason = m.Labels["reason"]
			}
		}
	}
}
