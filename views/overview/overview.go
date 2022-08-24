package overview

import "github.com/coroot/coroot-focus/model"

type View struct {
	Applications []*Application `json:"applications"`
}

type Application struct {
	Id       model.ApplicationId `json:"id"`
	Category string              `json:"category"`
	Labels   model.Labels        `json:"labels"`

	Upstreams   []Link `json:"upstreams"`
	Downstreams []Link `json:"downstreams"`
}

type Link struct {
	Id        model.ApplicationId `json:"id"`
	Status    model.Status        `json:"status"`
	Direction string              `json:"direction"`
}

func Render(w *model.World) *View {
	var apps []*Application
	used := map[model.ApplicationId]bool{}
	for _, a := range w.Applications {
		app := Application{
			Id:          a.Id,
			Category:    category(a),
			Labels:      a.Labels(),
			Upstreams:   []Link{},
			Downstreams: []Link{},
		}
		upstreams := map[model.ApplicationId]model.Status{}
		downstreams := map[model.ApplicationId]bool{}
		for _, i := range a.Instances {
			for _, u := range i.Upstreams {
				if u.Obsolete() || u.RemoteInstance == nil || u.RemoteInstance.OwnerId == app.Id {
					continue
				}
				status := u.Status()
				if status >= upstreams[u.RemoteInstance.OwnerId] {
					upstreams[u.RemoteInstance.OwnerId] = status
				}
			}
			for _, d := range i.Downstreams {
				if d.Obsolete() || d.Instance == nil || d.Instance.OwnerId == app.Id {
					continue
				}
				downstreams[d.Instance.OwnerId] = true
			}
		}
		for id, status := range upstreams {
			app.Upstreams = append(app.Upstreams, Link{Id: id, Status: status})
			used[a.Id] = true
			used[id] = true
		}
		for id := range downstreams {
			app.Downstreams = append(app.Downstreams, Link{Id: id})
			used[a.Id] = true
			used[id] = true
		}

		apps = append(apps, &app)
	}
	var appsUsed []*Application
	for _, a := range apps {
		if !used[a.Id] {
			continue
		}
		appsUsed = append(appsUsed, a)
	}
	return &View{Applications: appsUsed}
}

func category(app *model.Application) string {
	if app.IsControlPlane() {
		return "control-plane"
	}
	if app.IsMonitoring() {
		return "monitoring"
	}
	return "application"
}
