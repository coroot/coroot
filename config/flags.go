package config

import (
	"github.com/coroot/coroot/db"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	configFile                 = kingpin.Flag("config", "Configuration file").Envar("CONFIG").String()
	listen                     = kingpin.Flag("listen", "Listen address - ip:port or :port").Envar("LISTEN").String()
	urlBasePath                = kingpin.Flag("url-base-path", "The base URL to run Coroot at a sub-path, e.g. /coroot/").Envar("URL_BASE_PATH").String()
	dataDir                    = kingpin.Flag("data-dir", `Path to the data directory`).Envar("DATA_DIR").String()
	cacheTTL                   = kingpin.Flag("cache-ttl", "Cache TTL").Envar("CACHE_TTL").Duration()
	cacheGcInterval            = kingpin.Flag("cache-gc-interval", "Cache GC interval").Envar("CACHE_GC_INTERVAL").Duration()
	pgConnectionString         = kingpin.Flag("pg-connection-string", "Postgres connection string (sqlite is used if not set)").Envar("PG_CONNECTION_STRING").String()
	doNotCheckSLO              = kingpin.Flag("do-not-check-slo", "Don't check SLO compliance").Envar("DO_NOT_CHECK_SLO").Bool()
	doNotCheckForDeployments   = kingpin.Flag("do-not-check-for-deployments", "Don't check for new deployments").Envar("DO_NOT_CHECK_FOR_DEPLOYMENTS").Bool()
	doNotCheckForUpdates       = kingpin.Flag("do-not-check-for-updates", "Don't check for new versions").Envar("DO_NOT_CHECK_FOR_UPDATES").Bool()
	disableUsageStatistics     = kingpin.Flag("disable-usage-statistics", "Disable usage statistics").Envar("DISABLE_USAGE_STATISTICS").Bool()
	authAnonymousRole          = kingpin.Flag("auth-anonymous-role", "Disable authentication and assign one of the following roles to the anonymous user: Admin, Editor, or Viewer.").Envar("AUTH_ANONYMOUS_ROLE").String()
	authBootstrapAdminPassword = kingpin.Flag("auth-bootstrap-admin-password", "Password for the default Admin user").Envar("AUTH_BOOTSTRAP_ADMIN_PASSWORD").String()
	developerMode              = kingpin.Flag("developer-mode", "If enabled, Coroot will not use embedded static assets").Envar("DEVELOPER_MODE").Bool()
	licenseKey                 = kingpin.Flag("license-key", "License key for Coroot Enterprise Edition.").Envar("LICENSE_KEY").String()

	globalClickhouseAddress         = kingpin.Flag("global-clickhouse-address", "").Envar("GLOBAL_CLICKHOUSE_ADDRESS").String()
	globalClickhouseUser            = kingpin.Flag("global-clickhouse-user", "").Envar("GLOBAL_CLICKHOUSE_USER").String()
	globalClickhousePassword        = kingpin.Flag("global-clickhouse-password", "").Envar("GLOBAL_CLICKHOUSE_PASSWORD").String()
	globalClickhouseInitialDatabase = kingpin.Flag("global-clickhouse-initial-database", "").Envar("GLOBAL_CLICKHOUSE_INITIAL_DATABASE").String()
	globalClickhouseTlsEnabled      = kingpin.Flag("global-clickhouse-tls-enabled", "").Envar("GLOBAL_CLICKHOUSE_TLS_ENABLED").Bool()
	globalClickhouseTlsSkipVerify   = kingpin.Flag("global-clickhouse-tls-skip-verify", "").Envar("GLOBAL_CLICKHOUSE_TLS_SKIP_VERIFY").Bool()

	globalPrometheusUrl            = kingpin.Flag("global-prometheus-url", "").Envar("GLOBAL_PROMETHEUS_URL").String()
	globalPrometheusTlsSkipVerify  = kingpin.Flag("global-prometheus-tls-skip-verify", "").Envar("GLOBAL_PROMETHEUS_TLS_SKIP_VERIFY").Bool()
	globalRefreshInterval          = kingpin.Flag("global-refresh-interval", "").Envar("GLOBAL_REFRESH_INTERVAL").Duration()
	globalPrometheusUser           = kingpin.Flag("global-prometheus-user", "").Envar("GLOBAL_PROMETHEUS_USER").String()
	globalPrometheusPassword       = kingpin.Flag("global-prometheus-password", "").Envar("GLOBAL_PROMETHEUS_PASSWORD").String()
	globalPrometheusCustomHeaders  = kingpin.Flag("global-prometheus-custom-headers", "").Envar("GLOBAL_PROMETHEUS_CUSTOM_HEADERS").StringMap()
	globalPrometheusRemoteWriteUrl = kingpin.Flag("global-prometheus-remote-write-url", "").Envar("GLOBAL_PROMETHEUS_REMOTE_WRITE_URL").String()

	bootstrapPrometheusUrl            = kingpin.Flag("bootstrap-prometheus-url", "").Envar("BOOTSTRAP_PROMETHEUS_URL").String()
	bootstrapRefreshInterval          = kingpin.Flag("bootstrap-refresh-interval", "").Envar("BOOTSTRAP_REFRESH_INTERVAL").Duration()
	bootstrapPrometheusExtraSelector  = kingpin.Flag("bootstrap-prometheus-extra-selector", "").Envar("BOOTSTRAP_PROMETHEUS_EXTRA_SELECTOR").String()
	bootstrapPrometheusRemoteWriteUrl = kingpin.Flag("bootstrap-prometheus-remote-write-url", "").Envar("BOOTSTRAP_PROMETHEUS_REMOTE_WRITE_URL").String()

	bootstrapClickhouseAddress  = kingpin.Flag("bootstrap-clickhouse-address", "").Envar("BOOTSTRAP_CLICKHOUSE_ADDRESS").String()
	bootstrapClickhouseUser     = kingpin.Flag("bootstrap-clickhouse-user", "").Envar("BOOTSTRAP_CLICKHOUSE_USER").String()
	bootstrapClickhousePassword = kingpin.Flag("bootstrap-clickhouse-password", "").Envar("BOOTSTRAP_CLICKHOUSE_PASSWORD").String()
	bootstrapClickhouseDatabase = kingpin.Flag("bootstrap-clickhouse-database", "").Envar("BOOTSTRAP_CLICKHOUSE_DATABASE").String()
)

