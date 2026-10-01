package db

import "testing"

func TestIntegrationWebhookValidateCustomFields(t *testing.T) {
	tests := []struct {
		name   string
		fields map[string]string
		valid  bool
	}{
		{name: "plain name", fields: map[string]string{"environment": "production"}, valid: true},
		{name: "underscore", fields: map[string]string{"team_name": "platform"}, valid: true},
		{name: "built-in field", fields: map[string]string{"status": "custom"}, valid: true},
		{name: "hyphen", fields: map[string]string{"team-name": "platform"}},
		{name: "leading digit", fields: map[string]string{"2team": "platform"}},
		{name: "capitalization collision", fields: map[string]string{"team": "a", "Team": "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &IntegrationWebhook{Url: "http://example.com", CustomFields: tt.fields}
			err := cfg.Validate()
			if (err == nil) != tt.valid {
				t.Fatalf("Validate() error = %v, want valid = %t", err, tt.valid)
			}
		})
	}
}
