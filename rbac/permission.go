package rbac

import "github.com/coroot/coroot/utils"

type Permission struct {
	scope  scope
	action action
	object object
}

func (p Permission) allows(action Action) bool {
	if !utils.GlobMatch(string(action.scope), string(p.scope)) {
		return false
	}
	if !utils.GlobMatch(string(action.action), string(p.action)) {
		return false
	}
	for k, av := range action.object {
		pv := p.object[k]
		if pv == "" {
			pv = "*"
		}
		if !utils.GlobMatch(av, pv) {
			return false
		}
	}
	return true
}

type PermissionSet []Permission

func (ps PermissionSet) Allows(action Action) bool {
	for _, p := range ps {
		if p.allows(action) {
			return true
		}
	}
	return false
}

func AdminPermissionSet() PermissionSet {
	return PermissionSet{{scope: "*", action: "*"}}
}

func EditorPermissionSet() PermissionSet {
	return append(ViewerPermissionSet(),
		Permission{scope: scopeProjectApplicationCategories, action: actionEdit},
		Permission{scope: scopeProjectCustomApplications, action: actionEdit},
		Permission{scope: scopeProjectInspections, action: actionEdit},
	)
}

func ViewerPermissionSet() PermissionSet {
	return PermissionSet{{scope: "*", action: actionView}}
}
