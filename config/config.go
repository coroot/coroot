package config

import (
	"fmt"
	"io"
	"net"
	"net/url"
	"os"

	"github.com/coroot/coroot/cloud"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"gopkg.in/yaml.v3"
)

type Config struct {
	ListenAddress string `yaml:"listen_address"`
	UrlBasePath   string `yaml:"url_base_path"`
	DataDir       string `yaml:"data_dir"`

	Cache    Cache    `yaml:"cache"`
	Traces   Traces   `yaml:"traces"`
	Logs     Logs     `yaml:"logs"`
	Profiles Profiles `yaml:"profiles"`

	Postgres         *Postgres   `yaml:"postgres"`
	GlobalPrometheus *Prometheus `yaml:"global_prometheus"`
	GlobalClickhouse *Clickhouse `yaml:"global_clickhouse"`

	Auth Auth `yaml:"auth"`

	Projects []Project `yaml:"projects"`

	DoNotCheckForDeployments bool `yaml:"do_not_check_for_deployments"`
	DoNotCheckForUpdates     bool `yaml:"do_not_check_for_updates"`
	DisableUsageStatistics   bool `yaml:"disable_usage_statistics"`

	DeveloperMode bool `yaml:"developer_mode"`

	ClickHouseSpaceManager ClickHouseSpaceManager `yaml:"clickhouse_space_manager"`

	CorootCloud *cloud.Settings `yaml:"corootCloud"`

	BootstrapClickhouse *Clickhouse `yaml:"-"`
	BootstrapPrometheus *Prometheus `yaml:"-"`
}

type ClickHouseSpaceManager struct {
	Enabled               bool `yaml:"enabled"`
	UsageThresholdPercent int  `yaml:"usage_threshold_percent"`
	MinPartitions         int  `yaml:"min_partitions"`
}

type Cache struct {
	TTL        timeseries.Duration `yaml:"ttl"`
	GCInterval timeseries.Duration `yaml:"gc_interval"`
}

type Traces struct {
	TTL timeseries.Duration `yaml:"ttl"`
}

type Logs struct {
	TTL timeseries.Duration `yaml:"ttl"`
}

type Profiles struct {
	TTL timeseries.Duration `yaml:"ttl"`
}

type Postgres struct {
	ConnectionString string `yaml:"connection_string"`
}

type Clickhouse struct {
	Address       string `yaml:"address"`
	User          string `yaml:"user"`
	Password      string `yaml:"password"`
	Database      string `yaml:"database"`
	TlsEnable     bool   `yaml:"tls_enable"`
	TlsSkipVerify bool   `yaml:"tls_skip_verify"`
}

func (c *Clickhouse) Validate() error {
	if c == nil {
		return nil
	}
	if c.Address == "" {
		return fmt.Errorf("address is required")
	}
	host, port, err := net.SplitHostPort(c.Address)
	if host == "" || port == "" {
		return fmt.Errorf("invalid address: %s", c.Address)
	}
	if err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}
	return nil
}

type Prometheus struct {
	Url             string              `yaml:"url"`
	RefreshInterval timeseries.Duration `yaml:"refresh_interval"`
	TlsSkipVerify   bool                `yaml:"tls_skip_verify"`
	User            string              `yaml:"user"`
	Password        string              `yaml:"password"`
	ExtraSelector   string              `yaml:"extra_selector"`
	CustomHeaders   map[string]string   `yaml:"custom_headers"`
	RemoteWriteUrl  string              `yaml:"remote_write_url"`
}

func validateUrl(urlString string) error {
	u, err := url.Parse(urlString)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("invalid url '%s': missing protocol scheme (http:// or https://)", urlString)
	}
	return nil
}

func (p *Prometheus) Validate() error {
	if p == nil {
		return nil
	}
	if p.Url == "" {
		return fmt.Errorf("url is required")
	}
	if err := validateUrl(p.Url); err != nil {
		return err
	}
	if p.RemoteWriteUrl != "" {
		if err := validateUrl(p.RemoteWriteUrl); err != nil {
			return err
		}
	}
	if p.RefreshInterval <= 0 {
		return fmt.Errorf("invalid refresh-interval: %d", p.RefreshInterval)
	}
	if !prom.IsSelectorValid(p.ExtraSelector) {
		return fmt.Errorf("invalid extra_selector: %s", p.ExtraSelector)
	}
	return nil
}

type Auth struct {
	AnonymousRole          string `yaml:"anonymous_role"`
	BootstrapAdminPassword string `yaml:"bootstrap_admin_password"`
}

func NewConfig() *Config {
	return &Config{
		ListenAddress: ":8080",
		UrlBasePath:   "/",
		DataDir:       "./data",

		Cache: Cache{
			TTL:        30 * timeseries.Day,
			GCInterval: 10 * timeseries.Minute,
		},

		Traces: Traces{
			TTL: 7 * timeseries.Day,
		},
		Logs: Logs{
			TTL: 7 * timeseries.Day,
		},
		Profiles: Profiles{
			TTL: 7 * timeseries.Day,
		},

		Auth: Auth{
			BootstrapAdminPassword: db.AdminUserDefaultPassword,
		},

		ClickHouseSpaceManager: ClickHouseSpaceManager{
			Enabled:               true,
			UsageThresholdPercent: 70,
			MinPartitions:         1,
		},
	}
}

