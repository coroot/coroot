package overview

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/dustin/go-humanize"
	"math"
	"net"
	"sort"
	"strconv"
	"strings"
)

type View struct {
	Applications []*Application `json:"applications"`
	Nodes        *model.Table   `json:"nodes"`
}

type Application struct {
	Id         model.ApplicationId       `json:"id"`
	Category   model.ApplicationCategory `json:"category"`
	Labels     model.Labels              `json:"labels"`
	Status     model.Status              `json:"status"`
	Indicators []model.Indicator         `json:"indicators"`

	Upstreams   []Link `json:"upstreams"`
	Downstreams []Link `json:"downstreams"`
}

type Link struct {
	Id     model.ApplicationId `json:"id"`
	Status model.Status        `json:"status"`
	Stats  []string            `json:"stats"`
	Weight float32             `json:"weight"`
}

func Render(w *model.World) *View {
	var apps []*Application
	used := map[model.ApplicationId]bool{}
	for _, a := range w.Applications {
		app := Application{
			Id:          a.Id,
			Category:    a.Category,
			Labels:      a.Labels(),
			Status:      a.Status,
			Indicators:  model.CalcIndicators(a),
			Upstreams:   []Link{},
			Downstreams: []Link{},
		}

		upstreams := map[model.ApplicationId]struct {
			status      model.Status
			connections []*model.Connection
		}{}
		downstreams := map[model.ApplicationId]bool{}
		for _, i := range a.Instances {
			if i.IsObsolete() {
				continue
			}
			for _, u := range i.Upstreams {
				if u.IsObsolete() || u.RemoteInstance == nil || u.RemoteInstance.OwnerId == app.Id {
					continue
				}
				status := u.Status()
				s := upstreams[u.RemoteInstance.OwnerId]
				if status >= s.status {
					s.status = status
				}
				s.connections = append(s.connections, u)
				upstreams[u.RemoteInstance.OwnerId] = s
			}
		}
		for _, d := range a.Downstreams {
			if d.IsObsolete() || d.Instance.OwnerId == app.Id {
				continue
			}
			downstreams[d.Instance.OwnerId] = true
		}

		for id, s := range upstreams {
			l := Link{Id: id, Status: s.status}
			requests := model.GetConnectionsRequestsSum(s.connections).Last()
			latency := model.GetConnectionsRequestsLatency(s.connections).Last()
			if !math.IsNaN(requests) {
				l.Weight = float32(requests)
				l.Stats = append(l.Stats, utils.FormatFloat(requests)+" rps")
			}
			if !math.IsNaN(latency) {
				l.Stats = append(l.Stats, utils.FormatLatency(latency))
			}
			app.Upstreams = append(app.Upstreams, l)
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

	table := &model.Table{Header: []string{"Node", "Status", "Availability zone", "IP", "CPU", "Memory", "Network", "Uptime"}}
	for _, n := range w.Nodes {
		node := model.NewTableCell(n.Name.Value())
		node.Link = model.NewRouterLink(n.Name.Value()).SetRoute("node").SetParam("name", n.Name.Value())
		ips := utils.NewStringSet()

		cpuPercent, memoryPercent := model.NewTableCell(), model.NewTableCell("")

		if t := n.InstanceType.Value(); t != "" {
			node.AddTag("Type: " + t)
		}
		if l := n.CpuCapacity.Last(); !math.IsNaN(l) {
			node.AddTag("vCPU: " + strconv.Itoa(int(l)))
		}
		if l := n.CpuUsagePercent.Last(); !math.IsNaN(l) {
			cpuPercent.SetProgress(int(l), "blue")
		}

		if total := n.MemoryTotalBytes.Last(); !math.IsNaN(total) {
			node.AddTag("memory: " + humanize.Bytes(uint64(total)))
			if avail := n.MemoryAvailableBytes.Last(); !math.IsNaN(avail) {
				memoryPercent.SetProgress(int(100-avail/total*100), "deep-purple")
			}
		}

		status := model.NewTableCell().SetStatus(model.OK, "up")
		if !n.IsUp() {
			status.SetStatus(model.WARNING, "down (no metrics)")
		}

		network := model.NewTableCell()
		for _, iface := range n.NetInterfaces {
			if iface.Up.Last() != 1 {
				continue
			}
			if iface.RxBytes.IsEmpty() || iface.TxBytes.IsEmpty() {
				continue
			}
			for _, ip := range iface.Addresses {
				ips.Add(ip)
			}
			if ips.Len() == 0 {
				for _, instance := range n.Instances {
					for l := range instance.TcpListens {
						if ip := net.ParseIP(l.IP); ip != nil && !ip.IsLoopback() {
							ips.Add(l.IP)
						}
					}
				}
			}
			network.NetInterfaces = append(network.NetInterfaces, model.NetInterface{
				Name: iface.Name,
				Rx:   utils.HumanBits(iface.RxBytes.Last() * 8),
				Tx:   utils.HumanBits(iface.TxBytes.Last() * 8),
			})
		}
		sort.Slice(network.NetInterfaces, func(i, j int) bool {
			return network.NetInterfaces[i].Name < network.NetInterfaces[j].Name
		})

		uptime := model.NewTableCell()
		if v := n.Uptime.Last(); !math.IsNaN(v) {
			uptime.SetValue(utils.FormatDurationShort(timeseries.Duration(int64(v)), 1))
		}

		table.AddRow(
			node,
			status,
			model.NewTableCell(n.AvailabilityZone.Value()).SetUnit("("+strings.ToLower(n.CloudProvider.Value())+")"),
			model.NewTableCell(ips.Items()...),
			cpuPercent,
			memoryPercent,
			network,
			uptime,
		)
	}
	return &View{Applications: appsUsed, Nodes: table}
}
