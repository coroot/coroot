package config

import (
	"github.com/coroot/coroot/cloud"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"golang.org/x/exp/maps"
	"k8s.io/klog"
)

func (cfg *Config) Bootstrap(database *db.DB) error {
	if cfg.CorootCloud != nil {
		cloudAPI := cloud.API(database, "", "", "")
		err := cloudAPI.SaveSettings(*cfg.CorootCloud)
		if err != nil {
			return err
		}
	}

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
		p.Settings.Readonly = false
		p.Settings.Integrations.NotificationIntegrations.Readonly = false
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
		pp.Settings.Readonly = true
		pp.Settings.MemberProjects = p.MemberProjects
		pp.Settings.ApiKeys = p.ApiKeys
		if p.NotificationIntegrations != nil {
			pp.Settings.Integrations.NotificationIntegrations = *p.NotificationIntegrations
		}

		if p.RemoteCoroot != nil {
			pp.Prometheus = *p.RemoteCoroot.PrometheusConfig()
			if err = database.SaveProjectIntegration(pp, db.IntegrationTypePrometheus); err != nil {
				return err
			}
			pp.Settings.Integrations.Clickhouse = p.RemoteCoroot.ClickHouseConfig()
			if err = database.SaveProjectSettings(pp); err != nil {
				return err
			}
		}

		pp.Settings.Integrations.NotificationIntegrations.Readonly = p.NotificationIntegrations != nil
		if len(p.ApplicationCategories) > 0 {
			pp.Settings.ApplicationCategorySettings = map[model.ApplicationCategory]*db.ApplicationCategorySettings{}
			for _, c := range p.ApplicationCategories {
				pp.Settings.ApplicationCategorySettings[c.Name] = &c.ApplicationCategorySettings
			}
		}
		if len(p.CustomApplications) > 0 {
			pp.Settings.CustomApplications = map[string]model.CustomApplication{}
			for _, c := range p.CustomApplications {
				pp.Settings.CustomApplications[c.Name] = c.CustomApplication
			}
		}
		if p.InspectionOverrides != nil {
			for _, override := range p.InspectionOverrides.SLOAvailability {
				c := []model.CheckConfigSLOAvailability{{
					Custom:              false,
					ObjectivePercentage: override.ObjectivePercent,
				}}
				if err = database.SaveCheckConfig(pp.Id, override.ApplicationId, model.Checks.SLOAvailability.Id, c); err != nil {
					return err
				}
			}
			for _, override := range p.InspectionOverrides.SLOLatency {
				c := []model.CheckConfigSLOLatency{{
					Custom:              false,
					ObjectivePercentage: override.ObjectivePercent,
					ObjectiveBucket:     model.RoundUpToDefaultBucket(float32(override.ObjectiveThreshold.Seconds())),
				}}
				if err = database.SaveCheckConfig(pp.Id, override.ApplicationId, model.Checks.SLOLatency.Id, c); err != nil {
					return err
				}
			}
		}
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

	if cfg.DisableBuiltinAlerts {
		for _, p := range byName {
			if err := database.DisableBuiltinAlertingRules(p.Id); err != nil {
				klog.Errorln("failed to disable builtin alerting rules:", err)
			}
		}
	}

	for _, p := range cfg.Projects {
		pp := byName[p.Name]
		if pp == nil {
			continue
		}
		if len(p.AlertingRules) > 0 {
			if err := syncConfigAlertingRules(database, pp.Id, p.AlertingRules); err != nil {
				klog.Errorln("failed to sync config alerting rules:", err)
			}
		} else {
			if err := database.ClearAlertingRulesReadonly(pp.Id); err != nil {
				klog.Errorln("failed to clear alerting rules readonly:", err)
			}
		}
	}

	return nil
}

func syncConfigAlertingRules(database *db.DB, projectId db.ProjectId, configRules []AlertingRule) error {
	if err := database.ClearAlertingRulesReadonly(projectId); err != nil {
		return err
	}
	existing, err := database.GetAlertingRules(projectId)
	if err != nil {
		return err
	}
	byId := map[model.AlertingRuleId]*model.AlertingRule{}
	for _, r := range existing {
		byId[r.Id] = r
	}
	for _, cr := range configRules {
		ruleId := model.AlertingRuleId(cr.Id)
		if existingRule, ok := byId[ruleId]; ok {
			merged := applyConfigOverrides(existingRule, cr)
			merged.Readonly = true
			if err := database.UpdateAlertingRule(projectId, merged); err != nil {
				return err
			}
		} else {
			base := &model.AlertingRule{
				Id:       ruleId,
				Readonly: true,
				Enabled:  true,
			}
			newRule := applyConfigOverrides(base, cr)
			newRule.ProjectId = string(projectId)
			if err := database.CreateAlertingRule(projectId, newRule); err != nil {
				return err
			}
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
		return maps.Values(projects)[0], nil
	}
	return nil, nil
}
