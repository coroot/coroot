package db

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

type IntegrationType string

const (
	IntegrationTypePrometheus IntegrationType = "prometheus"
	IntegrationTypeClickhouse IntegrationType = "clickhouse"
	IntegrationTypeAWS        IntegrationType = "aws"
	IntegrationTypeSlack      IntegrationType = "slack"
	IntegrationTypePagerduty  IntegrationType = "pagerduty"
	IntegrationTypeTeams      IntegrationType = "teams"
	IntegrationTypeOpsgenie   IntegrationType = "opsgenie"
	IntegrationTypeWebhook    IntegrationType = "webhook"
)

type Integrations struct {
	Clickhouse *IntegrationClickhouse `json:"clickhouse,omitempty"`

	AWS *IntegrationAWS `json:"aws"`

	NotificationIntegrations
}

type NotificationIntegrations struct {
	Readonly bool   `json:"readonly" yaml:"-"`
	BaseUrl  string `json:"base_url" yaml:"baseURL"`

	Slack     *IntegrationSlack     `json:"slack,omitempty" yaml:"slack,omitempty"`
	Teams     *IntegrationTeams     `json:"teams,omitempty" yaml:"teams,omitempty"`
	Pagerduty *IntegrationPagerduty `json:"pagerduty,omitempty" yaml:"pagerduty,omitempty"`
	Opsgenie  *IntegrationOpsgenie  `json:"opsgenie,omitempty" yaml:"opsgenie,omitempty"`
	Webhook   *IntegrationWebhook   `json:"webhook,omitempty" yaml:"webhook,omitempty"`
}

func (i *NotificationIntegrations) Validate() error {
	if i.BaseUrl == "" {
		return fmt.Errorf("base url is required")
	}
	if _, err := url.Parse(i.BaseUrl); err != nil {
		return fmt.Errorf("invalid base url")
	}
	i.BaseUrl = strings.TrimRight(i.BaseUrl, "/")

	if i.Slack != nil {
		if err := i.Slack.Validate(); err != nil {
			return fmt.Errorf("invalid slack configuration: %w", err)
		}
	}
	if i.Teams != nil {
		if err := i.Teams.Validate(); err != nil {
			return fmt.Errorf("invalid teams configuration: %w", err)
		}
	}
	if i.Pagerduty != nil {
		if err := i.Pagerduty.Validate(); err != nil {
			return fmt.Errorf("invalid pagerduty configuration: %w", err)
		}
	}
	if i.Opsgenie != nil {
		if err := i.Opsgenie.Validate(); err != nil {
			return fmt.Errorf("invalid opsgenie configuration: %w", err)
		}
	}
	if i.Webhook != nil {
		if err := i.Webhook.Validate(); err != nil {
			return fmt.Errorf("invalid webhook configuration: %w", err)
		}
	}

	return nil

}

type IntegrationInfo struct {
	Type        IntegrationType `json:"type"`
	Configured  bool            `json:"configured"`
	Incidents   bool            `json:"incidents"`
	Deployments bool            `json:"deployments"`
	Title       string          `json:"title"`
	Details     string          `json:"details"`
}

func (integrations Integrations) GetInfo() []IntegrationInfo {
	var res []IntegrationInfo

	i := IntegrationInfo{Type: IntegrationTypeSlack, Title: "Slack"}
	if cfg := integrations.Slack; cfg != nil {
		i.Configured = true
		i.Incidents = cfg.Incidents
		i.Deployments = cfg.Deployments
		i.Details = fmt.Sprintf("default channel: #%s", cfg.DefaultChannel)
	}
	res = append(res, i)

	i = IntegrationInfo{Type: IntegrationTypeTeams, Title: "MS Teams"}
	if cfg := integrations.Teams; cfg != nil {
		i.Configured = true
		i.Incidents = cfg.Incidents
		i.Deployments = cfg.Deployments
	}
	res = append(res, i)

	i = IntegrationInfo{Type: IntegrationTypePagerduty, Title: "Pagerduty"}
	if cfg := integrations.Pagerduty; cfg != nil {
		i.Configured = true
		i.Incidents = cfg.Incidents
	}
	res = append(res, i)

	i = IntegrationInfo{Type: IntegrationTypeOpsgenie, Title: "Opsgenie"}
	if cfg := integrations.Opsgenie; cfg != nil {
		i.Configured = true
		i.Incidents = cfg.Incidents
		region := "US"
		if cfg.EUInstance {
			region = "EU"
		}
		i.Details = fmt.Sprintf("region: %s", region)
	}
	res = append(res, i)

	i = IntegrationInfo{Type: IntegrationTypeWebhook, Title: "Webhook"}
	if cfg := integrations.Webhook; cfg != nil {
		i.Configured = true
		i.Incidents = cfg.Incidents
		i.Deployments = cfg.Deployments
	}
	res = append(res, i)

	return res
}

