package db

import (
	"fmt"
	"strings"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/utils"
	"golang.org/x/exp/maps"
)

type ApplicationCategory struct {
	Name                 model.ApplicationCategory               `json:"name"`
	Builtin              bool                                    `json:"builtin"`
	Default              bool                                    `json:"default"`
	BuiltinPatterns      string                                  `json:"builtin_patterns"`
	CustomPatterns       string                                  `json:"custom_patterns"`
	NotificationSettings ApplicationCategoryNotificationSettings `json:"notification_settings"`
}

type ApplicationCategorySettings struct {
	CustomPatterns       []string                                `json:"custom_patterns,omitempty"`
	NotifyOfDeployments  bool                                    `json:"notify_of_deployments,omitempty"` // deprecated: use NotificationSettings
	NotificationSettings ApplicationCategoryNotificationSettings `json:"notification_settings"`
}

type ApplicationCategoryNotificationSettings struct {
	Incidents   ApplicationCategoryIncidentNotificationSettings   `json:"incidents,omitempty"`
	Deployments ApplicationCategoryDeploymentNotificationSettings `json:"deployments,omitempty"`
}

type ApplicationCategoryIncidentNotificationSettings struct {
	Enabled bool `json:"enabled"`
	ApplicationCategoryNotificationDestinations
}

type ApplicationCategoryDeploymentNotificationSettings struct {
	Enabled bool `json:"enabled"`
	ApplicationCategoryNotificationDestinations
}

type ApplicationCategoryNotificationDestinations struct {
	Slack     *ApplicationCategoryNotificationSettingsSlack     `json:"slack,omitempty"`
	Teams     *ApplicationCategoryNotificationSettingsTeams     `json:"teams,omitempty"`
	Pagerduty *ApplicationCategoryNotificationSettingsPagerduty `json:"pagerduty,omitempty"`
	Opsgenie  *ApplicationCategoryNotificationSettingsOpsgenie  `json:"opsgenie,omitempty"`
	Webhook   *ApplicationCategoryNotificationSettingsWebhook   `json:"webhook,omitempty"`
}

func (s ApplicationCategoryNotificationDestinations) hasEnabled() bool {
	return (s.Slack != nil && s.Slack.Enabled) ||
		(s.Teams != nil && s.Teams.Enabled) ||
		(s.Pagerduty != nil && s.Pagerduty.Enabled) ||
		(s.Opsgenie != nil && s.Opsgenie.Enabled) ||
		(s.Webhook != nil && s.Webhook.Enabled)
}

type ApplicationCategoryNotificationSettingsSlack struct {
	Enabled bool   `json:"enabled"`
	Channel string `json:"channel"`
}

type ApplicationCategoryNotificationSettingsTeams struct {
	Enabled bool `json:"enabled"`
}

type ApplicationCategoryNotificationSettingsPagerduty struct {
	Enabled bool `json:"enabled"`
}

type ApplicationCategoryNotificationSettingsOpsgenie struct {
	Enabled bool `json:"enabled"`
}

type ApplicationCategoryNotificationSettingsWebhook struct {
	Enabled bool `json:"enabled"`
}

func (p *Project) CalcApplicationCategory(appId model.ApplicationId) model.ApplicationCategory {
	id := fmt.Sprintf("%s/%s", appId.Namespace, appId.Name)

	settings := p.Settings.ApplicationCategorySettings
	names := maps.Keys(settings)
	utils.SortSlice(names)
	for _, name := range names {
		if s := settings[name]; s == nil || len(s.CustomPatterns) == 0 {
			continue
		} else if utils.GlobMatch(id, s.CustomPatterns...) {
			return name
		}
	}

	names = maps.Keys(model.BuiltinCategoryPatterns)
	utils.SortSlice(names)
	for _, name := range names {
		if utils.GlobMatch(id, model.BuiltinCategoryPatterns[name]...) {
			return name
		}
	}

	return model.ApplicationCategoryApplication
}

