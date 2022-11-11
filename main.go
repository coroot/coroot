package main

import (
	"github.com/coroot/coroot/alerts"
	"github.com/coroot/coroot/api"
	"github.com/coroot/coroot/cache"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/stats"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/gorilla/mux"
	"gopkg.in/alecthomas/kingpin.v2"
	"k8s.io/klog"
	"net/http"
	_ "net/http/pprof"
	"path"
)

var version = "unknown"

func main() {
	listen := kingpin.Flag("listen", "listen address - ip:port or :port").Envar("LISTEN").Default("0.0.0.0:8080").String()
	dataDir := kingpin.Flag("data-dir", `path to data directory`).Envar("DATA_DIR").Default("/data").String()
	cacheTTL := kingpin.Flag("cache-ttl", "cache TTL").Envar("CACHE_TTL").Default("720h").Duration()
	cacheGcInterval := kingpin.Flag("cache-gc-interval", "cache GC interval").Envar("CACHE_GC_INTERVAL").Default("10m").Duration()
	pgConnString := kingpin.Flag("pg-connection-string", "Postgres connection string (sqlite is used if not set)").Envar("PG_CONNECTION_STRING").String()
	disableStats := kingpin.Flag("disable-usage-statistics", "disable usage statistics").Envar("DISABLE_USAGE_STATISTICS").Bool()
	readOnly := kingpin.Flag("read-only", "enable the read-only mode when configuration changes don't take effect").Envar("READ_ONLY").Bool()
	bootstrapPrometheusUrl := kingpin.Flag("bootstrap-prometheus-url", "if set, Coroot will create a project for this Prometheus URL").Envar("BOOTSTRAP_PROMETHEUS_URL").String()
	bootstrapRefreshInterval := kingpin.Flag("bootstrap-refresh-interval", "refresh interval for the project created upon bootstrap").Envar("BOOTSTRAP_REFRESH_INTERVAL").Duration()
	sloCheckInterval := kingpin.Flag("slo-check-interval", "how often to check SLO compliance").Envar("SLO_CHECK_INTERVAL").Default("1m").Duration()

	kingpin.Version(version)
	kingpin.Parse()

	klog.Infof("version: %s, read-only: %t", version, *readOnly)

	if err := utils.CreateDirectoryIfNotExists(*dataDir); err != nil {
		klog.Exitln(err)
	}

	database, err := db.Open(*dataDir, *pgConnString)
	if err != nil {
		klog.Exitln(err)
	}

	if *bootstrapPrometheusUrl != "" && *bootstrapRefreshInterval > 0 {
		projects, err := database.GetProjectNames()
		if err != nil {
			klog.Exitln(err)
		}
		if len(projects) == 0 {
			p := db.Project{
				Name: "default",
				Prometheus: db.Prometheus{
					Url:             *bootstrapPrometheusUrl,
					RefreshInterval: timeseries.Duration(int64((*bootstrapRefreshInterval).Seconds())),
				},
			}
			klog.Infof("creating project: %s(%s, %s)", p.Name, *bootstrapPrometheusUrl, *bootstrapRefreshInterval)
			if _, err := database.SaveProject(p); err != nil {
				klog.Exitln(err)
			}
		}
	}

	cacheConfig := cache.Config{
		Path: path.Join(*dataDir, "cache"),
		GC: &cache.GcConfig{
			TTL:      *cacheTTL,
			Interval: *cacheGcInterval,
		},
	}
	promCache, err := cache.NewCache(cacheConfig, database)
	if err != nil {
		klog.Exitln(err)
	}

	var statsCollector *stats.Collector
	if !*disableStats {
		statsCollector = stats.NewCollector(*dataDir, version, database, promCache)
	}

	if *sloCheckInterval > 0 {
		alerts.NewAlertManager(database, promCache).Start(*sloCheckInterval)
	}

	api := api.NewApi(promCache, database, statsCollector, *readOnly)

	r := mux.NewRouter()
	r.PathPrefix("/debug/pprof/").Handler(http.DefaultServeMux)
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {}).Methods(http.MethodGet)

	r.HandleFunc("/api/projects", api.Projects).Methods(http.MethodGet)
	r.HandleFunc("/api/project/", api.Project).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}", api.Project).Methods(http.MethodGet, http.MethodPost, http.MethodDelete)
	r.HandleFunc("/api/project/{project}/status", api.Status).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/overview", api.Overview).Methods(http.MethodGet)
	r.HandleFunc("/api/project/{project}/search", api.Search).Methods(http.MethodGet)
	r.HandleFunc("/api/project/{project}/configs", api.Configs).Methods(http.MethodGet)
	r.HandleFunc("/api/project/{project}/categories", api.Categories).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/integrations", api.Integrations).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/integrations/slack", api.IntegrationsSlack).Methods(http.MethodGet, http.MethodPost, http.MethodDelete)
	r.HandleFunc("/api/project/{project}/app/{app}", api.App).Methods(http.MethodGet)
	r.HandleFunc("/api/project/{project}/app/{app}/check/{check}/config", api.Check).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/node/{node}", api.Node).Methods(http.MethodGet)
	r.PathPrefix("/api/project/{project}/prom").HandlerFunc(api.Prom)

	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	r.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/index.html")
	})

	klog.Infoln("listening on", *listen)
	klog.Fatalln(http.ListenAndServe(*listen, r))
}
