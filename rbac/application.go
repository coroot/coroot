package rbac

import "github.com/coroot/coroot/model"

type ApplicationActionSet struct {
	project   *ProjectActionSet
	category  model.ApplicationCategory
	namespace string
	kind      model.ApplicationKind
	name      string
}

func (as ApplicationActionSet) object() object {
	o := as.project.object()
	o["application_category"] = string(as.category)
	o["application_namespace"] = as.namespace
	o["application_kind"] = string(as.kind)
	o["application_name"] = as.name
	return o
}

func (as ApplicationActionSet) View() Action {
	return Action{scope: scopeApplication, action: actionView, object: as.object()}
}
