package model

type ApplicationCategory string

const (
	ApplicationCategoryApplication  ApplicationCategory = "application"
	ApplicationCategoryControlPlane ApplicationCategory = "control-plane"
	ApplicationCategoryMonitoring   ApplicationCategory = "monitoring"
)

func (c ApplicationCategory) Default() bool {
	return c == ApplicationCategoryApplication
}

func (c ApplicationCategory) Builtin() bool {
	_, ok := BuiltinCategoryPatterns[c]
	return ok
}

func (c ApplicationCategory) Auxiliary() bool {
	return c.Monitoring() || c.ControlPlane()
}

func (c ApplicationCategory) Monitoring() bool {
	return c == ApplicationCategoryMonitoring
}

func (c ApplicationCategory) ControlPlane() bool {
	return c == ApplicationCategoryControlPlane
}

var BuiltinCategoryPatterns = map[ApplicationCategory][]string{
	ApplicationCategoryApplication: {},
	ApplicationCategoryControlPlane: {
		"kube-system/*",
		"*/kubelet",
		"*/kube-apiserver",
		"*/k3s",
		"*/k3s-agent",
		"*/systemd*",
		"*/containerd",
		"*/docker*",
		"*/*chaos-*",
		"istio*/*",
		"amazon-cloudwatch/*",
		"karpenter/*",
		"cert-manager/*",
		"argocd/*",
		"flux-system/*",
		"linkerd/*",
		"vault/*",
		"keda/*",
		"keycloak/*",
		"longhorn-system/*",
		"calico*/*",
		"_/esm-cache",
		"_/*motd*",
		"_/*apt*",
		"_/*fwupd*",
		"_/snap*",
		"keptn-system/*",
		"kyverno/*",
		"litmus/*",
		"openshift*/*",
		"_/crio*",
		"*/coredns",
		"chaos-mesh/*",
		"cilium/*",
		"external-dns/*",
		"gpu-operator/*",
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
		"*/*vmalert*",
		"datadog/*",
		"*/*datadog*",
		"*/*karma*",
	},
}
