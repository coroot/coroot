package model

type ApplicationCategory string

const (
	ApplicationCategoryApplication      ApplicationCategory = "application"
	ApplicationCategoryControlPlane     ApplicationCategory = "control-plane"
	ApplicationCategoryStorage          ApplicationCategory = "storage"
	ApplicationCategoryServiceMesh      ApplicationCategory = "service-mesh"
	ApplicationCategoryDatabase         ApplicationCategory = "database"
	ApplicationCategorySecurity         ApplicationCategory = "security"
	ApplicationCategoryNetworking       ApplicationCategory = "networking"
	ApplicationCategoryMonitoring       ApplicationCategory = "monitoring"
	ApplicationCategoryChaosEngineering ApplicationCategory = "chaos-engineering"
	ApplicationCategoryMessaging        ApplicationCategory = "messaging"
)

func (c ApplicationCategory) Default() bool {
	return c == ApplicationCategoryApplication
}

func (c ApplicationCategory) Builtin() bool {
	_, ok := BuiltinCategoryPatterns[c]
	return ok
}

func (c ApplicationCategory) Auxiliary() bool {
	return c.ControlPlane() ||
		c.Storage() ||
		c.ServiceMesh() ||
		c.Security() ||
		c.Networking() ||
		c.Monitoring() ||
		c.ChaosEngineering()
}

func (c ApplicationCategory) Monitoring() bool {
	return c == ApplicationCategoryMonitoring
}

func (c ApplicationCategory) ControlPlane() bool {
	return c == ApplicationCategoryControlPlane
}

func (c ApplicationCategory) Storage() bool {
	return c == ApplicationCategoryStorage
}

func (c ApplicationCategory) ServiceMesh() bool {
	return c == ApplicationCategoryServiceMesh
}

func (c ApplicationCategory) Security() bool {
	return c == ApplicationCategorySecurity
}

func (c ApplicationCategory) Networking() bool {
	return c == ApplicationCategoryNetworking
}

func (c ApplicationCategory) ChaosEngineering() bool {
	return c == ApplicationCategoryChaosEngineering
}

var BuiltinCategoryPatterns = map[ApplicationCategory][]string{
	ApplicationCategoryApplication: {},
	ApplicationCategoryControlPlane: {
		"kube-system/*",
		"*/kubelet",
		"*/kube-apiserver",
		"*/coredns",
		"*/k3s",
		"*/k3s-agent",
		"*/systemd*",
		"*/containerd",
		"*/docker*",
		"_/crio*",
		"argocd/*",
		"flux-system/*",
		"keda/*",
		"karpenter/*",
		"openshift*/*",
		"keptn-system/*",
		"_/esm-cache",
		"_/*motd*",
		"_/*apt*",
		"_/*fwupd*",
		"_/snap*",
		"metallb-system/*",
		"knative-serving/*",
		"kubernetes-dashboard/*",
		"backstage/*",
		"gpu-operator/*",
	},
	ApplicationCategoryStorage: {
		"longhorn-system/*",
		"rook-ceph/*",
		"minio-operator/*",
		"velero/*",
	},
	ApplicationCategoryServiceMesh: {
		"istio-system/*",
		"linkerd/*",
		"consul/*",
		"kuma-system/*",
	},
	ApplicationCategorySecurity: {
		"cert-manager/*",
		"vault/*",
		"keycloak/*",
		"kyverno/*",
		"falco/*",
		"gatekeeper-system/*",
		"trivy-system/*",
	},
	ApplicationCategoryNetworking: {
		"calico-system/*",
		"external-dns/*",
		"cilium/*",
		"flannel/*",
		"traefik/*",
		"ingress-nginx/*",
	},
	ApplicationCategoryDatabase: {
		"postgresql/*",
		"redis/*",
		"mongodb/*",
		"mysql/*",
		"mariadb/*",
		"cassandra/*",
	},
	ApplicationCategoryMessaging: {
		"rabbitmq/*",
		"kafka/*",
		"strimzi/*",
	},
	ApplicationCategoryMonitoring: {
		"monitoring/*",
		"prometheus/*",
		"*/*prometheus*",
		"grafana/*",
		"*/*grafana*",
		"*/*alertmanager*",
		"coroot/*",
		"*/*coroot*",
		"metrics-server/*",
		"loki/*",
		"observability/*",
		"jaeger/*",
		"thanos/*",
		"sentry/*",
		"*/*victoria-metrics*",
		"*/*victoria-logs*",
		"*/*vminsert*",
		"*/*vmselect*",
		"*/*vmstorage*",
		"*/*vmagent*",
		"datadog/*",
		"*/*datadog*",
		"elasticsearch/*",
		"kibana/*",
		"fluent-bit/*",
		"amazon-cloudwatch/*",
	},
	ApplicationCategoryChaosEngineering: {
		"*/*chaos-*",
		"chaos-mesh/*",
		"litmus/*",
	},
}
