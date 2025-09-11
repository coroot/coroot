package config

import (
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/timeseries"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	configFile                                  = kingpin.Flag("config", "Configuration file").Envar("CONFIG").String()
	listen                                      = kingpin.Flag("listen", "Listen address - ip:port or :port").Envar("LISTEN").String()
	grpcDisabled                                = kingpin.Flag("grpc-disabled", "Disable gRPC server").Envar("GRPC_DISABLED").Bool()
	grpcListen                                  = kingpin.Flag("grpc-listen", "gRPC listen address - ip:port or :port").Envar("GRPC_LISTEN").String()
	tlsCertFile                                 = kingpin.Flag("tls-cert-file", "Path to the TLS certificate file").Envar("TLS_CERT_FILE").String()
	tlsKeyFile                                  = kingpin.Flag("tls-key-file", "Path to the TLS private key file").Envar("TLS_KEY_FILE").String()
	urlBasePath                                 = kingpin.Flag("url-base-path", "The base URL to run Coroot at a sub-path, e.g. /coroot/").Envar("URL_BASE_PATH").String()
	dataDir                                     = kingpin.Flag("data-dir", `Path to the data directory`).Envar("DATA_DIR").String()
	cacheTTL                                    = timeseries.DurationFlag(kingpin.Flag("cache-ttl", "Cache TTL (e.g. 8h, 2d, 1w; default 30d)").Envar("CACHE_TTL"))
	cacheGcInterval                             = timeseries.DurationFlag(kingpin.Flag("cache-gc-interval", "Cache GC interval").Envar("CACHE_GC_INTERVAL"))
	tracesTTL                                   = timeseries.DurationFlag(kingpin.Flag("traces-ttl", "Traces TTL (e.g. 8h, 3d, 2w; default 7d)").Envar("TRACES_TTL"))
	logsTTL                                     = timeseries.DurationFlag(kingpin.Flag("logs-ttl", "Logs TTL (e.g. 8h, 3d, 2w; default 7d)").Envar("LOGS_TTL"))
	profilesTTL                                 = timeseries.DurationFlag(kingpin.Flag("profiles-ttl", "Profiles TTL (e.g. 8h, 3d, 2w; default 7d)").Envar("PROFILES_TTL"))
	pgConnectionString                          = kingpin.Flag("pg-connection-string", "Postgres connection string (sqlite is used if not set)").Envar("PG_CONNECTION_STRING").String()
	doNotCheckForDeployments                    = kingpin.Flag("do-not-check-for-deployments", "Don't check for new deployments").Envar("DO_NOT_CHECK_FOR_DEPLOYMENTS").Bool()
	doNotCheckForUpdates                        = kingpin.Flag("do-not-check-for-updates", "Don't check for new versions").Envar("DO_NOT_CHECK_FOR_UPDATES").Bool()
	disableUsageStatistics                      = kingpin.Flag("disable-usage-statistics", "Disable usage statistics").Envar("DISABLE_USAGE_STATISTICS").Bool()
	authAnonymousRole                           = kingpin.Flag("auth-anonymous-role", "Disable authentication and assign one of the following roles to the anonymous user: Admin, Editor, or Viewer.").Envar("AUTH_ANONYMOUS_ROLE").String()
	authBootstrapAdminPassword                  = kingpin.Flag("auth-bootstrap-admin-password", "Password for the default Admin user").Envar("AUTH_BOOTSTRAP_ADMIN_PASSWORD").String()
	developerMode                               = kingpin.Flag("developer-mode", "If enabled, Coroot will not use embedded static assets").Envar("DEVELOPER_MODE").Bool()
	clickHouseSpaceManagerDisabled              = kingpin.Flag("disable-clickhouse-space-manager", "If enabled, Coroot will manage ClickHouse disk space by removing old partitions").Envar("CLICKHOUSE_SPACE_MANAGER_DISABLED").Bool()
	clickHouseSpaceManagerUsageThresholdPercent = kingpin.Flag("clickhouse-space-manager-usage-threshold", "Disk usage percentage threshold for triggering partition cleanup in ClickHouse").Envar("CLICKHOUSE_SPACE_MANAGER_USAGE_THRESHOLD").Int()
	clickHouseSpaceManagerMinPartitions         = kingpin.Flag("clickhouse-space-manager-min-partitions", "Minimum number of partitions to keep when cleaning up ClickHouse disk space").Envar("CLICKHOUSE_SPACE_MANAGER_MIN_PARTITIONS").Int()

	globalClickhouseAddress         = kingpin.Flag("global-clickhouse-address", "").Envar("GLOBAL_CLICKHOUSE_ADDRESS").String()
	globalClickhouseUser            = kingpin.Flag("global-clickhouse-user", "").Envar("GLOBAL_CLICKHOUSE_USER").String()
	globalClickhousePassword        = kingpin.Flag("global-clickhouse-password", "").Envar("GLOBAL_CLICKHOUSE_PASSWORD").String()
	globalClickhouseInitialDatabase = kingpin.Flag("global-clickhouse-initial-database", "").Envar("GLOBAL_CLICKHOUSE_INITIAL_DATABASE").String()
	globalClickhouseTlsEnabled      = kingpin.Flag("global-clickhouse-tls-enabled", "").Envar("GLOBAL_CLICKHOUSE_TLS_ENABLED").Bool()
	globalClickhouseTlsSkipVerify   = kingpin.Flag("global-clickhouse-tls-skip-verify", "").Envar("GLOBAL_CLICKHOUSE_TLS_SKIP_VERIFY").Bool()

	globalPrometheusUrl            = kingpin.Flag("global-prometheus-url", "").Envar("GLOBAL_PROMETHEUS_URL").String()
	globalPrometheusTlsSkipVerify  = kingpin.Flag("global-prometheus-tls-skip-verify", "").Envar("GLOBAL_PROMETHEUS_TLS_SKIP_VERIFY").Bool()
	globalRefreshInterval          = timeseries.DurationFlag(kingpin.Flag("global-refresh-interval", "").Envar("GLOBAL_REFRESH_INTERVAL"))
	globalPrometheusUser           = kingpin.Flag("global-prometheus-user", "").Envar("GLOBAL_PROMETHEUS_USER").String()
	globalPrometheusPassword       = kingpin.Flag("global-prometheus-password", "").Envar("GLOBAL_PROMETHEUS_PASSWORD").String()
	globalPrometheusCustomHeaders  = kingpin.Flag("global-prometheus-custom-headers", "").Envar("GLOBAL_PROMETHEUS_CUSTOM_HEADERS").StringMap()
	globalPrometheusRemoteWriteUrl = kingpin.Flag("global-prometheus-remote-write-url", "").Envar("GLOBAL_PROMETHEUS_REMOTE_WRITE_URL").String()

	bootstrapPrometheusUrl            = kingpin.Flag("bootstrap-prometheus-url", "").Envar("BOOTSTRAP_PROMETHEUS_URL").String()
	bootstrapRefreshInterval          = timeseries.DurationFlag(kingpin.Flag("bootstrap-refresh-interval", "").Envar("BOOTSTRAP_REFRESH_INTERVAL"))
	bootstrapPrometheusExtraSelector  = kingpin.Flag("bootstrap-prometheus-extra-selector", "").Envar("BOOTSTRAP_PROMETHEUS_EXTRA_SELECTOR").String()
	bootstrapPrometheusRemoteWriteUrl = kingpin.Flag("bootstrap-prometheus-remote-write-url", "").Envar("BOOTSTRAP_PROMETHEUS_REMOTE_WRITE_URL").String()

	bootstrapClickhouseAddress  = kingpin.Flag("bootstrap-clickhouse-address", "").Envar("BOOTSTRAP_CLICKHOUSE_ADDRESS").String()
	bootstrapClickhouseUser     = kingpin.Flag("bootstrap-clickhouse-user", "").Envar("BOOTSTRAP_CLICKHOUSE_USER").String()
	bootstrapClickhousePassword = kingpin.Flag("bootstrap-clickhouse-password", "").Envar("BOOTSTRAP_CLICKHOUSE_PASSWORD").String()
	bootstrapClickhouseDatabase = kingpin.Flag("bootstrap-clickhouse-database", "").Envar("BOOTSTRAP_CLICKHOUSE_DATABASE").String()
)

