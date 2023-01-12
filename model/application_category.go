package model

import (
	"fmt"
	"github.com/coroot/coroot/utils"
	"sort"
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

var BuiltinCategoryPatterns = map[ApplicationCategory][]string{
	ApplicationCategoryApplication: {},
	ApplicationCategoryControlPlane: {
		"kube-system/*",
		"*/kubelet",
		"*/kube-apiserver",
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
		if i.ApplicationTypes()["etcd"] {
			for _, d := range app.Downstreams {
				if d.Instance.ApplicationTypes()["kube-apiserver"] {
					return ApplicationCategoryControlPlane
				}
			}
		}
	}

	return ApplicationCategoryApplication
}
