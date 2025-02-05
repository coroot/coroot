package config

import (
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"time"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"gopkg.in/yaml.v3"
	"k8s.io/klog"
)

type Config struct {
	ListenAddress string `yaml:"listen_address"`
	UrlBasePath   string `yaml:"url_base_path"`
	DataDir       string `yaml:"data_dir"`
	LicenseKey    string `yaml:"license_key"`

	Cache Cache `yaml:"cache"`

	Postgres         *Postgres   `yaml:"postgres"`
	GlobalPrometheus *Prometheus `yaml:"global_prometheus"`
	GlobalClickhouse *Clickhouse `yaml:"global_clickhouse"`

	Auth Auth `yaml:"auth"`

	Projects []Project `yaml:"projects"`

	DoNotCheckSLO            bool `yaml:"do_not_check_slo"`
	DoNotCheckForDeployments bool `yaml:"do_not_check_for_deployments"`
	DoNotCheckForUpdates     bool `yaml:"do_not_check_for_updates"`
	DisableUsageStatistics   bool `yaml:"disable_usage_statistics"`

	DeveloperMode bool `yaml:"developer_mode"`

	BootstrapClickhouse *Clickhouse `yaml:"-"`
	BootstrapPrometheus *Prometheus `yaml:"-"`
}

type Cache struct {
	TTL        time.Duration `yaml:"ttl"`
	GCInterval time.Duration `yaml:"gc_interval"`
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
	Url             string            `yaml:"url"`
	RefreshInterval time.Duration     `yaml:"refresh_interval"`
	TlsSkipVerify   bool              `yaml:"tls_skip_verify"`
	User            string            `yaml:"user"`
	Password        string            `yaml:"password"`
	ExtraSelector   string            `yaml:"extra_selector"`
	CustomHeaders   map[string]string `yaml:"custom_headers"`
	RemoteWriteUrl  string            `yaml:"remote_write_url"`
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

type Project struct {
	Name    string      `yaml:"name"`
	ApiKeys []db.ApiKey `yaml:"api_keys"`
}

func Load() *Config {
	cfg := &Config{
		ListenAddress: ":8080",
		UrlBasePath:   "/",
		DataDir:       "./data",

		Cache: Cache{
			TTL:        30 * 24 * time.Hour,
			GCInterval: 10 * time.Minute,
		},

		Auth: Auth{
			BootstrapAdminPassword: db.AdminUserDefaultPassword,
		},
	}
	err := cfg.load()
	if err != nil {
		klog.Exitln(err)
	}
	return cfg
}

func (cfg *Config) load() error {
	if *configFile != "" {
		f, err := os.Open(*configFile)
		if err != nil {
			return err
		}
		defer f.Close()
		data, err := io.ReadAll(f)
		if err != nil {
			return err
		}
		if err = yaml.Unmarshal(data, cfg); err != nil {
			return err
		}
	}

	cfg.applyFlags()

	var err error
	cfg.UrlBasePath, err = url.JoinPath("/", cfg.UrlBasePath, "/")
	if err != nil {
		return fmt.Errorf("invalid url_base_path: %s", cfg.UrlBasePath)
	}

	for i, p := range cfg.Projects {
		if p.Name == "" {
			return fmt.Errorf("invalid project #%d: name is required", i)
		}
		if len(p.ApiKeys) == 0 {
			return fmt.Errorf("invalid project '%s': no api_keys defined", p.Name)
		}
		for ik, k := range p.ApiKeys {
			if k.Key == "" {
				return fmt.Errorf("invalid api_key #%d for project '%s': key is required", ik, p.Name)
			}
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
		RefreshInterval: timeseries.DurationFromStandard(prometheus.RefreshInterval),
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
		RefreshInterval: timeseries.DurationFromStandard(prometheus.RefreshInterval),
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
