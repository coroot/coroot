package applications

import (
	"sort"
	"strings"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
)

type CategoriesView struct {
	Categories   []Category        `json:"categories"`
	Integrations map[string]string `json:"integrations"`
}

type Category struct {
	Name                model.ApplicationCategory `json:"name"`
	Builtin             bool                      `json:"builtin"`
	Default             bool                      `json:"default"`
	BuiltinPatterns     string                    `json:"builtin_patterns"`
	CustomPatterns      string                    `json:"custom_patterns"`
	NotifyOfDeployments bool                      `json:"notify_of_deployments"`
}

func RenderCategories(p *db.Project) *CategoriesView {
	var categories []Category
	for c, ps := range model.BuiltinCategoryPatterns {
		categories = append(categories, Category{
			Name:                c,
			Builtin:             c.Builtin(),
			Default:             c.Default(),
			BuiltinPatterns:     strings.Join(ps, " "),
			CustomPatterns:      strings.Join(p.Settings.ApplicationCategories[c], " "),
			NotifyOfDeployments: p.Settings.ApplicationCategorySettings[c].NotifyOfDeployments,
		})
	}
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].Name < categories[j].Name
	})

	var custom []Category
	for c, ps := range p.Settings.ApplicationCategories {
		if _, ok := model.BuiltinCategoryPatterns[c]; ok {
			continue
		}
		custom = append(custom, Category{
			Name:                c,
			CustomPatterns:      strings.Join(ps, " "),
			NotifyOfDeployments: p.Settings.ApplicationCategorySettings[c].NotifyOfDeployments,
		})
	}
	sort.Slice(custom, func(i, j int) bool {
		return custom[i].Name < custom[j].Name
	})

	categories = append(categories, custom...)

	v := &CategoriesView{Categories: categories, Integrations: map[string]string{}}

	for _, i := range p.Settings.Integrations.GetInfo() {
		if i.Configured && i.Deployments {
			v.Integrations[i.Title] = i.Details
		}
	}
	return v
}
