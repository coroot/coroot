package config

import (
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

type RemoteCoroot struct {
	Url              string        `yaml:"url"`
	TlsSkipVerify    bool          `yaml:"tlsSkipVerify"`
	ApiKey           string        `yaml:"apiKey"`
	MetricResolution time.Duration `yaml:"metricResolution"`
}

func (rc *RemoteCoroot) Validate() error {
	if _, err := url.Parse(rc.Url); err != nil {
		return fmt.Errorf("invalid url %s: %w", rc.Url, err)
	}
	if len(rc.ApiKey) == 0 {
		return fmt.Errorf("missing api key")
	}
	if rc.MetricResolution < time.Second {
		return fmt.Errorf("metric_resolution too short")
	}
	return nil
}

func (rc *RemoteCoroot) ClickHouseConfig() *db.IntegrationClickhouse {
	u, _ := url.Parse(rc.Url)
	host, port, _ := net.SplitHostPort(u.Host)
	if host == "" {
		host = u.Host
	}
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return &db.IntegrationClickhouse{
		Global:        true,
		Protocol:      "coroot",
		Addr:          net.JoinHostPort(host, port),
		Auth:          utils.BasicAuth{User: "default", Password: rc.ApiKey},
		Database:      "default",
		TlsEnable:     u.Scheme == "https",
		TlsSkipVerify: rc.TlsSkipVerify,
	}
}

func (rc *RemoteCoroot) PrometheusConfig() *db.IntegrationPrometheus {
	return &db.IntegrationPrometheus{
		Global:          true,
		Url:             rc.Url,
		RefreshInterval: timeseries.DurationFromStandard(rc.MetricResolution),
		TlsSkipVerify:   rc.TlsSkipVerify,
		CustomHeaders:   []utils.Header{{Key: "X-API-Key", Value: rc.ApiKey}},
	}
}

type AlertingRule struct {
	Id                   string                     `yaml:"id"`
	Name                 *string                    `yaml:"name,omitempty"`
	Source               *model.AlertSource         `yaml:"source,omitempty"`
	Selector             *model.AppSelector         `yaml:"selector,omitempty"`
	Severity             *string                    `yaml:"severity,omitempty"`
	For                  *timeseries.Duration       `yaml:"for,omitempty"`
	KeepFiringFor        *timeseries.Duration       `yaml:"keepFiringFor,omitempty"`
	Templates            *model.AlertTemplates      `yaml:"templates,omitempty"`
	NotificationCategory *model.ApplicationCategory `yaml:"notificationCategory,omitempty"`
	Enabled              *bool                      `yaml:"enabled,omitempty"`
}

func (ar *AlertingRule) Validate() error {
	if ar.Id == "" {
		return fmt.Errorf("id is required")
	}
	if ar.Source != nil {
		switch ar.Source.Type {
		case model.AlertSourceTypeCheck:
			if ar.Source.Check == nil || ar.Source.Check.CheckId == "" {
				return fmt.Errorf("source.check.checkId is required for check source")
			}
			configs := model.GetCheckConfigs()
			if _, ok := configs[ar.Source.Check.CheckId]; !ok {
				return fmt.Errorf("unknown checkId: %s", ar.Source.Check.CheckId)
			}
		case model.AlertSourceTypePromQL:
			if ar.Source.PromQL == nil || ar.Source.PromQL.Expression == "" {
				return fmt.Errorf("source.promql.expression is required for promql source")
			}
		case model.AlertSourceTypeLogPatterns:
			if ar.Source.LogPattern == nil || len(ar.Source.LogPattern.Severities) == 0 {
				return fmt.Errorf("source.logPattern.severities is required for log_patterns source")
			}
		case model.AlertSourceTypeKubernetesEvents:
			if ar.Source.KubernetesEvents == nil {
				return fmt.Errorf("source.kubernetesEvents is required for kubernetes_events source")
			}
		default:
			return fmt.Errorf("invalid source type: %s", ar.Source.Type)
		}
	}
	if ar.Severity != nil {
		switch *ar.Severity {
		case "warning", "critical":
		default:
			return fmt.Errorf("invalid severity: %s (must be 'warning' or 'critical')", *ar.Severity)
		}
	}
	return nil
}

func applyConfigOverrides(base *model.AlertingRule, override AlertingRule) *model.AlertingRule {
	result := *base
	if override.Name != nil {
		result.Name = *override.Name
	}
	if override.Source != nil {
		result.Source = *override.Source
	}
	if override.Selector != nil {
		result.Selector = *override.Selector
	}
	if override.Severity != nil {
		switch *override.Severity {
		case "warning":
			result.Severity = model.WARNING
		case "critical":
			result.Severity = model.CRITICAL
		}
	}
	if override.For != nil {
		result.For = *override.For
	}
	if override.KeepFiringFor != nil {
		result.KeepFiringFor = *override.KeepFiringFor
	}
	if override.Templates != nil {
		if override.Templates.Summary != "" {
			result.Templates.Summary = override.Templates.Summary
		}
		if override.Templates.Description != "" {
			result.Templates.Description = override.Templates.Description
		}
	}
	if override.NotificationCategory != nil {
		result.NotificationCategory = *override.NotificationCategory
	}
	if override.Enabled != nil {
		result.Enabled = *override.Enabled
	}
	return &result
}

type Project struct {
	Name string `yaml:"name"`

	MemberProjects []string `yaml:"memberProjects"`

	RemoteCoroot *RemoteCoroot `yaml:"remoteCoroot"`

	ApiKeys      []db.ApiKey `yaml:"apiKeys"`
	ApiKeysSnake []db.ApiKey `yaml:"api_keys"` // TODO: remove

	NotificationIntegrations *db.NotificationIntegrations `yaml:"notificationIntegrations"`
	ApplicationCategories    []ApplicationCategory        `yaml:"applicationCategories"`
	CustomApplications       []CustomApplication          `yaml:"customApplications"`

	AlertingRules []AlertingRule `yaml:"alertingRules"`

	InspectionOverrides *InspectionOverrides `yaml:"inspectionOverrides"`
}

func (p *Project) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("name is required")
	}

	if len(p.ApiKeys) == 0 {
		p.ApiKeys = p.ApiKeysSnake
	}
	if len(p.ApiKeys) == 0 && len(p.MemberProjects) == 0 && p.RemoteCoroot == nil {
		return fmt.Errorf("no api keys defined")
	}
	for i, k := range p.ApiKeys {
		if err := k.Validate(); err != nil {
			return fmt.Errorf("invalid api key #%d: %w", i, err)
		}
	}

	if p.NotificationIntegrations != nil {
		if err := p.NotificationIntegrations.Validate(); err != nil {
			return fmt.Errorf("invalid notification integrations: %w", err)
		}
	}

	for i, c := range p.ApplicationCategories {
		if err := c.Validate(); err != nil {
			return fmt.Errorf("invalid application category #%d: %w", i, err)
		}
	}

	for i, c := range p.CustomApplications {
		if err := c.Validate(); err != nil {
			return fmt.Errorf("invalid custom application #%d: %w", i, err)
		}
	}
	for i, ar := range p.AlertingRules {
		if err := ar.Validate(); err != nil {
			return fmt.Errorf("invalid alerting rule #%d: %w", i, err)
		}
	}
	if p.InspectionOverrides != nil {
		if err := p.InspectionOverrides.Validate(); err != nil {
			return err
		}
	}
	if p.RemoteCoroot != nil {
		if err := p.RemoteCoroot.Validate(); err != nil {
			return fmt.Errorf("invalie remoteCoroot config: %w", err)
		}
	}
	return nil
}

