package rbac

type action string
type scope string

const (
	actionView action = "view"
	actionEdit action = "edit"

	scopeUsers                        scope = "users"
	scopeProjectSettings              scope = "project.settings"
	scopeProjectIntegrations          scope = "project.integrations"
	scopeProjectApplicationCategories scope = "project.application_categories"
	scopeProjectCustomApplications    scope = "project.custom_applications"
	scopeProjectInspections           scope = "project.inspections"
	scopeProjectInstrumentations      scope = "project.instrumentations"
	scopeProjectTraces                scope = "project.traces"
	scopeProjectCosts                 scope = "project.costs"
	scopeApplication                  scope = "project.application"
	scopeNode                         scope = "project.node"
)

type object map[string]string

type Action struct {
	scope  scope
	action action
	object object
}
