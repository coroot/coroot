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
	"syscall"
	"text/template"

	"github.com/coroot/coroot/api"
	"github.com/coroot/coroot/cache"
	cloud_pricing "github.com/coroot/coroot/cloud-pricing"
	"github.com/coroot/coroot/collector"
	"github.com/coroot/coroot/config"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/rbac"
	"github.com/coroot/coroot/stats"
	"github.com/coroot/coroot/utils"
	"github.com/coroot/coroot/watchers"
	"github.com/gorilla/mux"
	"golang.org/x/term"
	"gopkg.in/alecthomas/kingpin.v2"
	"k8s.io/klog"
)

var version = "unknown"

//go:embed static
var static embed.FS

func main() {
	kingpin.Command("run", "Run Coroot server").Default()
	cmdSetAdminPassword := kingpin.Command("set-admin-password", "Set password for the default Admin user")

	cmd := kingpin.Parse()

	klog.Infof("version: %s", version)

	cfg := config.Load()

	var err error
	if err = utils.CreateDirectoryIfNotExists(cfg.DataDir); err != nil {
		klog.Exitln(err)
	}

	var database *db.DB
	if cfg.Postgres != nil && cfg.Postgres.ConnectionString != "" {
		klog.Infoln("database type: postgres")
		database, err = db.NewPostgres(cfg.Postgres.ConnectionString)
	} else {
		klog.Infoln("database type: sqlite")
		database, err = db.NewSqlite(cfg.DataDir)
	}
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

	err = cfg.Bootstrap(database)
	if err != nil {
		klog.Exitln(err)
	}

	globalClickhouse := cfg.GetGlobalClickhouse()
	globalPrometheus := cfg.GetGlobalPrometheus()

	cacheConfig := cache.Config{
		Path: path.Join(cfg.DataDir, "cache"),
		GC: &cache.GcConfig{
			TTL:      cfg.Cache.TTL,
			Interval: cfg.Cache.GCInterval,
		},
	}
	promCache, err := cache.NewCache(cacheConfig, database, globalPrometheus)
	if err != nil {
		klog.Exitln(err)
	}

	coll := collector.New(database, promCache, globalClickhouse, globalPrometheus)
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		coll.Close()
		os.Exit(0)
	}()

	pricing, err := cloud_pricing.NewManager(path.Join(cfg.DataDir, "cloud-pricing"))
	if err != nil {
		klog.Exitln(err)
	}

	watchers.Start(database, promCache, pricing, !cfg.DoNotCheckSLO, !cfg.DoNotCheckForDeployments)

	a := api.NewApi(promCache, database, coll, pricing, rbac.NewStaticRoleManager(), globalClickhouse, globalPrometheus)
	err = a.AuthInit(cfg.Auth.AnonymousRole, cfg.Auth.BootstrapAdminPassword)
	if err != nil {
		klog.Exitln(err)
	}

	instanceUuid := utils.GetInstanceUuid(cfg.DataDir)

	var statsCollector *stats.Collector
	if !cfg.DisableUsageStatistics {
		statsCollector = stats.NewCollector(instanceUuid, version, database, promCache, pricing, globalClickhouse)
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
	if cfg.UrlBasePath != "/" {
		r = router.PathPrefix(cfg.UrlBasePath).Subrouter()
	}
	r.HandleFunc("/api/login", a.Login).Methods(http.MethodPost)
	r.HandleFunc("/api/logout", a.Logout).Methods(http.MethodPost)

	r.HandleFunc("/api/user", a.Auth(a.User)).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/users", a.Auth(a.Users)).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/roles", a.Auth(a.Roles)).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/sso", a.Auth(a.SSO)).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/", a.Auth(a.Project)).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}", a.Auth(a.Project)).Methods(http.MethodGet, http.MethodPost, http.MethodDelete)
	r.HandleFunc("/api/project/{project}/status", a.Auth(a.Status)).Methods(http.MethodGet)
	r.HandleFunc("/api/project/{project}/api_keys", a.Auth(a.ApiKeys)).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/overview/{view}", a.Auth(a.Overview)).Methods(http.MethodGet)
	r.HandleFunc("/api/project/{project}/incident/{incident}", a.Auth(a.Incident)).Methods(http.MethodGet)
	r.HandleFunc("/api/project/{project}/inspections", a.Auth(a.Inspections)).Methods(http.MethodGet)
	r.HandleFunc("/api/project/{project}/categories", a.Auth(a.Categories)).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/custom_applications", a.Auth(a.CustomApplications)).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/integrations", a.Auth(a.Integrations)).Methods(http.MethodGet, http.MethodPut)
	r.HandleFunc("/api/project/{project}/integrations/{type}", a.Auth(a.Integration)).Methods(http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPost)
	r.HandleFunc("/api/project/{project}/app/{app}", a.Auth(a.Application)).Methods(http.MethodGet)
	r.HandleFunc("/api/project/{project}/app/{app}/rca", a.Auth(a.RCA)).Methods(http.MethodGet)
	r.HandleFunc("/api/project/{project}/app/{app}/inspection/{type}/config", a.Auth(a.Inspection)).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/app/{app}/instrumentation/{type}", a.Auth(a.Instrumentation)).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/app/{app}/profiling", a.Auth(a.Profiling)).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/app/{app}/tracing", a.Auth(a.Tracing)).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/app/{app}/logs", a.Auth(a.Logs)).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/project/{project}/app/{app}/risks", a.Auth(a.Risks)).Methods(http.MethodPost)
	r.HandleFunc("/api/project/{project}/node/{node}", a.Auth(a.Node)).Methods(http.MethodGet)
	r.PathPrefix("/api/project/{project}/prom").HandlerFunc(a.Auth(a.Prom))

	r.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		statsCollector.RegisterRequest(r)
	}).Methods(http.MethodPost)

	if cfg.DeveloperMode {
		r.PathPrefix("/static/").Handler(http.StripPrefix(cfg.UrlBasePath+"static/", http.FileServer(http.Dir("./static"))))
	} else {
		r.PathPrefix("/static/").Handler(http.StripPrefix(cfg.UrlBasePath, http.FileServer(utils.NewStaticFSWrapper(static))))
	}

	indexHtml := readIndexHtml(cfg.UrlBasePath, version, instanceUuid, !cfg.DoNotCheckForUpdates, cfg.DeveloperMode)
	r.PathPrefix("").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(indexHtml)
	})

	router.PathPrefix("").Handler(http.RedirectHandler(cfg.UrlBasePath, http.StatusMovedPermanently))

	klog.Infoln("listening on", cfg.ListenAddress)
	klog.Fatalln(http.ListenAndServe(cfg.ListenAddress, router))
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
		Edition         string
	}{
		BasePath:        basePath,
		Version:         version,
		Uuid:            instanceUuid,
		CheckForUpdates: checkForUpdates,
		Edition:         "Community",
	})
	if err != nil {
		klog.Exitln(err)
	}
	return buf.Bytes()
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