type IntegrationPrometheus struct {
	Global          bool                `json:"global"`
	Url             string              `json:"url"`
	RefreshInterval timeseries.Duration `json:"refresh_interval"`
	TlsSkipVerify   bool                `json:"tls_skip_verify"`
	BasicAuth       *utils.BasicAuth    `json:"basic_auth"`
	ExtraSelector   string              `json:"extra_selector"`
	CustomHeaders   []utils.Header      `json:"custom_headers"`
	RemoteWriteUrl  string              `json:"remote_write_url"`
	ExtraLabels     map[string]string   `json:"-"`
}

type IntegrationClickhouse struct {
	Global          bool            `json:"global"`
	Protocol        string          `json:"protocol"`
	Addr            string          `json:"addr"`
	Auth            utils.BasicAuth `json:"auth"`
	Database        string          `json:"database"`
	InitialDatabase string          `json:"-"`
	TlsEnable       bool            `json:"tls_enable"`
	TlsSkipVerify   bool            `json:"tls_skip_verify"`
}

type IntegrationSlack struct {
	Token          string `json:"token" yaml:"token"`
	DefaultChannel string `json:"default_channel" yaml:"defaultChannel"`
	Incidents      bool   `json:"incidents" yaml:"incidents"`
	Deployments    bool   `json:"deployments" yaml:"deployments"`
}

func (i *IntegrationSlack) Validate() error {
	if i.Token == "" {
		return fmt.Errorf("token is required")
	}
	if i.DefaultChannel == "" {
		return fmt.Errorf("default channel is required")
	}
	return nil
}

type IntegrationTeams struct {
	WebhookUrl  string `json:"webhook_url" yaml:"webhookURL"`
	Incidents   bool   `json:"incidents" yaml:"incidents"`
	Deployments bool   `json:"deployments" yaml:"deployments"`
}

func (i *IntegrationTeams) Validate() error {
	if i.WebhookUrl == "" {
		return fmt.Errorf("webhook url is required")
	}
	return nil
}

type IntegrationPagerduty struct {
	IntegrationKey string `json:"integration_key" yaml:"integrationKey"`
	Incidents      bool   `json:"incidents" yaml:"incidents"`
}

func (i *IntegrationPagerduty) Validate() error {
	if i.IntegrationKey == "" {
		return fmt.Errorf("integration key is required")
	}
	return nil
}

type IntegrationOpsgenie struct {
	ApiKey     string `json:"api_key" yaml:"apiKey"`
	EUInstance bool   `json:"eu_instance" yaml:"euInstance"`
	Incidents  bool   `json:"incidents" yaml:"incidents"`
}

func (i *IntegrationOpsgenie) Validate() error {
	if i.ApiKey == "" {
		return fmt.Errorf("api key is required")
	}
	return nil
}

type IntegrationWebhook struct {
	Url                string           `json:"url" yaml:"url"`
	TlsSkipVerify      bool             `json:"tls_skip_verify" yaml:"tlsSkipVerify"`
	BasicAuth          *utils.BasicAuth `json:"basic_auth" yaml:"basicAuth"`
	CustomHeaders      []utils.Header   `json:"custom_headers" yaml:"customHeaders"`
	Incidents          bool             `json:"incidents" yaml:"incidents"`
	Deployments        bool             `json:"deployments" yaml:"deployments"`
	IncidentTemplate   string           `json:"incident_template" yaml:"incidentTemplate"`
	DeploymentTemplate string           `json:"deployment_template" yaml:"deploymentTemplate"`
}

func (i *IntegrationWebhook) Validate() error {
	if i.Url == "" {
		return fmt.Errorf("url is required")
	}
	if i.Incidents && i.IncidentTemplate == "" {
		return fmt.Errorf("incident template is required")
	}
	if i.Deployments && i.DeploymentTemplate == "" {
		return fmt.Errorf("deployment template is required")
	}
	return nil
}

type IntegrationAWS struct {
	Region          string `json:"region"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`

	RDSTagFilters         map[string]string `json:"rds_tag_filters"`
	ElasticacheTagFilters map[string]string `json:"elasticache_tag_filters"`
}

func (db *DB) SaveIntegrationsBaseUrl(id ProjectId, baseUrl string) error {
	p, err := db.GetProject(id)
	if err != nil {
		return err
	}
	p.Settings.Integrations.BaseUrl = baseUrl
	return db.SaveProjectSettings(p)
}