func (p *Project) GetApplicationCategories() map[model.ApplicationCategory]*ApplicationCategory {
	res := map[model.ApplicationCategory]*ApplicationCategory{}
	for c, settings := range p.Settings.ApplicationCategorySettings {
		if c.Builtin() {
			continue
		}
		category := &ApplicationCategory{
			Name: c,
		}
		if settings != nil {
			category.CustomPatterns = strings.Join(settings.CustomPatterns, " ")
		}
		res[c] = category
	}
	for c, patterns := range model.BuiltinCategoryPatterns {
		category := &ApplicationCategory{
			Name:            c,
			Builtin:         true,
			Default:         c.Default(),
			BuiltinPatterns: strings.Join(patterns, " "),
		}
		if settings := p.Settings.ApplicationCategorySettings[c]; settings != nil {
			category.CustomPatterns = strings.Join(settings.CustomPatterns, " ")
		}
		res[c] = category
	}

	for _, category := range res {
		categorySettings := p.Settings.ApplicationCategorySettings[category.Name]
		if categorySettings == nil {
			categorySettings = &ApplicationCategorySettings{}
		}
		category.NotificationSettings = categorySettings.NotificationSettings
		notifyOfDeployments := category.Default || categorySettings.NotifyOfDeployments

		{
			integrationSlack := p.Settings.Integrations.Slack
			if integrationSlack != nil {
				if integrationSlack.Incidents {
					if category.NotificationSettings.Incidents.Slack == nil {
						category.NotificationSettings.Incidents.Enabled = true
						category.NotificationSettings.Incidents.Slack = &ApplicationCategoryNotificationSettingsSlack{Enabled: true}
					}
					if category.NotificationSettings.Incidents.Slack.Channel == "" {
						category.NotificationSettings.Incidents.Slack.Channel = integrationSlack.DefaultChannel
					}
				}
				if integrationSlack.Deployments {
					if category.NotificationSettings.Deployments.Slack == nil {
						category.NotificationSettings.Deployments.Enabled = notifyOfDeployments
						category.NotificationSettings.Deployments.Slack = &ApplicationCategoryNotificationSettingsSlack{Enabled: notifyOfDeployments}
					}
					if category.NotificationSettings.Deployments.Slack.Channel == "" {
						category.NotificationSettings.Deployments.Slack.Channel = integrationSlack.DefaultChannel
					}
				}
			}
			if integrationSlack == nil || !integrationSlack.Incidents {
				category.NotificationSettings.Incidents.Slack = nil
			}
			if integrationSlack == nil || !integrationSlack.Deployments {
				category.NotificationSettings.Deployments.Slack = nil
			}
		}
		{
			integrationTeams := p.Settings.Integrations.Teams
			if integrationTeams != nil {
				if integrationTeams.Incidents {
					if category.NotificationSettings.Incidents.Teams == nil {
						category.NotificationSettings.Incidents.Enabled = true
						category.NotificationSettings.Incidents.Teams = &ApplicationCategoryNotificationSettingsTeams{Enabled: true}
					}
				}
				if integrationTeams.Deployments {
					if category.NotificationSettings.Deployments.Teams == nil {
						category.NotificationSettings.Deployments.Enabled = notifyOfDeployments
						category.NotificationSettings.Deployments.Teams = &ApplicationCategoryNotificationSettingsTeams{Enabled: notifyOfDeployments}
					}
				}
			}
			if integrationTeams == nil || !integrationTeams.Incidents {
				category.NotificationSettings.Incidents.Teams = nil
			}
			if integrationTeams == nil || !integrationTeams.Deployments {
				category.NotificationSettings.Deployments.Teams = nil
			}
		}
		{
			integrationWebhook := p.Settings.Integrations.Webhook
			if integrationWebhook != nil {
				if integrationWebhook.Incidents {
					if category.NotificationSettings.Incidents.Webhook == nil {
						category.NotificationSettings.Incidents.Enabled = true
						category.NotificationSettings.Incidents.Webhook = &ApplicationCategoryNotificationSettingsWebhook{Enabled: true}
					}
				}
				if integrationWebhook.Deployments {
					if category.NotificationSettings.Deployments.Webhook == nil {
						category.NotificationSettings.Deployments.Enabled = notifyOfDeployments
						category.NotificationSettings.Deployments.Webhook = &ApplicationCategoryNotificationSettingsWebhook{Enabled: notifyOfDeployments}
					}
				}
			}
			if integrationWebhook == nil || !integrationWebhook.Incidents {
				category.NotificationSettings.Incidents.Webhook = nil
			}
			if integrationWebhook == nil || !integrationWebhook.Deployments {
				category.NotificationSettings.Deployments.Webhook = nil
			}
		}
		{
			integrationPagerduty := p.Settings.Integrations.Pagerduty
			if integrationPagerduty != nil {
				if integrationPagerduty.Incidents {
					category.NotificationSettings.Incidents.Enabled = true
					if category.NotificationSettings.Incidents.Pagerduty == nil {
						category.NotificationSettings.Incidents.Pagerduty = &ApplicationCategoryNotificationSettingsPagerduty{Enabled: true}
					}
				}
			}
			if integrationPagerduty == nil || !integrationPagerduty.Incidents {
				category.NotificationSettings.Incidents.Pagerduty = nil
			}
		}
		{
			integrationOpsgenie := p.Settings.Integrations.Opsgenie
			if integrationOpsgenie != nil {
				if integrationOpsgenie.Incidents {
					category.NotificationSettings.Incidents.Enabled = true
					if category.NotificationSettings.Incidents.Opsgenie == nil {
						category.NotificationSettings.Incidents.Opsgenie = &ApplicationCategoryNotificationSettingsOpsgenie{Enabled: true}
					}
				}
			}
			if integrationOpsgenie == nil || !integrationOpsgenie.Incidents {
				category.NotificationSettings.Incidents.Opsgenie = nil
			}
		}

		if !category.NotificationSettings.Incidents.hasEnabled() {
			category.NotificationSettings.Incidents.Enabled = false
		}
		if !category.NotificationSettings.Deployments.hasEnabled() {
			category.NotificationSettings.Deployments.Enabled = false
		}
	}

	return res
}

