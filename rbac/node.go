package rbac

type NodeActionSet struct {
	project *ProjectActionSet
	name    string
}

func (as NodeActionSet) object() object {
	o := as.project.object()
	o["node_name"] = as.name
	return o
}

func (as NodeActionSet) View() Action {
	return Action{scope: scopeNode, action: actionView, object: as.object()}
}
