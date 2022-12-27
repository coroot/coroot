package auditor

import (
	"github.com/coroot/coroot/deployments"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/dustin/go-humanize/english"
	"k8s.io/klog"
	"math"
	"sort"
	"strconv"
	"strings"
)

const (
	significantPercentageDifference = 5
)

func (a *appAuditor) deployments() {
	if len(a.app.Deployments) == 0 {
		return
	}
	report := a.addReport(model.AuditReportDeployments)
	deploymentStatusCheck := report.CreateCheck(model.Checks.DeploymentStatus)

	availabilityObjectivePercentage := 99.0
	if configs, _ := a.w.CheckConfigs.GetAvailability(a.app.Id); len(configs) > 0 {
		availabilityObjectivePercentage = configs[0].ObjectivePercentage
	}
	latencyObjectiveBucket := 0.5
	latencyObjectivePercentage := 99.0
	if configs, _ := a.w.CheckConfigs.GetLatency(a.app.Id); len(configs) > 0 {
		latencyObjectiveBucket = configs[0].ObjectiveBucket
		latencyObjectivePercentage = configs[0].ObjectivePercentage
	}
	memoryLeakThreshold := int64(a.w.CheckConfigs.GetSimple(model.Checks.MemoryLeak.Id, a.app.Id).Threshold * 1024 * 1024)
	deploymentDurationThreshold := timeseries.Duration(a.w.CheckConfigs.GetSimple(model.Checks.DeploymentStatus.Id, a.app.Id).Threshold)

	table := report.GetOrCreateTable("Deployment", "Active", "Summary").SetSorted(true)
	now := timeseries.Now()
	nextStartedAt := now
	for i, d := range a.app.Deployments {
		startedAt := utils.FormatDuration(now.Sub(d.StartedAt), 1)

		var images []string
		if d.Details != nil {
			for _, image := range d.Details.ContainerImages {
				images = append(images, lastPart(image, "/"))
			}
		}
		name := model.NewTableCell().SetStatus(model.UNKNOWN, lastPart(d.Name, "-")).AddTag(startedAt + " ago")
		if len(images) > 0 {
			name.Value += ": " + strings.Join(images, ", ")
		}
		if d.Name == "" {
			name.SetStub("unknown")
		}
		lifetime := nextStartedAt.Sub(d.StartedAt)
		active := model.NewTableCell(utils.FormatDuration(lifetime, 1))
		nextStartedAt = d.StartedAt

		summary := model.NewTableCell()

		table.AddRow(name, active, summary)

		if d.FinishedAt.IsZero() {
			if i == 0 {
				deploymentStatusCheck.SetValue(float64(lifetime))
				if lifetime > deploymentDurationThreshold {
					name.UpdateStatus(model.CRITICAL)
					summary.AddDeploymentSummary(model.AuditReportInstances, false, d.StartedAt,
						"Rollout: the rollout has already been in progress for %s", utils.FormatDuration(lifetime, 1))
				}
			} else {
				summary.AddDeploymentSummary(model.AuditReportInstances, false, d.StartedAt, "Rollout: the rollout has been cancelled")
			}
			continue
		}

		curr := d.MetricsSnapshot

		if curr == nil {
			summary.SetStub("collecting data...")
			continue
		}

		if curr.Timestamp.IsZero() {
			summary.SetStub("not enough data due to the lifetime < %s", utils.FormatDuration(deployments.MinDeploymentLifetime, 1))
			continue
		}

		name.UpdateStatus(model.OK)

		var prev *model.MetricsSnapshot
		for j := i + 1; j < len(a.app.Deployments); j++ {
			ms := a.app.Deployments[j].MetricsSnapshot
			if ms != nil && !ms.Timestamp.IsZero() {
				prev = ms
				break
			}
		}

		// Availability
		if curr.Requests > 0 {
			vCurr := float64(curr.Requests-curr.Errors) * 100 / float64(curr.Requests)
			if vCurr < availabilityObjectivePercentage {
				name.UpdateStatus(model.CRITICAL)
				summary.AddDeploymentSummary(model.AuditReportSLO, false, d.StartedAt,
					"Availability: %s (objective: %s)",
					utils.FormatPercentage(vCurr), utils.FormatPercentage(availabilityObjectivePercentage))
			} else if prev != nil {
				if prev.Requests > 0 {
					vPrev := float64(prev.Requests-prev.Errors) * 100 / float64(prev.Requests)
					if vPrev < availabilityObjectivePercentage {
						summary.AddDeploymentSummary(model.AuditReportSLO, true, d.StartedAt,
							"Availability: %s (objective: %s)",
							utils.FormatPercentage(vCurr), utils.FormatPercentage(availabilityObjectivePercentage))
					}
				}
			}
		}

		// Latency
		if fast := getFastRequestsCount(curr.Latency, latencyObjectiveBucket); !math.IsNaN(fast) {
			vCurr := fast * 100 / float64(curr.Requests)
			if vCurr < latencyObjectivePercentage {
				name.UpdateStatus(model.CRITICAL)
				summary.AddDeploymentSummary(model.AuditReportSLO, false, d.StartedAt,
					"Latency: %s of requests faster %sms (objective: %s)",
					utils.FormatPercentage(vCurr), utils.FormatFloat(latencyObjectiveBucket*1000), utils.FormatPercentage(latencyObjectivePercentage))
			} else if prev != nil {
				if fast := getFastRequestsCount(prev.Latency, latencyObjectiveBucket); !math.IsNaN(fast) {
					vPrev := fast * 100 / float64(prev.Requests)
					if vPrev < latencyObjectivePercentage {
						summary.AddDeploymentSummary(model.AuditReportSLO, true, d.StartedAt,
							"Latency: %s of requests faster %sms (objective: %s)",
							utils.FormatPercentage(vCurr), utils.FormatFloat(latencyObjectiveBucket*1000), utils.FormatPercentage(latencyObjectivePercentage))
					}
				}
			}
		}

		// CPU
		if prev != nil && curr.Requests > 0 && prev.Requests > 0 && curr.CPUUsage > 0 && prev.CPUUsage > 0 {
			perRequestCurr := float64(curr.CPUUsage) / float64(curr.Requests)
			perRequestPrev := float64(prev.CPUUsage) / float64(prev.Requests)
			diff := (perRequestCurr - perRequestPrev) * 100 / perRequestPrev
			if math.Abs(diff) > significantPercentageDifference {
				summary.AddDeploymentSummary(model.AuditReportCPU, diff < 0, d.StartedAt, "CPU usage: %+.0f%%", diff)
			}
		}

		// Memory
		if curr.OOMKills > 0 {
			summary.AddDeploymentSummary(model.AuditReportMemory, false, d.StartedAt,
				"Memory: app containers have been restarted %s by the OOM killer", english.Plural(int(curr.OOMKills), "time", ""))
		} else {
			if curr.MemoryLeak > memoryLeakThreshold {
				value, unit := utils.FormatBytes(float64(curr.MemoryLeak))
				summary.AddDeploymentSummary(model.AuditReportMemory, false, d.StartedAt, "Memory: the memory leak detected (%s%s per hour)", value, unit)
			} else if prev != nil && prev.MemoryLeak > memoryLeakThreshold {
				summary.AddDeploymentSummary(model.AuditReportMemory, true, d.StartedAt, "Memory: looks like the memory leak has been fixed")
			}
		}

		// Restarts
		if restarts := curr.Restarts - curr.OOMKills; restarts > 0 {
			summary.AddDeploymentSummary(model.AuditReportInstances, false, d.StartedAt, "Crash: app containers have been restarted %s", english.Plural(int(restarts), "time", ""))
		}

		// Logs
		if curr.LogErrors > 0 {
			if prev == nil || prev.LogErrors == 0 {
				summary.AddDeploymentSummary(model.AuditReportLogs, false, d.StartedAt, "Logs: there are errors in the logs")
			} else if prev != nil && prev.LogErrors > 0 && curr.Requests > 0 && prev.Requests > 0 {
				perRequestCurr := float64(curr.LogErrors) / float64(curr.Requests)
				perRequestPrev := float64(prev.LogErrors) / float64(prev.Requests)
				diff := (perRequestCurr - perRequestPrev) * 100 / perRequestPrev
				if math.Abs(diff) > significantPercentageDifference {
					ok := false
					verb := "increased"
					if diff < 0 {
						ok = true
						verb = "decreased"
					}
					summary.AddDeploymentSummary(model.AuditReportLogs, ok, d.StartedAt, "Logs: the number of errors in the logs has %s %+.f%%", verb, diff)
				}
			}
		}

		if len(summary.DeploymentSummaries) == 0 {
			summary.SetStub("no notable changes")
		}
	}
}

type histogramBucket struct {
	le    float64
	count int64
}

func getFastRequestsCount(histogram map[string]int64, objectiveBucket float64) float64 {
	var buckets []histogramBucket
	for leStr, count := range histogram {
		le, err := strconv.ParseFloat(leStr, 64)
		if err != nil {
			klog.Warningln(err)
			continue
		}
		buckets = append(buckets, histogramBucket{le: le, count: count})
	}
	if len(buckets) == 0 {
		return timeseries.NaN
	}
	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].le < buckets[j].le
	})
	res := timeseries.NaN
	for _, b := range buckets {
		if b.le <= objectiveBucket {
			res = float64(b.count)
			continue
		}
		break
	}
	return res
}

func lastPart(rs string, sep string) string {
	parts := strings.Split(rs, sep)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
