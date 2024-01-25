package main

import (
	"bytes"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path"
	"strings"
	"text/template"
	"time"

	"github.com/coroot/coroot/api"
	"github.com/coroot/coroot/cache"
	cloud_pricing "github.com/coroot/coroot/cloud-pricing"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/stats"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/coroot/coroot/watchers"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gopkg.in/alecthomas/kingpin.v2"
	"k8s.io/klog"
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
	doNotCheckSLO := kingpin.Flag("do-not-check-slo", "don't check SLO compliance").Envar("DO_NOT_CHECK_SLO").Bool()
	doNotCheckForDeployments := kingpin.Flag("do-not-check-for-deployments", "don't check for new deployments").Envar("DO_NOT_CHECK_FOR_DEPLOYMENTS").Bool()
	doNotCheckForUpdates := kingpin.Flag("do-not-check-for-updates", "don't check for new versions").Envar("DO_NOT_CHECK_FOR_UPDATES").Bool()
	bootstrapClickhouseAddr := kingpin.Flag("bootstrap-clickhouse-address", "if set, Coroot will add a Clickhouse integration for the default project").Envar("BOOTSTRAP_CLICKHOUSE_ADDRESS").String()
	bootstrapClickhouseUser := kingpin.Flag("bootstrap-clickhouse-user", "Clickhouse user").Envar("BOOTSTRAP_CLICKHOUSE_USER").Default("default").String()
	bootstrapClickhousePassword := kingpin.Flag("bootstrap-clickhouse-password", "Clickhouse password").Envar("BOOTSTRAP_CLICKHOUSE_PASSWORD").String()
	bootstrapClickhouseDatabase := kingpin.Flag("bootstrap-clickhouse-database", "Clickhouse database").Envar("BOOTSTRAP_CLICKHOUSE_DATABASE").Default("default").String()
	bootstrapClickhouseTracesTable := kingpin.Flag("bootstrap-clickhouse-traces-table", "Clickhouse traces table").Envar("BOOTSTRAP_CLICKHOUSE_TRACES_TABLE").Default("otel_traces").String()
	bootstrapClickhouseLogsTable := kingpin.Flag("bootstrap-clickhouse-logs-table", "Clickhouse logs table").Envar("BOOTSTRAP_CLICKHOUSE_LOGS_TABLE").Default("otel_logs").String()

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
	if err = database.MigrateDefault(); err != nil {
		klog.Exitln(err)
	}

	bootstrapPrometheus(database, *bootstrapPrometheusUrl, *bootstrapRefreshInterval, *bootstrapPrometheusExtraSelector)
	bootstrapClickhouse(database, *bootstrapClickhouseAddr, *bootstrapClickhouseUser, *bootstrapClickhousePassword, *bootstrapClickhouseDatabase, *bootstrapClickhouseTracesTable, *bootstrapClickhouseLogsTable)

	cacheConfig := cache.Config{
		Path: path.Join(*dataDir, "cache"),
		GC: &cache.GcConfig{
			TTL:      *cacheTTL,
			Interval: *cacheGcInterval,
		},
	}
	if err = utils.CreateDirectoryIfNotExists(cacheConfig.Path); err != nil {
		klog.Exitln(err)
	}
	cacheState, err := db.Open(cacheConfig.Path, "")
	promCache, err := cache.NewCache(cacheConfig, database, cacheState, cache.DefaultPrometheusClientFactory)
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

	watchers.Start(database, promCache, pricing, !*doNotCheckSLO, !*doNotCheckForDeployments)

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
	r.HandleFunc("/api/project/{project}/configs", a.Configs).Methods(http.MethodGet)
	r.HandleFunc("/api/project/{project}/categories", a.Categories).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/integrations", a.Integrations).Methods(http.MethodGet, http.MethodPut)
	r.HandleFunc("/api/project/{project}/integrations/{type}", a.Integration).Methods(http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPost)
	r.HandleFunc("/api/project/{project}/app/{app}", a.App).Methods(http.MethodGet)
	r.HandleFunc("/api/project/{project}/app/{app}/check/{check}/config", a.Check).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/app/{app}/profile", a.Profile).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/app/{app}/tracing", a.Tracing).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/app/{app}/logs", a.Logs).Methods(http.MethodGet, http.MethodPost)
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

func getOrCreateDefaultProject(database *db.DB) *db.Project {
	projects, err := database.GetProjects()
	if err != nil {
		klog.Exitln(err)
	}
	switch len(projects) {
	case 0:
		p := db.Project{Name: "default"}
		klog.Infof("creating default project")
		if p.Id, err = database.SaveProject(p); err != nil {
			klog.Exitln(err)
		}
		return &p
	case 1:
		return projects[0]
	}
	return nil
}

func bootstrapPrometheus(database *db.DB, url string, refreshInterval time.Duration, extraSelector string) {
	if url == "" || refreshInterval == 0 {
		return
	}
	if !prom.IsSelectorValid(extraSelector) {
		klog.Exitf("invalid Prometheus extra selector: %s", extraSelector)
	}
	p := getOrCreateDefaultProject(database)
	if p == nil {
		return
	}
	if p.Prometheus.Url != "" {
		return
	}
	p.Prometheus = db.IntegrationsPrometheus{
		Url:             url,
		RefreshInterval: timeseries.Duration(int64((refreshInterval).Seconds())),
		ExtraSelector:   extraSelector,
	}
	if err := database.SaveProjectIntegration(p, db.IntegrationTypePrometheus); err != nil {
		klog.Exitln(err)
	}
}

func bootstrapClickhouse(database *db.DB, addr, user, password, databaseName, tracesTable, logsTable string) {
	if addr == "" || user == "" || password == "" || databaseName == "" {
		return
	}
	p := getOrCreateDefaultProject(database)
	if p == nil {
		return
	}
	var save bool
	if cfg := p.Settings.Integrations.Clickhouse; cfg == nil {
		p.Settings.Integrations.Clickhouse = &db.IntegrationClickhouse{
			Protocol: "native",
			Addr:     addr,
			Auth: utils.BasicAuth{
				User:     user,
				Password: password,
			},
			Database:    databaseName,
			TracesTable: tracesTable,
			LogsTable:   logsTable,
		}
		save = true
	} else {
		if cfg.TracesTable == "" && tracesTable != "" {
			cfg.TracesTable = tracesTable
			save = true
		}
		if cfg.LogsTable == "" && logsTable != "" {
			cfg.LogsTable = logsTable
			save = true
		}
	}
	if !save {
		return
	}
	if err := database.SaveProjectIntegration(p, db.IntegrationTypeClickhouse); err != nil {
		klog.Exitln(err)
	}
}
