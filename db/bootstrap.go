package db

import (
	"fmt"
	"time"

	"github.com/coroot/coroot/utils"

	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
)

func (db *DB) getOrCreateDefaultProject() (*Project, error) {
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

func (db *DB) BootstrapPrometheusIntegration(url string, refreshInterval time.Duration, extraSelector string) error {
	if url == "" || refreshInterval == 0 {
		return nil
	}
	if !prom.IsSelectorValid(extraSelector) {
		return fmt.Errorf("invalid Prometheus extra selector: %s", extraSelector)
	}
	p, err := db.getOrCreateDefaultProject()
	if err != nil {
		return err
	}
	if p == nil {
		return nil
	}
	if p.Prometheus.Url != "" {
		return nil
	}
	p.Prometheus = IntegrationsPrometheus{
		Url:             url,
		RefreshInterval: timeseries.Duration(int64((refreshInterval).Seconds())),
		ExtraSelector:   extraSelector,
	}
	err = db.SaveProjectIntegration(p, IntegrationTypePrometheus)
	if err != nil {
		return err
	}
	return nil
}

func (db *DB) BootstrapClickhouseIntegration(addr, user, password, databaseName string) error {
	if addr == "" || user == "" || databaseName == "" {
		return nil
	}
	p, err := db.getOrCreateDefaultProject()
	if err != nil {
		return err
	}
	if p == nil {
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
	err = db.SaveProjectIntegration(p, IntegrationTypeClickhouse)
	if err != nil {
		return err
	}
	return nil
}
