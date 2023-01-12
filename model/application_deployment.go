package model

import (
	"fmt"
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
	ApplicationDeploymentMetricsSnapshotShift  = 10 * timeseries.Minute
	ApplicationDeploymentMetricsSnapshotWindow = 20 * timeseries.Minute
	ApplicationDeploymentMinLifetime           = ApplicationDeploymentMetricsSnapshotShift + ApplicationDeploymentMetricsSnapshotWindow
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
			images = append(images, utils.LastPart(i, "/"))
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

	Restarts    int64   `json:"restarts"`
	CPUUsage    float32 `json:"cpu_usage"`
	MemoryLeak  int64   `json:"memory_leak"`
	OOMKills    int64   `json:"oom_kills"`
	LogErrors   int64   `json:"log_errors"`
	LogWarnings int64   `json:"log_warnings"`
}

type ApplicationDeploymentNotifications struct {
	State ApplicationDeploymentState `json:"state"`
	Slack struct {
		Channel  string `json:"channel,omitempty"`
		ThreadTs string `json:"thread_ts,omitempty"`
	} `json:"slack"`
}

type ApplicationDeploymentSummary struct {
	Report  AuditReportName `json:"report"`
	Ok      bool            `json:"ok"`
	Message string          `json:"message"`
	Time    timeseries.Time `json:"time"`
}

type ApplicationDeploymentStatus struct {
	Status     Status
	State      ApplicationDeploymentState
	Message    string
	Lifetime   timeseries.Duration
	Summary    []ApplicationDeploymentSummary
	Deployment *ApplicationDeployment
}

func CalcApplicationDeploymentStatuses(app *Application, checkConfigs CheckConfigs, now timeseries.Time) []ApplicationDeploymentStatus {
	durationThreshold := timeseries.Duration(checkConfigs.GetSimple(Checks.DeploymentStatus.Id, app.Id).Threshold)
	res := make([]ApplicationDeploymentStatus, 0, len(app.Deployments))
	for i, d := range app.Deployments {
		last := i == len(app.Deployments)-1
		s := ApplicationDeploymentStatus{Deployment: d}
		if last {
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
			s.Summary, s.Status = CalcApplicationDeploymentSummary(d.ApplicationId, checkConfigs, d.StartedAt, d.MetricsSnapshot, prev)
		case !d.FinishedAt.IsZero():
			s.Status = OK
			s.State = ApplicationDeploymentStateDeployed
			s.Message = "The service has been successfully deployed"
		case !last:
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

func CalcApplicationDeploymentSummary(appId ApplicationId, checkConfigs CheckConfigs, t timeseries.Time, curr, prev *MetricsSnapshot) ([]ApplicationDeploymentSummary, Status) {
	availabilityObjectivePercentage := 99.0
	if configs, _ := checkConfigs.GetAvailability(appId); len(configs) > 0 {
		availabilityObjectivePercentage = configs[0].ObjectivePercentage
	}
	latencyObjectiveBucket := 0.5
	latencyObjectivePercentage := 99.0
	if configs, _ := checkConfigs.GetLatency(appId); len(configs) > 0 {
		latencyObjectiveBucket = configs[0].ObjectiveBucket
		latencyObjectivePercentage = configs[0].ObjectivePercentage
	}
	memoryLeakThreshold := int64(checkConfigs.GetSimple(Checks.MemoryLeak.Id, appId).Threshold * 1024 * 1024)
	significantPercentageDifference := 5.0

	status := OK
	var res []ApplicationDeploymentSummary
	add := func(r AuditReportName, ok bool, format string, a ...any) {
		res = append(res, ApplicationDeploymentSummary{Report: r, Ok: ok, Message: fmt.Sprintf(format, a...)})

	}

	// Availability
	if curr.Requests > 0 {
		vCurr := float64(curr.Requests-curr.Errors) * 100 / float64(curr.Requests)
		v := utils.FormatPercentage(vCurr)
		o := utils.FormatPercentage(availabilityObjectivePercentage)
		if vCurr < availabilityObjectivePercentage {
			status = CRITICAL
			add(AuditReportSLO, false, "Availability: %s (objective: %s)", v, o)
		} else if prev != nil {
			if prev.Requests > 0 {
				vPrev := float64(prev.Requests-prev.Errors) * 100 / float64(prev.Requests)
				if vPrev < availabilityObjectivePercentage {
					add(AuditReportSLO, true, "Availability: %s (objective: %s)", v, o)
				}
			}
		}
	}

	// Latency
	if fast := getFastRequestsCount(curr.Latency, latencyObjectiveBucket); !math.IsNaN(fast) {
		vCurr := fast * 100 / float64(curr.Requests)
		v := utils.FormatPercentage(vCurr)
		b := utils.FormatFloat(latencyObjectiveBucket * 1000)
		o := utils.FormatPercentage(latencyObjectivePercentage)
		if vCurr < latencyObjectivePercentage {
			status = CRITICAL
			add(AuditReportSLO, false, "Latency: %s of requests faster %sms (objective: %s)", v, b, o)
		} else if prev != nil {
			if fast := getFastRequestsCount(prev.Latency, latencyObjectiveBucket); !math.IsNaN(fast) {
				vPrev := fast * 100 / float64(prev.Requests)
				if vPrev < latencyObjectivePercentage {
					add(AuditReportSLO, true, "Latency: %s of requests faster %sms (objective: %s)", v, b, o)
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
			add(AuditReportCPU, diff < 0, "CPU usage: %+.0f%% (compared to the previous deployment)", diff)
		}
	}

	// Memory
	if curr.OOMKills > 0 {
		v := english.Plural(int(curr.OOMKills), "time", "")
		add(AuditReportMemory, false, "Memory: app containers have been restarted %s by the OOM killer", v)
	} else {
		if curr.MemoryLeak > memoryLeakThreshold {
			value, unit := utils.FormatBytes(float64(curr.MemoryLeak))
			add(AuditReportMemory, false, "Memory: the memory leak detected (%s%s per hour)", value, unit)
		} else if prev != nil && prev.MemoryLeak > memoryLeakThreshold {
			add(AuditReportMemory, true, "Memory: looks like the memory leak has been fixed")
		}
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
				add(AuditReportLogs, ok, "Logs: the number of errors in the logs has %s %+.f%%", verb, diff)
			}
		}
	}

	for i := range res {
		res[i].Time = t
	}
	return res, status
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
