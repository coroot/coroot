package api

import (
	"context"
	"errors"
	"github.com/coroot/coroot/api/views"
	"github.com/coroot/coroot/auditor"
	"github.com/coroot/coroot/cache"
	"github.com/coroot/coroot/constructor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/gorilla/mux"
	"k8s.io/klog"
	"net/http"
	"sort"
	"time"
)

type Api struct {
	cache    *cache.Cache
	db       *db.DB
	readOnly bool
}

func NewApi(cache *cache.Cache, db *db.DB, readOnly bool) *Api {
	return &Api{cache: cache, db: db, readOnly: readOnly}
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
		res := ProjectForm{}
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
		var form ProjectForm
		if err := ReadAndValidate(r, &form); err != nil {
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
		var form ProjectStatusForm
		if err := ReadAndValidate(r, &form); err != nil {
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
			utils.WriteJson(w, views.Status(nil, nil, nil))
			return
		}
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	cacheStatus, err := api.cache.GetCacheClient(project).GetStatus()
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	now := timeseries.Now()
	world, err := api.loadWorld(r.Context(), project, now.Add(-timeseries.Hour), now)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	utils.WriteJson(w, views.Status(project, cacheStatus, world))
}

func (api *Api) Overview(w http.ResponseWriter, r *http.Request) {
	world, project, err := api.loadWorldByRequest(r)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if world == nil {
		return
	}
	auditor.Audit(world, project)
	utils.WriteJson(w, views.Overview(world))
}

func (api *Api) Search(w http.ResponseWriter, r *http.Request) {
	world, _, err := api.loadWorldByRequest(r)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if world == nil {
		return
	}
	utils.WriteJson(w, views.Search(world))
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
		var form ApplicationCategoryForm
		if err := ReadAndValidate(r, &form); err != nil {
			klog.Warningln("bad request:", err)
			http.Error(w, "Invalid name or patterns", http.StatusBadRequest)
			return
		}
		if err := api.db.SaveApplicationCategory(projectId, form.Name, form.NewName, form.customPatterns, form.NotifyOfDeployments); err != nil {
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
		var form IntegrationsForm
		if err := ReadAndValidate(r, &form); err != nil {
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
	form := NewIntegrationForm(t)
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
		if err := ReadAndValidate(r, form); err != nil {
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
	user, password := "", ""
	if p.BasicAuth != nil {
		user, password = p.BasicAuth.User, p.BasicAuth.Password
	}
	c, err := prom.NewApiClient(p.Url, user, password, p.TlsSkipVerify, p.ExtraSelector)
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
	world, project, err := api.loadWorldByRequest(r)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if world == nil {
		return
	}
	app := world.GetApplication(id)
	if app == nil {
		klog.Warningln("application not found:", id)
		http.Error(w, "Application not found", http.StatusNotFound)
		return
	}
	incidents, err := api.db.GetIncidentsByApp(project.Id, app.Id, world.Ctx.From, world.Ctx.To)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	auditor.Audit(world, project)
	utils.WriteJson(w, views.Application(world, app, incidents))
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
			configs, def := checkConfigs.GetAvailability(appId)
			res.Form = CheckConfigSLOAvailabilityForm{Configs: configs, Default: def}
		case model.Checks.SLOLatency.Id:
			configs, def := checkConfigs.GetLatency(appId, model.CalcApplicationCategory(appId, project.Settings.ApplicationCategories))
			res.Form = CheckConfigSLOLatencyForm{Configs: configs, Default: def}
		default:
			form := CheckConfigForm{
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
			var form CheckConfigSLOAvailabilityForm
			if err := ReadAndValidate(r, &form); err != nil {
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
			var form CheckConfigSLOLatencyForm
			if err := ReadAndValidate(r, &form); err != nil {
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
			var form CheckConfigForm
			if err := ReadAndValidate(r, &form); err != nil {
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
		var form ApplicationSettingsPyroscopeForm
		if err := ReadAndValidate(r, &form); err != nil {
			klog.Warningln("bad request:", err)
			http.Error(w, "invalid data", http.StatusBadRequest)
			return
		}
		klog.Infoln(form)
		if err := api.db.SaveApplicationSetting(projectId, appId, &form.ApplicationSettingsPyroscope); err != nil {
			klog.Errorln(err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		return
	}

	world, project, err := api.loadWorldByRequest(r)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if world == nil {
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
	utils.WriteJson(w, views.Profile(r.Context(), project, app, settings, q, world.Ctx))
}

func (api *Api) Node(w http.ResponseWriter, r *http.Request) {
	nodeName := mux.Vars(r)["node"]
	world, _, err := api.loadWorldByRequest(r)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if world == nil {
		return
	}
	node := world.GetNode(nodeName)
	if node == nil {
		klog.Warningf("node not found: %s ", nodeName)
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}
	utils.WriteJson(w, views.Node(world, node))
}

func (api *Api) loadWorld(ctx context.Context, project *db.Project, from, to timeseries.Time) (*model.World, error) {
	cc := api.cache.GetCacheClient(project)
	cacheTo, err := cc.GetTo()
	if err != nil {
		return nil, err
	}

	step := project.Prometheus.RefreshInterval
	from = from.Truncate(step)
	to = to.Truncate(step)

	if cacheTo.IsZero() || cacheTo.Before(from) {
		return nil, nil
	}

	duration := to.Sub(from)
	if cacheTo.Before(to) {
		to = cacheTo
		from = to.Add(-duration)
	}
	step = increaseStepForBigDurations(duration, step)

	t := time.Now()
	world, err := constructor.New(api.db, project, cc).LoadWorld(ctx, from, to, step, nil)
	klog.Infof("world loaded in %s", time.Since(t))
	return world, err
}

func (api *Api) loadWorldByRequest(r *http.Request) (*model.World, *db.Project, error) {
	projectId := db.ProjectId(mux.Vars(r)["project"])
	project, err := api.db.GetProject(projectId)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			klog.Warningln("project not found:", projectId)
			return nil, nil, nil
		}
		return nil, nil, err
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

	world, err := api.loadWorld(r.Context(), project, from, to)
	return world, project, err
}

func increaseStepForBigDurations(duration, step timeseries.Duration) timeseries.Duration {
	switch {
	case duration > 5*24*timeseries.Hour:
		return maxDuration(step, 60*timeseries.Minute)
	case duration > 24*timeseries.Hour:
		return maxDuration(step, 15*timeseries.Minute)
	case duration > 12*timeseries.Hour:
		return maxDuration(step, 10*timeseries.Minute)
	case duration > 6*timeseries.Hour:
		return maxDuration(step, 5*timeseries.Minute)
	case duration > 4*timeseries.Hour:
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
