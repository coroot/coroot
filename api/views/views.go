package views

import (
	"github.com/coroot/coroot/api/views/application"
	"github.com/coroot/coroot/api/views/categories"
	"github.com/coroot/coroot/api/views/configs"
	"github.com/coroot/coroot/api/views/node"
	"github.com/coroot/coroot/api/views/overview"
	"github.com/coroot/coroot/api/views/project"
	"github.com/coroot/coroot/api/views/search"
	"github.com/coroot/coroot/cache"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
)

func Status(p *db.Project, cacheStatus *cache.Status, w *model.World) *project.Status {
	return project.RenderStatus(p, cacheStatus, w)
}

func Overview(w *model.World, p *db.Project) *overview.View {
	return overview.Render(w, p)
}

func Application(w *model.World, app *model.Application) *application.View {
	return application.Render(w, app)
}

func Node(w *model.World, n *model.Node) *model.AuditReport {
	return node.Render(w, n)
}

func Search(w *model.World) *search.View {
	return search.Render(w)
}

func Configs(checkConfigs model.CheckConfigs) *configs.View {
	return configs.Render(checkConfigs)
}

func Categories(p *db.Project) *categories.View {
	return categories.Render(p)
}
