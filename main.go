package main

import (
	"github.com/coroot/coroot-focus/cache"
	"github.com/coroot/coroot-focus/constructor"
	"github.com/coroot/coroot-focus/db"
	"github.com/coroot/coroot-focus/model"
	"github.com/coroot/coroot-focus/prom"
	"github.com/coroot/coroot-focus/utils"
	"github.com/coroot/coroot-focus/views"
	"github.com/gorilla/mux"
	"gopkg.in/alecthomas/kingpin.v2"
	"k8s.io/klog"
	"net/http"
	"path"
	"time"
)

type Focus struct {
	constructor *constructor.Constructor
}

func (f *Focus) Health(w http.ResponseWriter, r *http.Request) {
	return
}

func (f *Focus) Overview(w http.ResponseWriter, r *http.Request) {
	world, err := f.loadWorld(r)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	klog.Infoln("world", world == nil)
	utils.WriteJson(w, views.Overview(world))
}

func (f *Focus) App(w http.ResponseWriter, r *http.Request) {
	id, err := model.NewApplicationIdFromString(mux.Vars(r)["app"])
	if err != nil {
		klog.Warningf("invalid application_id %s: %s ", mux.Vars(r)["app"], err)
		http.Error(w, "invalid application_id: "+mux.Vars(r)["app"], http.StatusBadRequest)
		return
	}
	world, err := f.loadWorld(r)
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

func (f *Focus) loadWorld(r *http.Request) (*model.World, error) {
	now := time.Now()
	return f.constructor.LoadWorld(r.Context(), now.Add(-1*time.Hour), now)
}

func main() {
	dataDir := kingpin.Flag("datadir", `Path to data directory`).Required().String()
	prometheusUrl := kingpin.Flag("prometheus", `Prometheus URL`).Required().String()
	scrapeInterval := kingpin.Flag("scrapeInterval", `Prometheus scrape interval`).Default("30s").Duration()
	skipTlsVerify := kingpin.Flag("skipTlsVerify", `Don't verify the certificate of the Prometheus`).Bool()
	cacheTTL := kingpin.Flag("cache-ttl", `Cache TTL`).Default("720h").Duration()
	cacheGcInterval := kingpin.Flag("cache-gc-interval", `Cache GC interval`).Default("10m").Duration()

	kingpin.Parse()

	if err := utils.CreateDirectoryIfNotExists(*dataDir); err != nil {
		klog.Exitln(err)
	}
	db, err := db.Open(path.Join(*dataDir, "db.sqlite"))
	if err != nil {
		klog.Exitln(err)
	}
	promApiClient, err := prom.NewApiClient(*prometheusUrl, *skipTlsVerify, *scrapeInterval)
	if err != nil {
		klog.Exitln(err)
	}

	cacheConfig := cache.Config{
		Path: path.Join(*dataDir, "cache"),
		GC: &cache.GcConfig{
			TTL:      *cacheTTL,
			Interval: *cacheGcInterval,
		},
	}

	promCache, err := cache.NewCache(cacheConfig, db, promApiClient, *scrapeInterval)
	if err != nil {
		klog.Exitln(err)
	}
	focus := &Focus{constructor: constructor.New(promCache.GetCacheClient(), *scrapeInterval)}

	r := mux.NewRouter()
	r.HandleFunc("/health", focus.Health).Methods(http.MethodGet)
	r.HandleFunc("/api/overview", focus.Overview).Methods(http.MethodGet)
	r.HandleFunc("/api/app/{app}", focus.App).Methods(http.MethodGet)

	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	r.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/index.html")
	})

	klog.Infoln("listening on :8080")
	klog.Fatalln(http.ListenAndServe(":8080", r))
}
