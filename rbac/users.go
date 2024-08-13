package rbac

func Users() UsersActionSet {
	return UsersActionSet{}
}

type UsersActionSet struct{}

func (as UsersActionSet) Edit() Action {
	return Action{scope: scopeUsers, action: actionEdit, object: nil}
}
