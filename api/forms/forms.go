package forms

import (
	"context"
	"errors"
	"fmt"
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

	slugRe  = regexp.MustCompile("^[-_0-9a-z]{3,}$")
	emailRe = regexp.MustCompile(`^[^@\r\n\t\f\v ]+@[^@\r\n\t\f\v ]+\.[a-z]+$`)
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

type ApiKeyForm struct {
	Action string `json:"action"`
	db.ApiKey
}

func (f *ApiKeyForm) Valid() bool {
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

type CustomApplicationForm struct {
	Name    string `json:"name"`
	NewName string `json:"new_name"`

	InstancePatternsStr string `json:"instance_patterns"`
	InstancePatterns    []string
}

func (f *CustomApplicationForm) Valid() bool {
	if !slugRe.MatchString(f.NewName) {
		return false
	}
	f.InstancePatterns = strings.Fields(f.InstancePatternsStr)
	if !utils.GlobValidate(f.InstancePatterns) {
		return false
	}
	return true
}

type ApplicationInstrumentationForm struct {
	model.ApplicationInstrumentation
}

func (f *ApplicationInstrumentationForm) Valid() bool {
	return f.Port != ""
}

type ApplicationSettingsProfilingForm struct {
	model.ApplicationSettingsProfiling
}

func (f *ApplicationSettingsProfilingForm) Valid() bool {
	return true
}

type ApplicationSettingsTracingForm struct {
	model.ApplicationSettingsTracing
}

func (f *ApplicationSettingsTracingForm) Valid() bool {
	return true
}

type ApplicationSettingsLogsForm struct {
	model.ApplicationSettingsLogs
}

func (f *ApplicationSettingsLogsForm) Valid() bool {
	return true
}

type ApplicationSettingsRisksForm struct {
	Action string        `json:"action"`
	Key    model.RiskKey `json:"key"`
	Reason string        `json:"reason"`
}

func (f *ApplicationSettingsRisksForm) Valid() bool {
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

func NewIntegrationForm(t db.IntegrationType, globalClickHouse *db.IntegrationClickhouse, globalPrometheus *db.IntegrationPrometheus) IntegrationForm {
	switch t {
	case db.IntegrationTypePrometheus:
		return &IntegrationFormPrometheus{global: globalPrometheus}
	case db.IntegrationTypeClickhouse:
		return &IntegrationFormClickhouse{global: globalClickHouse}
	case db.IntegrationTypeAWS:
		return &IntegrationFormAWS{}
	case db.IntegrationTypeSlack:
		return &IntegrationFormSlack{}
	case db.IntegrationTypeTeams:
		return &IntegrationFormTeams{}
	case db.IntegrationTypePagerduty:
		return &IntegrationFormPagerduty{}
	case db.IntegrationTypeOpsgenie:
		return &IntegrationFormOpsgenie{}
	case db.IntegrationTypeWebhook:
		return &IntegrationFormWebhook{}
	}
	return nil
}

type IntegrationFormPrometheus struct {
	db.IntegrationPrometheus
	global *db.IntegrationPrometheus
}

func (f *IntegrationFormPrometheus) Valid() bool {
	if _, err := url.Parse(f.IntegrationPrometheus.Url); err != nil {
		return false
	}
	if f.RemoteWriteUrl != "" {
		if _, err := url.Parse(f.IntegrationPrometheus.RemoteWriteUrl); err != nil {
			return false
		}
	}
	if !prom.IsSelectorValid(f.IntegrationPrometheus.ExtraSelector) {
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
	cfg := project.PrometheusConfig(f.global)
	if cfg.Url == "" {
		f.RefreshInterval = db.DefaultRefreshInterval
		return
	}
	f.IntegrationPrometheus = *cfg
	if masked {
		f.Url = "http://<hidden>"
		if f.BasicAuth != nil {
			f.BasicAuth.User = "<hidden>"
			f.BasicAuth.Password = "<hidden>"
		}
		for i := range f.CustomHeaders {
			f.CustomHeaders[i].Value = "<hidden>"
		}
		if f.RemoteWriteUrl != "" {
			f.RemoteWriteUrl = "<hidden>"
		}
	}
}

func (f *IntegrationFormPrometheus) Update(ctx context.Context, project *db.Project, clear bool) error {
	if f.global != nil {
		return fmt.Errorf("global Prometheus configuration is used and cannot be changed")
	}
	if err := f.Test(ctx, project); err != nil {
		return err
	}
	project.Prometheus = f.IntegrationPrometheus
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
	global *db.IntegrationClickhouse
}

func (f *IntegrationFormClickhouse) Valid() bool {
	if _, _, err := net.SplitHostPort(f.Addr); err != nil {
		return false
	}
	return true
}

func (f *IntegrationFormClickhouse) Get(project *db.Project, masked bool) {
	cfg := project.ClickHouseConfig(f.global)
	if cfg != nil {
		f.IntegrationClickhouse = *cfg
	}
	if f.Protocol == "" {
		f.Protocol = "native"
	}
	if f.Database == "" {
		f.Database = "default"
	}
	if f.Auth.User == "" {
		f.Auth.User = "default"
	}
	if masked {
		f.Addr = "<hidden>"
		f.Auth.User = "<hidden>"
		f.Auth.Password = "<hidden>"
	}
}

func (f *IntegrationFormClickhouse) Update(ctx context.Context, project *db.Project, clear bool) error {
	if f.global != nil {
		return fmt.Errorf("global ClickHouse configuration is used and cannot be changed")
	}
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
	config.TlsEnable = f.TlsEnable
	config.TlsSkipVerify = f.TlsSkipVerify
	client, err := clickhouse.NewClient(config, false)
	if err != nil {
		return err
	}
	if err = client.Ping(ctx); err != nil {
		return err
	}
	return nil
}

type IntegrationFormAWS struct {
	model.AWSConfig
}

func (f *IntegrationFormAWS) Valid() bool {
	return f.Region != "" && f.AccessKeyID != "" && f.SecretAccessKey != ""
}

func (f *IntegrationFormAWS) Get(project *db.Project, masked bool) {
	cfg := project.Settings.Integrations.AWS
	if cfg != nil {
		f.AWSConfig = *cfg
	}
	if masked {
		f.AccessKeyID = "<hidden>"
		f.SecretAccessKey = "<hidden>"
	}
}

func (f *IntegrationFormAWS) Update(ctx context.Context, project *db.Project, clear bool) error {
	cfg := &f.AWSConfig
	if clear {
		cfg = nil
	} else {
		if err := f.Test(ctx, project); err != nil {
			return err
		}
	}
	project.Settings.Integrations.AWS = cfg
	return nil
}

func (f *IntegrationFormAWS) Test(ctx context.Context, project *db.Project) error {
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
		f.Token = "<hidden>"
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
	return notifications.NewSlack(f.Token, f.DefaultChannel).SendIncident(ctx, project.Settings.Integrations.BaseUrl, testIncidentNotification(project))
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
		f.WebhookUrl = "<hidden>"
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
	return notifications.NewTeams(f.WebhookUrl).SendIncident(ctx, project.Settings.Integrations.BaseUrl, testIncidentNotification(project))
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
		f.IntegrationKey = "<hidden>"
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
	return notifications.NewPagerduty(f.IntegrationKey).SendIncident(ctx, project.Settings.Integrations.BaseUrl, testIncidentNotification(project))
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
		f.ApiKey = "<hidden>"
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
	return notifications.NewOpsgenie(f.ApiKey, f.EUInstance).SendIncident(ctx, project.Settings.Integrations.BaseUrl, testIncidentNotification(project))
}

type IntegrationFormWebhook struct {
	db.IntegrationWebhook
}

func (f *IntegrationFormWebhook) Valid() bool {
	if f.Url == "" {
		return false
	}
	if f.Incidents && f.IncidentTemplate == "" {
		return false
	}
	if f.Deployments && f.DeploymentTemplate == "" {
		return false
	}
	return true
}

func (f *IntegrationFormWebhook) Get(project *db.Project, masked bool) {
	cfg := project.Settings.Integrations.Webhook
	if cfg == nil {
		f.Incidents = true
		f.Deployments = true
		return
	}
	f.IntegrationWebhook = *cfg
	if masked {
		f.Url = "<hidden>"
	}
}

func (f *IntegrationFormWebhook) Update(ctx context.Context, project *db.Project, clear bool) error {
	cfg := &f.IntegrationWebhook
	if clear {
		cfg = nil
	}
	project.Settings.Integrations.Webhook = cfg
	return nil
}

func (f *IntegrationFormWebhook) Test(ctx context.Context, project *db.Project) error {
	cfg := &f.IntegrationWebhook
	wh := notifications.NewWebhook(cfg)
	if cfg.Incidents {
		err := wh.SendIncident(ctx, project.Settings.Integrations.BaseUrl, testIncidentNotification(project))
		if err != nil {
			return err
		}
	}
	if cfg.Deployments {
		err := wh.SendDeployment(ctx, project, testDeploymentNotification())
		if err != nil {
			return err
		}
	}
	return nil
}

func testIncidentNotification(project *db.Project) *db.IncidentNotification {
	return &db.IncidentNotification{
		ProjectId:     project.Id,
		ApplicationId: model.NewApplicationId("default", model.ApplicationKindDeployment, "test-alert-fake-app"),
		IncidentKey:   "123ab456",
		Status:        model.WARNING,
		Details: &db.IncidentNotificationDetails{
			Reports: []db.IncidentNotificationDetailsReport{
				{Name: model.AuditReportSLO, Check: model.Checks.SLOLatency.Title, Message: "error budget burn rate is 20x within 1 hour"},
				{Name: model.AuditReportNetwork, Check: model.Checks.NetworkRTT.Title, Message: "high network latency to 2 upstream services"},
			},
		},
	}
}

func testDeploymentNotification() model.ApplicationDeploymentStatus {
	return model.ApplicationDeploymentStatus{
		Status: model.OK,
		State:  model.ApplicationDeploymentStateSummary,
		Summary: []model.ApplicationDeploymentSummary{
			{Report: model.AuditReportSLO, Ok: false, Message: "Availability: 87% (objective: 99%)"},
			{Report: model.AuditReportCPU, Ok: false, Message: "CPU usage: +21% (+$37/mo) compared to the previous deployment"},
			{Report: model.AuditReportCPU, Ok: true, Message: "Memory: looks like the memory leak has been fixed"},
		},
		Deployment: &model.ApplicationDeployment{
			ApplicationId: model.NewApplicationId("default", model.ApplicationKindDeployment, "test-deployment-fake-app"),
			Name:          "123ab456",
			Details:       &model.ApplicationDeploymentDetails{ContainerImages: []string{"app:v1.8.2"}},
		},
	}
}
