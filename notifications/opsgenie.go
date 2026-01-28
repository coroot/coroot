package notifications

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/opsgenie/opsgenie-go-sdk-v2/alert"
	"github.com/opsgenie/opsgenie-go-sdk-v2/client"
	"github.com/sirupsen/logrus"
)

type Opsgenie struct {
	client *alert.Client
}

func NewOpsgenie(apiKey string, euInstance bool) *Opsgenie {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	cfg := &client.Config{
		ApiKey: apiKey,
		Logger: logger,
	}
	if euInstance {
		cfg.OpsGenieAPIURL = client.API_URL_EU
	}
	c, _ := alert.NewClient(cfg)
	return &Opsgenie{client: c}
}

func (og *Opsgenie) SendIncident(ctx context.Context, baseUrl string, n *db.IncidentNotification) error {
	if n.Status == model.OK {
		req := &alert.CloseAlertRequest{
			IdentifierType:  alert.ALIAS,
			IdentifierValue: n.ExternalKey,
			Source:          "Coroot",
		}
		_, err := og.client.Close(ctx, req)
		return err
	}

	req := &alert.CreateAlertRequest{
		Message: fmt.Sprintf("[%s] %s is not meeting its SLOs", strings.ToUpper(n.Status.String()), n.ApplicationId.Name),
		Alias:   n.ExternalKey,
		Source:  "Coroot",
	}
	switch n.Status {
	case model.CRITICAL:
		req.Priority = alert.P2
	case model.WARNING:
		req.Priority = alert.P3
	case model.INFO:
		req.Priority = alert.P4
	}
	if n.Details != nil && len(n.Details.Reports) > 0 {
		for _, r := range n.Details.Reports {
			req.Description += fmt.Sprintf("• %s / %s: %s\n", r.Name, r.Check, r.Message)
		}
	}
	req.Description += fmt.Sprintf("\n%s", incidentUrl(baseUrl, n))
	_, err := og.client.Create(ctx, req)
	return err
}

func (og *Opsgenie) SendAlert(ctx context.Context, baseUrl string, n *db.AlertNotification) error {
	if n.Status == model.OK {
		req := &alert.CloseAlertRequest{
			IdentifierType:  alert.ALIAS,
			IdentifierValue: n.ExternalKey,
			Source:          "Coroot",
		}
		_, err := og.client.Close(ctx, req)
		return err
	}

	displayName := alertDisplayName(n)
	req := &alert.CreateAlertRequest{
		Message: fmt.Sprintf("[%s] %s: %s", strings.ToUpper(n.Status.String()), displayName, n.Details.Summary),
		Alias:   n.ExternalKey,
		Source:  "Coroot",
	}
	switch n.Status {
	case model.CRITICAL:
		req.Priority = alert.P2
	case model.WARNING:
		req.Priority = alert.P3
	case model.INFO:
		req.Priority = alert.P4
	}
	if n.Details != nil {
		if n.Details.RuleName != "" {
			req.Description += fmt.Sprintf("Alerting rule: %s\n", n.Details.RuleName)
		}
		for _, d := range n.Details.Details {
			req.Description += fmt.Sprintf("%s: %s\n", d.Name, d.Value)
		}
	}
	req.Description += fmt.Sprintf("\n%s", alertUrl(baseUrl, n))
	_, err := og.client.Create(ctx, req)
	return err
}

func (og *Opsgenie) SendDeployment(ctx context.Context, project *db.Project, ds model.ApplicationDeploymentStatus) error {
	return fmt.Errorf("not supported")
}
