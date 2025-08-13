package config

import (
	"fmt"
	"time"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
)

type Project struct {
	Name string `yaml:"name"`

	ApiKeys      []db.ApiKey `yaml:"apiKeys"`
	ApiKeysSnake []db.ApiKey `yaml:"api_keys"` // TODO: remove

	NotificationIntegrations *db.NotificationIntegrations `yaml:"notificationIntegrations"`
	ApplicationCategories    []ApplicationCategory        `yaml:"applicationCategories"`
	CustomApplications       []CustomApplication          `yaml:"customApplications"`

	InspectionOverrides *InspectionOverrides `yaml:"inspectionOverrides"`
}

func (p *Project) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("name is required")
	}

	if len(p.ApiKeys) == 0 {
		p.ApiKeys = p.ApiKeysSnake
	}
	if len(p.ApiKeys) == 0 {
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
	if p.InspectionOverrides != nil {
		if err := p.InspectionOverrides.Validate(); err != nil {
			return err
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
