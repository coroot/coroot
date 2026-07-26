package users

import (
	"sort"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/rbac"
)

type Users struct {
	Users []User          `json:"users"`
	Roles []rbac.RoleName `json:"roles"`
}

type User struct {
	Id        int           `json:"id,omitempty"`
	Email     string        `json:"email"`
	Name      string        `json:"name"`
	Role      rbac.RoleName `json:"role"`
	Anonymous bool          `json:"anonymous,omitempty"`
	Readonly  bool          `json:"readonly,omitempty"`
	Projects  []Project     `json:"projects,omitempty"`
	// Menu controls which sidebar entries the UI shows (RBAC-driven).
	Menu Menu `json:"menu"`
}

// Menu is the permission-derived sidebar visibility for the current user.
type Menu struct {
	Settings      bool `json:"settings"`
	Project       bool `json:"project"`
	Nodes         bool `json:"nodes"`
	Kubernetes    bool `json:"kubernetes"`
	Costs         bool `json:"costs"`
	AlertingRules bool `json:"alerting_rules"`
	Inspections   bool `json:"inspections"`
}

type Project struct {
	Id   db.ProjectId `json:"id"`
	Name string       `json:"name"`
}

func RenderUsers(users []*db.User, roles []rbac.Role) *Users {
	v := &Users{}

	for _, user := range users {
		u := User{
			Id:       user.Id,
			Email:    user.Email,
			Name:     user.Name,
			Readonly: user.IsDefaultAdmin(),
		}
		if len(user.Roles) > 0 {
			u.Role = user.Roles[0]
		}
		v.Users = append(v.Users, u)
	}

	for _, role := range roles {
		v.Roles = append(v.Roles, role.Name)
	}

	return v
}

func RenderUser(user *db.User, projects map[db.ProjectId]string, viewonly bool, menu Menu) *User {
	v := &User{
		Name:      user.Name,
		Email:     user.Email,
		Anonymous: user.Anonymous,
		Readonly:  viewonly,
		Menu:      menu,
	}

	if len(user.Roles) > 0 {
		v.Role = user.Roles[0]
	}

	for id, name := range projects {
		v.Projects = append(v.Projects, Project{Id: id, Name: name})
	}
	sort.Slice(v.Projects, func(i, j int) bool {
		return v.Projects[i].Name < v.Projects[j].Name
	})

	return v
}
