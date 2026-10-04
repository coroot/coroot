package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/coroot/coroot/api/forms"
	"github.com/coroot/coroot/api/views"
	"github.com/coroot/coroot/clickhouse"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/rbac"
	"github.com/coroot/coroot/utils"
	"github.com/gorilla/mux"
	"k8s.io/klog"
)

func (api *Api) ProjectRumSettings(w http.ResponseWriter, r *http.Request, u *db.User) {
	vars := mux.Vars(r)
	projectId := vars["project"]
	project, err := api.db.GetProject(db.ProjectId(projectId))
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	isAllowed := api.IsAllowed(u, rbac.Actions.Project(projectId).Settings().Edit())

	type rumSettingsResp struct {
		Editable         bool    `json:"editable"`
		GeoEnabled       bool    `json:"geo_enabled"`
		ReplayEnabled    bool    `json:"replay_enabled"`
		ReplaySampleRate float64 `json:"replay_sample_rate"`
		RawTTL           string  `json:"raw_ttl"`
		ReplayTTL        string  `json:"replay_ttl"`
		AggregatesTTL    string  `json:"aggregates_ttl"`
		DefaultRawTTL    string  `json:"default_raw_ttl"`
		DefaultReplayTTL string  `json:"default_replay_ttl"`
		DefaultAggTTL    string  `json:"default_aggregates_ttl"`
	}

	defaults := clickhouse.EffectiveRumRetention(nil, api.cfg.Rum)
	if r.Method == http.MethodGet {
		res := rumSettingsResp{
			Editable:         isAllowed && !project.Settings.Readonly && !project.Multicluster(),
			DefaultRawTTL:    defaults.Raw.ShortString(),
			DefaultReplayTTL: defaults.Replay.ShortString(),
			DefaultAggTTL:    defaults.Aggregates.ShortString(),
		}
		if project.Settings.Rum != nil {
			res.GeoEnabled = project.Settings.Rum.GeoEnabled
			res.ReplayEnabled = project.Settings.Rum.ReplayEnabled
			res.ReplaySampleRate = project.Settings.Rum.ReplaySampleRate
			if project.Settings.Rum.Retention != nil {
				if project.Settings.Rum.Retention.RawTTL > 0 {
					res.RawTTL = project.Settings.Rum.Retention.RawTTL.ShortString()
				}
				if project.Settings.Rum.Retention.ReplayTTL > 0 {
					res.ReplayTTL = project.Settings.Rum.Retention.ReplayTTL.ShortString()
				}
				if project.Settings.Rum.Retention.AggregatesTTL > 0 {
					res.AggregatesTTL = project.Settings.Rum.Retention.AggregatesTTL.ShortString()
				}
			}
		}
		utils.WriteJson(w, res)
		return
	}

	if !isAllowed {
		http.Error(w, "You are not allowed to configure RUM settings.", http.StatusForbidden)
		return
	}
	if project.Settings.Readonly {
		http.Error(w, "This project is defined through the config and cannot be modified via the UI.", http.StatusForbidden)
		return
	}
	var form forms.ProjectRumSettingsForm
	if err := forms.ReadAndValidate(r, &form); err != nil {
		klog.Warningln("bad request:", err)
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}
	if project.Settings.Rum == nil {
		project.Settings.Rum = &db.RumProjectSettings{}
	}
	project.Settings.Rum.GeoEnabled = form.GeoEnabled
	project.Settings.Rum.ReplayEnabled = form.ReplayEnabled
	project.Settings.Rum.ReplaySampleRate = form.ReplaySampleRate
	project.Settings.Rum.Retention = form.Retention()
	if err := api.db.SaveProjectSettings(project); err != nil {
		klog.Errorln("failed to save rum settings:", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := clickhouse.ApplyRumRetentionForProject(ctx, project, api.globalClickHouse, api.cfg.Rum); err != nil {
			klog.Errorln("apply rum retention after settings save:", err)
		}
	}()
}

func (api *Api) enrichWorldWithRum(ctx context.Context, project *db.Project, world *model.World) {
	if project == nil || world == nil {
		return
	}
	chs := clickhouse.GetClients(api.db, project, api.globalClickHouse)
	defer chs.Close()
	if chs.Error != nil {
		return
	}
	clickhouse.EnrichWorldWithRum(ctx, chs, world, project)
}

func clickhouseClusterIdForApp(project *db.Project, app *model.Application) string {
	if app == nil {
		return string(project.Id)
	}
	if app.Id.Kind == model.ApplicationKindRumClient {
		if project.Multicluster() && len(project.Settings.MemberProjects) > 0 {
			return project.Settings.MemberProjects[0]
		}
		return string(project.Id)
	}
	return app.Id.ClusterId
}

func (api *Api) Rum(w http.ResponseWriter, r *http.Request, u *db.User) {
	projectId := mux.Vars(r)["project"]
	appId, err := GetApplicationId(r)
	if err != nil {
		klog.Warningln(err)
		http.Error(w, "invalid application id", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodPost {
		if !api.IsAllowed(u, rbac.Actions.Project(projectId).Inspections().Edit()) {
			http.Error(w, "You are not allowed to configure RUM settings.", http.StatusForbidden)
			return
		}
		project, err := api.db.GetProject(db.ProjectId(projectId))
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				http.Error(w, "Project not found.", http.StatusNotFound)
				return
			}
			klog.Errorln("failed to get project:", err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		configProjectId, ok := api.appConfigProjectId(project, appId)
		if !ok {
			klog.Warningln("application doesn't belong to any member project:", appId)
			http.Error(w, "Application not found", http.StatusNotFound)
			return
		}
		var form forms.ApplicationSettingsRumForm
		if err := forms.ReadAndValidate(r, &form); err != nil {
			klog.Warningln("bad request:", err)
			http.Error(w, "invalid data", http.StatusBadRequest)
			return
		}
		if err := api.db.SaveApplicationSetting(configProjectId, appId, &form.ApplicationSettingsRum); err != nil {
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
	api.enrichWorldWithRum(r.Context(), project, world)
	app := world.GetApplication(appId)
	if app == nil {
		klog.Warningln("application not found:", appId)
		http.Error(w, "Application not found", http.StatusNotFound)
		return
	}
	q := r.URL.Query()
	clusterId := clickhouseClusterIdForApp(project, app)
	var chClient *clickhouse.Client
	if chClient, err = api.GetClickhouseClient(project, clusterId); err != nil || chClient == nil {
		if !project.Multicluster() {
			chClient, err = api.GetClickhouseClient(project, string(project.Id))
		}
		if err != nil || chClient == nil {
			klog.Warningln(err)
			http.Error(w, "ClickHouse is not available", http.StatusInternalServerError)
			return
		}
	}
	defer chClient.Close()
	if q.Get("replay") == "1" {
		if !api.IsAllowed(u, rbac.Actions.Project(projectId).RumReplay().View()) {
			http.Error(w, "You are not allowed to view session replay.", http.StatusForbidden)
			return
		}
	}
	rumView := views.Rum(r.Context(), chClient, app, q, world)
	if rumView != nil {
		ttls := clickhouse.EffectiveRumRetention(project, api.cfg.Rum)
		rumView.RawRetention = ttls.Raw.ShortString()
		rumView.AggregatesRetention = ttls.Aggregates.ShortString()
	}
	utils.WriteJson(w, api.WithContext(project, cacheStatus, world, rumView))
}
