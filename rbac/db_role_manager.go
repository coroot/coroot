package rbac

// DBRoleStore loads and mutates custom roles persisted outside the static builtins.
type DBRoleStore interface {
	GetCustomRoles() ([]Role, error)
	UpsertCustomRole(name RoleName, permissions PermissionSet) error
	DeleteCustomRole(name RoleName) error
}

// DBRoleManager merges builtin Admin/Editor/Viewer with custom roles from a store.
type DBRoleManager struct {
	store DBRoleStore
}

func NewDBRoleManager(store DBRoleStore) *DBRoleManager {
	return &DBRoleManager{store: store}
}

func (m *DBRoleManager) GetRoles() ([]Role, error) {
	custom, err := m.store.GetCustomRoles()
	if err != nil {
		return nil, err
	}
	roles := make([]Role, 0, len(Roles)+len(custom))
	roles = append(roles, Roles...)
	roles = append(roles, custom...)
	return roles, nil
}

func (m *DBRoleManager) SaveRole(name RoleName, permissions PermissionSet) error {
	return m.store.UpsertCustomRole(name, permissions)
}

func (m *DBRoleManager) DeleteRole(name RoleName) error {
	return m.store.DeleteCustomRole(name)
}

// MutableRoleManager is implemented by role managers that support custom role CRUD.
type MutableRoleManager interface {
	RoleManager
	SaveRole(name RoleName, permissions PermissionSet) error
	DeleteRole(name RoleName) error
}
