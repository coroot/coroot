package model

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/dustin/go-humanize/english"
	"k8s.io/klog"
)

const (
	ApplicationDeploymentMetricsSnapshotShift  = 10 * timeseries.Minute
	ApplicationDeploymentMetricsSnapshotWindow = 20 * timeseries.Minute
	ApplicationDeploymentMinLifetime           = ApplicationDeploymentMetricsSnapshotShift + ApplicationDeploymentMetricsSnapshotWindow

	significantPercentageDifference float32 = 5
)

type ApplicationDeploymentState int

const (
	ApplicationDeploymentStateStarted ApplicationDeploymentState = iota
	ApplicationDeploymentStateInProgress
	ApplicationDeploymentStateStuck
	ApplicationDeploymentStateCancelled
	ApplicationDeploymentStateDeployed
	ApplicationDeploymentStateSummary
)

type ApplicationDeployment struct {
	ApplicationId ApplicationId
	Name          string
	StartedAt     timeseries.Time
	FinishedAt    timeseries.Time

	Details *ApplicationDeploymentDetails

	MetricsSnapshot *MetricsSnapshot

	Notifications *ApplicationDeploymentNotifications
}

func (d *ApplicationDeployment) Hash() string {
	return utils.LastPart(d.Name, "-")
}

func (d *ApplicationDeployment) Id() string {
	return d.Hash() + ":" + strconv.FormatInt(int64(d.StartedAt), 10)
}

func (d *ApplicationDeployment) Version() string {
	res := d.Hash()
	if d.Details != nil && len(d.Details.ContainerImages) > 0 {
		var images []string
		for _, i := range d.Details.ContainerImages {
			images = append(images, utils.FormatImage(i))
		}
		res += ": " + strings.Join(images, ", ")
	}
	return res
}

type ApplicationDeploymentDetails struct {
	ContainerImages []string `json:"container_images"`
}

type MetricsSnapshot struct {
	Timestamp timeseries.Time     `json:"timestamp"`
	Duration  timeseries.Duration `json:"duration"`

	Requests int64            `json:"requests"`
	Errors   int64            `json:"errors"`
	Latency  map[string]int64 `json:"latency"`

	Restarts          int64   `json:"restarts"`
	CPUUsage          float32 `json:"cpu_usage"`
	MemoryLeakPercent float32 `json:"memory_leak_percent"`
	MemoryUsage       int64   `json:"memory_usage"`
	OOMKills          int64   `json:"oom_kills"`
	LogErrors         int64   `json:"log_errors"`
	LogWarnings       int64   `json:"log_warnings"`
}

type ApplicationDeploymentNotifications struct {
	State ApplicationDeploymentState `json:"state"`
	Slack struct {
		State    ApplicationDeploymentState `json:"state"`
		Channel  string                     `json:"channel,omitempty"`
		ThreadTs string                     `json:"thread_ts,omitempty"`
	} `json:"slack"`
	Teams struct {
		State ApplicationDeploymentState `json:"state"`
	} `json:"teams"`
	Webhook struct {
		State ApplicationDeploymentState `json:"state"`
	} `json:"webhook"`
}

type ApplicationDeploymentSummary struct {
	Report  AuditReportName `json:"report"`
	Ok      bool            `json:"ok"`
	Message string          `json:"message"`
	Time    timeseries.Time `json:"time"`
}

func (s ApplicationDeploymentSummary) Emoji() string {
	if s.Ok {
		return "🎉"
	}
	return "💔"
}

type ApplicationDeploymentStatus struct {
	Status     Status
	State      ApplicationDeploymentState
	Message    string
	Lifetime   timeseries.Duration
	Summary    []ApplicationDeploymentSummary
	Deployment *ApplicationDeployment
	Last       bool
}

