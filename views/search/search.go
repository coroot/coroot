package search

import (
	"github.com/coroot/coroot-focus/model"
	"sort"
)

type View struct {
	Applications []Application `json:"applications"`
	Nodes        []Node        `json:"nodes"`
}

type Application struct {
	Id model.ApplicationId `json:"id"`
}

type Node struct {
	Name string `json:"name"`
}

func Render(w *model.World) *View {
	res := &View{}
	for _, a := range w.Applications {
		res.Applications = append(res.Applications, Application{Id: a.Id})
	}
	for _, n := range w.Nodes {
		res.Nodes = append(res.Nodes, Node{Name: n.Name.Value()})
	}
	sort.Slice(res.Applications, func(i, j int) bool {
		return res.Applications[i].Id.Name < res.Applications[j].Id.Name
	})
	sort.Slice(res.Nodes, func(i, j int) bool {
		return res.Nodes[i].Name < res.Nodes[j].Name
	})
	return res
}
