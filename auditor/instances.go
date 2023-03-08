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

	up := timeseries.NewAggregate(timeseries.NanSum)

	availability := report.CreateCheck(model.Checks.InstanceAvailability)
	restarts := report.CreateCheck(model.Checks.InstanceRestarts)

	availableInstances := 0
	for _, i := range a.app.Instances {
		up.Add(i.UpAndRunning())

		status := model.NewTableCell().SetStatus(model.UNKNOWN, "unknown")
		if i.Rds != nil {
			switch {
			case math.IsNaN(i.Rds.LifeSpan.Last()):
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
			case "Failed":
				msg := "failed"
				if details := podContainerIssues(i); details != "" {
					msg += fmt.Sprintf(" (%s)", details)
				}
				status.SetStatus(model.WARNING, msg)
			case "Running":
				switch {
				case !i.IsUp():
					msg := ""
					if i.Node != nil && !i.Node.IsUp() {
						msg = "down (node down)"
					} else {
						if details := podContainerIssues(i); details != "" {
							msg = fmt.Sprintf("down (%s)", details)
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
		if *status.Status == model.OK {
			availableInstances++
		}
		restartsCount := int64(0)
		for _, c := range i.Containers {
			if r := c.Restarts.Reduce(timeseries.NanSum); !math.IsNaN(r) {
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
		node := model.NewTableCell().SetStatus(nodeStatus, i.NodeName())
		node.Link = model.NewRouterLink(i.NodeName()).SetRoute("node").SetParam("name", i.NodeName())
		report.GetOrCreateTable("Instance", "Status", "Restarts", "IP", "Node").AddRow(
			model.NewTableCell(i.Name),
			status,
			restartsCell,
			model.NewTableCell(instanceIPs(i.TcpListens)...),
			node,
		)
	}
	desired := a.app.DesiredInstances.Last()
	if a.app.Id.Kind == model.ApplicationKindUnknown {
		desired = float64(len(a.app.Instances))
	}
	if desired > 0 {
		if p := float64(availableInstances) / desired * 100; p < availability.Threshold {
			if p == 0 {
				availability.SetStatus(model.WARNING, "no instances available")
			} else {
				availability.SetStatus(model.WARNING, "only %.0f%% of the desired instances are currently available", p)
			}
		}
	}

	if a.app.Id.Kind == model.ApplicationKindExternalService {
		availability.SetStatus(model.UNKNOWN, "no data")
		restarts.SetStatus(model.UNKNOWN, "no data")
	}
	chart := report.GetOrCreateChart("Instances").Stacked().AddSeries("up", up)
	if !a.app.DesiredInstances.IsEmpty() {
		chart.SetThreshold("desired", a.app.DesiredInstances)
		chart.Threshold.Color = "red"
		chart.Threshold.Fill = true
	}
}

func podContainerIssues(i *model.Instance) string {
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
		return fmt.Sprintf("%s:%s", containerStatus, strings.Join(reasons.Items(), ","))
	}
	return ""
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
