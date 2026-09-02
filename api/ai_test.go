package api

import (
	"strings"
	"testing"

	"github.com/coroot/coroot/config"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/timeseries"
)

func TestInferUIProvider(t *testing.T) {
	cases := []struct {
		rca  config.RCA
		want string
	}{
		{config.RCA{}, ""},
		{config.RCA{Provider: config.RCAProviderCloud}, ""},
		{config.RCA{Provider: config.RCAProviderLocal, BaseUrl: "https://api.openai.com/v1"}, aiProviderOpenAI},
		{config.RCA{Provider: config.RCAProviderLocal, BaseUrl: "https://api.anthropic.com/v1/"}, aiProviderAnthropic},
		{config.RCA{Provider: config.RCAProviderLocal, BaseUrl: "https://openrouter.ai/api/v1"}, aiProviderOpenAICompatible},
	}
	for _, tc := range cases {
		if got := inferUIProvider(tc.rca); got != tc.want {
			t.Errorf("inferUIProvider(%+v) = %q, want %q", tc.rca, got, tc.want)
		}
	}
}

func TestRCAFromAIFormPreservesHiddenKey(t *testing.T) {
	current := config.RCA{
		Provider: config.RCAProviderLocal,
		BaseUrl:  "https://openrouter.ai/api/v1",
		ApiKey:   "sk-live",
		Model:    "z-ai/glm-5.2",
		Timeout:  timeseries.Minute,
	}
	got, err := rcaFromAIForm(AIForm{
		Provider: aiProviderOpenAICompatible,
		OpenAICompatible: AICompatibleForm{
			BaseUrl: "https://openrouter.ai/api/v1",
			ApiKey:  hiddenSecret,
			Model:   "z-ai/glm-5.2",
		},
	}, current)
	if err != nil {
		t.Fatal(err)
	}
	if got.ApiKey != "sk-live" {
		t.Errorf("hidden key was not preserved, got %q", got.ApiKey)
	}
	if !got.IsLocal() {
		t.Error("expected local provider")
	}
}

func TestRCAFromAIFormOpenAIDefaults(t *testing.T) {
	got, err := rcaFromAIForm(AIForm{
		Provider: aiProviderOpenAI,
		OpenAI:   AIKeyForm{ApiKey: "sk-openai"},
	}, config.RCA{Timeout: timeseries.Minute, AutoInvestigate: true, SystemPrompt: "keep me"})
	if err != nil {
		t.Fatal(err)
	}
	if got.BaseUrl != openAIBaseURL || got.Model != defaultOpenAI {
		t.Errorf("unexpected openai defaults: %+v", got)
	}
	if !got.AutoInvestigate || got.SystemPrompt != "keep me" {
		t.Error("UI save must keep timeout-adjacent RCA fields")
	}
}

func TestRCAFromAIFormDisable(t *testing.T) {
	got, err := rcaFromAIForm(AIForm{}, config.RCA{
		Provider: config.RCAProviderLocal,
		BaseUrl:  "https://openrouter.ai/api/v1",
		ApiKey:   "sk-live",
		Model:    "m",
		Timeout:  timeseries.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.IsLocal() || got.ApiKey != "" {
		t.Errorf("disable should clear the local provider, got %+v", got)
	}
}

func TestRCAFromAIFormRejectsUnknownAndMissingKey(t *testing.T) {
	if _, err := rcaFromAIForm(AIForm{Provider: "nope"}, config.RCA{}); err == nil {
		t.Error("expected unknown provider error")
	}
	if _, err := rcaFromAIForm(AIForm{Provider: aiProviderOpenAI}, config.RCA{}); err == nil || !strings.Contains(err.Error(), "API key") {
		t.Errorf("expected API key error, got %v", err)
	}
}

func TestApplyPersistedAIOverridesEnv(t *testing.T) {
	database, err := db.NewSqlite(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = database.Migrate(); err != nil {
		t.Fatal(err)
	}

	cfg := config.NewConfig()
	cfg.RCA = config.RCA{
		Provider: config.RCAProviderLocal,
		BaseUrl:  "https://openrouter.ai/api/v1",
		ApiKey:   "env-key",
		Model:    "env-model",
		Timeout:  timeseries.Minute,
	}
	api := &Api{cfg: cfg, db: database}
	api.applyPersistedAI()
	if api.cfg.RCA.Model != "env-model" {
		t.Fatalf("unset DB must keep env config, got %q", api.cfg.RCA.Model)
	}

	stored := storedAI{
		Provider: aiProviderOpenAICompatible,
		RCA: config.RCA{
			Provider: config.RCAProviderLocal,
			BaseUrl:  "https://example.test/v1",
			ApiKey:   "db-key",
			Model:    "db-model",
			Timeout:  timeseries.Minute,
		},
	}
	if err = database.SetSetting(AISettingName, stored); err != nil {
		t.Fatal(err)
	}
	api.applyPersistedAI()
	if api.cfg.RCA.Model != "db-model" || api.cfg.RCA.ApiKey != "db-key" {
		t.Fatalf("persisted settings should win, got %+v", api.cfg.RCA)
	}
}

func TestRCARunnerUpdateSwitchesClient(t *testing.T) {
	r := newRCARunner(localRCAConfig())
	if r.llm() == nil {
		t.Fatal("expected a client")
	}
	r.update(config.RCA{})
	if r.enabled() || r.llm() != nil {
		t.Error("disabling local RCA should drop the client")
	}
	r.update(localRCAConfig())
	if !r.enabled() || r.llm() == nil {
		t.Error("re-enabling local RCA should rebuild the client")
	}
}
