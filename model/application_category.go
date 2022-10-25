package model

import (
	"fmt"
	"github.com/coroot/coroot/utils"
	"sort"
)

type ApplicationCategory string

const (
	ApplicationCategoryControlPlane ApplicationCategory = "control-plane"
	ApplicationCategoryMonitoring   ApplicationCategory = "monitoring"
	ApplicationCategoryApplication  ApplicationCategory = "application"
)

var BuiltinCategoryPatterns = map[ApplicationCategory][]string{
	ApplicationCategoryControlPlane: {
		"kube-system/*",
		"*/kubelet",
	},
	ApplicationCategoryMonitoring: {
		"monitoring/*",
		"prometheus/*",
		"*/*prometheus*",
		"grafana/*",
		"*/*grafana*",
		"*/*alertmanager*",
		"coroot/*",
	},
}

func CalcApplicationCategory(app *Application, customPatterns map[ApplicationCategory][]string) ApplicationCategory {
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

	id := fmt.Sprintf("%s/%s", app.Id.Namespace, app.Id.Name)
	for _, c := range categories {
		if utils.GlobMatch(id, BuiltinCategoryPatterns[c]) || utils.GlobMatch(id, customPatterns[c]) {
			return c
		}
	}

	for _, i := range app.Instances {
		if i.ApplicationTypes()["k3s"] {
			return ApplicationCategoryControlPlane
		}
		if !i.ApplicationTypes()["etcd"] {
			continue
		}
		for _, d := range i.Downstreams {
			if d.Instance != nil || d.Instance.ApplicationTypes()["kube-apiserver"] {
				return ApplicationCategoryControlPlane
			}
		}
	}

	return ApplicationCategoryApplication
}
