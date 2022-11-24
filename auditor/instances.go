package auditor

import (
	"fmt"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"math"
	"net"
	"strconv"
	"strings"
)

func (a *appAuditor) instances() {
	report := a.addReport(model.AuditReportInstances)

	up := timeseries.Aggregate(timeseries.NanSum)

	availability := report.CreateCheck(model.Checks.InstanceAvailability)
	restarts := report.CreateCheck(model.Checks.InstanceRestarts)

	for _, i := range a.app.Instances {
		up.AddInput(i.UpAndRunning())

		status := model.NewTableCell().SetStatus(model.UNKNOWN, "unknown")
		if i.Rds != nil {
			switch {
			case math.IsNaN(timeseries.Last(i.Rds.LifeSpan)):
				status.SetStatus(model.WARNING, "down (no metrics)")
			case i.Rds.Status.Value() != "available":
				status.SetStatus(model.WARNING, i.Rds.Status.Value())
			default:
				status.SetStatus(model.OK, i.Rds.Status.Value())
			}
		} else if i.Pod == nil {
			if i.IsUp() {
				status.SetStatus(model.OK, "ok")
			} else {
				if a.app.Id.Kind != model.ApplicationKindExternalService {
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
		if *status.Status > model.OK {
			availability.AddItem(i.Name)
		}
		restartsCount := int64(0)
		for _, c := range i.Containers {
			if r := timeseries.Reduce(timeseries.NanSum, c.Restarts); !math.IsNaN(r) {
				restarts.Inc(int64(r))
				restartsCount += int64(r)
			}
		}
		restartsCell := model.NewTableCell()
		if restartsCount > 0 {
			restartsCell.SetValue(strconv.FormatInt(restartsCount, 10))
		}

		nodeStatus := model.UNKNOWN

		if i.Node != nil {
			if i.Node.IsUp() {
				nodeStatus = model.OK
			} else {
				nodeStatus = model.WARNING
			}
		}
		report.GetOrCreateTable("Instance", "Status", "Restarts", "IP", "Node").AddRow(
			model.NewTableCell(i.Name),
			status,
			restartsCell,
			model.NewTableCell(instanceIPs(i.TcpListens)...),
			model.NewTableCell().SetLink("node", i.NodeName()).SetStatus(nodeStatus, i.NodeName()),
		)
	}

	if a.app.Id.Kind == model.ApplicationKindExternalService {
		availability.SetStatus(model.UNKNOWN, "no data")
		restarts.SetStatus(model.UNKNOWN, "no data")
	}
	chart := report.GetOrCreateChart("Instances").Stacked().AddSeries("up", up)
	if a.app.DesiredInstances != nil {
		chart.SetThreshold("desired", a.app.DesiredInstances, timeseries.Any)
		chart.Threshold.Color = "red"
		chart.Threshold.Fill = true
	}
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
