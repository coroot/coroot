package views

import (
	"github.com/coroot/coroot-focus/model"
	"github.com/coroot/coroot-focus/views/application"
	"github.com/coroot/coroot-focus/views/overview"
)

func Overview(w *model.World) *overview.View {
	return overview.Render(w)
}

func Application(w *model.World, app *model.Application) *application.View {
	return application.Render(w, app)
}
