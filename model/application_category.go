package model

import (
	"fmt"
	"sort"

	"github.com/coroot/coroot/utils"
)

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
		"istio-system/*",
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
		"calico-system/*",
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
	},
}

func CalcApplicationCategory(appId ApplicationId, customPatterns map[ApplicationCategory][]string) ApplicationCategory {
	categories := make([]ApplicationCategory, 0, len(BuiltinCategoryPatterns)+len(customPatterns))
	for c := range BuiltinCategoryPatterns {
		categories = append(categories, c)
	}
	for c := range customPatterns {
		if _, ok := BuiltinCategoryPatterns[c]; ok {
			continue
		}
		categories = append(categories, c)
	}
	sort.Slice(categories, func(i, j int) bool {
		return categories[i] < categories[j]
	})

	id := fmt.Sprintf("%s/%s", appId.Namespace, appId.Name)
	for _, c := range categories {
		if utils.GlobMatch(id, BuiltinCategoryPatterns[c]...) || utils.GlobMatch(id, customPatterns[c]...) {
			return c
		}
	}

	return ApplicationCategoryApplication
}
