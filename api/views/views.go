package views

import (
	"context"
	"github.com/coroot/coroot/api/views/application"
	"github.com/coroot/coroot/api/views/categories"
	"github.com/coroot/coroot/api/views/configs"
	"github.com/coroot/coroot/api/views/integrations"
	"github.com/coroot/coroot/api/views/logs"
	"github.com/coroot/coroot/api/views/node"
	"github.com/coroot/coroot/api/views/overview"
	"github.com/coroot/coroot/api/views/profile"
	"github.com/coroot/coroot/api/views/project"
	"github.com/coroot/coroot/api/views/search"
	"github.com/coroot/coroot/api/views/trace"
	"github.com/coroot/coroot/cache"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/profiling"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/tracing"
	"net/url"
)

func Status(p *db.Project, cacheStatus *cache.Status, w *model.World) *project.Status {
	return project.RenderStatus(p, cacheStatus, w)
}

func Overview(w *model.World, view string) *overview.Overview {
	return overview.Render(w, view)
}

func Application(w *model.World, app *model.Application) *application.View {
	return application.Render(w, app)
}

func Profile(ctx context.Context, pyroscope *profiling.PyroscopeClient, app *model.Application, appSettings *db.ApplicationSettings, q url.Values, wCtx timeseries.Context) *profile.View {
	return profile.Render(ctx, pyroscope, app, appSettings, q, wCtx)
}

func Tracing(ctx context.Context, clickhouse *tracing.ClickhouseClient, app *model.Application, appSettings *db.ApplicationSettings, q url.Values, w *model.World) *trace.View {
	return trace.Render(ctx, clickhouse, app, appSettings, q, w)
}

func Logs(ctx context.Context, clickhouse *tracing.ClickhouseClient, app *model.Application, appSettings *db.ApplicationSettings, q url.Values, w *model.World) *logs.View {
	return logs.Render(ctx, clickhouse, app, appSettings, q, w)
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