func (cfg *Config) applyFlags() {
	if *listen != "" {
		cfg.ListenAddress = *listen
	}
	if *urlBasePath != "" {
		cfg.UrlBasePath = *urlBasePath
	}
	if *dataDir != "" {
		cfg.DataDir = *dataDir
	}
	if *cacheTTL > 0 {
		cfg.Cache.TTL = *cacheTTL
	}
	if *cacheGcInterval > 0 {
		cfg.Cache.GCInterval = *cacheGcInterval
	}
	if *pgConnectionString != "" {
		cfg.Postgres = &Postgres{ConnectionString: *pgConnectionString}
	}
	if *doNotCheckSLO {
		cfg.DoNotCheckSLO = *doNotCheckSLO
	}
	if *doNotCheckForDeployments {
		cfg.DoNotCheckForDeployments = *doNotCheckForDeployments
	}
	if *doNotCheckForUpdates {
		cfg.DoNotCheckForUpdates = *doNotCheckForUpdates
	}
	if *disableUsageStatistics {
		cfg.DisableUsageStatistics = *disableUsageStatistics
	}
	if *authAnonymousRole != "" {
		cfg.Auth.AnonymousRole = *authAnonymousRole
	}
	if *authBootstrapAdminPassword != "" {
		cfg.Auth.BootstrapAdminPassword = *authBootstrapAdminPassword
	}
	if *developerMode {
		cfg.DeveloperMode = *developerMode
	}
	if *licenseKey != "" {
		cfg.LicenseKey = *licenseKey
	}

	keep := cfg.GlobalClickhouse != nil || *globalClickhouseAddress != ""
	if cfg.GlobalClickhouse == nil {
		cfg.GlobalClickhouse = &Clickhouse{}
	}
	if *globalClickhouseAddress != "" {
		cfg.GlobalClickhouse.Address = *globalClickhouseAddress
	}
	if *globalClickhouseUser != "" {
		cfg.GlobalClickhouse.User = *globalClickhouseUser
	}
	if *globalClickhousePassword != "" {
		cfg.GlobalClickhouse.Password = *globalClickhousePassword
	}
	if *globalClickhouseInitialDatabase != "" {
		cfg.GlobalClickhouse.Database = *globalClickhouseInitialDatabase
	}
	if *globalClickhouseTlsEnabled {
		cfg.GlobalClickhouse.TlsEnable = *globalClickhouseTlsEnabled
	}
	if *globalClickhouseTlsSkipVerify {
		cfg.GlobalClickhouse.TlsSkipVerify = *globalClickhouseTlsSkipVerify
	}
	if !keep {
		cfg.GlobalClickhouse = nil
	}

	keep = cfg.GlobalPrometheus != nil || *globalPrometheusUrl != ""
	if cfg.GlobalPrometheus == nil {
		cfg.GlobalPrometheus = &Prometheus{
			CustomHeaders: map[string]string{},
		}
	}
	if *globalPrometheusUrl != "" {
		cfg.GlobalPrometheus.Url = *globalPrometheusUrl
	}
	if *globalPrometheusTlsSkipVerify {
		cfg.GlobalPrometheus.TlsSkipVerify = *globalPrometheusTlsSkipVerify
	}
	if *globalRefreshInterval > 0 {
		cfg.GlobalPrometheus.RefreshInterval = *globalRefreshInterval
	}
	if *globalPrometheusUser != "" {
		cfg.GlobalPrometheus.User = *globalPrometheusUser
	}
	if *globalPrometheusPassword != "" {
		cfg.GlobalPrometheus.Password = *globalPrometheusPassword
	}
	if *globalPrometheusRemoteWriteUrl != "" {
		cfg.GlobalPrometheus.RemoteWriteUrl = *globalPrometheusRemoteWriteUrl
	}
	for name, value := range *globalPrometheusCustomHeaders {
		cfg.GlobalPrometheus.CustomHeaders[name] = value
	}
	if !keep {
		cfg.GlobalPrometheus = nil
	}

	if *bootstrapPrometheusUrl != "" {
		cfg.BootstrapPrometheus = &Prometheus{
			Url:             *bootstrapPrometheusUrl,
			RefreshInterval: *bootstrapRefreshInterval,
			ExtraSelector:   *bootstrapPrometheusExtraSelector,
			RemoteWriteUrl:  *bootstrapPrometheusRemoteWriteUrl,
		}
		if cfg.BootstrapPrometheus.RefreshInterval <= 0 {
			cfg.BootstrapPrometheus.RefreshInterval = db.DefaultRefreshInterval
		}
	}
	if *bootstrapClickhouseAddress != "" {
		cfg.BootstrapClickhouse = &Clickhouse{
			Address:  *bootstrapClickhouseAddress,
			User:     *bootstrapClickhouseUser,
			Password: *bootstrapClickhousePassword,
			Database: *bootstrapClickhouseDatabase,
		}
	}
}
