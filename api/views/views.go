package views

import (
	"github.com/coroot/coroot/api/views/application"
	"github.com/coroot/coroot/api/views/categories"
	"github.com/coroot/coroot/api/views/configs"
	"github.com/coroot/coroot/api/views/integrations"
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

func Overview(w *model.World) *overview.View {
	return overview.Render(w)
}

func Application(w *model.World, app *model.Application, incidents []db.Incident) *application.View {
	return application.Render(w, app, incidents)
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

func Integrations(p *db.Project) *integrations.View {
	return integrations.Render(p)
}
