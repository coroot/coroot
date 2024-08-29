package rbac

import (
	"github.com/coroot/coroot/utils"
)

type Permission struct {
	Scope  Scope  `json:"scope"`
	Action Verb   `json:"action"`
	Object Object `json:"object"`
}

func NewPermission(scope Scope, action Verb, object Object) Permission {
	return Permission{Scope: scope, Action: action, Object: object}
}

func (p Permission) allows(action Action) bool {
	if !utils.GlobMatch(string(action.Scope), string(p.Scope)) {
		return false
	}
	if !utils.GlobMatch(string(action.Action), string(p.Action)) {
		return false
	}
	for k, av := range action.Object {
		pv := p.Object[k]
		if pv == "" {
			pv = "*"
		}
		if !utils.GlobMatch(av, pv) {
			return false
		}
	}
	return true
}

func (p Permission) allowsForObject(action Action) (bool, Object) {
	if !utils.GlobMatch(string(action.Scope), string(p.Scope)) {
		return false, nil
	}
	if !utils.GlobMatch(string(action.Action), string(p.Action)) {
		return false, nil
	}
	object := Object{}
	for k, v := range p.Object {
		_, ok := action.Object[k]
		if ok {
			object[k] = v
		}
	}
	if len(object) == 0 {
		object = nil
	}
	return true, object
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

func (ps PermissionSet) AllowsForObjects(action Action) []Object {
	var objects []Object
	for _, p := range ps {
		ok, obj := p.allowsForObject(action)
		if !ok {
			continue
		}
		if obj == nil {
			return []Object{}
		}
		objects = append(objects, obj)
	}
	return objects
}
