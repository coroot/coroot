package main

import (
	"bytes"
	"embed"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"text/template"

	"github.com/coroot/coroot/api"
	"github.com/coroot/coroot/cache"
	cloud_pricing "github.com/coroot/coroot/cloud-pricing"
	"github.com/coroot/coroot/collector"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/rbac"
	"github.com/coroot/coroot/stats"
	"github.com/coroot/coroot/utils"
	"github.com/coroot/coroot/watchers"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"golang.org/x/term"
	"gopkg.in/alecthomas/kingpin.v2"
	"k8s.io/klog"
)

var version = "unknown"

//go:embed static
var static embed.FS

func main() {
	listen := kingpin.Flag("listen", "Listen address - ip:port or :port").Envar("LISTEN").Default("0.0.0.0:8080").String()
	urlBasePath := kingpin.Flag("url-base-path", "The base URL to run Coroot at a sub-path, e.g. /coroot/").Envar("URL_BASE_PATH").Default("/").String()
	dataDir := kingpin.Flag("data-dir", `Path to the data directory`).Envar("DATA_DIR").Default("./data").String()
	cacheTTL := kingpin.Flag("cache-ttl", "Cache TTL").Envar("CACHE_TTL").Default("720h").Duration()
	cacheGcInterval := kingpin.Flag("cache-gc-interval", "Cache GC interval").Envar("CACHE_GC_INTERVAL").Default("10m").Duration()
	pgConnString := kingpin.Flag("pg-connection-string", "Postgres connection string (sqlite is used if not set)").Envar("PG_CONNECTION_STRING").String()
	disableStats := kingpin.Flag("disable-usage-statistics", "Disable usage statistics").Envar("DISABLE_USAGE_STATISTICS").Bool()
	bootstrapPrometheusUrl := kingpin.Flag("bootstrap-prometheus-url", "If set, Coroot will create a project for this Prometheus URL").Envar("BOOTSTRAP_PROMETHEUS_URL").String()
	bootstrapRefreshInterval := kingpin.Flag("bootstrap-refresh-interval", "Refresh interval for the project created upon bootstrap").Envar("BOOTSTRAP_REFRESH_INTERVAL").Duration()
	bootstrapPrometheusExtraSelector := kingpin.Flag("bootstrap-prometheus-extra-selector", "Prometheus extra selector for the project created upon bootstrap").Envar("BOOTSTRAP_PROMETHEUS_EXTRA_SELECTOR").String()
	doNotCheckSLO := kingpin.Flag("do-not-check-slo", "Don't check SLO compliance").Envar("DO_NOT_CHECK_SLO").Bool()
	doNotCheckForDeployments := kingpin.Flag("do-not-check-for-deployments", "Don't check for new deployments").Envar("DO_NOT_CHECK_FOR_DEPLOYMENTS").Bool()
	doNotCheckForUpdates := kingpin.Flag("do-not-check-for-updates", "Don't check for new versions").Envar("DO_NOT_CHECK_FOR_UPDATES").Bool()
	bootstrapClickhouseAddr := kingpin.Flag("bootstrap-clickhouse-address", "If set, Coroot will add a Clickhouse integration for the default project").Envar("BOOTSTRAP_CLICKHOUSE_ADDRESS").String()
	bootstrapClickhouseUser := kingpin.Flag("bootstrap-clickhouse-user", "Clickhouse user").Envar("BOOTSTRAP_CLICKHOUSE_USER").Default("default").String()
	bootstrapClickhousePassword := kingpin.Flag("bootstrap-clickhouse-password", "Clickhouse password").Envar("BOOTSTRAP_CLICKHOUSE_PASSWORD").String()
	bootstrapClickhouseDatabase := kingpin.Flag("bootstrap-clickhouse-database", "Clickhouse database").Envar("BOOTSTRAP_CLICKHOUSE_DATABASE").Default("default").String()
	developerMode := kingpin.Flag("developer-mode", "If enabled, Coroot will not use embedded static assets").Envar("DEVELOPER_MODE").Default("false").Bool()
	authAnonymousRole := kingpin.Flag("auth-anonymous-role", "Disable authentication and assign one of the following roles to the anonymous user: Admin, Editor, or Viewer.").Envar("AUTH_ANONYMOUS_ROLE").String()
	authBootstrapAdminPassword := kingpin.Flag("auth-bootstrap-admin-password", "Password for the default Admin user").Envar("AUTH_BOOTSTRAP_ADMIN_PASSWORD").Default(db.AdminUserDefaultPassword).String()

	kingpin.Command("run", "Run Coroot server").Default()
	cmdSetAdminPassword := kingpin.Command("set-admin-password", "Set password for the default Admin user")

	cmd := kingpin.Parse()

	klog.Infof("version: %s", version)

	if err := utils.CreateDirectoryIfNotExists(*dataDir); err != nil {
		klog.Exitln(err)
	}

	database, err := db.Open(*dataDir, *pgConnString)
	if err != nil {
		klog.Exitln(err)
	}
	if err = database.Migrate(); err != nil {
		klog.Exitln(err)
	}

	switch cmd {
	case cmdSetAdminPassword.FullCommand():
		err = setAdminPassword(database)
		if err != nil {
			fmt.Println("Failed to set admin password:", err)
		} else {
			fmt.Println("Admin password set successfully.")
		}
		return
	}

	if err = database.BootstrapPrometheusIntegration(*bootstrapPrometheusUrl, *bootstrapRefreshInterval, *bootstrapPrometheusExtraSelector); err != nil {
		klog.Exitln(err)
	}
	if err = database.BootstrapClickhouseIntegration(*bootstrapClickhouseAddr, *bootstrapClickhouseUser, *bootstrapClickhousePassword, *bootstrapClickhouseDatabase); err != nil {
		klog.Exitln(err)
	}

	cacheConfig := cache.Config{
		Path: path.Join(*dataDir, "cache"),
		GC: &cache.GcConfig{
			TTL:      *cacheTTL,
			Interval: *cacheGcInterval,
		},
	}
	promCache, err := cache.NewCache(cacheConfig, database, cache.DefaultPrometheusClientFactory)
	if err != nil {
		klog.Exitln(err)
	}

	coll := collector.New(database, promCache)
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		coll.Close()
		os.Exit(0)
	}()

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

	a := api.NewApi(promCache, database, coll, pricing, rbac.NewStaticRoleManager())
	err = a.AuthInit(*authAnonymousRole, *authBootstrapAdminPassword)
	if err != nil {
		klog.Exitln(err)
	}

	router := mux.NewRouter()
	router.PathPrefix("/debug/pprof/").Handler(http.DefaultServeMux)
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {}).Methods(http.MethodGet)

	router.HandleFunc("/v1/metrics", coll.Metrics)
	router.HandleFunc("/v1/traces", coll.Traces)
	router.HandleFunc("/v1/logs", coll.Logs)
	router.HandleFunc("/v1/profiles", coll.Profiles)
	router.HandleFunc("/v1/config", coll.Config)

	r := router
	cleanUrlBasePath(urlBasePath)
	if *urlBasePath != "/" {
		r = router.PathPrefix(strings.TrimRight(*urlBasePath, "/")).Subrouter()
	}
	r.HandleFunc("/api/login", a.Login).Methods(http.MethodPost)
	r.HandleFunc("/api/logout", a.Logout).Methods(http.MethodPost)

	r.HandleFunc("/api/user", a.Auth(a.User)).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/users", a.Auth(a.Users)).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/roles", a.Auth(a.Roles)).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/", a.Auth(a.Project)).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}", a.Auth(a.Project)).Methods(http.MethodGet, http.MethodPost, http.MethodDelete)
	r.HandleFunc("/api/project/{project}/status", a.Auth(a.Status)).Methods(http.MethodGet)
	r.HandleFunc("/api/project/{project}/overview/{view}", a.Auth(a.Overview)).Methods(http.MethodGet)
	r.HandleFunc("/api/project/{project}/inspections", a.Auth(a.Inspections)).Methods(http.MethodGet)
	r.HandleFunc("/api/project/{project}/categories", a.Auth(a.Categories)).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/custom_applications", a.Auth(a.CustomApplications)).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/integrations", a.Auth(a.Integrations)).Methods(http.MethodGet, http.MethodPut)
	r.HandleFunc("/api/project/{project}/integrations/{type}", a.Auth(a.Integration)).Methods(http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPost)
	r.HandleFunc("/api/project/{project}/app/{app}", a.Auth(a.Application)).Methods(http.MethodGet)
	r.HandleFunc("/api/project/{project}/app/{app}/inspection/{type}/config", a.Auth(a.Inspection)).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/app/{app}/instrumentation/{type}", a.Auth(a.Instrumentation)).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/app/{app}/profiling", a.Auth(a.Profiling)).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/app/{app}/tracing", a.Auth(a.Tracing)).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/app/{app}/logs", a.Auth(a.Logs)).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/node/{node}", a.Auth(a.Node)).Methods(http.MethodGet)
	r.PathPrefix("/api/project/{project}/prom").HandlerFunc(a.Auth(a.Prom))

	r.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		statsCollector.RegisterRequest(r)
	}).Methods(http.MethodPost)

	if *developerMode {
		r.PathPrefix("/static/").Handler(http.StripPrefix(*urlBasePath+"static/", http.FileServer(http.Dir("./static"))))
	} else {
		r.PathPrefix("/static/").Handler(http.StripPrefix(*urlBasePath, http.FileServer(utils.NewStaticFSWrapper(static))))
	}

	indexHtml := readIndexHtml(*urlBasePath, version, instanceUuid, !*doNotCheckForUpdates, *developerMode)
	r.PathPrefix("").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(indexHtml)
	})

	router.PathPrefix("").Handler(http.RedirectHandler(*urlBasePath, http.StatusMovedPermanently))

	klog.Infoln("listening on", *listen)
	klog.Fatalln(http.ListenAndServe(*listen, router))
}

func readIndexHtml(basePath, version, instanceUuid string, checkForUpdates bool, developerMode bool) []byte {
	var (
		err error
		tpl *template.Template
	)
	if developerMode {
		tpl, err = template.ParseFiles("./static/index.html")
	} else {
		tpl, err = template.ParseFS(static, "static/index.html")
	}
	if err != nil {
		klog.Exitln(err)
	}
	buf := bytes.Buffer{}
	err = tpl.Execute(&buf, struct {
		BasePath        string
		Version         string
		Uuid            string
		CheckForUpdates bool
	}{
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

func setAdminPassword(db *db.DB) error {
	fmt.Print("Enter new password: ")
	data, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println("")
	if err != nil {
		return err
	}
	password := string(data)
	if password == "" {
		return fmt.Errorf("password cannot be blank")
	}
	fmt.Print("Confirm new password: ")
	data, err = term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println("")
	if err != nil {
		return err
	}
	confirm := string(data)
	if password != confirm {
		return fmt.Errorf("passwords do not match")
	}
	err = db.CreateAdminIfNotExists(password)
	if err != nil {
		return err
	}
	err = db.SetAdminPassword(password)
	if err != nil {
		return err
	}
	return nil
}
