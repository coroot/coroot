package main

import (
	"github.com/coroot/coroot-focus/constructor"
	"github.com/coroot/coroot-focus/model"
	"github.com/coroot/coroot-focus/prometheus"
	"github.com/coroot/coroot-focus/utils"
	"github.com/coroot/coroot-focus/view"
	"github.com/gorilla/mux"
	"gopkg.in/alecthomas/kingpin.v2"
	"k8s.io/klog"
	"net/http"
	"time"
)

type Focus struct {
	constructor *constructor.Constructor
}

func (f *Focus) Health(w http.ResponseWriter, r *http.Request) {
	return
}

func (f *Focus) App(w http.ResponseWriter, r *http.Request) {
	id, err := model.NewApplicationIdFromString(mux.Vars(r)["app"])
	if err != nil {
		klog.Warningf("invalid application_id %s: %s ", mux.Vars(r)["app"], err)
		http.Error(w, "invalid application_id: "+mux.Vars(r)["app"], http.StatusBadRequest)
		return
	}
	now := time.Now()
	world, err := f.constructor.LoadWorld(r.Context(), now.Add(-4*time.Hour), now)
	if err != nil {
		klog.Errorln(err)
	}
	app := world.GetApplication(id)
	if app == nil {
		klog.Warningf("application not found: %s ", id, err)
		http.Error(w, "application not found", http.StatusNotFound)
		return
	}
	utils.WriteJson(w, view.RenderApp(world, app))
}

func main() {
	prometheusUrl := kingpin.Flag("prometheus", `Prometheus URL`).Required().String()
	scrapeInterval := kingpin.Flag("scrapeInterval", `Prometheus scrape interval`).Default("30s").Duration()
	skipTlsVerify := kingpin.Flag("skipTlsVerify", `Don't verify the certificate of the Prometheus`).Bool()

	kingpin.Parse()

	prom, err := prometheus.NewClient(*prometheusUrl, *skipTlsVerify, *scrapeInterval)
	if err != nil {
		klog.Fatalln(err)
	}
	focus := &Focus{constructor: constructor.New(prom)}

	r := mux.NewRouter()
	r.HandleFunc("/health", focus.Health).Methods(http.MethodGet)
	r.HandleFunc("/app/{app}", focus.App).Methods(http.MethodGet)

	klog.Infoln("listening on :8080")
	klog.Fatalln(http.ListenAndServe(":8080", r))
}
