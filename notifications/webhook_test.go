package notifications

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
)

func TestWebhookCustomFields(t *testing.T) {
	requests := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		requests <- body
	}))
	defer server.Close()

	cfg := &db.IntegrationWebhook{
		Url:              server.URL,
		Incidents:        true,
		IncidentTemplate: `{"environment":"{{ .Environment }}","payload":{{ json . }}}`,
		CustomFields:     map[string]string{"environment": "production", "status": "custom"},
	}
	wh := NewWebhook(cfg)
	if err := wh.SendIncident(context.Background(), "http://coroot", &db.IncidentNotification{Status: model.CRITICAL}); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Environment string         `json:"environment"`
		Payload     map[string]any `json:"payload"`
	}
	if err := json.Unmarshal(<-requests, &got); err != nil {
		t.Fatal(err)
	}
	if got.Environment != "production" || got.Payload["environment"] != "production" || got.Payload["status"] != "CRITICAL" {
		t.Fatalf("unexpected webhook payload: %+v", got)
	}
}

func TestWebhookInvalidStoredCustomField(t *testing.T) {
	cfg := &db.IntegrationWebhook{
		Url:                "http://example.com",
		Incidents:          true,
		Deployments:        true,
		IncidentTemplate:   `{{ json . }}`,
		DeploymentTemplate: `{{ json . }}`,
		AlertTemplate:      `{{ json . }}`,
		CustomFields:       map[string]string{"team-name": "platform"},
	}
	wh := NewWebhook(cfg)
	tests := []struct {
		name string
		send func() error
	}{
		{name: "incident", send: func() error {
			return wh.SendIncident(context.Background(), "http://coroot", &db.IncidentNotification{})
		}},
		{name: "alert", send: func() error { return wh.SendAlert(context.Background(), "http://coroot", &db.AlertNotification{}) }},
		{name: "deployment", send: func() error { return wh.SendDeployment(context.Background(), nil, model.ApplicationDeploymentStatus{}) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.send(); err == nil || !strings.Contains(err.Error(), "team-name") {
				t.Fatalf("send() error = %v, want invalid field error", err)
			}
		})
	}
}
