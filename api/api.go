package api

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"time"

	"github.com/coroot/coroot/api/forms"
	"github.com/coroot/coroot/api/views"
	"github.com/coroot/coroot/auditor"
	"github.com/coroot/coroot/cache"
	"github.com/coroot/coroot/clickhouse"
	cloud_pricing "github.com/coroot/coroot/cloud-pricing"
	"github.com/coroot/coroot/constructor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/gorilla/mux"
	"k8s.io/klog"
)

type Api struct {
	cache    *cache.Cache
	db       *db.DB
	pricing  *cloud_pricing.Manager
	readOnly bool
}

func NewApi(cache *cache.Cache, db *db.DB, pricing *cloud_pricing.Manager, readOnly bool) *Api {
	return &Api{cache: cache, db: db, pricing: pricing, readOnly: readOnly}
}

func (api *Api) Projects(w http.ResponseWriter, _ *http.Request) {
	projects, err := api.db.GetProjectNames()
	if err != nil {
		klog.Errorln("failed to get projects:", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	type Project struct {
		Id   db.ProjectId `json:"id"`
		Name string       `json:"name"`
	}
	res := make([]Project, 0, len(projects))
	for id, name := range projects {
		res = append(res, Project{Id: id, Name: name})
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].Name < res[j].Name
	})
	utils.WriteJson(w, res)
}

