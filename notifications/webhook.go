package notifications

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"text/template"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

type WebHook struct {
	webhookUrl         string
	incidentTemplate   string
	deploymentTemplate string
}

type IncidentTemplateValues struct {
	Status    string
	App       model.ApplicationId
	Reports   []db.IncidentNotificationDetailsReport
	URL       string
	Timestamp timeseries.Time
}

type Project struct {
	Id   string
	Name string
}

type DeploymentTemplateValues struct {
	Status  string
	App     model.ApplicationId
	Project Project
	Summary []string
	URL     string
}

func NewWebHook(webhookUrl string, incidentTemplate string, deploymentTemplate string) *WebHook {
	return &WebHook{
		webhookUrl:         webhookUrl,
		incidentTemplate:   incidentTemplate,
		deploymentTemplate: deploymentTemplate,
	}
}

func (t *WebHook) SendIncident(ctx context.Context, baseUrl string, n *db.IncidentNotification) error {
	tmpl, err := template.New("incidentTemplate").Parse(t.incidentTemplate)
	if err != nil {
		return fmt.Errorf("WebHookIntegration: cant parse incidentTemplate: %s", err)
	}

	var data bytes.Buffer
	err = tmpl.Execute(&data, IncidentTemplateValues{
		Status:    strings.ToUpper(fmt.Sprint(n.Status)),
		App:       n.ApplicationId,
		Reports:   n.Details.Reports,
		URL:       incidentUrl(baseUrl, n),
		Timestamp: n.Timestamp,
	})
	if err != nil {
		return fmt.Errorf("WebHookIntegration: cant fill incidentTemplate: %s", err)
	}

	resp, err := http.Post(t.webhookUrl, "application/json", &data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return t.validateResponse(resp)
}

func (t *WebHook) SendDeployment(ctx context.Context, project *db.Project, ds model.ApplicationDeploymentStatus) error {
	tmpl, err := template.New("deploymentTemplate").Parse(t.deploymentTemplate)
	if err != nil {
		return fmt.Errorf("WebHookIntegration: cant parse deploymentTemplate: %s", err)
	}

	d := ds.Deployment

	status := "Deployed"
	switch ds.State {
	case model.ApplicationDeploymentStateInProgress:
		return nil
	case model.ApplicationDeploymentStateStuck:
		status = "Stuck"
	case model.ApplicationDeploymentStateCancelled:
		status = "Cancelled"
	}

	var summary []string

	if ds.State == model.ApplicationDeploymentStateSummary {
		summary = append(summary, "No notable changes")
		if len(ds.Summary) > 0 {
			for _, s := range ds.Summary {
				summary = append(summary, fmt.Sprintf("%s %s", s.Emoji(), s.Message))
			}
		}
	}

	var data bytes.Buffer
	var projectDeploy = Project{
		Name: *&project.Name,
		Id:   string(project.Id),
	}

	err = tmpl.Execute(&data, DeploymentTemplateValues{
		Status:  status,
		App:     d.ApplicationId,
		Project: projectDeploy,
		Summary: summary,
		URL:     deploymentUrl(project.Settings.Integrations.BaseUrl, project.Id, d),
	})
	if err != nil {
		return fmt.Errorf("WebHookIntegration: cant fill deploymentTemplate: %s", err)
	}

	resp, err := http.Post(t.webhookUrl, "application/json", &data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return t.validateResponse(resp)
}

func (t *WebHook) validateResponse(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK {
		respBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("WebHookIntegration: error reading response body: %s", err)
		}
		return fmt.Errorf("WebHookIntegration: invalid response status code: %d, response body: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
