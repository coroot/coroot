package application

import (
	"fmt"
	"github.com/coroot/coroot-focus/model"
	"github.com/coroot/coroot-focus/timeseries"
	"github.com/coroot/coroot-focus/utils"
	"github.com/coroot/coroot-focus/views/widgets"
	"math"
	"net"
	"strconv"
	"strings"
)

func instances(ctx timeseries.Context, app *model.Application) *widgets.Dashboard {
	dash := widgets.NewDashboard(ctx, "Instances")

	up := timeseries.Aggregate(timeseries.NanSum)

	for _, i := range app.Instances {
		up.AddInput(i.UpAndRunning())

		status := widgets.NewTableCell("").SetStatus(model.UNKNOWN, "unknown")
		if i.Pod == nil {
			if i.IsUp() {
				status.SetStatus(model.OK, "ok")
			} else {
				status.SetStatus(model.WARNING, "down (no metrics)")
				if i.Node != nil && !i.Node.IsUp() {
					status.SetStatus(model.WARNING, "down (node down)")
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

		nodeStatus := model.UNKNOWN

		if i.Node != nil {
			if i.Node.IsUp() {
				nodeStatus = model.OK
			} else {
				nodeStatus = model.WARNING
			}
		}
		dash.GetOrCreateTable("Instance", "Status", "Restarts", "IP", "Node").AddRow(
			widgets.NewTableCell(i.Name),
			status,
			widgets.NewTableCell(strconv.FormatInt(restarts, 10)),
			widgets.NewTableCell("").SetValues(instanceIPs(i.TcpListens)),
			widgets.NewTableCell("").SetLink("node").SetStatus(nodeStatus, i.NodeName()),
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