func (api *Api) Project(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := db.ProjectId(vars["project"])

	switch r.Method {

	case http.MethodGet:
		res := forms.ProjectForm{}
		if id != "" {
			project, err := api.db.GetProject(id)
			if err != nil {
				if errors.Is(err, db.ErrNotFound) {
					klog.Warningln("project not found:", id)
					return
				}
				klog.Errorln("failed to get project:", err)
				http.Error(w, "", http.StatusInternalServerError)
				return
			}
			res.Name = project.Name
		}
		utils.WriteJson(w, res)

	case http.MethodPost:
		if api.readOnly {
			return
		}
		var form forms.ProjectForm
		if err := forms.ReadAndValidate(r, &form); err != nil {
			klog.Warningln("bad request:", err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}
		project := db.Project{
			Id:   id,
			Name: form.Name,
		}
		id, err := api.db.SaveProject(project)
		if err != nil {
			if errors.Is(err, db.ErrConflict) {
				http.Error(w, "This project name is already being used.", http.StatusConflict)
				return
			}
			klog.Errorln("failed to save project:", err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		http.Error(w, string(id), http.StatusOK)

	case http.MethodDelete:
		if api.readOnly {
			return
		}
		if err := api.db.DeleteProject(id); err != nil {
			klog.Errorln("failed to delete project:", err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		http.Error(w, "", http.StatusOK)

	default:
		http.Error(w, "", http.StatusMethodNotAllowed)
	}
}

func (api *Api) Status(w http.ResponseWriter, r *http.Request) {
	projectId := db.ProjectId(mux.Vars(r)["project"])
	if r.Method == http.MethodPost {
		if api.readOnly {
			return
		}
		var form forms.ProjectStatusForm
		if err := forms.ReadAndValidate(r, &form); err != nil {
			klog.Warningln("bad request:", err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}
		var appType model.ApplicationType
		var mute bool
		switch {
		case form.Mute != nil:
			mute = true
			appType = *form.Mute
		case form.UnMute != nil:
			mute = false
			appType = *form.UnMute
		}
		if err := api.db.ToggleConfigurationHint(projectId, appType, mute); err != nil {
			klog.Errorln("failed to toggle:", err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		return
	}
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
	world, cacheStatus, err := api.loadWorld(r.Context(), project, now.Add(-timeseries.Hour), now)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	utils.WriteJson(w, renderStatus(project, cacheStatus, world))
}

func (api *Api) Overview(w http.ResponseWriter, r *http.Request) {
	world, project, cacheStatus, err := api.loadWorldByRequest(r)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if project == nil || world == nil {
		utils.WriteJson(w, withContext(project, cacheStatus, world, nil))
		return
	}
	auditor.Audit(world, project, nil)
	utils.WriteJson(w, withContext(project, cacheStatus, world, views.Overview(world, mux.Vars(r)["view"])))
}

func (api *Api) Configs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectId := db.ProjectId(vars["project"])
	checkConfigs, err := api.db.GetCheckConfigs(projectId)
	if err != nil {
		klog.Errorln("failed to get check configs:", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	utils.WriteJson(w, views.Configs(checkConfigs))
}

func (api *Api) Categories(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectId := db.ProjectId(vars["project"])

	if r.Method == http.MethodPost {
		if api.readOnly {
			return
		}
		var form forms.ApplicationCategoryForm
		if err := forms.ReadAndValidate(r, &form); err != nil {
			klog.Warningln("bad request:", err)
			http.Error(w, "Invalid name or patterns", http.StatusBadRequest)
			return
		}
		if err := api.db.SaveApplicationCategory(projectId, form.Name, form.NewName, form.CustomPatterns, form.NotifyOfDeployments); err != nil {
			klog.Errorln("failed to save:", err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		return
	}

	p, err := api.db.GetProject(projectId)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	utils.WriteJson(w, views.Categories(p))
}

func (api *Api) Integrations(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectId := db.ProjectId(vars["project"])

	if r.Method == http.MethodPut {
		if api.readOnly {
			return
		}
		var form forms.IntegrationsForm
		if err := forms.ReadAndValidate(r, &form); err != nil {
			klog.Warningln("bad request:", err)
			http.Error(w, "Invalid base url", http.StatusBadRequest)
			return
		}
		if err := api.db.SaveIntegrationsBaseUrl(projectId, form.BaseUrl); err != nil {
			klog.Errorln("failed to save:", err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		return
	}

	p, err := api.db.GetProject(projectId)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	utils.WriteJson(w, views.Integrations(p))
}

func (api *Api) Integration(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectId := db.ProjectId(vars["project"])
	project, err := api.db.GetProject(projectId)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	t := db.IntegrationType(vars["type"])
	form := forms.NewIntegrationForm(t)
	if form == nil {
		klog.Warningln("unknown integration type:", t)
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodGet {
		form.Get(project, api.readOnly)
		utils.WriteJson(w, form)
		return
	}

	if api.readOnly {
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
}

func (api *Api) Prom(w http.ResponseWriter, r *http.Request) {
	projectId := db.ProjectId(mux.Vars(r)["project"])
	project, err := api.db.GetProject(projectId)
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

func (api *Api) App(w http.ResponseWriter, r *http.Request) {
	id, err := model.NewApplicationIdFromString(mux.Vars(r)["app"])
	if err != nil {
		klog.Warningln(err)
		http.Error(w, "invalid application id: "+mux.Vars(r)["app"], http.StatusBadRequest)
		return
	}
	world, project, cacheStatus, err := api.loadWorldByRequest(r)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if project == nil || world == nil {
		utils.WriteJson(w, withContext(project, cacheStatus, world, nil))
		return
	}
	app := world.GetApplication(id)
	if app == nil {
		klog.Warningln("application not found:", id)
		http.Error(w, "Application not found", http.StatusNotFound)
		return
	}
	auditor.Audit(world, project, app)
	if cfg := project.Settings.Integrations.Clickhouse; cfg != nil && cfg.ProfilingEnabled() {
		app.AddReport(model.AuditReportProfiling, &model.Widget{Profiling: &model.Profiling{ApplicationId: app.Id}, Width: "100%"})
	}
	if cfg := project.Settings.Integrations.Clickhouse; cfg != nil && cfg.TracingEnabled() {
		app.AddReport(model.AuditReportTracing, &model.Widget{Tracing: &model.Tracing{ApplicationId: app.Id}, Width: "100%"})
	}
	utils.WriteJson(w, withContext(project, cacheStatus, world, views.Application(world, app)))
}

func (api *Api) Check(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectId := db.ProjectId(vars["project"])
	appId, err := model.NewApplicationIdFromString(vars["app"])
	if err != nil {
		klog.Warningln(err)
		http.Error(w, "invalid application id: "+vars["app"], http.StatusBadRequest)
		return
	}
	checkId := model.CheckId(vars["check"])

	switch r.Method {

	case http.MethodGet:
		project, err := api.db.GetProject(projectId)
		if err != nil {
			klog.Errorln("failed to get project:", err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		checkConfigs, err := api.db.GetCheckConfigs(projectId)
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
		if api.readOnly {
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
			if err := api.db.SaveCheckConfig(projectId, appId, checkId, form.Configs); err != nil {
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
			if err := api.db.SaveCheckConfig(projectId, appId, checkId, form.Configs); err != nil {
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
				if err := api.db.SaveCheckConfig(projectId, id, checkId, cfg); err != nil {
					klog.Errorln("failed to save check config:", err)
					http.Error(w, "", http.StatusInternalServerError)
					return
				}
			}
			return
		}
	}
}

func (api *Api) Profile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectId := db.ProjectId(vars["project"])
	appId, err := model.NewApplicationIdFromString(vars["app"])
	if err != nil {
		klog.Warningln(err)
		http.Error(w, "invalid application id: "+vars["app"], http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodPost {
		if api.readOnly {
			return
		}
		var form forms.ApplicationSettingsProfilingForm
		if err := forms.ReadAndValidate(r, &form); err != nil {
			klog.Warningln("bad request:", err)
			http.Error(w, "invalid data", http.StatusBadRequest)
			return
		}
		klog.Infoln(form)
		if err := api.db.SaveApplicationSetting(projectId, appId, &form.ApplicationSettingsProfiling); err != nil {
			klog.Errorln(err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		return
	}

	world, project, cacheStatus, err := api.loadWorldByRequest(r)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if project == nil || world == nil {
		utils.WriteJson(w, withContext(project, cacheStatus, world, nil))
		return
	}
	app := world.GetApplication(appId)
	if app == nil {
		klog.Warningln("application not found:", appId)
		http.Error(w, "Application not found", http.StatusNotFound)
		return
	}
	settings, err := api.db.GetApplicationSettings(project.Id, app.Id)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	var ch *clickhouse.Client
	if cfg := project.Settings.Integrations.Clickhouse; cfg != nil && cfg.ProfilingEnabled() {
		config := clickhouse.NewClientConfig(cfg.Addr, cfg.Auth.User, cfg.Auth.Password)
		config.Protocol = cfg.Protocol
		config.Database = cfg.Database
		config.TracesTable = cfg.TracesTable
		config.LogsTable = cfg.LogsTable
		config.TlsEnable = cfg.TlsEnable
		config.TlsSkipVerify = cfg.TlsSkipVerify
		ch, err = clickhouse.NewClient(config)
		if err != nil {
			klog.Warningln(err)
		}
	}
	q := r.URL.Query()
	auditor.Audit(world, project, nil)
	utils.WriteJson(w, withContext(project, cacheStatus, world, views.Profiling(r.Context(), ch, app, settings, q, world.Ctx)))
}

func (api *Api) Tracing(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectId := db.ProjectId(vars["project"])
	appId, err := model.NewApplicationIdFromString(vars["app"])
	if err != nil {
		klog.Warningln(err)
		http.Error(w, "invalid application id: "+vars["app"], http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodPost {
		if api.readOnly {
			return
		}
		var form forms.ApplicationSettingsTracingForm
		if err := forms.ReadAndValidate(r, &form); err != nil {
			klog.Warningln("bad request:", err)
			http.Error(w, "invalid data", http.StatusBadRequest)
			return
		}
		if err := api.db.SaveApplicationSetting(projectId, appId, &form.ApplicationSettingsTracing); err != nil {
			klog.Errorln(err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		return
	}

	world, project, cacheStatus, err := api.loadWorldByRequest(r)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if project == nil || world == nil {
		utils.WriteJson(w, withContext(project, cacheStatus, world, nil))
		return
	}
	app := world.GetApplication(appId)
	if app == nil {
		klog.Warningln("application not found:", appId)
		http.Error(w, "Application not found", http.StatusNotFound)
		return
	}
	settings, err := api.db.GetApplicationSettings(project.Id, app.Id)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	q := r.URL.Query()
	var ch *clickhouse.Client
	if cfg := project.Settings.Integrations.Clickhouse; cfg != nil && cfg.TracingEnabled() {
		config := clickhouse.NewClientConfig(cfg.Addr, cfg.Auth.User, cfg.Auth.Password)
		config.Protocol = cfg.Protocol
		config.Database = cfg.Database
		config.TracesTable = cfg.TracesTable
		config.LogsTable = cfg.LogsTable
		config.TlsEnable = cfg.TlsEnable
		config.TlsSkipVerify = cfg.TlsSkipVerify
		ch, err = clickhouse.NewClient(config)
		if err != nil {
			klog.Warningln(err)
		}
	}
	auditor.Audit(world, project, nil)
	utils.WriteJson(w, withContext(project, cacheStatus, world, views.Tracing(r.Context(), ch, app, settings, q, world)))
}

func (api *Api) Logs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectId := db.ProjectId(vars["project"])
	appId, err := model.NewApplicationIdFromString(vars["app"])
	if err != nil {
		klog.Warningln(err)
		http.Error(w, "invalid application id: "+vars["app"], http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodPost {
		if api.readOnly {
			return
		}
		var form forms.ApplicationSettingsLogsForm
		if err := forms.ReadAndValidate(r, &form); err != nil {
			klog.Warningln("bad request:", err)
			http.Error(w, "invalid data", http.StatusBadRequest)
			return
		}
		if err := api.db.SaveApplicationSetting(projectId, appId, &form.ApplicationSettingsLogs); err != nil {
			klog.Errorln(err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		return
	}

	world, project, cacheStatus, err := api.loadWorldByRequest(r)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if project == nil || world == nil {
		utils.WriteJson(w, withContext(project, cacheStatus, world, nil))
		return
	}
	app := world.GetApplication(appId)
	if app == nil {
		klog.Warningln("application not found:", appId)
		http.Error(w, "Application not found", http.StatusNotFound)
		return
	}
	settings, err := api.db.GetApplicationSettings(project.Id, app.Id)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	var ch *clickhouse.Client
	if cfg := project.Settings.Integrations.Clickhouse; cfg != nil && cfg.LogsEnabled() {
		config := clickhouse.NewClientConfig(cfg.Addr, cfg.Auth.User, cfg.Auth.Password)
		config.Protocol = cfg.Protocol
		config.Database = cfg.Database
		config.TracesTable = cfg.TracesTable
		config.LogsTable = cfg.LogsTable
		config.TlsEnable = cfg.TlsEnable
		config.TlsSkipVerify = cfg.TlsSkipVerify
		ch, err = clickhouse.NewClient(config)
		if err != nil {
			klog.Warningln(err)
		}
	}
	auditor.Audit(world, project, nil)
	q := r.URL.Query()
	utils.WriteJson(w, withContext(project, cacheStatus, world, views.Logs(r.Context(), ch, app, settings, q, world)))
}

func (api *Api) Node(w http.ResponseWriter, r *http.Request) {
	nodeName := mux.Vars(r)["node"]
	world, project, cacheStatus, err := api.loadWorldByRequest(r)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if project == nil || world == nil {
		utils.WriteJson(w, withContext(project, cacheStatus, world, nil))
		return
	}
	node := world.GetNode(nodeName)
	if node == nil {
		klog.Warningf("node not found: %s ", nodeName)
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}
	auditor.Audit(world, project, nil)
	utils.WriteJson(w, withContext(project, cacheStatus, world, auditor.AuditNode(world, node)))
}

func (api *Api) loadWorld(ctx context.Context, project *db.Project, from, to timeseries.Time) (*model.World, *cache.Status, error) {
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
		from = to.Add(-duration)
	}
	step = increaseStepForBigDurations(duration, step)

	ctr := constructor.New(api.db, project, cacheClient, api.pricing)
	world, err := ctr.LoadWorld(ctx, from, to, step, nil)
	return world, cacheStatus, err
}

func (api *Api) loadWorldByRequest(r *http.Request) (*model.World, *db.Project, *cache.Status, error) {
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
			from = incident.OpenedAt.Add(-timeseries.Hour)
			if incident.Resolved() && incident.ResolvedAt.Add(timeseries.Hour).Before(to) {
				to = incident.ResolvedAt.Add(timeseries.Hour)
			}
		}
	}

	world, cacheStatus, err := api.loadWorld(r.Context(), project, from, to)
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