func (p *Project) NewApplicationCategory() *ApplicationCategory {
	category := &ApplicationCategory{}
	if slack := p.Settings.Integrations.Slack; slack != nil {
		if slack.Incidents {
			category.NotificationSettings.Incidents.Slack = &ApplicationCategoryNotificationSettingsSlack{Channel: slack.DefaultChannel}
		}
		if slack.Deployments {
			category.NotificationSettings.Deployments.Slack = &ApplicationCategoryNotificationSettingsSlack{Channel: slack.DefaultChannel}
		}
	}
	if teams := p.Settings.Integrations.Teams; teams != nil {
		if teams.Incidents {
			category.NotificationSettings.Incidents.Teams = &ApplicationCategoryNotificationSettingsTeams{}
		}
		if teams.Deployments {
			category.NotificationSettings.Deployments.Teams = &ApplicationCategoryNotificationSettingsTeams{}
		}
	}
	if webhook := p.Settings.Integrations.Webhook; webhook != nil {
		if webhook.Incidents {
			category.NotificationSettings.Incidents.Webhook = &ApplicationCategoryNotificationSettingsWebhook{}
		}
		if webhook.Deployments {
			category.NotificationSettings.Deployments.Webhook = &ApplicationCategoryNotificationSettingsWebhook{}
		}
	}
	if pagerduty := p.Settings.Integrations.Pagerduty; pagerduty != nil {
		if pagerduty.Incidents {
			category.NotificationSettings.Incidents.Pagerduty = &ApplicationCategoryNotificationSettingsPagerduty{}
		}
	}
	if opsgenie := p.Settings.Integrations.Opsgenie; opsgenie != nil {
		if opsgenie.Incidents {
			category.NotificationSettings.Incidents.Opsgenie = &ApplicationCategoryNotificationSettingsOpsgenie{}
		}
	}
	return category
}

func (db *DB) SaveApplicationCategory(project *Project, name model.ApplicationCategory, category *ApplicationCategory) error {
	settings := project.Settings.ApplicationCategorySettings
	if settings == nil {
		settings = map[model.ApplicationCategory]*ApplicationCategorySettings{}
		project.Settings.ApplicationCategorySettings = settings
	}

	if category == nil { // delete
		if !name.Builtin() {
			delete(settings, name)
			return db.SaveProjectSettings(project)
		}
		return nil
	}

	if name != category.Name && (name.Builtin() || settings[category.Name] != nil) {
		return ErrConflict
	}

	if !name.Builtin() && category.Name != name {
		delete(settings, name)
	}
	categorySettings := settings[category.Name]
	if categorySettings == nil {
		categorySettings = &ApplicationCategorySettings{}
		settings[category.Name] = categorySettings
	}
	if !category.Name.Default() {
		categorySettings.CustomPatterns = strings.Fields(category.CustomPatterns)
	}
	categorySettings.NotificationSettings = category.NotificationSettings
	if slack := categorySettings.NotificationSettings.Incidents.Slack; slack != nil {
		if s := project.Settings.Integrations.Slack; s != nil && slack.Channel == s.DefaultChannel {
			slack.Channel = ""
		}
	}
	if slack := categorySettings.NotificationSettings.Deployments.Slack; slack != nil {
		if s := project.Settings.Integrations.Slack; s != nil && slack.Channel == s.DefaultChannel {
			slack.Channel = ""
		}
	}

	return db.SaveProjectSettings(project)
}

func (db *DB) migrateApplicationCategories(p *Project) error {
	if p.Settings.ApplicationCategories == nil {
		return nil
	}
	if p.Settings.ApplicationCategorySettings == nil {
		p.Settings.ApplicationCategorySettings = map[model.ApplicationCategory]*ApplicationCategorySettings{}
	}
	for name, patterns := range p.Settings.ApplicationCategories {
		if settings := p.Settings.ApplicationCategorySettings[name]; settings != nil && len(settings.CustomPatterns) == 0 {
			settings.CustomPatterns = patterns
		} else {
			p.Settings.ApplicationCategorySettings[name] = &ApplicationCategorySettings{CustomPatterns: patterns}
		}
	}
	//p.Settings.ApplicationCategories = nil
	return db.SaveProjectSettings(p)
}