type ApplicationCategory struct {
	Name                           model.ApplicationCategory `yaml:"name"`
	db.ApplicationCategorySettings `yaml:",inline"`
}

func (c *ApplicationCategory) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

type CustomApplication struct {
	Name                    string `yaml:"name"`
	model.CustomApplication `yaml:",inline"`
}

func (c *CustomApplication) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

type SLOAvailabilityOverride struct {
	ApplicationId    model.ApplicationId `yaml:"applicationId"`
	ObjectivePercent float32             `yaml:"objectivePercent"`
}

func (o *SLOAvailabilityOverride) Validate() error {
	if o.ObjectivePercent < 0 || o.ObjectivePercent > 100 {
		return fmt.Errorf("invalid objective_percent")
	}
	return nil
}

type SLOLatencyOverride struct {
	ApplicationId      model.ApplicationId `yaml:"applicationId"`
	ObjectivePercent   float32             `yaml:"objectivePercent"`
	ObjectiveThreshold time.Duration       `yaml:"objectiveThreshold"`
}

func (o *SLOLatencyOverride) Validate() error {
	if o.ObjectivePercent < 0 || o.ObjectivePercent > 100 {
		return fmt.Errorf("invalid objective_percent")
	}
	bucket := model.RoundUpToDefaultBucket(float32(o.ObjectiveThreshold.Seconds()))
	if bucket > model.DefaultHistogramBuckets[len(model.DefaultHistogramBuckets)-1] {
		return fmt.Errorf("invalid objective_threshold: must match one of the standard buckets [5ms, 10ms, 25ms, 50ms, 100ms, 250ms, 500ms, 1s, 2.5s, 5s, 10s]")
	}
	return nil
}

type InspectionOverrides struct {
	SLOAvailability []SLOAvailabilityOverride `yaml:"sloAvailability"`
	SLOLatency      []SLOLatencyOverride      `yaml:"sloLatency"`
}

func (io *InspectionOverrides) Validate() error {
	for _, o := range io.SLOAvailability {
		if err := o.Validate(); err != nil {
			return fmt.Errorf("invalid application availability SLO override for app %s: %w", o.ApplicationId.String(), err)
		}
	}
	for _, o := range io.SLOLatency {
		if err := o.Validate(); err != nil {
			return fmt.Errorf("invalid application latency SLO override for app %s: %w", o.ApplicationId.String(), err)
		}
	}
	return nil
}
