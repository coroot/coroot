package overview

import (
	"github.com/coroot/coroot-focus/model"
	"github.com/coroot/coroot-focus/utils"
	"github.com/coroot/coroot-focus/views/widgets"
	"github.com/dustin/go-humanize"
	"math"
	"strconv"
	"strings"
)

type View struct {
	Applications []*Application `json:"applications"`
	Nodes        *widgets.Table `json:"nodes"`
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

	nodes := &widgets.Table{Header: []string{"node", "status", "availability zone", "IP"}}
	for _, n := range w.Nodes {
		node := widgets.NewTableCell(n.Name.Value()).SetLink("node")
		ips := utils.NewStringSet()
		if n.CpuCapacity != nil {
			if vcpu := n.CpuCapacity.Last(); !math.IsNaN(vcpu) {
				node.AddTag("vCPU: " + strconv.Itoa(int(vcpu)))
			}
		}
		if total := n.MemoryTotalBytes.Last(); !math.IsNaN(total) {
			node.AddTag("memory: " + humanize.Bytes(uint64(total)))
		}

		status := widgets.NewTableCell("").SetStatus(model.OK, "up")
		if !n.IsUp() {
			status.SetStatus(model.WARNING, "down (no metrics)")
		}
		for _, iface := range n.NetInterfaces {
			for _, ip := range iface.Addresses {
				ips.Add(ip)
			}
		}
		nodes.AddRow(
			node,
			status,
			widgets.NewTableCell(n.AvailabilityZone.Value()).AddTag("cloud: "+strings.ToLower(n.CloudProvider.Value())),
			widgets.NewTableCell("").SetValues(ips.Items()),
		)
	}
	return &View{Applications: appsUsed, Nodes: nodes}
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
