package views

import (
	"context"
	"net/url"

	"github.com/coroot/coroot/api/views/application"
	"github.com/coroot/coroot/api/views/aws"
	"github.com/coroot/coroot/api/views/categories"
	"github.com/coroot/coroot/api/views/configs"
	"github.com/coroot/coroot/api/views/integrations"
	"github.com/coroot/coroot/api/views/logs"
	"github.com/coroot/coroot/api/views/overview"
	"github.com/coroot/coroot/api/views/profiling"
	"github.com/coroot/coroot/api/views/tracing"
	"github.com/coroot/coroot/clickhouse"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func Overview(ctx context.Context, ch *clickhouse.Client, w *model.World, view, query string) *overview.Overview {
	return overview.Render(ctx, ch, w, view, query)
}

func Application(w *model.World, app *model.Application) *application.View {
	return application.Render(w, app)
}

func Profiling(ctx context.Context, ch *clickhouse.Client, app *model.Application, q url.Values, wCtx timeseries.Context) *profiling.View {
	return profiling.Render(ctx, ch, app, q, wCtx)
}

func Tracing(ctx context.Context, ch *clickhouse.Client, app *model.Application, q url.Values, w *model.World) *tracing.View {
	return tracing.Render(ctx, ch, app, q, w)
}

func Logs(ctx context.Context, ch *clickhouse.Client, app *model.Application, q url.Values, w *model.World) *logs.View {
	return logs.Render(ctx, ch, app, q, w)
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

func AWS(w *model.World) *aws.View {
	return aws.Render(w)
}
