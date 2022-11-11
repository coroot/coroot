package db

type Integrations struct {
	BaseUrl string `json:"base_url"`

	Slack *IntegrationSlack `json:"slack,omitempty"`
}

type IntegrationSlack struct {
	Token          string `json:"token"`
	DefaultChannel string `json:"default_channel"`
	Enabled        bool   `json:"enabled"`
}

func (db *DB) SaveIntegrationsBaseUrl(id ProjectId, baseUrl string) error {
	p, err := db.GetProject(id)
	if err != nil {
		return err
	}
	p.Settings.Integrations.BaseUrl = baseUrl
	return db.saveProjectSettings(p)
}

func (db *DB) SaveIntegrationsSlack(id ProjectId, slack *IntegrationSlack) error {
	p, err := db.GetProject(id)
	if err != nil {
		return err
	}
	p.Settings.Integrations.Slack = slack
	return db.saveProjectSettings(p)
}
