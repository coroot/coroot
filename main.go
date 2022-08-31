package main

import (
	"github.com/coroot/coroot-focus/api"
	"github.com/coroot/coroot-focus/cache"
	"github.com/coroot/coroot-focus/db"
	"github.com/coroot/coroot-focus/prom"
	"github.com/coroot/coroot-focus/utils"
	"github.com/gorilla/mux"
	"gopkg.in/alecthomas/kingpin.v2"
	"k8s.io/klog"
	"net/http"
	"path"
)

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
	promApiClient, err := prom.NewApiClient(*prometheusUrl, *skipTlsVerify)
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

	api := api.NewApi(promCache, db)

	r := mux.NewRouter()
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {}).Methods(http.MethodGet)

	r.HandleFunc("/api/projects", api.Projects).Methods(http.MethodGet)
	r.HandleFunc("/api/project/", api.Project).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}", api.Project).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/overview", api.Overview).Methods(http.MethodGet)
	r.HandleFunc("/api/project/{project}/search", api.Search).Methods(http.MethodGet)
	r.HandleFunc("/api/project/{project}/app/{app}", api.App).Methods(http.MethodGet)
	r.HandleFunc("/api/project/{project}/node/{node}", api.Node).Methods(http.MethodGet)

	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	r.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/index.html")
	})

	klog.Infoln("listening on :8080")
	klog.Fatalln(http.ListenAndServe(":8080", r))
}
