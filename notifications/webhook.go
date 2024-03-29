package notifications

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/template"

	"github.com/coroot/coroot/utils"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
)

type Webhook struct {
	cfg *db.IntegrationWebhook
}

type IncidentTemplateValues struct {
	Status      string                                 `json:"status"`
	Application model.ApplicationId                    `json:"application"`
	Reports     []db.IncidentNotificationDetailsReport `json:"reports"`
	URL         string                                 `json:"url"`
}

type DeploymentTemplateValues struct {
	Status      string              `json:"status"`
	Application model.ApplicationId `json:"application"`
	Version     string              `json:"version"`
	Summary     []string            `json:"summary"`
	URL         string              `json:"url"`
}

func NewWebhook(cfg *db.IntegrationWebhook) *Webhook {
	return &Webhook{cfg: cfg}
}

func (wh *Webhook) SendIncident(ctx context.Context, baseUrl string, n *db.IncidentNotification) error {
	tmpl, err := template.New("incidentTemplate").Funcs(templateFunctions).Parse(wh.cfg.IncidentTemplate)
	if err != nil {
		return fmt.Errorf("invalid incident template: %s", err)
	}

	var data bytes.Buffer
	err = tmpl.Execute(&data, IncidentTemplateValues{
		Status:      strings.ToUpper(n.Status.String()),
		Application: n.ApplicationId,
		Reports:     n.Details.Reports,
		URL:         incidentUrl(baseUrl, n),
	})
	if err != nil {
		return fmt.Errorf("invalid incident template: %s", err)
	}

	return wh.send(ctx, data.Bytes())
}

func (wh *Webhook) SendDeployment(ctx context.Context, project *db.Project, ds model.ApplicationDeploymentStatus) error {
	tmpl, err := template.New("deploymentTemplate").Funcs(templateFunctions).Parse(wh.cfg.DeploymentTemplate)
	if err != nil {
		return fmt.Errorf("invalid deployment template: %s", err)
	}

	status := "Deployed"
	var summary []string
	switch ds.State {
	case model.ApplicationDeploymentStateInProgress:
		return nil
	case model.ApplicationDeploymentStateStuck:
		status = "Stuck"
	case model.ApplicationDeploymentStateCancelled:
		status = "Cancelled"
	case model.ApplicationDeploymentStateSummary:
		for _, s := range ds.Summary {
			summary = append(summary, fmt.Sprintf("%s %s", s.Emoji(), s.Message))
		}
		if len(summary) == 0 {
			summary = append(summary, "No notable changes")
		}
	}

	var data bytes.Buffer
	err = tmpl.Execute(&data, DeploymentTemplateValues{
		Application: ds.Deployment.ApplicationId,
		Status:      status,
		Version:     ds.Deployment.Version(),
		Summary:     summary,
		URL:         deploymentUrl(project.Settings.Integrations.BaseUrl, project.Id, ds.Deployment),
	})
	if err != nil {
		return fmt.Errorf("invalid deployment template: %s", err)
	}

	return wh.send(ctx, data.Bytes())
}

func (wh *Webhook) send(ctx context.Context, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, wh.cfg.Url, bytes.NewReader(utils.EscapeJsonMultilineStrings(data)))
	if err != nil {
		return err
	}
	if wh.cfg.BasicAuth != nil {
		req.SetBasicAuth(wh.cfg.BasicAuth.User, wh.cfg.BasicAuth.Password)
	}
	req.Header.Set("Content-Type", "application/json")
	for _, h := range wh.cfg.CustomHeaders {
		req.Header.Add(h.Key, h.Value)
	}
	httpClient := &http.Client{}
	if wh.cfg.TlsSkipVerify {
		httpClient.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("response status: %s", resp.Status)
		}
		return fmt.Errorf("%s: %s", resp.Status, string(body))
	}

	return nil
}

var (
	templateFunctions = template.FuncMap{
		"json": func(arg any) (string, error) {
			data, err := json.Marshal(arg)
			return string(data), err
		},
	}
)
