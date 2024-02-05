package forms

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/coroot/coroot/clickhouse"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/notifications"
	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/utils"
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
}

func (f *ProjectForm) Valid() bool {
	if !slugRe.MatchString(f.Name) {
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
	Name    model.ApplicationCategory `json:"name"`
	NewName model.ApplicationCategory `json:"new_name"`

	CustomPatternsStr string `json:"custom_patterns"`
	CustomPatterns    []string

	NotifyOfDeployments bool `json:"notify_of_deployments"`
}

func (f *ApplicationCategoryForm) Valid() bool {
	if !slugRe.MatchString(string(f.NewName)) {
		return false
	}
	f.CustomPatterns = strings.Fields(f.CustomPatternsStr)
	if !utils.GlobValidate(f.CustomPatterns) {
		return false
	}
	for _, p := range f.CustomPatterns {
		if strings.Count(p, "/") != 1 || strings.Index(p, "/") < 1 {
			return false
		}
	}
	return true
}

type ApplicationSettingsProfilingForm struct {
	db.ApplicationSettingsProfiling
}

func (f *ApplicationSettingsProfilingForm) Valid() bool {
	return true
}

type ApplicationSettingsTracingForm struct {
	db.ApplicationSettingsTracing
}

func (f *ApplicationSettingsTracingForm) Valid() bool {
	return true
}

type ApplicationSettingsLogsForm struct {
	db.ApplicationSettingsLogs
}

func (f *ApplicationSettingsLogsForm) Valid() bool {
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
	Update(ctx context.Context, project *db.Project, clear bool) error
	Test(ctx context.Context, project *db.Project) error
}

func NewIntegrationForm(t db.IntegrationType) IntegrationForm {
	switch t {
	case db.IntegrationTypePrometheus:
		return &IntegrationFormPrometheus{}
	case db.IntegrationTypeClickhouse:
		return &IntegrationFormClickhouse{}
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

type IntegrationFormPrometheus struct {
	db.IntegrationsPrometheus
}

func (f *IntegrationFormPrometheus) Valid() bool {
	if _, err := url.Parse(f.IntegrationsPrometheus.Url); err != nil {
		return false
	}
	if !prom.IsSelectorValid(f.IntegrationsPrometheus.ExtraSelector) {
		return false
	}
	var validHeaders []utils.Header
	for _, h := range f.CustomHeaders {
		if h.Valid() {
			validHeaders = append(validHeaders, h)
		}
	}
	f.CustomHeaders = validHeaders
	return true
}

func (f *IntegrationFormPrometheus) Get(project *db.Project, masked bool) {
	cfg := project.Prometheus
	if cfg.Url == "" {
		f.RefreshInterval = db.DefaultRefreshInterval
		return
	}
	f.IntegrationsPrometheus = cfg
	if masked {
		f.Url = "http://<hidden>"
		if f.BasicAuth != nil {
			f.BasicAuth.User = "<user>"
			f.BasicAuth.Password = "<password>"
		}
		for i := range f.CustomHeaders {
			f.CustomHeaders[i].Value = "<header>"
		}
	}
}

func (f *IntegrationFormPrometheus) Update(ctx context.Context, project *db.Project, clear bool) error {
	if err := f.Test(ctx, project); err != nil {
		return err
	}
	project.Prometheus = f.IntegrationsPrometheus
	return nil
}

func (f *IntegrationFormPrometheus) Test(ctx context.Context, project *db.Project) error {
	config := prom.NewClientConfig(f.Url, f.RefreshInterval)
	config.BasicAuth = f.BasicAuth
	config.TlsSkipVerify = f.TlsSkipVerify
	config.ExtraSelector = f.ExtraSelector
	config.CustomHeaders = f.CustomHeaders
	client, err := prom.NewClient(config)
	if err != nil {
		return err
	}
	if err := client.Ping(ctx); err != nil {
		return err
	}
	return nil
}

type IntegrationFormClickhouse struct {
	db.IntegrationClickhouse
}

func (f *IntegrationFormClickhouse) Valid() bool {
	if _, _, err := net.SplitHostPort(f.Addr); err != nil {
		return false
	}
	return true
}

func (f *IntegrationFormClickhouse) Get(project *db.Project, masked bool) {
	cfg := project.Settings.Integrations.Clickhouse
	if cfg != nil {
		f.IntegrationClickhouse = *cfg
	}
	if f.Protocol == "" {
		f.Protocol = "native"
	}
	if f.Database == "" {
		f.Database = "default"
	}
	if cfg == nil && f.TracesTable == "" {
		f.TracesTable = "otel_traces"
	}
	if cfg == nil && f.LogsTable == "" {
		f.LogsTable = "otel_logs"
	}
	if masked {
		f.Addr = "<hidden>"
		f.Auth.User = "<user>"
		f.Auth.Password = "<password>"
	}
}

func (f *IntegrationFormClickhouse) Update(ctx context.Context, project *db.Project, clear bool) error {
	cfg := &f.IntegrationClickhouse
	if clear {
		cfg = nil
	} else {
		if err := f.Test(ctx, project); err != nil {
			return err
		}
	}
	project.Settings.Integrations.Clickhouse = cfg
	return nil
}

func (f *IntegrationFormClickhouse) Test(ctx context.Context, project *db.Project) error {
	config := clickhouse.NewClientConfig(f.Addr, f.Auth.User, f.Auth.Password)
	config.Protocol = f.Protocol
	config.Database = f.Database
	config.TracesTable = f.TracesTable
	config.LogsTable = f.LogsTable
	config.TlsEnable = f.TlsEnable
	config.TlsSkipVerify = f.TlsSkipVerify
	client, err := clickhouse.NewClient(config)
	if err != nil {
		return err
	}
	if err = client.Ping(ctx); err != nil {
		return err
	}
	if f.TracingEnabled() {
		if _, err = client.GetServicesFromTraces(ctx); err != nil {
			return err
		}
	}
	if f.LogsEnabled() {
		if _, err = client.GetServicesFromLogs(ctx); err != nil {
			return err
		}
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

func (f *IntegrationFormSlack) Update(ctx context.Context, project *db.Project, clear bool) error {
	cfg := &f.IntegrationSlack
	if clear {
		cfg = nil
	}
	project.Settings.Integrations.Slack = cfg
	return nil
}

func (f *IntegrationFormSlack) Test(ctx context.Context, project *db.Project) error {
	return notifications.NewSlack(f.Token, f.DefaultChannel).SendIncident(ctx, project.Settings.Integrations.BaseUrl, testNotification(project))
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

func (f *IntegrationFormTeams) Update(ctx context.Context, project *db.Project, clear bool) error {
	cfg := &f.IntegrationTeams
	if clear {
		cfg = nil
	}
	project.Settings.Integrations.Teams = cfg
	return nil
}

func (f *IntegrationFormTeams) Test(ctx context.Context, project *db.Project) error {
	return notifications.NewTeams(f.WebhookUrl).SendIncident(ctx, project.Settings.Integrations.BaseUrl, testNotification(project))
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

func (f *IntegrationFormPagerduty) Update(ctx context.Context, project *db.Project, clear bool) error {
	cfg := &f.IntegrationPagerduty
	if clear {
		cfg = nil
	}
	project.Settings.Integrations.Pagerduty = cfg
	return nil
}

func (f *IntegrationFormPagerduty) Test(ctx context.Context, project *db.Project) error {
	return notifications.NewPagerduty(f.IntegrationKey).SendIncident(ctx, project.Settings.Integrations.BaseUrl, testNotification(project))
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

func (f *IntegrationFormOpsgenie) Update(ctx context.Context, project *db.Project, clear bool) error {
	cfg := &f.IntegrationOpsgenie
	if clear {
		cfg = nil
	}
	project.Settings.Integrations.Opsgenie = cfg
	return nil
}

func (f *IntegrationFormOpsgenie) Test(ctx context.Context, project *db.Project) error {
	return notifications.NewOpsgenie(f.ApiKey, f.EUInstance).SendIncident(ctx, project.Settings.Integrations.BaseUrl, testNotification(project))
}

func testNotification(project *db.Project) *db.IncidentNotification {
	return &db.IncidentNotification{
		ProjectId:     project.Id,
		ApplicationId: model.NewApplicationId("default", model.ApplicationKindDeployment, "test-alert-fake-app"),
		IncidentKey:   "fake",
		Status:        model.INFO,
		Details: &db.IncidentNotificationDetails{
			Reports: []db.IncidentNotificationDetailsReport{
				{Name: model.AuditReportSLO, Check: model.Checks.SLOLatency.Title, Message: "error budget burn rate is 20x within 1 hour"},
				{Name: model.AuditReportNetwork, Check: model.Checks.NetworkRTT.Title, Message: "high network latency to 2 upstream services"},
			},
		},
	}
}
