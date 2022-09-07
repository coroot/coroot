package application

import (
	"fmt"
	widgets2 "github.com/coroot/coroot/api/views/widgets"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"math"
	"net"
	"strconv"
	"strings"
)

func instances(ctx timeseries.Context, app *model.Application) *widgets2.Dashboard {
	dash := widgets2.NewDashboard(ctx, "Instances")

	up := timeseries.Aggregate(timeseries.NanSum)

	for _, i := range app.Instances {
		up.AddInput(i.UpAndRunning())

		status := widgets2.NewTableCell().SetStatus(model.UNKNOWN, "unknown")
		if i.Pod == nil {
			if i.IsUp() {
				status.SetStatus(model.OK, "ok")
			} else {
				if app.Id.Kind != model.ApplicationKindExternalService {
					status.SetStatus(model.WARNING, "down (no metrics)")
					if i.Node != nil && !i.Node.IsUp() {
						status.SetStatus(model.WARNING, "down (node down)")
					}
				}
			}
		} else {
			if i.Pod.IsObsolete() {
				continue
			}
			switch i.Pod.Phase {
			case "Pending":
				msg := "down (pending"
				if !i.Pod.Scheduled {
					msg += ", not scheduled"
				}
				reasons := utils.NewStringSet()
				for _, c := range i.Containers {
					if c.Status == model.ContainerStatusWaiting {
						reasons.Add(c.Reason)
					}
				}
				if reasons.Len() > 0 {
					msg += fmt.Sprintf(" (%s)", strings.Join(reasons.Items(), ", "))
				}
				msg += ")"
				status.SetStatus(model.WARNING, msg)
			case "Succeeded":
				status.SetStatus(model.OK, "succeeded")
			case "Running":
				switch {
				case !i.IsUp():
					msg := ""
					if i.Node != nil && !i.Node.IsUp() {
						msg = "down (node down)"
					} else {
						reasons := utils.NewStringSet()
						containerStatus := ""
						for _, c := range i.Containers {
							if c.Status == model.ContainerStatusRunning {
								continue
							}
							containerStatus = string(c.Status)
							reasons.Add(c.Reason)
							if c.Status == model.ContainerStatusTerminated {
								reasons.Add(c.LastTerminatedReason)
							}
						}
						if reasons.Len() > 0 {
							msg = fmt.Sprintf(
								"down (%s - %s)",
								containerStatus,
								strings.Join(reasons.Items(), ","),
							)
						} else {
							msg = "down (no metrics)"
						}
						status.SetStatus(model.WARNING, msg)
					}
				case !i.Pod.IsReady():
					status.SetStatus(model.WARNING, "down (readiness probe failed)")
				default:
					status.SetStatus(model.OK, "up (running)")
				}
			case "Error":
				status.SetStatus(model.WARNING, "down (error)")
			}
		}
		restarts := int64(0)
		for _, c := range i.Containers {
			if r := timeseries.Reduce(timeseries.NanSum, c.Restarts); !math.IsNaN(r) {
				restarts += int64(r)
			}
		}
		restartsCell := widgets2.NewTableCell()
		if restarts > 0 {
			restartsCell.SetValue(strconv.FormatInt(restarts, 10))
		}

		nodeStatus := model.UNKNOWN

		if i.Node != nil {
			if i.Node.IsUp() {
				nodeStatus = model.OK
			} else {
				nodeStatus = model.WARNING
			}
		}
		dash.GetOrCreateTable("Instance", "Status", "Restarts", "IP", "Node").AddRow(
			widgets2.NewTableCell(i.Name),
			status,
			restartsCell,
			widgets2.NewTableCell(instanceIPs(i.TcpListens)...),
			widgets2.NewTableCell().SetLink("node").SetStatus(nodeStatus, i.NodeName()),
		)
	}

	chart := dash.GetOrCreateChart("Instances").Stacked().AddSeries("up", up)
	if app.DesiredInstances != nil {
		chart.SetThreshold("desired", app.DesiredInstances, timeseries.Any)
		chart.Threshold.Color = "red"
		chart.Threshold.Fill = true
	}
	return dash
}

func instanceIPs(listens map[model.Listen]bool) []string {
	ips := utils.NewStringSet()
	for l := range listens {
		if ip := net.ParseIP(l.IP); !ip.IsLoopback() {
			ips.Add(l.IP)
		}
	}
	return ips.Items()
}
