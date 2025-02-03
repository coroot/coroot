package config

import (
	"github.com/coroot/coroot/db"
	"k8s.io/klog"
)

func (cfg *Config) Bootstrap(database *db.DB) error {
	if len(cfg.Projects) == 0 {
		p, err := getOrCreateDefaultProject(database)
		if err != nil {
			return err
		}
		if p != nil {
			prometheus := cfg.GetBootstrapPrometheus()
			if p.Prometheus.Url == "" && prometheus != nil && prometheus.Url != "" && prometheus.RefreshInterval > 0 {
				p.Prometheus = *prometheus
				err = database.SaveProjectIntegration(p, db.IntegrationTypePrometheus)
				if err != nil {
					return err
				}
			}
			clickhouse := cfg.GetBootstrapClickhouse()
			if p.Settings.Integrations.Clickhouse == nil && clickhouse != nil && clickhouse.Addr != "" {
				p.Settings.Integrations.Clickhouse = clickhouse
				err = database.SaveProjectIntegration(p, db.IntegrationTypeClickhouse)
				if err != nil {
					return err
				}
			}
		}
	}

	ps, err := database.GetProjects()
	if err != nil {
		return err
	}
	byName := map[string]*db.Project{}
	for _, p := range ps {
		byName[p.Name] = p
		p.Settings.Configurable = true
	}
	for _, p := range cfg.Projects {
		pp := byName[p.Name]
		if pp == nil {
			pp = &db.Project{Name: p.Name}
			klog.Infoln("creating project:", pp.Name)
			err = database.SaveProject(pp)
			if err != nil {
				return err
			}
			byName[pp.Name] = pp
		}
		pp.Settings.ApiKeys = p.ApiKeys
		pp.Settings.Configurable = false
	}
	for _, p := range byName {
		if p.Settings.ApiKeys == nil {
			p.Settings.ApiKeys = append(p.Settings.ApiKeys, db.ApiKey{Key: string(p.Id), Description: "default"})
		}
		err = database.SaveProjectSettings(p)
		if err != nil {
			return err
		}
	}
	return nil
}

func getOrCreateDefaultProject(database *db.DB) (*db.Project, error) {
	projects, err := database.GetProjects()
	if err != nil {
		return nil, err
	}
	switch len(projects) {
	case 0:
		p := &db.Project{Name: "default"}
		klog.Infoln("creating project:", p.Name)
		if err = database.SaveProject(p); err != nil {
			return nil, err
		}
		return p, nil
	case 1:
		return projects[0], nil
	}
	return nil, nil
}
