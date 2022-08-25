package views

import (
	"github.com/coroot/coroot-focus/model"
	"github.com/coroot/coroot-focus/views/application"
	"github.com/coroot/coroot-focus/views/overview"
	"github.com/coroot/coroot-focus/views/search"
)

func Overview(w *model.World) *overview.View {
	return overview.Render(w)
}

func Application(w *model.World, app *model.Application) *application.View {
	return application.Render(w, app)
}

func Search(w *model.World) *search.View {
	return search.Render(w)
}
