package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"text/template"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

type WebHook struct {
	webhookUrl string
	correctJSON string
	incidentTemplate string
}

type IncidentTemplateValues struct {
	StatusOK bool
	StatusINFO bool
	StatusWARNING bool
	StatusCRITICAL bool
	Status string
	App model.ApplicationId
	Reports []db.IncidentNotificationDetailsReport
	URL string
	Timestamp timeseries.Time
}

func NewWebHook(webhookUrl, correctJSON, incidentTemplate string) *WebHook {
	return &WebHook{
		webhookUrl: webhookUrl,
		correctJSON: correctJSON,
		incidentTemplate: incidentTemplate,
	}
}

func (t *WebHook) SendIncident(ctx context.Context, baseUrl string, n *db.IncidentNotification) error {
	// Parse template
	tmpl, err := template.New("incidentTemplate").Parse(t.incidentTemplate)
	if err != nil {
		return fmt.Errorf("WebHookIntegration: cant parse incidentTemplate: %s", err)
	}
	// Fill template
	var data bytes.Buffer
	err = tmpl.Execute(&data, IncidentTemplateValues{
		StatusOK: n.Status == model.OK,
		StatusINFO: n.Status == model.INFO,
		StatusWARNING: n.Status == model.WARNING,
		StatusCRITICAL: n.Status == model.CRITICAL,
		Status: strings.ToUpper( fmt.Sprint(n.Status) ),
		App: n.ApplicationId,
		Reports: n.Details.Reports,
		URL: incidentUrl(baseUrl, n),
		Timestamp: n.Timestamp,
	})
	if err != nil {
		return fmt.Errorf("WebHookIntegration: cant fill incidentTemplate: %s", err)
	}

	// Send
	resp, err := http.Post(t.webhookUrl, "application/json", &data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check response
	//
	// Unpack correct_json in map[string]interface{}
	var correctData map[string]interface{}
	err = json.Unmarshal([]byte(t.correctJSON), &correctData)
	if err != nil {
		return fmt.Errorf("WebHookIntegration: invalid correctJSON: %s", err)
	}
	// Unpack resp_json in map[string]interface{}
	var respData map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&respData)
	if err != nil {
		return fmt.Errorf("WebHookIntegration: invalid response from endpoint: %s", err)
	}
	// Check all fields(and values) from correct_json in resp_json
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
