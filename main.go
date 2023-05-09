package main

import (
	"bytes"
	"github.com/coroot/coroot/api"
	"github.com/coroot/coroot/cache"
	"github.com/coroot/coroot/cloud-pricing"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/notifications"
	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/stats"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/coroot/coroot/watchers/deployments"
	"github.com/coroot/coroot/watchers/incidents"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gopkg.in/alecthomas/kingpin.v2"
	"k8s.io/klog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path"
	"strings"
	"text/template"
	"time"
)

var version = "unknown"

func main() {
	listen := kingpin.Flag("listen", "listen address - ip:port or :port").Envar("LISTEN").Default("0.0.0.0:8080").String()
	urlBasePath := kingpin.Flag("url-base-path", "the base URL to run Coroot at a sub-path, e.g. /coroot/").Envar("URL_BASE_PATH").Default("/").String()
	dataDir := kingpin.Flag("data-dir", `path to the data directory`).Envar("DATA_DIR").Default("/data").String()
	cacheTTL := kingpin.Flag("cache-ttl", "cache TTL").Envar("CACHE_TTL").Default("720h").Duration()
	cacheGcInterval := kingpin.Flag("cache-gc-interval", "cache GC interval").Envar("CACHE_GC_INTERVAL").Default("10m").Duration()
	pgConnString := kingpin.Flag("pg-connection-string", "Postgres connection string (sqlite is used if not set)").Envar("PG_CONNECTION_STRING").String()
	disableStats := kingpin.Flag("disable-usage-statistics", "disable usage statistics").Envar("DISABLE_USAGE_STATISTICS").Bool()
	readOnly := kingpin.Flag("read-only", "enable the read-only mode when configuration changes don't take effect").Envar("READ_ONLY").Bool()
	bootstrapPrometheusUrl := kingpin.Flag("bootstrap-prometheus-url", "if set, Coroot will create a project for this Prometheus URL").Envar("BOOTSTRAP_PROMETHEUS_URL").String()
	bootstrapRefreshInterval := kingpin.Flag("bootstrap-refresh-interval", "refresh interval for the project created upon bootstrap").Envar("BOOTSTRAP_REFRESH_INTERVAL").Duration()
	bootstrapPrometheusExtraSelector := kingpin.Flag("bootstrap-prometheus-extra-selector", "Prometheus extra selector for the project created upon bootstrap").Envar("BOOTSTRAP_PROMETHEUS_EXTRA_SELECTOR").String()
	sloCheckInterval := kingpin.Flag("slo-check-interval", "how often to check SLO compliance").Envar("SLO_CHECK_INTERVAL").Default("1m").Duration()
	deploymentsWatchInterval := kingpin.Flag("deployments-watch-interval", "how often to check new deployments").Envar("DEPLOYMENTS_WATCH_INTERVAL").Default("1m").Duration()
	doNotCheckForUpdates := kingpin.Flag("do-not-check-for-updates", "don't check for new versions").Envar("DO_NOT_CHECK_FOR_UPDATES").Bool()
	bootstrapPyroscopeUrl := kingpin.Flag("bootstrap-pyroscope-url", "if set, Coroot will add a Pyroscope integration for the default project").Envar("BOOTSTRAP_PYROSCOPE_URL").String()
	bootstrapClickhouseAddr := kingpin.Flag("bootstrap-clickhouse-address", "if set, Coroot will add a Clickhouse integration for the default project").Envar("BOOTSTRAP_CLICKHOUSE_ADDRESS").String()
	bootstrapClickhouseUser := kingpin.Flag("bootstrap-clickhouse-user", "Clickhouse user").Envar("BOOTSTRAP_CLICKHOUSE_USER").Default("default").String()
	bootstrapClickhousePassword := kingpin.Flag("bootstrap-clickhouse-password", "Clickhouse password").Envar("BOOTSTRAP_CLICKHOUSE_PASSWORD").String()
	bootstrapClickhouseDatabase := kingpin.Flag("bootstrap-clickhouse-database", "Clickhouse database").Envar("BOOTSTRAP_CLICKHOUSE_DATABASE").Default("default").String()
	bootstrapClickhouseTracesTable := kingpin.Flag("bootstrap-clickhouse-traces-table", "Clickhouse traces table").Envar("BOOTSTRAP_CLICKHOUSE_TRACES_TABLE").Default("otel_traces").String()

	kingpin.Version(version)
	kingpin.Parse()

	klog.Infof("version: %s, url-base-path: %s, read-only: %t", version, *urlBasePath, *readOnly)

	if err := utils.CreateDirectoryIfNotExists(*dataDir); err != nil {
		klog.Exitln(err)
	}

	database, err := db.Open(*dataDir, *pgConnString)
	if err != nil {
		klog.Exitln(err)
	}

	bootstrapPrometheus(database, *bootstrapPrometheusUrl, *bootstrapRefreshInterval, *bootstrapPrometheusExtraSelector)
	bootstrapPyroscope(database, *bootstrapPyroscopeUrl)
	bootstrapClickhouse(database, *bootstrapClickhouseAddr, *bootstrapClickhouseUser, *bootstrapClickhousePassword, *bootstrapClickhouseDatabase, *bootstrapClickhouseTracesTable)

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

	pricing, err := cloud_pricing.NewManager(path.Join(*dataDir, "cloud-pricing"))
	if err != nil {
		klog.Exitln(err)
	}

	instanceUuid := getInstanceUuid(*dataDir)

	var statsCollector *stats.Collector
	if !*disableStats {
		statsCollector = stats.NewCollector(instanceUuid, version, database, promCache, pricing)
	}

	notifier := notifications.NewIncidentNotifier(database)

	if *sloCheckInterval > 0 {
		incidents.NewWatcher(database, promCache, notifier).Start(*sloCheckInterval)
	}

	if *deploymentsWatchInterval > 0 {
		deployments.NewWatcher(database, promCache, pricing).Start(*deploymentsWatchInterval)
	}

	a := api.NewApi(promCache, database, pricing, *readOnly)

	router := mux.NewRouter()
	router.PathPrefix("/debug/pprof/").Handler(http.DefaultServeMux)
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {}).Methods(http.MethodGet)

	r := router
	cleanUrlBasePath(urlBasePath)
	if *urlBasePath != "/" {
		r = router.PathPrefix(strings.TrimRight(*urlBasePath, "/")).Subrouter()
	}
	r.HandleFunc("/api/projects", a.Projects).Methods(http.MethodGet)
	r.HandleFunc("/api/project/", a.Project).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}", a.Project).Methods(http.MethodGet, http.MethodPost, http.MethodDelete)
	r.HandleFunc("/api/project/{project}/status", a.Status).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/overview/{view}", a.Overview).Methods(http.MethodGet)
	r.HandleFunc("/api/project/{project}/search", a.Search).Methods(http.MethodGet)
	r.HandleFunc("/api/project/{project}/configs", a.Configs).Methods(http.MethodGet)
	r.HandleFunc("/api/project/{project}/categories", a.Categories).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/integrations", a.Integrations).Methods(http.MethodGet, http.MethodPut)
	r.HandleFunc("/api/project/{project}/integrations/{type}", a.Integration).Methods(http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPost)
	r.HandleFunc("/api/project/{project}/app/{app}", a.App).Methods(http.MethodGet)
	r.HandleFunc("/api/project/{project}/app/{app}/check/{check}/config", a.Check).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/app/{app}/profile", a.Profile).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/app/{app}/tracing", a.Tracing).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/node/{node}", a.Node).Methods(http.MethodGet)
	r.PathPrefix("/api/project/{project}/prom").HandlerFunc(a.Prom)

	r.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		statsCollector.RegisterRequest(r)
	}).Methods(http.MethodPost)

	r.PathPrefix("/static/").Handler(http.StripPrefix(*urlBasePath+"static/", http.FileServer(http.Dir("./static"))))

	indexHtml := readIndexHtml(*urlBasePath, version, instanceUuid, !*doNotCheckForUpdates)
	r.PathPrefix("").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(indexHtml)
	})

	router.PathPrefix("").Handler(http.RedirectHandler(*urlBasePath, http.StatusMovedPermanently))

	klog.Infoln("listening on", *listen)
	klog.Fatalln(http.ListenAndServe(*listen, router))
}

