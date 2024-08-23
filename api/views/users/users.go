package users

import (
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/rbac"
)

type View struct {
	Users []User          `json:"users"`
	Roles []rbac.RoleName `json:"roles"`
}

type User struct {
	Id       int           `json:"id"`
	Email    string        `json:"email"`
	Name     string        `json:"name"`
	Role     rbac.RoleName `json:"role"`
	Readonly bool          `json:"readonly"`
}

func Render(users []*db.User, roles []rbac.Role) *View {
	v := &View{}

	for _, user := range users {
		var role rbac.RoleName
		if len(user.Roles) > 0 {
			role = user.Roles[0]
		}
		v.Users = append(v.Users, User{Id: user.Id, Email: user.Email, Name: user.Name, Role: role, Readonly: user.Email == db.AdminUserLogin})
	}

	for _, role := range roles {
		v.Roles = append(v.Roles, role.Name)
	}

	return v
}
