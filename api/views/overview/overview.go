package overview

import (
	"github.com/coroot/coroot/model"
)

type View struct {
	Views        []string       `json:"views"`
	Applications []*Application `json:"applications"`
	Costs        *Costs         `json:"costs"`
	Nodes        *model.Table   `json:"nodes"`
}

func Render(w *model.World, view string) *View {
	v := &View{
		Views: []string{"applications", "nodes"},
	}
	for _, n := range w.Nodes {
		if n.Price != nil {
			v.Views = append(v.Views, "costs")
			break
		}
	}

	switch view {
	case "applications":
		v.Applications = renderApplications(w)
	case "nodes":
		v.Nodes = renderNodes(w)
	case "costs":
		v.Costs = renderCosts(w)
	}
	return v
}
