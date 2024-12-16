package db

import (
	"fmt"
	"time"

	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"k8s.io/klog"
)

func (db *DB) GetOrCreateDefaultProject() (*Project, error) {
	projects, err := db.GetProjects()
	if err != nil {
		return nil, err
	}
	switch len(projects) {
	case 0:
		p := Project{Name: "default"}
		klog.Infoln("creating default project")
		if p.Id, err = db.SaveProject(p); err != nil {
			return nil, err
		}
		return &p, nil
	case 1:
		return projects[0], nil
	}
	return nil, nil
}

func (db *DB) BootstrapPrometheusIntegration(p *Project, url string, refreshInterval time.Duration, extraSelector string) error {
	if p == nil {
		return nil
	}
	if url == "" || refreshInterval == 0 {
		return nil
	}
	if !prom.IsSelectorValid(extraSelector) {
		return fmt.Errorf("invalid Prometheus extra selector: %s", extraSelector)
	}
	if p.Prometheus.Url != "" {
		return nil
	}
	p.Prometheus = IntegrationsPrometheus{
		Url:             url,
		RefreshInterval: timeseries.Duration(int64((refreshInterval).Seconds())),
		ExtraSelector:   extraSelector,
	}
	return db.SaveProjectIntegration(p, IntegrationTypePrometheus)
}

func (db *DB) BootstrapClickhouseIntegration(p *Project, addr, user, password, databaseName string) error {
	if p == nil {
		return nil
	}
	if addr == "" || user == "" || databaseName == "" {
		return nil
	}
	var save bool
	if cfg := p.Settings.Integrations.Clickhouse; cfg == nil {
		p.Settings.Integrations.Clickhouse = &IntegrationClickhouse{
			Protocol: "native",
			Addr:     addr,
			Auth: utils.BasicAuth{
				User:     user,
				Password: password,
			},
			Database: databaseName,
		}
		save = true
	}
	if !save {
		return nil
	}
	return db.SaveProjectIntegration(p, IntegrationTypeClickhouse)
}

func (db *DB) BootstrapApiKeys() error {
	projects, err := db.GetProjects()
	if err != nil {
		return err
	}
	for _, p := range projects {
		if p.Settings.ApiKeys != nil {
			continue
		}
		p.Settings.ApiKeys = append(p.Settings.ApiKeys, ApiKey{Key: string(p.Id), Description: "default"})
		err = db.SaveProjectSettings(p)
		if err != nil {
			return err
		}
	}
	return nil
}