func CalcApplicationDeploymentStatuses(app *Application, checkConfigs CheckConfigs, now timeseries.Time) []ApplicationDeploymentStatus {
	durationThreshold := timeseries.Duration(checkConfigs.GetSimple(Checks.DeploymentStatus.Id, app.Id).Threshold)
	res := make([]ApplicationDeploymentStatus, 0, len(app.Deployments))
	for i, d := range app.Deployments {
		s := ApplicationDeploymentStatus{Deployment: d, Last: i == len(app.Deployments)-1}
		if s.Last {
			s.Lifetime = now.Sub(d.StartedAt)
		} else {
			s.Lifetime = app.Deployments[i+1].StartedAt.Sub(d.StartedAt)
		}

		switch {
		case d.MetricsSnapshot != nil:
			s.State = ApplicationDeploymentStateSummary
			var prev *MetricsSnapshot
			for j := i - 1; j >= 0; j-- {
				if ms := app.Deployments[j].MetricsSnapshot; ms != nil {
					prev = ms
					break
				}
			}
			s.Summary, s.Status = CalcApplicationDeploymentSummary(app, checkConfigs, d.StartedAt, d.MetricsSnapshot, prev)
		case !d.FinishedAt.IsZero():
			s.Status = OK
			s.State = ApplicationDeploymentStateDeployed
			s.Message = "The service has been successfully deployed"
		case !s.Last:
			s.Status = WARNING
			s.State = ApplicationDeploymentStateCancelled
			s.Message = "The deployment has been cancelled"
		case now.Sub(d.StartedAt) > durationThreshold:
			s.Status = CRITICAL
			s.State = ApplicationDeploymentStateStuck
			s.Message = fmt.Sprintf("The rollout has been in progress for over %s", utils.FormatDuration(durationThreshold, 1))
		default:
			s.Status = WARNING
			s.State = ApplicationDeploymentStateInProgress
			s.Message = "The rollout is in progress"
		}
		res = append(res, s)
	}
	return res
}

