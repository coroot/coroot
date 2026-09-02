package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/coroot/coroot/api/forms"
	"github.com/coroot/coroot/config"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/rbac"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"k8s.io/klog"
)

const (
	AISettingName = "ai"

	hiddenSecret = "<hidden>"

	aiProviderAnthropic        = "anthropic"
	aiProviderOpenAI           = "openai"
	aiProviderOpenAICompatible = "openai_compatible"

	openAIBaseURL    = "https://api.openai.com/v1"
	anthropicBaseURL = "https://api.anthropic.com/v1"
	defaultOpenAI    = "gpt-4o"
	defaultAnthropic = "claude-sonnet-4-5"
)

type AIKeyForm struct {
	ApiKey string `json:"api_key"`
}

type AICompatibleForm struct {
	BaseUrl string `json:"base_url"`
	ApiKey  string `json:"api_key"`
	Model   string `json:"model"`
}

type AIForm struct {
	Provider         string           `json:"provider"`
	Anthropic        AIKeyForm        `json:"anthropic"`
	OpenAI           AIKeyForm        `json:"openai"`
	OpenAICompatible AICompatibleForm `json:"openai_compatible"`
}

func (f *AIForm) Valid() bool {
	switch f.Provider {
	case "", aiProviderAnthropic, aiProviderOpenAI, aiProviderOpenAICompatible:
		return true
	default:
		return false
	}
}

type storedAI struct {
	Provider string     `json:"provider"`
	RCA      config.RCA `json:"rca"`
}

type aiResponse struct {
	Readonly         bool             `json:"readonly"`
	Provider         string           `json:"provider"`
	Anthropic        AIKeyForm        `json:"anthropic"`
	OpenAI           AIKeyForm        `json:"openai"`
	OpenAICompatible AICompatibleForm `json:"openai_compatible"`
}

func (api *Api) applyPersistedAI() {
	if api.db == nil {
		return
	}
	stored, err := api.loadStoredAI()
	if err != nil {
		if !errors.Is(err, db.ErrNotFound) {
			klog.Errorln("failed to load AI settings:", err)
		}
		return
	}
	if err = stored.RCA.Validate(); err != nil {
		klog.Errorln("ignoring invalid persisted AI settings:", err)
		return
	}
	api.cfg.RCA = stored.RCA
}

func (api *Api) loadStoredAI() (storedAI, error) {
	var stored storedAI
	if err := api.db.GetSetting(AISettingName, &stored); err != nil {
		return stored, err
	}
	return stored, nil
}

func (api *Api) applyRCA(cfg config.RCA) {
	api.cfg.RCA = cfg
	if api.rca != nil {
		api.rca.update(cfg)
	}
}

func (api *Api) AI(w http.ResponseWriter, r *http.Request, u *db.User) {
	canEdit := api.IsAllowed(u, rbac.Actions.Settings().Edit())

	if r.Method == http.MethodPost {
		if !canEdit {
			http.Error(w, "You are not allowed to edit global settings.", http.StatusForbidden)
			return
		}
		var form AIForm
		if err := forms.ReadAndValidate(r, &form); err != nil {
			klog.Warningln("bad request:", err)
			http.Error(w, "Invalid AI settings.", http.StatusBadRequest)
			return
		}
		rca, err := rcaFromAIForm(form, api.effectiveRCA())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		stored := storedAI{Provider: form.Provider, RCA: rca}
		if err = api.db.SetSetting(AISettingName, stored); err != nil {
			klog.Errorln(err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		api.applyRCA(rca)
		return
	}

	utils.WriteJson(w, api.aiGET(canEdit))
}

func (api *Api) aiGET(canEdit bool) aiResponse {
	stored, err := api.loadStoredAI()
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		klog.Errorln(err)
	}
	rca := api.effectiveRCA()
	provider := stored.Provider
	if err != nil {
		provider = inferUIProvider(rca)
	}
	form := aiFormFromRCA(provider, rca)
	return aiResponse{
		Readonly:         !canEdit,
		Provider:         form.Provider,
		Anthropic:        form.Anthropic,
		OpenAI:           form.OpenAI,
		OpenAICompatible: form.OpenAICompatible,
	}
}

func (api *Api) effectiveRCA() config.RCA {
	if api.rca != nil {
		return api.rca.config()
	}
	return api.cfg.RCA
}

func inferUIProvider(rca config.RCA) string {
	if !rca.IsLocal() {
		return ""
	}
	switch strings.TrimRight(rca.BaseUrl, "/") {
	case openAIBaseURL:
		return aiProviderOpenAI
	case anthropicBaseURL:
		return aiProviderAnthropic
	default:
		return aiProviderOpenAICompatible
	}
}

func aiFormFromRCA(provider string, rca config.RCA) AIForm {
	if provider == "" {
		provider = inferUIProvider(rca)
	}
	f := AIForm{Provider: provider}
	switch provider {
	case aiProviderAnthropic:
		f.Anthropic.ApiKey = maskSecret(rca.ApiKey)
	case aiProviderOpenAI:
		f.OpenAI.ApiKey = maskSecret(rca.ApiKey)
	case aiProviderOpenAICompatible:
		f.OpenAICompatible.BaseUrl = rca.BaseUrl
		f.OpenAICompatible.ApiKey = maskSecret(rca.ApiKey)
		f.OpenAICompatible.Model = rca.Model
	}
	return f
}

func rcaFromAIForm(f AIForm, current config.RCA) (config.RCA, error) {
	rca := current
	if rca.Timeout <= 0 {
		rca.Timeout = 5 * timeseries.Minute
	}

	switch f.Provider {
	case "":
		rca.Provider = ""
		rca.BaseUrl = ""
		rca.ApiKey = ""
		rca.Model = ""
		return rca, nil
	case aiProviderAnthropic:
		rca.Provider = config.RCAProviderLocal
		rca.BaseUrl = anthropicBaseURL
		rca.ApiKey = revealSecret(f.Anthropic.ApiKey, current.ApiKey)
		if current.BaseUrl == anthropicBaseURL && current.Model != "" {
			rca.Model = current.Model
		} else {
			rca.Model = defaultAnthropic
		}
	case aiProviderOpenAI:
		rca.Provider = config.RCAProviderLocal
		rca.BaseUrl = openAIBaseURL
		rca.ApiKey = revealSecret(f.OpenAI.ApiKey, current.ApiKey)
		if current.BaseUrl == openAIBaseURL && current.Model != "" {
			rca.Model = current.Model
		} else {
			rca.Model = defaultOpenAI
		}
	case aiProviderOpenAICompatible:
		rca.Provider = config.RCAProviderLocal
		rca.BaseUrl = strings.TrimRight(f.OpenAICompatible.BaseUrl, "/")
		rca.ApiKey = revealSecret(f.OpenAICompatible.ApiKey, current.ApiKey)
		rca.Model = strings.TrimSpace(f.OpenAICompatible.Model)
	default:
		return rca, fmt.Errorf("unknown provider %q", f.Provider)
	}

	if rca.ApiKey == "" {
		return rca, fmt.Errorf("API key is required")
	}
	if err := rca.Validate(); err != nil {
		return rca, err
	}
	return rca, nil
}

func maskSecret(v string) string {
	if v == "" {
		return ""
	}
	return hiddenSecret
}

func revealSecret(submitted, current string) string {
	if submitted == "" || submitted == hiddenSecret {
		return current
	}
	return submitted
}