type Options struct {
	BasePath        string
	Version         string
	Uuid            string
	CheckForUpdates bool
}

func readIndexHtml(basePath, version, instanceUuid string, checkForUpdates bool) []byte {
	tpl, err := template.ParseFiles("./static/index.html")
	if err != nil {
		klog.Exitln(err)
	}
	buf := bytes.Buffer{}
	err = tpl.Execute(&buf, Options{
		BasePath:        basePath,
		Version:         version,
		Uuid:            instanceUuid,
		CheckForUpdates: checkForUpdates,
	})
	if err != nil {
		klog.Exitln(err)
	}
	return buf.Bytes()
}

func cleanUrlBasePath(urlBasePath *string) {
	bp := strings.Trim(*urlBasePath, "/")
	if bp == "" {
		bp = "/"
	} else {
		bp = "/" + bp + "/"
	}
	*urlBasePath = bp
}

func getInstanceUuid(dataDir string) string {
	instanceUuid := ""
	filePath := path.Join(dataDir, "instance.uuid")
	data, err := os.ReadFile(filePath)
	if err != nil && !os.IsNotExist(err) {
		klog.Errorln("failed to read instance id:", err)
	}
	instanceUuid = strings.TrimSpace(string(data))
	if _, err := uuid.Parse(instanceUuid); err != nil {
		instanceUuid = uuid.NewString()
		if err := os.WriteFile(filePath, []byte(instanceUuid), 0644); err != nil {
			klog.Errorln("failed to write instance id:", err)
			return ""
		}
	}
	return instanceUuid
}

