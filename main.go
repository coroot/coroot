package main

import (
	"github.com/coroot/coroot/api"
	"github.com/coroot/coroot/cache"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/utils"
	"github.com/gorilla/mux"
	"gopkg.in/alecthomas/kingpin.v2"
	"k8s.io/klog"
	"net/http"
	"path"
)

func main() {
	listen := kingpin.Flag("listen", "listen address - ip:port or :port").Default("0.0.0.0:8080").String()
	dataDir := kingpin.Flag("datadir", `path to data directory`).Required().String()
	cacheTTL := kingpin.Flag("cache-ttl", `cache TTL`).Default("720h").Duration()
	cacheGcInterval := kingpin.Flag("cache-gc-interval", `cache GC interval`).Default("10m").Duration()

	kingpin.Parse()

	if err := utils.CreateDirectoryIfNotExists(*dataDir); err != nil {
		klog.Exitln(err)
	}
	db, err := db.Open(path.Join(*dataDir, "db.sqlite"))
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

	promCache, err := cache.NewCache(cacheConfig, db)
	if err != nil {
		klog.Exitln(err)
	}

	api := api.NewApi(promCache, db)

	r := mux.NewRouter()
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {}).Methods(http.MethodGet)

	r.HandleFunc("/api/projects", api.Projects).Methods(http.MethodGet)
	r.HandleFunc("/api/project/", api.Project).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}", api.Project).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/status", api.Status).Methods(http.MethodGet)
	r.HandleFunc("/api/project/{project}/overview", api.Overview).Methods(http.MethodGet)
	r.HandleFunc("/api/project/{project}/search", api.Search).Methods(http.MethodGet)
	r.HandleFunc("/api/project/{project}/app/{app}", api.App).Methods(http.MethodGet)
	r.HandleFunc("/api/project/{project}/node/{node}", api.Node).Methods(http.MethodGet)

	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	r.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/index.html")
	})

	klog.Infoln("listening on", *listen)
	klog.Fatalln(http.ListenAndServe(*listen, r))
}
