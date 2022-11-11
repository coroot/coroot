package api

import (
	"errors"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/utils"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

var (
	ErrInvalidForm = errors.New("invalid form")

	slugRe = regexp.MustCompile("^[-_0-9a-z]{3,}$")
)

type Form interface {
	Valid() bool
}

func ReadAndValidate(r *http.Request, f Form) error {
	if err := utils.ReadJson(r, f); err != nil {
		return err
	}
	if !f.Valid() {
		return ErrInvalidForm
	}
	return nil
}

type ProjectForm struct {
	Name string `json:"name"`

	Prometheus db.Prometheus `json:"prometheus"`
}

func (f *ProjectForm) Valid() bool {
	if !slugRe.MatchString(f.Name) {
		return false
	}
	if _, err := url.Parse(f.Prometheus.Url); err != nil {
		return false
	}
	return true
}

type ProjectStatusForm struct {
	Mute   *model.ApplicationType `json:"mute"`
	UnMute *model.ApplicationType `json:"unmute"`
}

func (f *ProjectStatusForm) Valid() bool {
	return true
}

type CheckConfigForm struct {
	Configs []*model.CheckConfigSimple `json:"configs"`
}

func (f *CheckConfigForm) Valid() bool {
	return true
}

type CheckConfigSLOAvailabilityForm struct {
	Configs []model.CheckConfigSLOAvailability `json:"configs"`
	Empty   bool                               `json:"empty"`
}

func (f *CheckConfigSLOAvailabilityForm) Valid() bool {
	for _, c := range f.Configs {
		if c.TotalRequestsQuery == "" || c.FailedRequestsQuery == "" {
			return false
		}
	}
	return true
}

type CheckConfigSLOLatencyForm struct {
	Configs []model.CheckConfigSLOLatency `json:"configs"`
	Empty   bool                          `json:"empty"`
}

func (f *CheckConfigSLOLatencyForm) Valid() bool {
	for _, c := range f.Configs {
		if c.HistogramQuery == "" || c.ObjectiveBucket <= 0 {
			return false
		}
	}
	return true
}

type ApplicationCategoryForm struct {
	Name           model.ApplicationCategory `json:"name"`
	NewName        model.ApplicationCategory `json:"new_name"`
	CustomPatterns string                    `json:"custom_patterns"`
	customPatterns []string
}

func (f *ApplicationCategoryForm) Valid() bool {
	if !slugRe.MatchString(string(f.NewName)) {
		return false
	}
	f.customPatterns = strings.Fields(f.CustomPatterns)
	if !utils.GlobValidate(f.customPatterns) {
		return false
	}
	for _, p := range f.customPatterns {
		if strings.Count(p, "/") != 1 || strings.Index(p, "/") < 1 {
			return false
		}
	}
	return true
}

type IntegrationsForm struct {
	BaseUrl string `json:"base_url"`
}

func (f *IntegrationsForm) Valid() bool {
	if _, err := url.Parse(f.BaseUrl); err != nil || f.BaseUrl == "" {
		return false
	}
	f.BaseUrl = strings.TrimRight(f.BaseUrl, "/")
	return true
}

type IntegrationsSlackForm struct {
	Token   string `json:"token"`
	Channel string `json:"channel"`
	Enabled bool   `json:"enabled"`
}

func (f *IntegrationsSlackForm) Valid() bool {
	if f.Token == "" || f.Channel == "" {
		return false
	}

	return true
}
