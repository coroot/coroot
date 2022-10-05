package api

import (
	"context"
	"errors"
	"github.com/coroot/coroot/api/views"
	"github.com/coroot/coroot/cache"
	"github.com/coroot/coroot/constructor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/stats"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/gorilla/mux"
	"k8s.io/klog"
	"net/http"
	"time"
)

type Api struct {
	cache *cache.Cache
	db    *db.DB
	stats *stats.Collector
}

func NewApi(cache *cache.Cache, db *db.DB, stats *stats.Collector) *Api {
	return &Api{cache: cache, db: db, stats: stats}
}

func (api *Api) Projects(w http.ResponseWriter, r *http.Request) {
	api.stats.RegisterRequest(r)
	projects, err := api.db.GetProjects()
	if err != nil {
		klog.Errorln("failed to get projects:", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	type Project struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	}
	res := make([]Project, 0, len(projects))
	for _, p := range projects {
		res = append(res, Project{Id: string(p.Id), Name: p.Name})
	}
	utils.WriteJson(w, res)
}

func (api *Api) Project(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := db.ProjectId(vars["project"])

	switch r.Method {

	case http.MethodGet:
		res := ProjectForm{}
		res.Prometheus.RefreshInterval = db.DefaultRefreshInterval
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
			res.Prometheus = project.Prometheus
		}
		utils.WriteJson(w, res)

	case http.MethodPost:
		var form ProjectForm
		if err := ReadAndValidate(r, &form); err != nil {
			klog.Warningln("bad request:", err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}
		project := db.Project{
			Id:         id,
			Name:       form.Name,
			Prometheus: form.Prometheus,
		}
		p := project.Prometheus
		user, password := "", ""
		if p.BasicAuth != nil {
			user, password = p.BasicAuth.User, p.BasicAuth.Password
		}
		promClient, err := prom.NewApiClient(p.Url, user, password, p.TlsSkipVerify)
		if err != nil {
			klog.Errorln("failed to get api client:", err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if err := promClient.Ping(ctx); err != nil {
			klog.Warningln("failed to ping prometheus:", err)
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
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
	world, err := api.loadWorldByRequest(r)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if world == nil {
		return
	}
	utils.WriteJson(w, views.Overview(world))
}

func (api *Api) Search(w http.ResponseWriter, r *http.Request) {
	world, err := api.loadWorldByRequest(r)
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

func (api *Api) App(w http.ResponseWriter, r *http.Request) {
	id, err := model.NewApplicationIdFromString(mux.Vars(r)["app"])
	if err != nil {
		klog.Warningf("invalid application_id %s: %s ", mux.Vars(r)["app"], err)
		http.Error(w, "invalid application_id: "+mux.Vars(r)["app"], http.StatusBadRequest)
		return
	}
	world, err := api.loadWorldByRequest(r)
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
	utils.WriteJson(w, views.Application(world, app))
}

func (api *Api) Check(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectId := db.ProjectId(vars["project"])
	appId, err := model.NewApplicationIdFromString(vars["app"])
	if err != nil {
		klog.Warningf("invalid application_id %s: %s ", vars["app"], err)
		http.Error(w, "invalid application_id: "+vars["app"], http.StatusBadRequest)
		return
	}
	checkId := model.CheckId(vars["check"])

	switch r.Method {

	case http.MethodGet:
		checkConfigs, err := api.db.GetCheckConfigs(projectId)
		if err != nil {
			klog.Errorln("failed to get check configs:", err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		switch checkId {
		case model.Checks.SLOAvailability.Id:
			configs := checkConfigs.GetAvailability(appId)
			if len(configs) == 0 {
				configs = append(configs, model.CheckConfigSLOAvailability{
					TotalRequestsQuery:  "",
					FailedRequestsQuery: "",
					ObjectivePercentage: model.Checks.SLOAvailability.DefaultThreshold,
				})
			}
			utils.WriteJson(w, CheckConfigAvailabilityForm{Configs: configs})
			return
		default:
			configs := checkConfigs.GetSimpleAll(checkId, appId)
			if len(configs) != 3 {
				http.Error(w, "", http.StatusNotFound)
				return
			}
			form := CheckConfigForm{GlobalThreshold: configs[0].Threshold}
			if configs[1] != nil {
				form.ProjectThreshold = &configs[1].Threshold
			}
			if configs[2] != nil {
				form.ApplicationThreshold = &configs[2].Threshold
			}
			utils.WriteJson(w, form)
			return
		}

	case http.MethodPost:
		switch checkId {
		case model.Checks.SLOAvailability.Id:
			var form CheckConfigAvailabilityForm
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
			for id, t := range map[model.ApplicationId]*float64{model.ApplicationId{}: form.ProjectThreshold, appId: form.ApplicationThreshold} {
				var cfg any = nil
				if t != nil {
					cfg = model.CheckConfigSimple{Threshold: *t}
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

func (api *Api) Node(w http.ResponseWriter, r *http.Request) {
	nodeName := mux.Vars(r)["node"]
	world, err := api.loadWorldByRequest(r)
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

	checkConfigs, err := api.db.GetCheckConfigs(project.Id)
	if err != nil {
		return nil, err
	}

	world, err := constructor.New(cc, checkConfigs).LoadWorld(ctx, from, to, step, nil)
	return world, err
}

func (api *Api) loadWorldByRequest(r *http.Request) (*model.World, error) {
	projectId := db.ProjectId(mux.Vars(r)["project"])
	now := timeseries.Now()
	q := r.URL.Query()
	from := utils.ParseTimeFromUrl(now, q, "from", now.Add(-timeseries.Hour))
	to := utils.ParseTimeFromUrl(now, q, "to", now)
	project, err := api.db.GetProject(projectId)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			klog.Warningln("project not found:", projectId)
			return nil, nil
		}
		return nil, err
	}
	return api.loadWorld(r.Context(), project, from, to)
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
