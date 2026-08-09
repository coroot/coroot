package api

import (
	"errors"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/coroot/coroot/api/views"
	"github.com/coroot/coroot/auditor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
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

// parseNamespacesQuery splits a comma-separated namespaces= query into a set of
// lowercased namespace names. Empty input means "no restriction".
func parseNamespacesQuery(raw string) map[string]bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	out := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		ns := strings.ToLower(strings.TrimSpace(part))
		if ns != "" {
			out[ns] = true
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// worldWithNamespaces returns a shallow copy of w whose Applications map
// contains only apps in the given namespaces. nil/empty allowlist = all apps.
func worldWithNamespaces(w *model.World, namespaces map[string]bool) *model.World {
	if w == nil || len(namespaces) == 0 {
		return w
	}
	cp := *w
	cp.Applications = map[model.ApplicationId]*model.Application{}
	for id, app := range w.Applications {
		if namespaces[strings.ToLower(app.Id.Namespace)] {
			cp.Applications[id] = app
		}
	}
	return &cp
}

func (api *Api) integrationLoadProjectWorld(r *http.Request) (*db.Project, *model.World, error) {
	q := r.URL.Query()
	projectId := strings.TrimSpace(q.Get("project"))
	if projectId == "" {
		return nil, nil, errIntegrationBadRequest("project is required")
	}
	project, err := api.db.GetProject(db.ProjectId(projectId))
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, nil, errIntegrationBadRequest("project not found")
		}
		return nil, nil, err
	}
	from, to, _, truncated := api.getTimeContext(project.Id, q.Get("from"), q.Get("to"), "", "")
	world, _, err := api.LoadWorld(r.Context(), project, from, to)
	if err != nil {
		return nil, nil, err
	}
	if world == nil {
		step := increaseStepForBigDurations(from, to, 15*timeseries.Second)
		world = model.NewWorld(from, to.Add(-step), step, step)
	}
	world.Ctx.Truncated = truncated
	return project, world, nil
}

type integrationBadRequest string

func (e integrationBadRequest) Error() string { return string(e) }

func errIntegrationBadRequest(msg string) error { return integrationBadRequest(msg) }

// IntegrationOverviewApplications returns the Coroot applications overview for
// trusted S2S callers (Kubero Observability page). Optional namespaces= filters
// to those Kubernetes namespaces only.
//
// GET /api/integration/overview/applications?project=&from=&to=&namespaces=
func (api *Api) IntegrationOverviewApplications(w http.ResponseWriter, r *http.Request) {
	if !api.checkHandoffSecret(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	project, world, err := api.integrationLoadProjectWorld(r)
	if err != nil {
		var bad integrationBadRequest
		if errors.As(err, &bad) {
			http.Error(w, string(bad), http.StatusBadRequest)
			return
		}
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	namespaces := parseNamespacesQuery(r.URL.Query().Get("namespaces"))
	renderWorld := worldWithNamespaces(world, namespaces)
	auditor.Audit(renderWorld, project, nil, nil)

	chs := api.GetClickhouseClients(project)
	defer chs.Close()

	ov := views.Overview(r.Context(), chs, project, renderWorld, "applications", "", views.OverviewOpts{})
	utils.WriteJson(w, map[string]any{
		"applications": ov.Applications,
		"categories":   ov.Categories,
	})
}

// IntegrationOverviewLogs returns project-wide log search + histogram for
// trusted S2S callers (Kubero Logs page). When namespaces= is set, ClickHouse
// queries are restricted to services of apps in those namespaces.
//
// GET /api/integration/overview/logs?project=&from=&to=&query=&namespaces=
func (api *Api) IntegrationOverviewLogs(w http.ResponseWriter, r *http.Request) {
	if !api.checkHandoffSecret(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	project, world, err := api.integrationLoadProjectWorld(r)
	if err != nil {
		var bad integrationBadRequest
		if errors.As(err, &bad) {
			http.Error(w, string(bad), http.StatusBadRequest)
			return
		}
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	namespaces := parseNamespacesQuery(r.URL.Query().Get("namespaces"))
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if query == "" {
		query = `{"view":"messages","agent":true,"otel":true,"limit":100,"filters":[]}`
	}

	renderWorld := world
	restrictTelemetry := false
	if len(namespaces) > 0 {
		renderWorld = worldWithNamespaces(world, namespaces)
		restrictTelemetry = true
	}
	auditor.Audit(renderWorld, project, nil, nil)

	chs := api.GetClickhouseClients(project)
	defer chs.Close()
	if chs.Error != nil {
		klog.Warningln(chs.Error)
	}

	ov := views.Overview(r.Context(), chs, project, renderWorld, "logs", query, views.OverviewOpts{
		RestrictTelemetry: restrictTelemetry,
		FullWorld:         world,
	})
	utils.WriteJson(w, map[string]any{"logs": ov.Logs})
}

// IntegrationStatus returns collector health (prometheus / node agent /
// kube-state-metrics) for trusted S2S callers.
//
// GET /api/integration/status?project=
func (api *Api) IntegrationStatus(w http.ResponseWriter, r *http.Request) {
	if !api.checkHandoffSecret(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	projectId := strings.TrimSpace(r.URL.Query().Get("project"))
	if projectId == "" {
		http.Error(w, "project is required", http.StatusBadRequest)
		return
	}
	project, err := api.db.GetProject(db.ProjectId(projectId))
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			http.Error(w, "project not found", http.StatusBadRequest)
			return
		}
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	now := timeseries.Now()
	world, cacheStatus, err := api.LoadWorld(r.Context(), project, now.Add(-timeseries.Hour), now)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	utils.WriteJson(w, map[string]any{
		"status": renderStatus(project, cacheStatus, world, api.globalPrometheus),
	})
}
