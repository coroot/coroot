package model

import (
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

type AlertingRuleId string

type AlertSourceType string

const (
	AlertSourceTypeCheck       AlertSourceType = "check"
	AlertSourceTypeLogPatterns AlertSourceType = "log_patterns"
	AlertSourceTypePromQL      AlertSourceType = "promql"
)

type LogPatternSource struct {
	Severities      []string `json:"severities" yaml:"severities"`
	MinCount        int      `json:"min_count" yaml:"min_count"`
	MaxAlertsPerApp int      `json:"max_alerts_per_app" yaml:"max_alerts_per_app"`
	EvaluateWithAI  bool     `json:"evaluate_with_ai" yaml:"evaluate_with_ai"`
}

type CheckSource struct {
	CheckId CheckId `json:"check_id" yaml:"check_id"`
}

type PromQLSource struct {
	Expression string `json:"expression" yaml:"expression"`
}

type AlertSource struct {
	Type       AlertSourceType   `json:"type" yaml:"type"`
	Check      *CheckSource      `json:"check,omitempty" yaml:"check,omitempty"`
	LogPattern *LogPatternSource `json:"log_pattern,omitempty" yaml:"log_pattern,omitempty"`
	PromQL     *PromQLSource     `json:"promql,omitempty" yaml:"promql,omitempty"`
}

type AppSelectorType string

const (
	AppSelectorTypeAll          AppSelectorType = "all"
	AppSelectorTypeCategory     AppSelectorType = "category"
	AppSelectorTypeApplications AppSelectorType = "applications"
)

type AppSelector struct {
	Type                  AppSelectorType `json:"type" yaml:"type"`
	Categories            []string        `json:"categories,omitempty" yaml:"categories,omitempty"`
	ApplicationIdPatterns []string        `json:"application_id_patterns,omitempty" yaml:"application_id_patterns,omitempty"`
}

type AlertTemplates struct {
	Summary     string `json:"summary" yaml:"summary"`
	Description string `json:"description" yaml:"description"`
}

type AlertingRule struct {
	Id                   AlertingRuleId      `json:"id"`
	ProjectId            string              `json:"project_id"`
	Name                 string              `json:"name"`
	Source               AlertSource         `json:"source"`
	Selector             AppSelector         `json:"selector"`
	Severity             Status              `json:"severity"`
	For                  timeseries.Duration `json:"for"`
	KeepFiringFor        timeseries.Duration `json:"keep_firing_for"`
	Templates            AlertTemplates      `json:"templates"`
	NotificationCategory ApplicationCategory `json:"notification_category,omitempty"`
	Enabled              bool                `json:"enabled"`
	Readonly             bool                `json:"readonly"`
	Builtin              bool                `json:"builtin"`
	CreatedAt            timeseries.Time     `json:"created_at"`
	UpdatedAt            timeseries.Time     `json:"updated_at"`
}

func (r *AlertingRule) Matches(app *Application) bool {
	switch r.Selector.Type {
	case AppSelectorTypeAll:
		return true
	case AppSelectorTypeCategory:
		for _, cat := range r.Selector.Categories {
			if string(app.Category) == cat {
				return true
			}
		}
		return false
	case AppSelectorTypeApplications:
		return utils.GlobMatch(app.Id.StringWithoutClusterId(), r.Selector.ApplicationIdPatterns...)
	}
	return false
}

func (r *AlertingRule) MatchesAlert(a *Alert) bool {
	switch r.Selector.Type {
	case AppSelectorTypeAll:
		return true
	case AppSelectorTypeCategory:
		for _, cat := range r.Selector.Categories {
			if string(a.ApplicationCategory) == cat {
				return true
			}
		}
		return false
	case AppSelectorTypeApplications:
		return utils.GlobMatch(a.ApplicationId.StringWithoutClusterId(), r.Selector.ApplicationIdPatterns...)
	}
	return false
}

func BuiltinAlertingRules() []AlertingRule {
	return []AlertingRule{
		{
			Id:   "storage-space",
			Name: "Low disk space",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.StorageSpace.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           5 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "Disk space is running low. If the volume fills up completely, the application may fail to write data, causing errors and potential data loss.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "memory-oom",
			Name: "Out of memory kills",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.MemoryOOM.Id},
			},
			Selector: AppSelector{Type: AppSelectorTypeAll},
			Severity: WARNING,
			Templates: AlertTemplates{
				Description: "Containers are being terminated by the kernel due to memory limits. This can cause request failures and temporary service unavailability.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "memory-pressure",
			Name: "Memory pressure",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.MemoryPressure.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           5 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "High memory stall time indicates the application is waiting for memory operations, leading to increased latency and degraded performance.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "cpu-limit",
			Name: "Container CPU utilization",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.CPUContainer.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           5 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "Application is being throttled due to CPU limits. This can cause increased response times and request timeouts.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "instance-availability",
			Name: "Instance availability",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.InstanceAvailability.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           2 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "Some application instances are unavailable. This reduces capacity and may affect service availability.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "instance-restarts",
			Name: "Instance restarts",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.InstanceRestarts.Id},
			},
			Selector: AppSelector{Type: AppSelectorTypeAll},
			Severity: WARNING,
			Templates: AlertTemplates{
				Description: "Application instances are restarting frequently. This may indicate crashes, health check failures, or configuration issues.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "network-connectivity",
			Name: "Network connectivity",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.NetworkConnectivity.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           2 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "Network connectivity issues detected. This may cause failed requests to upstream services.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "redis-availability",
			Name: "Redis availability",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.RedisAvailability.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           2 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "Some Redis instances are unavailable. This may cause degraded performance or failures for dependent applications.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "postgres-availability",
			Name: "Postgres availability",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.PostgresAvailability.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           2 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "Some PostgreSQL instances are unavailable. This may cause failures for dependent applications.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "postgres-replication-lag",
			Name: "Postgres replication lag",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.PostgresReplicationLag.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           2 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "PostgreSQL replica is falling behind the primary. Read queries on replicas may return stale data.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "postgres-latency",
			Name: "Postgres latency",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.PostgresLatency.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           5 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "PostgreSQL query latency is elevated. This may cause slow response times for dependent applications.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "postgres-connections",
			Name: "Postgres connections",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.PostgresConnections.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           5 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "PostgreSQL connection pool is nearing capacity. New connections may be rejected.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "redis-latency",
			Name: "Redis latency",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.RedisLatency.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           5 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "Redis latency is elevated. This may cause slow response times for dependent applications.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "mongodb-availability",
			Name: "MongoDB availability",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.MongodbAvailability.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           2 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "Some MongoDB instances are unavailable. This may cause failures for dependent applications.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "mongodb-replication-lag",
			Name: "MongoDB replication lag",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.MongodbReplicationLag.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           2 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "MongoDB replica is falling behind the primary. Read queries on secondaries may return stale data.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "mysql-availability",
			Name: "MySQL availability",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.MysqlAvailability.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           2 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "Some MySQL instances are unavailable. This may cause failures for dependent applications.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "mysql-replication-lag",
			Name: "MySQL replication lag",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.MysqlReplicationLag.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           2 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "MySQL replica is falling behind the primary. Read queries on replicas may return stale data.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "mysql-connections",
			Name: "MySQL connections",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.MysqlConnections.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           5 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "MySQL connection pool is nearing capacity. New connections may be rejected.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "memcached-availability",
			Name: "Memcached availability",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.MemcachedAvailability.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           2 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "Some Memcached instances are unavailable. This may cause degraded performance for dependent applications.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "storage-io-load",
			Name: "Storage I/O load",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.StorageIOLoad.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           5 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "Storage I/O load is high. This may cause slow read/write operations and degraded application performance.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "network-rtt",
			Name: "Network RTT (in-cluster)",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.NetworkRTT.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           5 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "Network round-trip time between services within the cluster is elevated.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "network-rtt-external",
			Name: "Network RTT (external)",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.NetworkRTTExternal.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           5 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "Network round-trip time to external services is elevated.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "network-rtt-other-clusters",
			Name: "Network RTT (cross-cluster)",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.NetworkRTTOtherClusters.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           5 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "Network round-trip time to services in other clusters is elevated.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "network-tcp-connections",
			Name: "Network TCP connections",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.NetworkTCPConnections.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           5 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "High number of TCP connection failures. This may indicate network issues or unavailable upstream services.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "jvm-availability",
			Name: "JVM availability",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.JvmAvailability.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           2 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "JVM metrics are not being collected. This may indicate the application is down or instrumentation is misconfigured.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "jvm-safepoint-time",
			Name: "JVM safepoint time",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.JvmSafepointTime.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           5 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "JVM is spending significant time at safepoints. This may cause application pauses and increased latency.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "dotnet-availability",
			Name: ".NET availability",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.DotNetAvailability.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           2 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: ".NET metrics are not being collected. This may indicate the application is down or instrumentation is misconfigured.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "dns-latency",
			Name: "DNS latency",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.DnsLatency.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           5 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "DNS resolution latency is elevated. This may cause slow connection establishment to other services.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "dns-server-errors",
			Name: "DNS server errors",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.DnsServerErrors.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           2 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "DNS server errors detected. This may cause failures when connecting to other services.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "dns-nxdomain-errors",
			Name: "DNS NXDOMAIN errors",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.DnsNxdomainErrors.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           2 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "Application is trying to resolve non-existent domain names. Check for typos in service names or missing services.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "python-gil-waiting-time",
			Name: "Python GIL waiting time",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.PythonGILWaitingTime.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           5 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "Python threads are spending significant time waiting for the GIL. This may cause increased latency and reduced throughput.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "nodejs-event-loop-blocked-time",
			Name: "Node.js event loop blocked time",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.NodejsEventLoopBlockedTime.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           5 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "The Node.js event loop is being blocked for extended periods. This may cause increased latency and unresponsive behavior.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "mysql-replication-status",
			Name: "MySQL replication status",
			Source: AlertSource{
				Type:  AlertSourceTypeCheck,
				Check: &CheckSource{CheckId: Checks.MysqlReplicationStatus.Id},
			},
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			For:           2 * timeseries.Minute,
			KeepFiringFor: 5 * timeseries.Minute,
			Templates: AlertTemplates{
				Description: "MySQL replication thread is not running. The replica may not be receiving updates from the primary.",
			},
			Enabled: true,
			Builtin: true,
		},
		{
			Id:   "new-log-patterns",
			Name: "Log errors",
			Source: AlertSource{
				Type: AlertSourceTypeLogPatterns,
				LogPattern: &LogPatternSource{
					Severities:      []string{SeverityError.String(), SeverityFatal.String()},
					MinCount:        10,
					MaxAlertsPerApp: 20,
					EvaluateWithAI:  true,
				},
			},
			KeepFiringFor: timeseries.Hour,
			Selector:      AppSelector{Type: AppSelectorTypeAll},
			Severity:      WARNING,
			Templates: AlertTemplates{
				Description: "A new log pattern has been detected. Review the log messages to determine whether this indicates an error, a misconfiguration, or expected behavior. Suppress the alert if this is not a problem.",
			},
			Enabled: true,
			Builtin: true,
		},
	}
}
