package model

import "github.com/coroot/coroot/timeseries"

type ApplicationDeployment struct {
	ApplicationId ApplicationId
	Name          string
	StartedAt     timeseries.Time
	FinishedAt    timeseries.Time

	Details *ApplicationDeploymentDetails

	MetricsSnapshot *MetricsSnapshot
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