func CalcApplicationDeploymentSummary(app *Application, checkConfigs CheckConfigs, t timeseries.Time, curr, prev *MetricsSnapshot) ([]ApplicationDeploymentSummary, Status) {
	availabilityCfg, _ := checkConfigs.GetAvailability(app.Id)
	latencyCfg, _ := checkConfigs.GetLatency(app.Id, app.Category)

	status := OK
	var res []ApplicationDeploymentSummary
	add := func(r AuditReportName, ok bool, format string, a ...any) {
		res = append(res, ApplicationDeploymentSummary{Report: r, Ok: ok, Message: fmt.Sprintf(format, a...)})
	}

	// Availability
	if curr.Requests > 0 {
		vCurr := float32(curr.Requests-curr.Errors) * 100 / float32(curr.Requests)
		v := utils.FormatPercentage(vCurr)
		o := utils.FormatPercentage(float32(availabilityCfg.ObjectivePercentage))
		if vCurr < availabilityCfg.ObjectivePercentage {
			status = CRITICAL
			add(AuditReportSLO, false, "Availability: %s (objective: %s)", v, o)
		} else if prev != nil {
			if prev.Requests > 0 {
				vPrev := float32(prev.Requests-prev.Errors) * 100 / float32(prev.Requests)
				if vPrev < availabilityCfg.ObjectivePercentage {
					add(AuditReportSLO, true, "Availability: %s (objective: %s)", v, o)
				}
			}
		}
	}

	// Latency
	if fast := getFastRequestsCount(curr.Latency, latencyCfg.ObjectiveBucket); !timeseries.IsNaN(fast) {
		vCurr := fast * 100 / float32(curr.Requests)
		v := utils.FormatPercentage(vCurr)
		b := utils.FormatFloat(latencyCfg.ObjectiveBucket * 1000)
		o := utils.FormatPercentage(latencyCfg.ObjectivePercentage)
		if vCurr < latencyCfg.ObjectivePercentage {
			status = CRITICAL
			add(AuditReportSLO, false, "Latency: %s of requests faster %sms (objective: %s)", v, b, o)
		} else if prev != nil {
			if fast := getFastRequestsCount(prev.Latency, latencyCfg.ObjectiveBucket); !timeseries.IsNaN(fast) {
				vPrev := fast * 100 / float32(prev.Requests)
				if vPrev < latencyCfg.ObjectivePercentage {
					add(AuditReportSLO, true, "Latency: %s of requests faster %sms (objective: %s)", v, b, o)
				}
			}
		}
	}

	// CPU
	if prev != nil && curr.Requests > 0 && prev.Requests > 0 && curr.CPUUsage > 0 && prev.CPUUsage > 0 {
		perRequestCurr := curr.CPUUsage / float32(curr.Requests)
		perRequestPrev := prev.CPUUsage / float32(prev.Requests)
		diffPercent := (perRequestCurr - perRequestPrev) * 100 / perRequestPrev
		if float32(math.Abs(float64(diffPercent))) > significantPercentageDifference {
			var totalPrice, count float32
			for _, i := range app.Instances {
				if i.Node == nil || i.Node.Price == nil {
					continue
				}
				totalPrice += i.Node.Price.PerCPUCore
				count++
			}
			avgPricePerCpu := totalPrice / count
			var costs string
			if totalPrice > 0 {
				prevAvgCpuUsage := prev.CPUUsage / float32(ApplicationDeploymentMetricsSnapshotWindow)
				diffCosts := prevAvgCpuUsage * avgPricePerCpu * diffPercent / 100
				costs = fmt.Sprintf(" (%s/mo)", utils.FormatMoney(diffCosts*float32(timeseries.Month)))
			}
			add(AuditReportCPU, diffPercent < 0, "CPU usage: %+.f%%%s compared to the previous deployment", diffPercent, costs)
		}
	}

	// Memory
	if curr.OOMKills > 0 {
		v := english.Plural(int(curr.OOMKills), "time", "")
		add(AuditReportMemory, false, "Memory: app containers have been restarted %s by the OOM killer", v)
	}
	if prev != nil && curr.MemoryUsage > 0 && prev.MemoryUsage > 0 {
		diffPercent := float32(curr.MemoryUsage-prev.MemoryUsage) * 100 / float32(prev.MemoryUsage)
		if float32(math.Abs(float64(diffPercent))) > significantPercentageDifference {
			var totalPrice, count float32
			for _, i := range app.Instances {
				if i.Node == nil || i.Node.Price == nil {
					continue
				}
				totalPrice += i.Node.Price.PerMemoryByte
				count++
			}
			avgPricePerByte := totalPrice / count
			var costs string
			if totalPrice > 0 {
				diffCosts := float32(prev.MemoryUsage) * avgPricePerByte * diffPercent / 100
				costs = fmt.Sprintf(" (%s/mo)", utils.FormatMoney(diffCosts*float32(timeseries.Month)))
			}
			add(AuditReportMemory, diffPercent < 0, "Memory usage: %+.f%%%s compared to the previous deployment", diffPercent, costs)
		}
	}
	if curr.MemoryLeakPercent > significantPercentageDifference {
		add(AuditReportMemory, false, "Memory: a memory leak detected (%+.f%% per hour)", curr.MemoryLeakPercent)
	} else if prev != nil && prev.MemoryLeakPercent > significantPercentageDifference {
		add(AuditReportMemory, true, "Memory: looks like the memory leak has been fixed")
	}

	// Restarts
	if restarts := curr.Restarts - curr.OOMKills; restarts > 0 {
		add(AuditReportInstances, false, "Crash: app containers have been restarted %s", english.Plural(int(restarts), "time", ""))
	}

	// Logs
	if curr.LogErrors > 0 {
		if prev == nil || prev.LogErrors == 0 {
			add(AuditReportLogs, false, "Logs: there are errors in the logs")
		} else if prev != nil && prev.LogErrors > 0 && curr.Requests > 0 && prev.Requests > 0 {
			if curr.LogErrors == 0 {
				add(AuditReportLogs, true, "Logs: there are no more errors in the logs")
			} else {
				perRequestCurr := float32(curr.LogErrors) / float32(curr.Requests)
				perRequestPrev := float32(prev.LogErrors) / float32(prev.Requests)
				diff := (perRequestCurr - perRequestPrev) * 100 / perRequestPrev
				if float32(math.Abs(float64(diff))) > significantPercentageDifference {
					ok := false
					verb := "increased"
					if diff < 0 {
						ok = true
						verb = "decreased"
					}
					add(AuditReportLogs, ok, "Logs: the number of errors in the logs has %s by %d%%", verb, int(math.Abs(float64(diff))))
				}
			}
		}
	}

	for i := range res {
		res[i].Time = t
	}
	return res, status
}

type histogramBucket struct {
	le    float32
	count int64
}

func getFastRequestsCount(histogram map[string]int64, objectiveBucket float32) float32 {
	var buckets []histogramBucket
	for leStr, count := range histogram {
		le, err := strconv.ParseFloat(leStr, 32)
		if err != nil {
			klog.Warningln(err)
			continue
		}
		buckets = append(buckets, histogramBucket{le: float32(le), count: count})
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
			res = float32(b.count)
			continue
		}
		break
	}
	return res
}
