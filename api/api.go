package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/coroot/coroot/api/forms"
	"github.com/coroot/coroot/api/views"
	"github.com/coroot/coroot/auditor"
	"github.com/coroot/coroot/cache"
	"github.com/coroot/coroot/clickhouse"
	pricing "github.com/coroot/coroot/cloud-pricing"
	"github.com/coroot/coroot/collector"
	"github.com/coroot/coroot/constructor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/rbac"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/gorilla/mux"
	"k8s.io/klog"
)

type Api struct {
	cache            *cache.Cache
	db               *db.DB
	collector        *collector.Collector
	pricing          *pricing.Manager
	roles            rbac.RoleManager
	globalClickHouse *db.IntegrationClickhouse
	globalPrometheus *db.IntegrationPrometheus

	authSecret        string
	authAnonymousRole rbac.RoleName
}

func NewApi(cache *cache.Cache, db *db.DB, collector *collector.Collector, pricing *pricing.Manager, roles rbac.RoleManager,
	globalClickHouse *db.IntegrationClickhouse, globalPrometheus *db.IntegrationPrometheus) *Api {
	return &Api{
		cache:            cache,
		db:               db,
		collector:        collector,
		pricing:          pricing,
		roles:            roles,
		globalClickHouse: globalClickHouse,
		globalPrometheus: globalPrometheus,
	}
}

