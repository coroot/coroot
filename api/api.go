package api

import (
	"github.com/coroot/coroot-focus/api/forms"
	"github.com/coroot/coroot-focus/api/views"
	"github.com/coroot/coroot-focus/cache"
	"github.com/coroot/coroot-focus/constructor"
	"github.com/coroot/coroot-focus/db"
	"github.com/coroot/coroot-focus/model"
	"github.com/coroot/coroot-focus/utils"
	"github.com/gorilla/mux"
	"k8s.io/klog"
	"net/http"
	"time"
)

type Api struct {
	cache *cache.Cache
	db    *db.DB
}

func NewApi(cache *cache.Cache, db *db.DB) *Api {
	return &Api{cache: cache, db: db}
}

func (api *Api) Projects(w http.ResponseWriter, r *http.Request) {
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
		form := forms.ProjectForm{}
		if id != "" {
			project, err := api.db.GetProject(id)
			if err != nil {
				klog.Errorln("failed to get project:", err)
				http.Error(w, "", http.StatusInternalServerError)
				return
			}
			form.Name = project.Name
			form.Prometheus = project.Prometheus
		}
		if form.Prometheus.RefreshInterval == 0 {
			form.Prometheus.RefreshInterval = db.DefaultRefreshInterval
		}
		utils.WriteJson(w, form)

	case http.MethodPost:
		var form forms.ProjectForm
		if err := forms.ReadAndValidate(r, &form); err != nil {
			klog.Warningln("bad request:", err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}
		project := db.Project{
			Id:         id,
			Name:       form.Name,
			Prometheus: form.Prometheus,
		}
		id, err := api.db.SaveProject(project)
		if err != nil {
			klog.Errorln("failed to save project:", err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		http.Error(w, string(id), http.StatusOK)

	default:
		http.Error(w, "", http.StatusMethodNotAllowed)
	}
}

func (api *Api) Overview(w http.ResponseWriter, r *http.Request) {
	world, err := api.loadWorld(r)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	utils.WriteJson(w, views.Overview(world))
}

func (api *Api) Search(w http.ResponseWriter, r *http.Request) {
	world, err := api.loadWorld(r)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
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
	world, err := api.loadWorld(r)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	app := world.GetApplication(id)
	if app == nil {
		klog.Warningf("application not found: %s ", id, err)
		http.Error(w, "application not found", http.StatusNotFound)
		return
	}
	utils.WriteJson(w, views.Application(world, app))
}

func (api *Api) Node(w http.ResponseWriter, r *http.Request) {
	nodeName := mux.Vars(r)["node"]
	world, err := api.loadWorld(r)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	node := world.GetNode(nodeName)
	if node == nil {
		klog.Warningf("node not found: %s ", nodeName, err)
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}
	utils.WriteJson(w, views.Node(world, node))
}

func (api *Api) loadWorld(r *http.Request) (*model.World, error) {
	now := time.Now()
	q := r.URL.Query()
	from := utils.ParseTimeFromUrl(now, q, "from", now.Add(-time.Hour))
	to := utils.ParseTimeFromUrl(now, q, "to", now)
	projectId := db.ProjectId(mux.Vars(r)["project"])
	project, err := api.db.GetProject(projectId)
	if err != nil {
		return nil, err
	}
	step := time.Duration(project.Prometheus.RefreshInterval) * time.Second
	c := constructor.New(api.cache.GetCacheClient(project), step)
	return c.LoadWorld(r.Context(), from, to)
}
