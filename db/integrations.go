package db

import (
	"fmt"

	"github.com/coroot/coroot/model"

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
	BaseUrl string `json:"base_url"`

	Slack     *IntegrationSlack     `json:"slack,omitempty"`
	Pagerduty *IntegrationPagerduty `json:"pagerduty,omitempty"`
	Teams     *IntegrationTeams     `json:"teams,omitempty"`
	Opsgenie  *IntegrationOpsgenie  `json:"opsgenie,omitempty"`
	Webhook   *IntegrationWebhook   `json:"webhook,omitempty"`

	Clickhouse *IntegrationClickhouse `json:"clickhouse,omitempty"`

	AWS *model.AWSConfig `json:"aws"`
}

type IntegrationInfo struct {
	Type        IntegrationType
	Configured  bool
	Incidents   bool
	Deployments bool
	Title       string
	Details     string
}

func (integrations Integrations) GetInfo() []IntegrationInfo {
	var res []IntegrationInfo

	i := IntegrationInfo{Type: IntegrationTypeSlack, Title: "Slack"}
	if cfg := integrations.Slack; cfg != nil {
		i.Configured = true
		i.Incidents = cfg.Incidents
		i.Deployments = cfg.Deployments
		i.Details = fmt.Sprintf("channel: #%s", cfg.DefaultChannel)
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
	Token          string `json:"token"`
	DefaultChannel string `json:"default_channel"`
	Enabled        bool   `json:"enabled"` // deprecated: use Incidents and Deployments
	Incidents      bool   `json:"incidents"`
	Deployments    bool   `json:"deployments"`
}

type IntegrationTeams struct {
	WebhookUrl  string `json:"webhook_url"`
	Incidents   bool   `json:"incidents"`
	Deployments bool   `json:"deployments"`
}

type IntegrationPagerduty struct {
	IntegrationKey string `json:"integration_key"`
	Incidents      bool   `json:"incidents"`
}

type IntegrationOpsgenie struct {
	ApiKey     string `json:"api_key"`
	EUInstance bool   `json:"eu_instance"`
	Incidents  bool   `json:"incidents"`
}

type IntegrationWebhook struct {
	Url                string           `json:"url"`
	TlsSkipVerify      bool             `json:"tls_skip_verify"`
	BasicAuth          *utils.BasicAuth `json:"basic_auth"`
	CustomHeaders      []utils.Header   `json:"custom_headers"`
	Incidents          bool             `json:"incidents"`
	Deployments        bool             `json:"deployments"`
	IncidentTemplate   string           `json:"incident_template"`
	DeploymentTemplate string           `json:"deployment_template"`
}

func (db *DB) SaveIntegrationsBaseUrl(id ProjectId, baseUrl string) error {
	p, err := db.GetProject(id)
	if err != nil {
		return err
	}
	p.Settings.Integrations.BaseUrl = baseUrl
	return db.SaveProjectSettings(p)
}
