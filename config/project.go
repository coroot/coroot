package config

import (
	"fmt"

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
