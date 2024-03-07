package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"text/template"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

type WebHook struct {
	webhookUrl         string
	correctResponse    string
	isJsonResponse     bool
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

type DeploymentTemplateValues struct {
	Status  string
	Title   string
	Summary []string
	URL     string
}

func NewWebHook(webhookUrl string, correctResponse string, isJsonResponse bool, incidentTemplate string, deploymentTemplate string) *WebHook {
	return &WebHook{
		webhookUrl:         webhookUrl,
		correctResponse:    correctResponse,
		isJsonResponse:     isJsonResponse,
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

	title := fmt.Sprintf("Deployment of **%s** to **%s**", d.ApplicationId.Name, project.Name)

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
	err = tmpl.Execute(&data, DeploymentTemplateValues{
		Status:  status,
		Title:   title,
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
	if !t.isJsonResponse {
		respBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("WebHookIntegration: invalid response: %s", err)
		}
		if t.correctResponse != string(respBody) {
			return fmt.Errorf("WebHookIntegration: invalid correctResponse: %s", err)
		}
		return nil
	}

	var correctData map[string]interface{}
	err := json.Unmarshal([]byte(t.correctResponse), &correctData)
	if err != nil {
		return fmt.Errorf("WebHookIntegration: invalid correctResponse: %s", err)
	}

	var respData map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&respData)
	if err != nil {
		return fmt.Errorf("WebHookIntegration: invalid response from endpoint: %s", err)
	}

	for key, value := range correctData {
		respValue, ok := respData[key]
		if !ok {
			return fmt.Errorf("WebHookIntegration: endpoint doesn't contains field:\n%s", key)
		} else if !reflect.DeepEqual(value, respValue) {
			return fmt.Errorf("WebHookIntegration: endpoint response %s == %s: %s", key, respValue, respData)
		}
	}

	return nil
}
