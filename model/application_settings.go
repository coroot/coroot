package model

type ApplicationSettings struct {
	Profiling *ApplicationSettingsProfiling `json:"profiling,omitempty"`
	Tracing   *ApplicationSettingsTracing   `json:"tracing,omitempty"`
	Logs      *ApplicationSettingsLogs      `json:"logs,omitempty"`

	Instrumentation map[ApplicationType]*ApplicationInstrumentation `json:"instrumentation,omitempty"`

	RiskOverrides []RiskOverride `json:"risk_overrides,omitempty"`
}

type ApplicationSettingsProfiling struct {
	Service string `json:"service"`
}

type ApplicationSettingsTracing struct {
	Service string `json:"service"`
}

type ApplicationSettingsLogs struct {
	Service string `json:"service"`
}

type ApplicationInstrumentation struct {
	Type        ApplicationType   `json:"type"`
	Host        string            `json:"host,omitempty"`
	Port        string            `json:"port"`
	Credentials Credentials       `json:"credentials"`
	Params      map[string]string `json:"params"`
	Disabled    bool              `json:"disabled"`
}

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func GetDefaultInstrumentation(t ApplicationType) *ApplicationInstrumentation {
	switch t {
	case ApplicationTypePostgres:
		return &ApplicationInstrumentation{Type: ApplicationTypePostgres, Port: "5432"}
	case ApplicationTypeRedis:
		return &ApplicationInstrumentation{Type: ApplicationTypeRedis, Port: "6379"}
	case ApplicationTypeMongodb:
		return &ApplicationInstrumentation{Type: ApplicationTypeMongodb, Port: "27017"}
	case ApplicationTypeMemcached:
		return &ApplicationInstrumentation{Type: ApplicationTypeMemcached, Port: "11211"}
	case ApplicationTypeMysql:
		return &ApplicationInstrumentation{Type: ApplicationTypeMysql, Port: "3306"}
	}
	return nil
}
