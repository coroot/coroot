package categories

import (
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"sort"
	"strings"
)

type View struct {
	Categories []Category `json:"categories"`
}

type Category struct {
	Name            model.ApplicationCategory `json:"name"`
	BuiltinPatterns string                    `json:"builtin_patterns"`
	CustomPatterns  string                    `json:"custom_patterns"`
}

func Render(p *db.Project) *View {
	var categories []Category

	for c, ps := range model.BuiltinCategoryPatterns {
		categories = append(categories, Category{
			Name:            c,
			BuiltinPatterns: strings.Join(ps, " "),
			CustomPatterns:  strings.Join(p.Settings.ApplicationCategories[c], " "),
		})
	}
	for c, ps := range p.Settings.ApplicationCategories {
		if _, ok := model.BuiltinCategoryPatterns[c]; ok {
			continue
		}
		categories = append(categories, Category{
			Name:           c,
			CustomPatterns: strings.Join(ps, " "),
		})
	}

	sort.Slice(categories, func(i, j int) bool {
		return categories[i].Name < categories[j].Name
	})

	return &View{Categories: categories}
}
