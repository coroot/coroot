package api

import (
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/utils"
	"k8s.io/klog"
)

type integrationIncident struct {
	Key           string `json:"key"`
	ApplicationId string `json:"application_id,omitempty"`
	Severity      string `json:"severity"`
	OpenedAt      int64  `json:"opened_at"`
	ShortSummary  string `json:"short_summary"`
	RCAStatus     string `json:"rca_status,omitempty"`
	Url           string `json:"url"`
}

func newIntegrationIncident(incident *model.ApplicationIncident, basePath, projectId string) integrationIncident {
	out := integrationIncident{
		Key:      incident.Key,
		Severity: incident.Severity.String(),
		OpenedAt: int64(incident.OpenedAt),
		// Falls back to a generated description ("High latency and errors") until the RCA lands.
		ShortSummary: incident.ShortDescription(),
		Url:          incidentUrl(basePath, projectId, incident.Key),
	}
	if incident.RCA != nil {
		out.RCAStatus = incident.RCA.Status
	}
	return out
}

// IntegrationAppIncident reports the currently open incident for a single application to
// trusted server-to-server callers such as Kubero, which hold the handoff secret but have
// no user session. It lets an external dashboard surface "this app is in trouble" without
// proxying a browser through Coroot.
//
// GET /api/integration/incident?project={projectId}&application={applicationId}
func (api *Api) IntegrationAppIncident(w http.ResponseWriter, r *http.Request) {
	if !api.checkHandoffSecret(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	q := r.URL.Query()
	projectId := strings.TrimSpace(q.Get("project"))
	applicationId := strings.TrimSpace(q.Get("application"))
	if projectId == "" || applicationId == "" {
		http.Error(w, "project and application are required", http.StatusBadRequest)
		return
	}

	appId, err := model.NewApplicationIdFromString(applicationId, projectId)
	if err != nil {
		klog.Warningln("invalid application id:", applicationId)
		http.Error(w, "invalid application id", http.StatusBadRequest)
		return
	}

	incident, err := api.db.GetLastOpenIncident(db.ProjectId(projectId), appId)
	if err != nil {
		klog.Errorln("failed to get incident:", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if incident == nil {
		utils.WriteJson(w, map[string]any{"incident": nil})
		return
	}

	utils.WriteJson(w, map[string]any{
		"incident": newIntegrationIncident(incident, api.cfg.UrlBasePath, projectId),
	})
}

// IntegrationProjectIncidents lists every open incident in a project for the same trusted
// callers. Kubero uses it to flag affected apps across its project/app list views without
// issuing one request per application.
//
// GET /api/integration/incidents?project={projectId}
func (api *Api) IntegrationProjectIncidents(w http.ResponseWriter, r *http.Request) {
	if !api.checkHandoffSecret(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	projectId := strings.TrimSpace(r.URL.Query().Get("project"))
	if projectId == "" {
		http.Error(w, "project is required", http.StatusBadRequest)
		return
	}

	incidents, err := api.db.GetOpenIncidents(db.ProjectId(projectId))
	if err != nil {
		klog.Errorln("failed to get incidents:", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	out := make([]integrationIncident, 0, len(incidents))
	for _, incident := range incidents {
		i := newIntegrationIncident(incident, api.cfg.UrlBasePath, projectId)
		i.ApplicationId = incident.ApplicationId.String()
		out = append(out, i)
	}
	utils.WriteJson(w, map[string]any{"incidents": out})
}

func incidentUrl(basePath, projectId, key string) string {
	if basePath == "" {
		basePath = "/"
	}
	p := path.Join(basePath, "p", projectId, "incidents")
	return p + "?" + url.Values{"incident": {key}}.Encode()
}