func (cfg *Config) ApplyFlags() {
	if *listen != "" {
		cfg.ListenAddress = *listen
	}
	cfg.GRPC.Disabled = *grpcDisabled
	if *grpcListen != "" {
		cfg.GRPC.ListenAddress = *grpcListen
	}
	if *tlsCertFile != "" && *tlsKeyFile != "" {
		if cfg.TLS == nil {
			cfg.TLS = &TLS{}
		}
		cfg.TLS.CertFile = *tlsCertFile
		cfg.TLS.KeyFile = *tlsKeyFile
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
	if *tracesTTL > 0 {
		cfg.Traces.TTL = *tracesTTL
	}
	if *logsTTL > 0 {
		cfg.Logs.TTL = *logsTTL
	}
	if *profilesTTL > 0 {
		cfg.Profiles.TTL = *profilesTTL
	}
	if *pgConnectionString != "" {
		cfg.Postgres = &Postgres{ConnectionString: *pgConnectionString}
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
	if *clickHouseSpaceManagerDisabled {
		cfg.ClickHouseSpaceManager.Enabled = false
	}
	if *clickHouseSpaceManagerUsageThresholdPercent > 0 {
		cfg.ClickHouseSpaceManager.UsageThresholdPercent = *clickHouseSpaceManagerUsageThresholdPercent
	}
	if *clickHouseSpaceManagerMinPartitions > 0 {
		cfg.ClickHouseSpaceManager.MinPartitions = *clickHouseSpaceManagerMinPartitions
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
