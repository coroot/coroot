package views

import (
	"github.com/coroot/coroot-focus/model"
	"github.com/coroot/coroot-focus/views/application"
	"github.com/coroot/coroot-focus/views/node"
	"github.com/coroot/coroot-focus/views/overview"
	"github.com/coroot/coroot-focus/views/search"
	"github.com/coroot/coroot-focus/views/widgets"
)

func Overview(w *model.World) *overview.View {
	return overview.Render(w)
}

func Application(w *model.World, app *model.Application) *application.View {
	return application.Render(w, app)
}

func Node(w *model.World, n *model.Node) *widgets.Dashboard {
	return node.Render(w, n)
}

func Search(w *model.World) *search.View {
	return search.Render(w)
}
