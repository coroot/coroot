package rbac

import "github.com/coroot/coroot/model"

func Project(id string) ProjectActionSet {
	return ProjectActionSet{id: id}
}

type ProjectActionSet struct {
	id string
}

func (as ProjectActionSet) object() object {
	return object{"project_id": as.id}
}

func (as ProjectActionSet) Settings() ProjectEditAction {
	return ProjectEditAction{project: &as, scope: scopeProjectSettings}
}

func (as ProjectActionSet) Integrations() ProjectEditAction {
	return ProjectEditAction{project: &as, scope: scopeProjectIntegrations}
}

func (as ProjectActionSet) ApplicationCategories() ProjectEditAction {
	return ProjectEditAction{project: &as, scope: scopeProjectApplicationCategories}
}

func (as ProjectActionSet) CustomApplications() ProjectEditAction {
	return ProjectEditAction{project: &as, scope: scopeProjectCustomApplications}
}

func (as ProjectActionSet) Inspections() ProjectEditAction {
	return ProjectEditAction{project: &as, scope: scopeProjectInspections}
}

func (as ProjectActionSet) Instrumentations() ProjectEditAction {
	return ProjectEditAction{project: &as, scope: scopeProjectInstrumentations}
}

func (as ProjectActionSet) Traces() ProjectViewAction {
	return ProjectViewAction{project: &as, scope: scopeProjectTraces}
}

func (as ProjectActionSet) Costs() ProjectViewAction {
	return ProjectViewAction{project: &as, scope: scopeProjectCosts}
}

func (as ProjectActionSet) Application(category model.ApplicationCategory, namespace string, kind model.ApplicationKind, name string) ApplicationActionSet {
	return ApplicationActionSet{project: &as, category: category, namespace: namespace, kind: kind, name: name}
}

func (as ProjectActionSet) Node(name string) NodeActionSet {
	return NodeActionSet{project: &as, name: name}
}

type ProjectViewAction struct {
	project *ProjectActionSet
	scope   scope
}

func (as ProjectViewAction) View() Action {
	return Action{scope: as.scope, action: actionView, object: as.project.object()}
}

type ProjectEditAction struct {
	project *ProjectActionSet
	scope   scope
}

func (as ProjectEditAction) Edit() Action {
	return Action{scope: as.scope, action: actionEdit, object: as.project.object()}
}
