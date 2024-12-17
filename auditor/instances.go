package auditor

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

func (a *appAuditor) instances() {
	report := a.addReport(model.AuditReportInstances)

	availabilityCheck := report.CreateCheck(model.Checks.InstanceAvailability)
	restartsCheck := report.CreateCheck(model.Checks.InstanceRestarts)

	instancesChart := report.GetOrCreateChart("Instances", nil).Stacked()
	restartsChart := report.GetOrCreateChart("Restarts", nil).Column()

	up := timeseries.NewAggregate(timeseries.NanSum)
	for _, i := range a.app.Instances {
		if instancesChart != nil {
			up.Add(i.UpAndRunning())
		}

		status := model.NewTableCell().SetStatus(model.UNKNOWN, "unknown")
		if i.Rds != nil {
			switch {
			case timeseries.IsNaN(i.Rds.LifeSpan.Last()):
				status.SetStatus(model.WARNING, "down (no metrics)")
			case i.Rds.Status.Value() != "available":
				status.SetStatus(model.WARNING, i.Rds.Status.Value())
			default:
				status.SetStatus(model.OK, i.Rds.Status.Value())
			}
		} else if i.Elasticache != nil {
			switch {
			case timeseries.IsNaN(i.Elasticache.LifeSpan.Last()):
				status.SetStatus(model.WARNING, "down (no metrics)")
			case i.Elasticache.Status.Value() != "available":
				status.SetStatus(model.WARNING, i.Elasticache.Status.Value())
			default:
				status.SetStatus(model.OK, i.Elasticache.Status.Value())
			}
		} else if i.Pod == nil {
			if i.IsUp() {
				status.SetStatus(model.OK, "ok")
			} else {
				if a.app.Id.Kind != model.ApplicationKindExternalService {
					status.SetStatus(model.WARNING, "down (no metrics)")
					if i.Node.IsDown() {
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
					switch {
					case i.Node.Status() == model.UNKNOWN:
						status.SetStatus(model.UNKNOWN, "unknown")
					case i.Node.IsDown():
						status.SetStatus(model.WARNING, "down (node down)")
					default:
						if details := podContainerIssues(i); details != "" {
							status.SetStatus(model.WARNING, fmt.Sprintf("down (%s)", details))
						} else {
							status.SetStatus(model.WARNING, "down (no metrics)")
						}
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
			availabilityCheck.Inc(1)
		}
		instanceRestarts := timeseries.NewAggregate(timeseries.NanSum)
		var instanceRestartsTotal int64
		for _, c := range i.Containers {
			instanceRestarts.Add(c.Restarts)
			if r := c.Restarts.Reduce(timeseries.NanSum); !timeseries.IsNaN(r) {
				restartsCheck.Inc(int64(r))
				instanceRestartsTotal += int64(r)
			}
		}
		if restartsChart != nil {
			restartsChart.AddSeries(i.Name, instanceRestarts)
		}
		restartsCell := model.NewTableCell()
		if instanceRestartsTotal > 0 {
			restartsCell.SetValue(strconv.FormatInt(instanceRestartsTotal, 10))
		}

		node := model.NewTableCell().SetStatus(i.Node.Status(), i.NodeName())
		node.Link = model.NewRouterLink(i.NodeName(), "overview").
			SetParam("view", "nodes").
			SetParam("id", i.NodeName())

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
		desired = float32(len(a.app.Instances))
	}
	if a.app.PeriodicJob() {
		availabilityCheck.SetStatus(model.OK, "not checked for periodic jobs")
		restartsCheck.SetStatus(model.OK, "not checked for periodic jobs")
		restartsCheck.ResetCounter()
	} else if desired > 0 {
		availabilityCheck.SetDesired(int64(desired))
		available := float32(availabilityCheck.Count())
		percentage := available / desired * 100
		switch {
		case available == 0:
			availabilityCheck.SetStatus(model.WARNING, "no instances available")
		case a.app.Id.Kind == model.ApplicationKindDaemonSet:
			if available < desired {
				availabilityCheck.SetStatus(model.WARNING, "some instances of the DaemonSet are not available")
			}
		case percentage < availabilityCheck.Threshold:
			availabilityCheck.SetStatus(model.WARNING, "%.0f/%.0f instances are currently available", available, desired)
		}
	}

	if a.app.Id.Kind == model.ApplicationKindExternalService {
		availabilityCheck.SetStatus(model.UNKNOWN, "no data")
		restartsCheck.SetStatus(model.UNKNOWN, "no data")
	}

	if instancesChart != nil {
		instancesChart.AddSeries("up", up)
		if !a.app.DesiredInstances.IsEmpty() {
			instancesChart.SetThreshold("desired", a.app.DesiredInstances)
			instancesChart.Threshold.Color = "red"
			instancesChart.Threshold.Fill = true
		}
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