func bootstrapPrometheus(database *db.DB, url string, refreshInterval time.Duration, extraSelector string) {
	if url == "" || refreshInterval == 0 {
		return
	}
	projects, err := database.GetProjectNames()
	if err != nil {
		klog.Exitln(err)
	}
	if len(projects) == 0 {
		if !prom.IsSelectorValid(extraSelector) {
			klog.Exitf("invalid Prometheus extra selector: %s", extraSelector)
		}
		p := db.Project{
			Name: "default",
			Prometheus: db.IntegrationsPrometheus{
				Url:             url,
				RefreshInterval: timeseries.Duration(int64((refreshInterval).Seconds())),
				ExtraSelector:   extraSelector,
			},
		}
		klog.Infof("creating project: %s(%s, %s)", p.Name, url, refreshInterval)
		if p.Id, err = database.SaveProject(p); err != nil {
			klog.Exitln(err)
		}
		if err := database.SaveProjectIntegration(&p, db.IntegrationTypePrometheus); err != nil {
			klog.Exitln(err)
		}
	}
}

func bootstrapPyroscope(database *db.DB, url string) {
	if url == "" {
		return
	}
	projects, err := database.GetProjects()
	if err != nil {
		klog.Exitln(err)
	}
	if len(projects) != 1 {
		return
	}
	project := projects[0]
	if project.Settings.Integrations.Pyroscope != nil {
		return
	}
	project.Settings.Integrations.Pyroscope = &db.IntegrationPyroscope{Url: url}
	if err := database.SaveProjectIntegration(project, db.IntegrationTypePyroscope); err != nil {
		klog.Exitln(err)
	}
}

func bootstrapClickhouse(database *db.DB, addr, user, password, databaseName, tracesTable string) {
	if addr == "" || user == "" || password == "" || databaseName == "" || tracesTable == "" {
		return
	}
	projects, err := database.GetProjects()
	if err != nil {
		klog.Exitln(err)
	}
	if len(projects) != 1 {
		return
	}
	project := projects[0]
	if project.Settings.Integrations.Clickhouse != nil {
		return
	}
	project.Settings.Integrations.Clickhouse = &db.IntegrationClickhouse{
		Protocol: "native",
		Addr:     addr,
		Auth: utils.BasicAuth{
			User:     user,
			Password: password,
		},
		Database:    databaseName,
		TracesTable: tracesTable,
	}
	if err := database.SaveProjectIntegration(project, db.IntegrationTypeClickhouse); err != nil {
		klog.Exitln(err)
	}
}
