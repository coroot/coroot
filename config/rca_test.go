package config

import (
	"strings"
	"testing"

	"github.com/coroot/coroot/timeseries"
)

func TestRCAValidateCloudProviderNeedsNothing(t *testing.T) {
	for _, provider := range []string{"", RCAProviderCloud} {
		c := RCA{Provider: provider}
		if err := c.Validate(); err != nil {
			t.Errorf("provider %q should be valid: %s", provider, err)
		}
		if c.IsLocal() {
			t.Errorf("provider %q should not be local", provider)
		}
	}
}

func TestRCAValidateRejectsUnknownProvider(t *testing.T) {
	c := RCA{Provider: "openai"}
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), "invalid provider") {
		t.Fatalf("expected an invalid provider error, got %v", err)
	}
}

func TestRCAValidateLocalRequirements(t *testing.T) {
	valid := RCA{Provider: RCAProviderLocal, BaseUrl: "https://openrouter.ai/api/v1", Model: "moonshotai/kimi-k2.6", Timeout: timeseries.Minute}
	if err := valid.Validate(); err != nil {
		t.Fatalf("expected a valid config, got %s", err)
	}
	if !valid.IsLocal() {
		t.Error("expected the config to be local")
	}

	cases := map[string]struct {
		mutate func(c *RCA)
		want   string
	}{
		"missing base url": {func(c *RCA) { c.BaseUrl = "" }, "base_url is required"},
		"invalid base url": {func(c *RCA) { c.BaseUrl = "openrouter.ai" }, "invalid url"},
		"missing model":    {func(c *RCA) { c.Model = "" }, "model is required"},
		"missing timeout":  {func(c *RCA) { c.Timeout = 0 }, "invalid timeout"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := valid
			tc.mutate(&c)
			err := c.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected an error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestRCADefaultsToCloud(t *testing.T) {
	cfg := NewConfig()
	if cfg.RCA.IsLocal() {
		t.Error("RCA must default to the cloud provider")
	}
	if cfg.RCA.AutoInvestigate {
		t.Error("automatic investigations must be opt-in")
	}
	if cfg.RCA.Timeout <= 0 {
		t.Error("expected a default timeout")
	}
}
