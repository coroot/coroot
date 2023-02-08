package api

import (
	"errors"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/notifications"
	"github.com/coroot/coroot/utils"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

var (
	ErrInvalidForm = errors.New("invalid form")

	slugRe = regexp.MustCompile("^[-_0-9a-z]{3,}$")
)

type Form interface {
	Valid() bool
}

func ReadAndValidate(r *http.Request, f Form) error {
	if err := utils.ReadJson(r, f); err != nil {
		return err
	}
	if !f.Valid() {
		return ErrInvalidForm
	}
	return nil
}

type ProjectForm struct {
	Name string `json:"name"`

	Prometheus db.Prometheus `json:"prometheus"`
}

func (f *ProjectForm) Valid() bool {
	if !slugRe.MatchString(f.Name) {
		return false
	}
	if _, err := url.Parse(f.Prometheus.Url); err != nil {
		return false
	}
	return true
}

type ProjectStatusForm struct {
	Mute   *model.ApplicationType `json:"mute"`
	UnMute *model.ApplicationType `json:"unmute"`
}

func (f *ProjectStatusForm) Valid() bool {
	return true
}

type CheckConfigForm struct {
	Configs []*model.CheckConfigSimple `json:"configs"`
}

func (f *CheckConfigForm) Valid() bool {
	return true
}

type CheckConfigSLOAvailabilityForm struct {
	Configs []model.CheckConfigSLOAvailability `json:"configs"`
	Default bool                               `json:"default"`
}

func (f *CheckConfigSLOAvailabilityForm) Valid() bool {
	for _, c := range f.Configs {
		if c.Custom && (c.TotalRequestsQuery == "" || c.FailedRequestsQuery == "") {
			return false
		}
	}
	return true
}

type CheckConfigSLOLatencyForm struct {
	Configs []model.CheckConfigSLOLatency `json:"configs"`
	Default bool                          `json:"default"`
}

func (f *CheckConfigSLOLatencyForm) Valid() bool {
	for _, c := range f.Configs {
		if c.Custom && (c.HistogramQuery == "" || c.ObjectiveBucket <= 0) {
			return false
		}
	}
	return true
}

type ApplicationCategoryForm struct {
	Name           model.ApplicationCategory `json:"name"`
	NewName        model.ApplicationCategory `json:"new_name"`
	CustomPatterns string                    `json:"custom_patterns"`
	customPatterns []string

	NotifyOfDeployments bool `json:"notify_of_deployments"`
}

func (f *ApplicationCategoryForm) Valid() bool {
	if !slugRe.MatchString(string(f.NewName)) {
		return false
	}
	f.customPatterns = strings.Fields(f.CustomPatterns)
	if !utils.GlobValidate(f.customPatterns) {
		return false
	}
	for _, p := range f.customPatterns {
		if strings.Count(p, "/") != 1 || strings.Index(p, "/") < 1 {
			return false
		}
	}
	return true
}

type IntegrationsForm struct {
	BaseUrl string `json:"base_url"`
}

func (f *IntegrationsForm) Valid() bool {
	if _, err := url.Parse(f.BaseUrl); err != nil || f.BaseUrl == "" {
		return false
	}
	f.BaseUrl = strings.TrimRight(f.BaseUrl, "/")
	return true
}

type IntegrationForm interface {
	Form
	Get(project *db.Project, masked bool)
	Update(project *db.Project, clear bool)
	GetNotificationClient() notifications.NotificationClient
}

func NewIntegrationForm(t db.IntegrationType) IntegrationForm {
	switch t {
	case db.IntegrationTypeSlack:
		return &IntegrationFormSlack{}
	case db.IntegrationTypeTeams:
		return &IntegrationFormTeams{}
	case db.IntegrationTypePagerduty:
		return &IntegrationFormPagerduty{}
	case db.IntegrationTypeOpsgenie:
		return &IntegrationFormOpsgenie{}
	}
	return nil
}

type IntegrationFormSlack struct {
	db.IntegrationSlack
}

func (f *IntegrationFormSlack) Valid() bool {
	if f.Token == "" || f.DefaultChannel == "" {
		return false
	}
	return true
}

func (f *IntegrationFormSlack) Get(project *db.Project, masked bool) {
	cfg := project.Settings.Integrations.Slack
	if cfg == nil {
		f.Incidents = true
		f.Deployments = true
		return
	}
	f.IntegrationSlack = *cfg
	if masked {
		f.Token = "<token>"
	}
}

func (f *IntegrationFormSlack) Update(project *db.Project, clear bool) {
	cfg := &f.IntegrationSlack
	if clear {
		cfg = nil
	}
	project.Settings.Integrations.Slack = cfg
}

func (f *IntegrationFormSlack) GetNotificationClient() notifications.NotificationClient {
	return notifications.NewSlack(f.Token, f.DefaultChannel)
}

type IntegrationFormTeams struct {
	db.IntegrationTeams
}

func (f *IntegrationFormTeams) Valid() bool {
	if f.WebhookUrl == "" {
		return false
	}
	return true
}

func (f *IntegrationFormTeams) Get(project *db.Project, masked bool) {
	cfg := project.Settings.Integrations.Teams
	if cfg == nil {
		f.Incidents = true
		f.Deployments = true
		return
	}
	f.IntegrationTeams = *cfg
	if masked {
		f.WebhookUrl = "<webhook_url>"
	}
}

func (f *IntegrationFormTeams) Update(project *db.Project, clear bool) {
	cfg := &f.IntegrationTeams
	if clear {
		cfg = nil
	}
	project.Settings.Integrations.Teams = cfg
}

func (f *IntegrationFormTeams) GetNotificationClient() notifications.NotificationClient {
	return notifications.NewTeams(f.WebhookUrl)
}

type IntegrationFormPagerduty struct {
	db.IntegrationPagerduty
}

func (f *IntegrationFormPagerduty) Valid() bool {
	if f.IntegrationKey == "" {
		return false
	}
	return true
}

func (f *IntegrationFormPagerduty) Get(project *db.Project, masked bool) {
	cfg := project.Settings.Integrations.Pagerduty
	if cfg == nil {
		f.Incidents = true
		return
	}
	f.IntegrationPagerduty = *cfg
	if masked {
		f.IntegrationKey = "<integration_key>"
	}
}

func (f *IntegrationFormPagerduty) Update(project *db.Project, clear bool) {
	cfg := &f.IntegrationPagerduty
	if clear {
		cfg = nil
	}
	project.Settings.Integrations.Pagerduty = cfg
}

func (f *IntegrationFormPagerduty) GetNotificationClient() notifications.NotificationClient {
	return notifications.NewPagerduty(f.IntegrationKey)
}

type IntegrationFormOpsgenie struct {
	db.IntegrationOpsgenie
}

func (f *IntegrationFormOpsgenie) Valid() bool {
	if f.ApiKey == "" {
		return false
	}
	return true
}

func (f *IntegrationFormOpsgenie) Get(project *db.Project, masked bool) {
	cfg := project.Settings.Integrations.Opsgenie
	if cfg == nil {
		f.Incidents = true
		return
	}
	f.IntegrationOpsgenie = *cfg
	if masked {
		f.ApiKey = "<api_key>"
	}
}

func (f *IntegrationFormOpsgenie) Update(project *db.Project, clear bool) {
	cfg := &f.IntegrationOpsgenie
	if clear {
		cfg = nil
	}
	project.Settings.Integrations.Opsgenie = cfg
}

func (f *IntegrationFormOpsgenie) GetNotificationClient() notifications.NotificationClient {
	return notifications.NewOpsgenie(f.ApiKey, f.EUInstance)
}