func Load() (*Config, error) {
	cfg := NewConfig()
	data, err := ReadFromFile()
	if err != nil {
		return nil, err
	}

	if len(data) > 0 {
		if err = yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	cfg.ApplyFlags()

	if err = cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func ReadFromFile() ([]byte, error) {
	if *configFile == "" {
		return nil, nil
	}
	f, err := os.Open(*configFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	data = []byte(os.ExpandEnv(string(data)))
	return data, nil
}

func (cfg *Config) Validate() error {
	var err error
	cfg.UrlBasePath, err = url.JoinPath("/", cfg.UrlBasePath, "/")
	if err != nil {
		return fmt.Errorf("invalid url_base_path: %s", cfg.UrlBasePath)
	}

	if cfg.CorootCloud != nil {
		if err = cfg.CorootCloud.Validate(); err != nil {
			return fmt.Errorf("invalid corootCloud settings: %w", err)
		}
	}

	for i, p := range cfg.Projects {
		if err = p.Validate(); err != nil {
			return fmt.Errorf("invalid project #%d: %w", i, err)
		}
	}

	if err = cfg.GlobalClickhouse.Validate(); err != nil {
		return fmt.Errorf("invalid global_clickhouse: %w", err)
	}
	if cfg.GlobalClickhouse != nil {
		cfg.BootstrapClickhouse = nil
	}
	if err = cfg.BootstrapClickhouse.Validate(); err != nil {
		return fmt.Errorf("invalid bootstrap_clickhouse: %w", err)
	}

	if err = cfg.GlobalPrometheus.Validate(); err != nil {
		return fmt.Errorf("invalid global_prometheus: %w", err)
	}
	if cfg.GlobalPrometheus != nil {
		cfg.BootstrapPrometheus = nil
	}
	if err = cfg.BootstrapPrometheus.Validate(); err != nil {
		return fmt.Errorf("invalid bootstrap_prometheus: %w", err)
	}
	if cfg.ClickHouseSpaceManager.UsageThresholdPercent < 0 || cfg.ClickHouseSpaceManager.UsageThresholdPercent > 100 {
		return fmt.Errorf("invalid usage_threshold_percent: %d", cfg.ClickHouseSpaceManager.UsageThresholdPercent)
	}

	return nil
}

func (cfg *Config) GetGlobalClickhouse() *db.IntegrationClickhouse {
	clickhouse := cfg.GlobalClickhouse
	if clickhouse == nil {
		return nil
	}
	c := &db.IntegrationClickhouse{
		Global:   true,
		Protocol: "native",
		Addr:     clickhouse.Address,
		Auth: utils.BasicAuth{
			User:     clickhouse.User,
			Password: clickhouse.Password,
		},
		Database:        "",
		InitialDatabase: clickhouse.Database,
		TlsEnable:       clickhouse.TlsEnable,
		TlsSkipVerify:   clickhouse.TlsSkipVerify,
	}
	if c.Auth.User == "" {
		c.Auth.User = "default"
	}
	if c.InitialDatabase == "" {
		c.InitialDatabase = "default"
	}
	return c
}

func (cfg *Config) GetBootstrapClickhouse() *db.IntegrationClickhouse {
	clickhouse := cfg.BootstrapClickhouse
	if clickhouse == nil {
		return nil
	}
	c := &db.IntegrationClickhouse{
		Protocol: "native",
		Addr:     clickhouse.Address,
		Auth: utils.BasicAuth{
			User:     clickhouse.User,
			Password: clickhouse.Password,
		},
		Database:      clickhouse.Database,
		TlsEnable:     clickhouse.TlsEnable,
		TlsSkipVerify: clickhouse.TlsSkipVerify,
	}
	if c.Auth.User == "" {
		c.Auth.User = "default"
	}
	if c.Database == "" {
		c.Database = "default"
	}
	return c
}

func (cfg *Config) GetProjects() []db.Project {
	var projects []db.Project
	for _, p := range cfg.Projects {
		pp := db.Project{Name: p.Name}
		pp.Settings.ApiKeys = p.ApiKeys
		projects = append(projects, pp)
	}
	return projects
}

func (cfg *Config) GetGlobalPrometheus() *db.IntegrationPrometheus {
	prometheus := cfg.GlobalPrometheus
	if prometheus == nil {
		return nil
	}
	p := &db.IntegrationPrometheus{
		Global:          true,
		Url:             prometheus.Url,
		RefreshInterval: prometheus.RefreshInterval,
		TlsSkipVerify:   prometheus.TlsSkipVerify,
		ExtraSelector:   prometheus.ExtraSelector,
		RemoteWriteUrl:  prometheus.RemoteWriteUrl,
	}
	if prometheus.User != "" && prometheus.Password != "" {
		p.BasicAuth = &utils.BasicAuth{
			User:     prometheus.User,
			Password: prometheus.Password,
		}
	}
	for k, v := range prometheus.CustomHeaders {
		p.CustomHeaders = append(p.CustomHeaders, utils.Header{Key: k, Value: v})
	}
	return p
}

func (cfg *Config) GetBootstrapPrometheus() *db.IntegrationPrometheus {
	prometheus := cfg.BootstrapPrometheus
	if prometheus == nil {
		return nil
	}
	p := &db.IntegrationPrometheus{
		Url:             prometheus.Url,
		RefreshInterval: prometheus.RefreshInterval,
		TlsSkipVerify:   prometheus.TlsSkipVerify,
		ExtraSelector:   prometheus.ExtraSelector,
		RemoteWriteUrl:  prometheus.RemoteWriteUrl,
	}
	if prometheus.User != "" && prometheus.Password != "" {
		p.BasicAuth = &utils.BasicAuth{
			User:     prometheus.User,
			Password: prometheus.Password,
		}
	}
	for k, v := range prometheus.CustomHeaders {
		p.CustomHeaders = append(p.CustomHeaders, utils.Header{Key: k, Value: v})
	}
	return p
}
