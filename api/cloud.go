package api

import (
	"net/http"

	"github.com/coroot/coroot/api/forms"
	"github.com/coroot/coroot/cloud"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/rbac"
	"github.com/coroot/coroot/utils"
	"k8s.io/klog"
)

type CloudIntegrationForm struct {
	ApiKey                     string `json:"api_key"`
	IncidentsAutoInvestigation bool   `json:"incidents_auto_investigation"`
}

func (f *CloudIntegrationForm) Valid() bool {
	return true
}

func (api *Api) Cloud(w http.ResponseWriter, r *http.Request, u *db.User) {
	if !api.IsAllowed(u, rbac.Actions.Settings().Edit()) {
		http.Error(w, "You are not allowed to edit global settings.", http.StatusForbidden)
		return
	}

	cloudAPI := cloud.API(api.db, api.deploymentUuid, api.instanceUuid, r.Referer())
	settings, err := cloudAPI.GetSettings()
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	if query := r.URL.Query().Get("query"); query == "status" {
		status := "unconfigured"
		if settings.ApiKey != "" {
			status = "configured"
		}
		utils.WriteJson(w, map[string]string{"status": status})
		return
	}

	if r.Method == http.MethodPost {
		var form CloudIntegrationForm
		if err = forms.ReadAndValidate(r, &form); err != nil {
			klog.Warningln("bad request:", err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}
		settings.ApiKey = form.ApiKey
		settings.RCA.DisableIncidentsAutoInvestigation = !form.IncidentsAutoInvestigation
		if err = cloudAPI.SaveSettings(settings); err != nil {
			klog.Errorln(err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		return
	}

	info, err := cloudAPI.IntegrationInfo(r.Context())
	if err != nil {
		klog.Errorln(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	res := struct {
		Form CloudIntegrationForm  `json:"form"`
		Info cloud.IntegrationInfo `json:"info"`
	}{
		Info: *info,
	}
	res.Form.ApiKey = settings.ApiKey
	res.Form.IncidentsAutoInvestigation = !settings.RCA.DisableIncidentsAutoInvestigation
	utils.WriteJson(w, res)
}