func (api *Api) User(w http.ResponseWriter, r *http.Request, u *db.User) {
	if r.Method == http.MethodPost {
		if u.Anonymous {
			return
		}
		var form forms.ChangePasswordForm
		if err := forms.ReadAndValidate(r, &form); err != nil {
			klog.Warningln("bad request:", err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}
		if err := api.db.ChangeUserPassword(u.Id, form.OldPassword, form.NewPassword); err != nil {
			klog.Errorln(err)
			switch {
			case errors.Is(err, db.ErrNotFound):
				http.Error(w, "User not found.", http.StatusNotFound)
			case errors.Is(err, db.ErrInvalid):
				http.Error(w, "Invalid old password.", http.StatusBadRequest)
			case errors.Is(err, db.ErrConflict):
				http.Error(w, "New password can't be the same as the old one.", http.StatusBadRequest)
			default:
				http.Error(w, "", http.StatusInternalServerError)
			}
			return
		}
		return
	}

	projects, err := api.db.GetProjectNames()
	if err != nil {
		klog.Errorln("failed to get projects:", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	viewonly := !api.IsAllowed(u, rbac.Actions.Project("*").Settings().Edit())
	utils.WriteJson(w, views.User(u, projects, viewonly))
}

func (api *Api) Users(w http.ResponseWriter, r *http.Request, u *db.User) {
	if !api.IsAllowed(u, rbac.Actions.Users().Edit()) {
		http.Error(w, "You are not allowed to edit users.", http.StatusForbidden)
		return
	}

	roles, err := api.roles.GetRoles()
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	if r.Method == http.MethodPost {
		var form forms.UserForm
		if err := forms.ReadAndValidate(r, &form); err != nil {
			klog.Warningln("bad request:", err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}
		if form.Email == db.AdminUserLogin {
			return
		}
		switch form.Action {
		case forms.UserActionCreate:
			if !form.Role.Valid(roles) {
				http.Error(w, fmt.Sprintf("Unknown role: %s", form.Name), http.StatusBadRequest)
				return
			}
			if err := api.db.AddUser(form.Email, form.Password, form.Name, form.Role); err != nil {
				klog.Errorln(err)
				if errors.Is(err, db.ErrConflict) {
					http.Error(w, "The user is already added.", http.StatusConflict)
					return
				}
				http.Error(w, "", http.StatusInternalServerError)
				return
			}
		case forms.UserActionUpdate:
			if !form.Role.Valid(roles) {
				http.Error(w, fmt.Sprintf("Unknown role: %s", form.Name), http.StatusBadRequest)
				return
			}
			if err := api.db.UpdateUser(form.Id, form.Email, form.Password, form.Name, form.Role); err != nil {
				klog.Errorln(err)
				http.Error(w, "", http.StatusInternalServerError)
				return
			}
		case forms.UserActionDelete:
			if err := api.db.DeleteUser(form.Id); err != nil {
				klog.Errorln(err)
				http.Error(w, "", http.StatusInternalServerError)
				return
			}
		}
		return
	}

	users, err := api.db.GetUsers()
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	utils.WriteJson(w, views.Users(users, roles))
}

func (api *Api) Roles(w http.ResponseWriter, r *http.Request, u *db.User) {
	if r.Method == http.MethodPost {
		http.Error(w, "", http.StatusMethodNotAllowed)
		return
	}
	qaSample := rbac.NewRole("QA",
		rbac.NewPermission(rbac.ScopeProjectAll, rbac.ActionAll, rbac.Object{"project_id": "staging"}),
	)
	dbaSample := rbac.NewRole("DBA",
		rbac.NewPermission(rbac.ScopeProjectInstrumentations, rbac.ActionEdit, nil),
		rbac.NewPermission(rbac.ScopeApplication, rbac.ActionView, rbac.Object{"application_category": "databases"}),
		rbac.NewPermission(rbac.ScopeNode, rbac.ActionView, rbac.Object{"node_name": "db*"}),
	)
	roles, err := api.roles.GetRoles()
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	utils.WriteJson(w, views.Roles(append(roles, qaSample, dbaSample)))
}

func (api *Api) SSO(w http.ResponseWriter, r *http.Request, u *db.User) {
	roles, err := api.roles.GetRoles()
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	res := struct {
		Roles       []rbac.RoleName `json:"roles"`
		DefaultRole rbac.RoleName   `json:"default_role"`
	}{
		DefaultRole: rbac.RoleViewer,
	}
	for _, role := range roles {
		res.Roles = append(res.Roles, role.Name)
	}
	utils.WriteJson(w, res)
}

func (api *Api) Project(w http.ResponseWriter, r *http.Request, u *db.User) {
	vars := mux.Vars(r)
	projectId := vars["project"]

	isAllowed := api.IsAllowed(u, rbac.Actions.Project(projectId).Settings().Edit())

	switch r.Method {

	case http.MethodGet:
		type ProjectSettings struct {
			Readonly        bool                `json:"readonly"`
			Name            string              `json:"name"`
			ApiKeys         any                 `json:"api_keys"`
			RefreshInterval timeseries.Duration `json:"refresh_interval"`
		}
		res := ProjectSettings{}
		if projectId != "" {
			project, err := api.db.GetProject(db.ProjectId(projectId))
			if err != nil {
				if errors.Is(err, db.ErrNotFound) {
					klog.Warningln("project not found:", projectId)
					return
				}
				klog.Errorln("failed to get project:", err)
				http.Error(w, "", http.StatusInternalServerError)
				return
			}
			prometheusCfg := project.PrometheusConfig(api.globalPrometheus)
			res.Readonly = !project.Settings.Configurable
			res.Name = project.Name
			res.RefreshInterval = prometheusCfg.RefreshInterval
			if isAllowed {
				res.ApiKeys = project.Settings.ApiKeys
			} else {
				res.ApiKeys = "permission denied"
			}
		}
		utils.WriteJson(w, res)

	case http.MethodPost:
		if !isAllowed {
			http.Error(w, "You are not allowed to configure the project.", http.StatusForbidden)
			return
		}
		var form forms.ProjectForm
		if err := forms.ReadAndValidate(r, &form); err != nil {
			klog.Warningln("bad request:", err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}
		isNew := projectId == ""
		project := &db.Project{
			Id:   db.ProjectId(projectId),
			Name: form.Name,
		}
		project.Settings.Configurable = true
		err := api.db.SaveProject(project)
		if err != nil {
			if errors.Is(err, db.ErrConflict) {
				http.Error(w, "This project name is already being used.", http.StatusConflict)
				return
			}
			klog.Errorln("failed to save project:", err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		if isNew && api.globalClickHouse != nil {
			err = api.collector.MigrateClickhouseDatabase(r.Context(), project)
			if err != nil {
				klog.Errorln("failed to migrate clickhouse database:", err)
				http.Error(w, "Failed to create or update clickhouse database", http.StatusInternalServerError)
				return
			}
		}
		http.Error(w, string(project.Id), http.StatusOK)

	case http.MethodDelete:
		if !isAllowed {
			http.Error(w, "You are not allowed to delete the project.", http.StatusForbidden)
			return
		}
		if err := api.db.DeleteProject(db.ProjectId(projectId)); err != nil {
			klog.Errorln("failed to delete project:", err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		http.Error(w, "", http.StatusOK)

	default:
		http.Error(w, "", http.StatusMethodNotAllowed)
	}
}

func (api *Api) Status(w http.ResponseWriter, r *http.Request, u *db.User) {
	projectId := db.ProjectId(mux.Vars(r)["project"])
	project, err := api.db.GetProject(projectId)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			klog.Warningln("project not found:", projectId)
			utils.WriteJson(w, Status{})
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
	utils.WriteJson(w, renderStatus(project, cacheStatus, world, api.globalPrometheus))
}

func (api *Api) Overview(w http.ResponseWriter, r *http.Request, u *db.User) {
	vars := mux.Vars(r)
	projectId := vars["project"]
	view := vars["view"]

	switch view {
	case "traces":
		if !api.IsAllowed(u, rbac.Actions.Project(projectId).Traces().View()) {
			http.Error(w, "You are not allowed to view traces.", http.StatusForbidden)
			return
		}
	case "costs":
		if !api.IsAllowed(u, rbac.Actions.Project(projectId).Costs().View()) {
			http.Error(w, "You are not allowed to view costs.", http.StatusForbidden)
			return
		}
	case "risks":
		if !api.IsAllowed(u, rbac.Actions.Project(projectId).Risks().View()) {
			http.Error(w, "You are not allowed to view risks.", http.StatusForbidden)
			return
		}
	}

	world, project, cacheStatus, err := api.LoadWorldByRequest(r)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if project == nil || world == nil {
		utils.WriteJson(w, api.WithContext(project, cacheStatus, world, nil))
		return
	}
	var ch *clickhouse.Client
	if ch, err = api.getClickhouseClient(project); err != nil {
		klog.Warningln(err)
	}
	auditor.Audit(world, project, nil, project.ClickHouseConfig(api.globalClickHouse) != nil)
	utils.WriteJson(w, api.WithContext(project, cacheStatus, world, views.Overview(r.Context(), ch, world, view, r.URL.Query().Get("query"))))
}

func (api *Api) ApiKeys(w http.ResponseWriter, r *http.Request, u *db.User) {
	vars := mux.Vars(r)
	projectId := vars["project"]

	project, err := api.db.GetProject(db.ProjectId(projectId))
	if err != nil {
		klog.Errorln("failed to get project:", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	isAllowed := api.IsAllowed(u, rbac.Actions.Project(projectId).Settings().Edit())

	if r.Method == http.MethodGet {
		res := struct {
			Editable bool        `json:"editable"`
			Keys     []db.ApiKey `json:"keys"`
		}{
			Editable: isAllowed && project.Settings.Configurable,
			Keys:     project.Settings.ApiKeys,
		}
		if !isAllowed {
			for i := range res.Keys {
				res.Keys[i].Key = ""
			}
		}
		utils.WriteJson(w, res)
		return
	}

	if !isAllowed {
		http.Error(w, "You are not allowed to configure API keys.", http.StatusForbidden)
		return
	}
	var form forms.ApiKeyForm
	if err = forms.ReadAndValidate(r, &form); err != nil {
		klog.Warningln("bad request:", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	switch form.Action {
	case "generate":
		form.Key = utils.RandomString(32)
		project.Settings.ApiKeys = append(project.Settings.ApiKeys, form.ApiKey)
	case "delete":
		project.Settings.ApiKeys = slices.DeleteFunc(project.Settings.ApiKeys, func(k db.ApiKey) bool {
			return k.Key == form.Key
		})
	case "edit":
		for i, k := range project.Settings.ApiKeys {
			if k.Key == form.Key {
				project.Settings.ApiKeys[i].Description = form.Description
			}
		}
	default:
		return
	}
	if err = api.db.SaveProjectSettings(project); err != nil {
		klog.Errorln("failed to save project api keys:", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
}

func (api *Api) Inspections(w http.ResponseWriter, r *http.Request, u *db.User) {
	vars := mux.Vars(r)
	projectId := vars["project"]
	checkConfigs, err := api.db.GetCheckConfigs(db.ProjectId(projectId))
	if err != nil {
		klog.Errorln("failed to get check configs:", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	utils.WriteJson(w, views.Inspections(checkConfigs))
}

func (api *Api) Categories(w http.ResponseWriter, r *http.Request, u *db.User) {
	vars := mux.Vars(r)
	projectId := vars["project"]

	if r.Method == http.MethodPost {
		if !api.IsAllowed(u, rbac.Actions.Project(projectId).ApplicationCategories().Edit()) {
			http.Error(w, "You are not allowed to configure application categories.", http.StatusForbidden)
			return
		}
		var form forms.ApplicationCategoryForm
		if err := forms.ReadAndValidate(r, &form); err != nil {
			klog.Warningln("bad request:", err)
			http.Error(w, "Invalid name or patterns", http.StatusBadRequest)
			return
		}
		if err := api.db.SaveApplicationCategory(db.ProjectId(projectId), form.Name, form.NewName, form.CustomPatterns, form.NotifyOfDeployments); err != nil {
			klog.Errorln("failed to save:", err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		return
	}

	p, err := api.db.GetProject(db.ProjectId(projectId))
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	utils.WriteJson(w, views.Categories(p))
}

func (api *Api) CustomApplications(w http.ResponseWriter, r *http.Request, u *db.User) {
	vars := mux.Vars(r)
	projectId := vars["project"]

	if r.Method == http.MethodPost {
		if !api.IsAllowed(u, rbac.Actions.Project(projectId).CustomApplications().Edit()) {
			http.Error(w, "You are not allowed to configure custom applications.", http.StatusForbidden)
			return
		}
		var form forms.CustomApplicationForm
		if err := forms.ReadAndValidate(r, &form); err != nil {
			klog.Warningln("bad request:", err)
			http.Error(w, "Invalid name or patterns", http.StatusBadRequest)
			return
		}
		if err := api.db.SaveCustomApplication(db.ProjectId(projectId), form.Name, form.NewName, form.InstancePatterns); err != nil {
			klog.Errorln("failed to save:", err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		return
	}
	p, err := api.db.GetProject(db.ProjectId(projectId))
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	utils.WriteJson(w, views.CustomApplications(p))
}

func (api *Api) Integrations(w http.ResponseWriter, r *http.Request, u *db.User) {
	vars := mux.Vars(r)
	projectId := vars["project"]

	if r.Method == http.MethodPut {
		if !api.IsAllowed(u, rbac.Actions.Project(projectId).Integrations().Edit()) {
			http.Error(w, "You are not allowed to configure notification integrations.", http.StatusForbidden)
			return
		}
		var form forms.IntegrationsForm
		if err := forms.ReadAndValidate(r, &form); err != nil {
			klog.Warningln("bad request:", err)
			http.Error(w, "Invalid base url", http.StatusBadRequest)
			return
		}
		if err := api.db.SaveIntegrationsBaseUrl(db.ProjectId(projectId), form.BaseUrl); err != nil {
			klog.Errorln("failed to save:", err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		return
	}

	p, err := api.db.GetProject(db.ProjectId(projectId))
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	utils.WriteJson(w, views.Integrations(p))
}

func (api *Api) Integration(w http.ResponseWriter, r *http.Request, u *db.User) {
	vars := mux.Vars(r)
	projectId := vars["project"]
	project, err := api.db.GetProject(db.ProjectId(projectId))
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	t := db.IntegrationType(vars["type"])
	form := forms.NewIntegrationForm(t, api.globalClickHouse, api.globalPrometheus)
	if form == nil {
		klog.Warningln("unknown integration type:", t)
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	isAllowed := api.IsAllowed(u, rbac.Actions.Project(projectId).Integrations().Edit())

	if r.Method == http.MethodGet {
		form.Get(project, !isAllowed)
		switch t {
		case db.IntegrationTypeAWS:
			world, _, _, err := api.LoadWorldByRequest(r)
			if err != nil {
				klog.Errorln(err)
			}
			utils.WriteJson(w, struct {
				Form forms.IntegrationForm `json:"form"`
				View any                   `json:"view"`
			}{
				Form: form,
				View: views.AWS(world),
			})
		default:
			utils.WriteJson(w, form)
		}
		return
	}

	if !isAllowed {
		http.Error(w, "You are not allowed to configure integrations.", http.StatusForbidden)
		return
	}

	switch r.Method {
	case http.MethodPost, http.MethodPut:
		if err := forms.ReadAndValidate(r, form); err != nil {
			klog.Warningln("bad request:", err)
			http.Error(w, "invalid data", http.StatusBadRequest)
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	switch r.Method {
	case http.MethodPost:
		if err := form.Test(ctx, project); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		return
	case http.MethodPut:
		if err := form.Update(ctx, project, false); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	case http.MethodDelete:
		if err := form.Update(ctx, project, true); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	default:
		http.Error(w, "", http.StatusMethodNotAllowed)
		return
	}

	if err := api.db.SaveProjectIntegration(project, t); err != nil {
		klog.Errorln("failed to save:", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if api.globalClickHouse == nil {
		err = api.collector.UpdateClickhouseClient(r.Context(), project)
		if err != nil {
			klog.Errorln("clickhouse error:", err)
			http.Error(w, "Clickhouse error: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func (api *Api) Prom(w http.ResponseWriter, r *http.Request, u *db.User) {
	vars := mux.Vars(r)
	projectId := vars["project"]
	project, err := api.db.GetProject(db.ProjectId(projectId))
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	p := project.Prometheus
	cfg := prom.NewClientConfig(p.Url, p.RefreshInterval)
	cfg.BasicAuth = p.BasicAuth
	cfg.TlsSkipVerify = p.TlsSkipVerify
	cfg.ExtraSelector = p.ExtraSelector
	cfg.CustomHeaders = p.CustomHeaders
	c, err := prom.NewClient(cfg)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	c.Proxy(r, w)
}

func (api *Api) Application(w http.ResponseWriter, r *http.Request, u *db.User) {
	vars := mux.Vars(r)
	projectId := vars["project"]
	appId, err := model.NewApplicationIdFromString(vars["app"])
	if err != nil {
		klog.Warningln(err)
		http.Error(w, "invalid application id: "+vars["app"], http.StatusBadRequest)
		return
	}
	world, project, cacheStatus, err := api.LoadWorldByRequest(r)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if project == nil || world == nil {
		utils.WriteJson(w, api.WithContext(project, cacheStatus, world, nil))
		return
	}
	app := world.GetApplication(appId)
	if app == nil {
		klog.Warningln("application not found:", appId)
		http.Error(w, "Application not found", http.StatusNotFound)
		return
	}

	if !api.IsAllowed(u, rbac.Actions.Project(projectId).Application(app.Category, app.Id.Namespace, app.Id.Kind, app.Id.Name).View()) {
		http.Error(w, "You are not allowed to view this application.", http.StatusForbidden)
		return
	}

	auditor.Audit(world, project, app, project.ClickHouseConfig(api.globalClickHouse) != nil)

	if project.ClickHouseConfig(api.globalClickHouse) != nil {
		app.AddReport(model.AuditReportProfiling, &model.Widget{Profiling: &model.Profiling{ApplicationId: app.Id}, Width: "100%"})
		app.AddReport(model.AuditReportTracing, &model.Widget{Tracing: &model.Tracing{ApplicationId: app.Id}, Width: "100%"})
	}
	utils.WriteJson(w, api.WithContext(project, cacheStatus, world, views.Application(world, app)))
}

func (api *Api) RCA(w http.ResponseWriter, r *http.Request, u *db.User) {
	world, project, cacheStatus, err := api.LoadWorldByRequest(r)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if project == nil || world == nil {
		utils.WriteJson(w, api.WithContext(project, cacheStatus, world, nil))
		return
	}
	utils.WriteJson(w, api.WithContext(project, cacheStatus, world, "not implemented"))
}

func (api *Api) Incident(w http.ResponseWriter, r *http.Request, u *db.User) {
	vars := mux.Vars(r)
	projectId := vars["project"]
	incidentKey := vars["incident"]
	incident, err := api.db.GetIncidentByKey(db.ProjectId(projectId), incidentKey)
	if err != nil {
		klog.Warningln("failed to get incident:", err)
		http.Error(w, "failed to get incident", http.StatusInternalServerError)
		return
	}
	if incident == nil {
		klog.Warningln("incident not found:", vars["key"])
		http.Error(w, "Incident not found", http.StatusNotFound)
		return
	}
	values := r.URL.Query()
	values.Add("incident", incidentKey)
	r.URL.RawQuery = values.Encode()

	world, project, cacheStatus, err := api.LoadWorldByRequest(r)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if project == nil || world == nil {
		utils.WriteJson(w, api.WithContext(project, cacheStatus, world, nil))
		return
	}
	app := world.GetApplication(incident.ApplicationId)
	if app == nil {
		klog.Warningln("application not found:", incident.ApplicationId)
		http.Error(w, "Application not found", http.StatusNotFound)
		return
	}
	if !api.IsAllowed(u, rbac.Actions.Project(projectId).Application(app.Category, app.Id.Namespace, app.Id.Kind, app.Id.Name).View()) {
		http.Error(w, "You are not allowed to view this application.", http.StatusForbidden)
		return
	}
	auditor.Audit(world, project, app, project.ClickHouseConfig(api.globalClickHouse) != nil)
	utils.WriteJson(w, api.WithContext(project, cacheStatus, world, views.Incident(world, app, incident)))
}

func (api *Api) Inspection(w http.ResponseWriter, r *http.Request, u *db.User) {
	vars := mux.Vars(r)
	projectId := vars["project"]
	appId, err := model.NewApplicationIdFromString(vars["app"])
	if err != nil {
		klog.Warningln(err)
		http.Error(w, "invalid application id: "+vars["app"], http.StatusBadRequest)
		return
	}
	checkId := model.CheckId(vars["type"])

	switch r.Method {
	case http.MethodGet:
		project, err := api.db.GetProject(db.ProjectId(projectId))
		if err != nil {
			klog.Errorln("failed to get project:", err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		checkConfigs, err := api.db.GetCheckConfigs(db.ProjectId(projectId))
		if err != nil {
			klog.Errorln("failed to get check configs:", err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		res := struct {
			Form         any               `json:"form"`
			Integrations map[string]string `json:"integrations"`
		}{
			Integrations: map[string]string{},
		}
		for _, i := range project.Settings.Integrations.GetInfo() {
			if i.Configured && i.Incidents {
				res.Integrations[i.Title] = i.Details
			}
		}
		switch checkId {
		case model.Checks.SLOAvailability.Id:
			cfg, def := checkConfigs.GetAvailability(appId)
			res.Form = forms.CheckConfigSLOAvailabilityForm{Configs: []model.CheckConfigSLOAvailability{cfg}, Default: def}
		case model.Checks.SLOLatency.Id:
			cfg, def := checkConfigs.GetLatency(appId, model.CalcApplicationCategory(appId, project.Settings.ApplicationCategories))
			res.Form = forms.CheckConfigSLOLatencyForm{Configs: []model.CheckConfigSLOLatency{cfg}, Default: def}
		default:
			form := forms.CheckConfigForm{
				Configs: checkConfigs.GetSimpleAll(checkId, appId),
			}
			if len(form.Configs) == 0 {
				http.Error(w, "", http.StatusNotFound)
				return
			}
			res.Form = form
		}
		utils.WriteJson(w, res)
		return

	case http.MethodPost:
		if !api.IsAllowed(u, rbac.Actions.Project(projectId).Inspections().Edit()) {
			http.Error(w, "You are not allowed to configure inspections.", http.StatusForbidden)
			return
		}
		switch checkId {
		case model.Checks.SLOAvailability.Id:
			var form forms.CheckConfigSLOAvailabilityForm
			if err := forms.ReadAndValidate(r, &form); err != nil {
				klog.Warningln("bad request:", err)
				http.Error(w, "", http.StatusBadRequest)
				return
			}
			if err := api.db.SaveCheckConfig(db.ProjectId(projectId), appId, checkId, form.Configs); err != nil {
				klog.Errorln("failed to save check config:", err)
				http.Error(w, "", http.StatusInternalServerError)
				return
			}
		case model.Checks.SLOLatency.Id:
			var form forms.CheckConfigSLOLatencyForm
			if err := forms.ReadAndValidate(r, &form); err != nil {
				klog.Warningln("bad request:", err)
				http.Error(w, "", http.StatusBadRequest)
				return
			}
			if err := api.db.SaveCheckConfig(db.ProjectId(projectId), appId, checkId, form.Configs); err != nil {
				klog.Errorln("failed to save check config:", err)
				http.Error(w, "", http.StatusInternalServerError)
				return
			}
		default:
			var form forms.CheckConfigForm
			if err := forms.ReadAndValidate(r, &form); err != nil {
				klog.Warningln("bad request:", err)
				http.Error(w, "", http.StatusBadRequest)
				return
			}
			for level, cfg := range form.Configs {
				var id model.ApplicationId
				switch level {
				case 0:
					continue
				case 1:
					id = model.ApplicationIdZero
				case 2:
					id = appId
				}
				if err := api.db.SaveCheckConfig(db.ProjectId(projectId), id, checkId, cfg); err != nil {
					klog.Errorln("failed to save check config:", err)
					http.Error(w, "", http.StatusInternalServerError)
					return
				}
			}
			return
		}
	}
}

func (api *Api) Instrumentation(w http.ResponseWriter, r *http.Request, u *db.User) {
	vars := mux.Vars(r)
	projectId := vars["project"]
	appId, err := model.NewApplicationIdFromString(vars["app"])
	if err != nil {
		klog.Warningln(err)
		http.Error(w, "invalid application id: "+vars["app"], http.StatusBadRequest)
		return
	}
	world, _, _, err := api.LoadWorldByRequest(r)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if world == nil {
		http.Error(w, "Application not found", http.StatusNotFound)
		return
	}
	app := world.GetApplication(appId)
	if app == nil {
		klog.Warningln("application not found:", appId)
		http.Error(w, "Application not found", http.StatusNotFound)
		return
	}

	isAllowed := api.IsAllowed(u, rbac.Actions.Project(projectId).Instrumentations().Edit())

	if r.Method == http.MethodPost {
		if !isAllowed {
			http.Error(w, "You are not allowed to configure database integrations.", http.StatusForbidden)
			return
		}
		var form forms.ApplicationInstrumentationForm
		if err = forms.ReadAndValidate(r, &form); err != nil {
			klog.Warningln("bad request:", err)
			http.Error(w, "invalid data", http.StatusBadRequest)
			return
		}
		if err = api.db.SaveApplicationSetting(db.ProjectId(projectId), appId, &form.ApplicationInstrumentation); err != nil {
			klog.Errorln(err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		return
	}

	t := model.ApplicationType(vars["type"]).InstrumentationType()
	var instrumentation *model.ApplicationInstrumentation
	if app.Settings != nil && app.Settings.Instrumentation != nil && app.Settings.Instrumentation[t] != nil {
		instrumentation = app.Settings.Instrumentation[t]
	} else {
		instrumentation = model.GetDefaultInstrumentation(t)
	}
	if instrumentation == nil {
		http.Error(w, fmt.Sprintf("unsupported instrumentation type: %s", t), http.StatusBadRequest)
		return
	}
	if !isAllowed {
		instrumentation.Credentials.Username = "<hidden>"
		instrumentation.Credentials.Password = "<hidden>"
	}
	utils.WriteJson(w, instrumentation)
}

func (api *Api) Profiling(w http.ResponseWriter, r *http.Request, u *db.User) {
	vars := mux.Vars(r)
	projectId := vars["project"]
	appId, err := model.NewApplicationIdFromString(vars["app"])
	if err != nil {
		klog.Warningln(err)
		http.Error(w, "invalid application id: "+vars["app"], http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodPost {
		if !api.IsAllowed(u, rbac.Actions.Project(projectId).Inspections().Edit()) {
			http.Error(w, "You are not allowed to configure profiling settings.", http.StatusForbidden)
			return
		}
		var form forms.ApplicationSettingsProfilingForm
		if err := forms.ReadAndValidate(r, &form); err != nil {
			klog.Warningln("bad request:", err)
			http.Error(w, "invalid data", http.StatusBadRequest)
			return
		}
		if err := api.db.SaveApplicationSetting(db.ProjectId(projectId), appId, &form.ApplicationSettingsProfiling); err != nil {
			klog.Errorln(err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		return
	}

	world, project, cacheStatus, err := api.LoadWorldByRequest(r)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if project == nil || world == nil {
		utils.WriteJson(w, api.WithContext(project, cacheStatus, world, nil))
		return
	}
	app := world.GetApplication(appId)
	if app == nil {
		klog.Warningln("application not found:", appId)
		http.Error(w, "Application not found", http.StatusNotFound)
		return
	}
	var ch *clickhouse.Client
	if ch, err = api.getClickhouseClient(project); err != nil {
		klog.Warningln(err)
		http.Error(w, "ClickHouse is not available", http.StatusInternalServerError)
		return
	}
	q := r.URL.Query()
	auditor.Audit(world, project, nil, project.ClickHouseConfig(api.globalClickHouse) != nil)
	utils.WriteJson(w, api.WithContext(project, cacheStatus, world, views.Profiling(r.Context(), ch, app, q, world.Ctx)))
}

func (api *Api) Tracing(w http.ResponseWriter, r *http.Request, u *db.User) {
	vars := mux.Vars(r)
	projectId := vars["project"]
	appId, err := model.NewApplicationIdFromString(vars["app"])
	if err != nil {
		klog.Warningln(err)
		http.Error(w, "invalid application id: "+vars["app"], http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodPost {
		if !api.IsAllowed(u, rbac.Actions.Project(projectId).Inspections().Edit()) {
			http.Error(w, "You are not allowed to configure tracing settings.", http.StatusForbidden)
			return
		}
		var form forms.ApplicationSettingsTracingForm
		if err := forms.ReadAndValidate(r, &form); err != nil {
			klog.Warningln("bad request:", err)
			http.Error(w, "invalid data", http.StatusBadRequest)
			return
		}
		if err := api.db.SaveApplicationSetting(db.ProjectId(projectId), appId, &form.ApplicationSettingsTracing); err != nil {
			klog.Errorln(err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		return
	}

	world, project, cacheStatus, err := api.LoadWorldByRequest(r)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if project == nil || world == nil {
		utils.WriteJson(w, api.WithContext(project, cacheStatus, world, nil))
		return
	}
	app := world.GetApplication(appId)
	if app == nil {
		klog.Warningln("application not found:", appId)
		http.Error(w, "Application not found", http.StatusNotFound)
		return
	}
	q := r.URL.Query()
	var ch *clickhouse.Client
	if ch, err = api.getClickhouseClient(project); err != nil {
		klog.Warningln(err)
		http.Error(w, "ClickHouse is not available", http.StatusInternalServerError)
		return
	}
	auditor.Audit(world, project, nil, project.ClickHouseConfig(api.globalClickHouse) != nil)
	utils.WriteJson(w, api.WithContext(project, cacheStatus, world, views.Tracing(r.Context(), ch, app, q, world)))
}

func (api *Api) Logs(w http.ResponseWriter, r *http.Request, u *db.User) {
	vars := mux.Vars(r)
	projectId := vars["project"]
	appId, err := model.NewApplicationIdFromString(vars["app"])
	if err != nil {
		klog.Warningln(err)
		http.Error(w, "invalid application id: "+vars["app"], http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodPost {
		if !api.IsAllowed(u, rbac.Actions.Project(projectId).Inspections().Edit()) {
			http.Error(w, "You are not allowed to configure logs settings.", http.StatusForbidden)
			return
		}
		var form forms.ApplicationSettingsLogsForm
		if err := forms.ReadAndValidate(r, &form); err != nil {
			klog.Warningln("bad request:", err)
			http.Error(w, "invalid data", http.StatusBadRequest)
			return
		}
		if err := api.db.SaveApplicationSetting(db.ProjectId(projectId), appId, &form.ApplicationSettingsLogs); err != nil {
			klog.Errorln(err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		return
	}

	world, project, cacheStatus, err := api.LoadWorldByRequest(r)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if project == nil || world == nil {
		utils.WriteJson(w, api.WithContext(project, cacheStatus, world, nil))
		return
	}
	app := world.GetApplication(appId)
	if app == nil {
		klog.Warningln("application not found:", appId)
		http.Error(w, "Application not found", http.StatusNotFound)
		return
	}
	ch, chErr := api.getClickhouseClient(project)
	if chErr != nil {
		klog.Warningln(chErr)
	}
	auditor.Audit(world, project, nil, project.ClickHouseConfig(api.globalClickHouse) != nil)
	q := r.URL.Query()
	res := views.Logs(r.Context(), ch, app, q, world)
	if chErr != nil {
		res.Message = "Failed to load logs: ClickHouse is not available"
	}
	utils.WriteJson(w, api.WithContext(project, cacheStatus, world, res))
}

func (api *Api) Risks(w http.ResponseWriter, r *http.Request, u *db.User) {
	vars := mux.Vars(r)
	projectId := vars["project"]
	appId, err := model.NewApplicationIdFromString(vars["app"])
	if err != nil {
		klog.Warningln(err)
		http.Error(w, "invalid application id: "+vars["app"], http.StatusBadRequest)
		return
	}

	world, project, cacheStatus, err := api.LoadWorldByRequest(r)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if project == nil || world == nil {
		utils.WriteJson(w, api.WithContext(project, cacheStatus, world, nil))
		return
	}
	app := world.GetApplication(appId)
	if app == nil {
		klog.Warningln("application not found:", appId)
		http.Error(w, "Application not found", http.StatusNotFound)
		return
	}
	if r.Method == http.MethodPost {
		if !api.IsAllowed(u, rbac.Actions.Project(projectId).Risks().Edit()) {
			http.Error(w, "You are not allowed to dismiss risks.", http.StatusForbidden)
			return
		}
		var form forms.ApplicationSettingsRisksForm
		if err := forms.ReadAndValidate(r, &form); err != nil {
			klog.Warningln("bad request:", err)
			http.Error(w, "invalid data", http.StatusBadRequest)
			return
		}
		var overrides []model.RiskOverride
		switch form.Action {
		case "dismiss":
			newRo := model.RiskOverride{
				Key: form.Key,
				Dismissal: &model.RiskDismissal{
					By:        u.Name,
					Timestamp: time.Now().Unix(),
					Reason:    form.Reason,
				},
			}
			if app.Settings != nil {
				for _, ro := range app.Settings.RiskOverrides {
					if ro.Key != form.Key {
						overrides = append(overrides, ro)
					}
				}
			}
			overrides = append(overrides, newRo)
		case "mark_as_active":
			if app.Settings != nil {
				for _, ro := range app.Settings.RiskOverrides {
					if ro.Key != form.Key {
						overrides = append(overrides, ro)
					}
				}
			}
		default:
			klog.Warningln("unknown risk action:", form.Action)
			http.Error(w, "", http.StatusBadRequest)
			return
		}
		if err = api.db.SaveApplicationSetting(db.ProjectId(projectId), appId, overrides); err != nil {
			klog.Errorln(err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		return
	}
}

func (api *Api) Node(w http.ResponseWriter, r *http.Request, u *db.User) {
	vars := mux.Vars(r)
	projectId := vars["project"]
	nodeName := vars["node"]
	if !api.IsAllowed(u, rbac.Actions.Project(projectId).Node(nodeName).View()) {
		http.Error(w, "You are not allowed to view this node.", http.StatusForbidden)
		return
	}
	world, project, cacheStatus, err := api.LoadWorldByRequest(r)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if project == nil || world == nil {
		utils.WriteJson(w, api.WithContext(project, cacheStatus, world, nil))
		return
	}
	node := world.GetNode(nodeName)
	if node == nil {
		klog.Warningf("node not found: %s ", nodeName)
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}
	auditor.Audit(world, project, nil, project.ClickHouseConfig(api.globalClickHouse) != nil)
	utils.WriteJson(w, api.WithContext(project, cacheStatus, world, auditor.AuditNode(world, node)))
}

func (api *Api) LoadWorld(ctx context.Context, project *db.Project, from, to timeseries.Time) (*model.World, *cache.Status, error) {
	cacheClient := api.cache.GetCacheClient(project.Id)

	cacheStatus, err := cacheClient.GetStatus()
	if err != nil {
		return nil, nil, err
	}

	cacheTo, err := cacheClient.GetTo()
	if err != nil {
		return nil, cacheStatus, err
	}

	if cacheTo.IsZero() || cacheTo.Before(from) {
		return nil, cacheStatus, nil
	}

	step, err := cacheClient.GetStep(from, to)
	if err != nil {
		return nil, cacheStatus, err
	}

	duration := to.Sub(from)
	if cacheTo.Before(to) {
		to = cacheTo
		duration = to.Sub(from)
	}
	step = increaseStepForBigDurations(duration, step)

	ctr := constructor.New(api.db, project, cacheClient, api.pricing)
	world, err := ctr.LoadWorld(ctx, from, to, step, nil)
	return world, cacheStatus, err
}

func (api *Api) LoadWorldByRequest(r *http.Request) (*model.World, *db.Project, *cache.Status, error) {
	projectId := db.ProjectId(mux.Vars(r)["project"])
	project, err := api.db.GetProject(projectId)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			klog.Warningln("project not found:", projectId)
			return nil, nil, nil, nil
		}
		return nil, nil, nil, err
	}

	now := timeseries.Now()
	q := r.URL.Query()
	from := utils.ParseTime(now, q.Get("from"), now.Add(-timeseries.Hour))
	to := utils.ParseTime(now, q.Get("to"), now)

	incidentKey := q.Get("incident")
	if incidentKey != "" {
		if incident, err := api.db.GetIncidentByKey(projectId, incidentKey); err != nil {
			klog.Warningln("failed to get incident:", err)
		} else {
			margin := model.MaxAlertRuleShortWindow + 15*timeseries.Minute
			from = incident.OpenedAt.Add(-margin)
			if incident.Resolved() {
				if t := incident.ResolvedAt.Add(margin); t.Before(to) {
					to = t
				}
			}
		}
	}

	world, cacheStatus, err := api.LoadWorld(r.Context(), project, from, to)
	if world == nil {
		step := increaseStepForBigDurations(to.Sub(from), 15*timeseries.Second)
		world = model.NewWorld(from, to.Add(-step), step, step)
	}
	return world, project, cacheStatus, err
}

func increaseStepForBigDurations(duration, step timeseries.Duration) timeseries.Duration {
	switch {
	case duration > 5*timeseries.Day:
		return maxDuration(step, 60*timeseries.Minute)
	case duration > timeseries.Day:
		return maxDuration(step, 15*timeseries.Minute)
	case duration > 12*timeseries.Hour:
		return maxDuration(step, 10*timeseries.Minute)
	case duration > 6*timeseries.Hour:
		return maxDuration(step, 5*timeseries.Minute)
	case duration > timeseries.Hour:
		return maxDuration(step, timeseries.Minute)
	}
	return step
}

func maxDuration(d1, d2 timeseries.Duration) timeseries.Duration {
	if d1 >= d2 {
		return d1
	}
	return d2
}

func (api *Api) getClickhouseClient(project *db.Project) (*clickhouse.Client, error) {
	cfg := project.ClickHouseConfig(api.globalClickHouse)
	if cfg == nil {
		return nil, nil
	}
	config := clickhouse.NewClientConfig(cfg.Addr, cfg.Auth.User, cfg.Auth.Password)
	config.Protocol = cfg.Protocol
	config.Database = cfg.Database
	config.TlsEnable = cfg.TlsEnable
	config.TlsSkipVerify = cfg.TlsSkipVerify
	distributed, err := api.collector.IsClickhouseDistributed(project)
	if err != nil {
		return nil, err
	}
	return clickhouse.NewClient(config, distributed)
}
