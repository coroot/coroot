package api

import (
	"errors"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/utils"
	"net/http"
	"net/url"
	"regexp"
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
