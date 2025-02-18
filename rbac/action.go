package rbac

type Verb string
type Scope string
type Object map[string]string

const (
	ActionAll  Verb = "*"
	ActionView Verb = "view"
	ActionEdit Verb = "edit"

	ScopeAll                          Scope = "*"
	ScopeUsers                        Scope = "users"
	ScopeRoles                        Scope = "roles"
	ScopeProjectAll                   Scope = "project.*"
	ScopeProjectSettings              Scope = "project.settings"
	ScopeProjectIntegrations          Scope = "project.integrations"
	ScopeProjectApplicationCategories Scope = "project.application_categories"
	ScopeProjectCustomApplications    Scope = "project.custom_applications"
	ScopeProjectInspections           Scope = "project.inspections"
	ScopeProjectInstrumentations      Scope = "project.instrumentations"
	ScopeProjectTraces                Scope = "project.traces"
	ScopeProjectCosts                 Scope = "project.costs"
	ScopeProjectAnomalies             Scope = "project.anomalies"
	ScopeProjectRisks                 Scope = "project.risks"
	ScopeApplication                  Scope = "project.application"
	ScopeNode                         Scope = "project.node"
)

type Action struct {
	Scope  Scope
	Action Verb
	Object Object
}

func NewAction(scope Scope, action Verb, object Object) Action {
	return Action{Scope: scope, Action: action, Object: object}
}
