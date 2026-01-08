package api

import (
	"net/http"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/prom"
	"github.com/gorilla/mux"
	"k8s.io/klog"
)

func (api *Api) PrometheusQueryRange(w http.ResponseWriter, r *http.Request, project *db.Project) {
	c, err := prom.NewClient(project.PrometheusConfig(api.globalPrometheus), project.ClickHouseConfig(api.globalClickHouse))
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	defer c.Close()
	c.QueryRangeHandler(r, w)
}

func (api *Api) PrometheusMetricMetadata(w http.ResponseWriter, r *http.Request, project *db.Project) {
	c, err := prom.NewClient(project.PrometheusConfig(api.globalPrometheus), project.ClickHouseConfig(api.globalClickHouse))
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	defer c.Close()
	c.MetricMetadata(r, w)
}

func (api *Api) PrometheusSeries(w http.ResponseWriter, r *http.Request, project *db.Project) {
	c, err := prom.NewClient(project.PrometheusConfig(api.globalPrometheus), project.ClickHouseConfig(api.globalClickHouse))
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	defer c.Close()
	c.Series(r, w)
}

func (api *Api) PrometheusLabelValues(w http.ResponseWriter, r *http.Request, project *db.Project) {
	vars := mux.Vars(r)
	c, err := prom.NewClient(project.PrometheusConfig(api.globalPrometheus), project.ClickHouseConfig(api.globalClickHouse))
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	defer c.Close()
	c.LabelValues(r, w, vars["labelName"])
}
