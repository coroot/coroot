package api

import (
	"crypto"
	"crypto/hmac"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coroot/coroot/api/forms"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/rbac"
	"github.com/coroot/coroot/utils"
	"k8s.io/klog"
)

type Session struct {
	Id int `json:"id"`
}

const (
	AuthSecretSettingName = "auth_secret"
	HashFunc              = crypto.SHA256
	SessionCookieName     = "coroot_session"
	SessionCookieTTL      = 7 * 24 * time.Hour
)

func (api *Api) AuthInit(anonymousRole string, adminPassword string) error {
	if anonymousRole != "" {
		role := rbac.RoleName(anonymousRole)
		roles, err := api.roles.GetRoles()
		if err != nil {
			return err
		}
		if !role.Valid(roles) {
			var names []rbac.RoleName
			for _, r := range roles {
				names = append(names, r.Name)
			}
			return fmt.Errorf("anonymous role must one of %s, got '%s'", names, role)
		}
		api.authAnonymousRole = role
		klog.Infoln("anonymous access enabled with the role:", role)
	}

	var secret string
	err := api.db.GetSetting(AuthSecretSettingName, &secret)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			secret = utils.RandomString(HashFunc.Size())
			err = api.db.SetSetting(AuthSecretSettingName, secret)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	api.authSecret = secret

	err = api.db.CreateAdminIfNotExists(adminPassword)
	if err != nil {
		return err
	}

	return nil
}

func (api *Api) Auth(h func(http.ResponseWriter, *http.Request, *db.User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if user := api.GetUser(r); user != nil {
			h(w, r, user)
			return
		}

		admin, err := api.db.DefaultAdminUserIsTheOnlyUser()
		if err != nil {
			klog.Errorln(err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		if admin != nil {
			http.Error(w, "set_admin_password", http.StatusUnauthorized)
		} else {
			http.Error(w, "", http.StatusUnauthorized)
		}
		return
	}
}

func (api *Api) Login(w http.ResponseWriter, r *http.Request) {
	var form forms.LoginForm
	if err := forms.ReadAndValidate(r, &form); err != nil {
		klog.Warningln("bad request:", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	var userId int
	switch form.Action {
	case "set_admin_password":
		admin, err := api.db.DefaultAdminUserIsTheOnlyUser()
		if err != nil {
			klog.Errorln(err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		if admin == nil {
			http.Error(w, "The admin password has already been set.", http.StatusUnauthorized)
			return
		}
		err = api.db.ChangeUserPassword(admin.Id, db.AdminUserDefaultPassword, form.Password)
		if err != nil {
			klog.Errorln(err)
			switch {
			case errors.Is(err, db.ErrNotFound):
				http.Error(w, "User not found.", http.StatusNotFound)
			case errors.Is(err, db.ErrInvalid):
				http.Error(w, "Invalid old password.", http.StatusBadRequest)
			case errors.Is(err, db.ErrConflict):
				http.Error(w, "New password can't be the same as the old one.", http.StatusBadRequest)
			default:
				http.Error(w, "", http.StatusInternalServerError)
			}
			return
		}
		userId = admin.Id
	default:
		id, err := api.db.AuthUser(form.Email, form.Password)
		if err != nil {
			klog.Errorln(err)
			if errors.Is(err, db.ErrNotFound) {
				http.Error(w, "Invalid email or password.", http.StatusNotFound)
			} else {
				http.Error(w, "", http.StatusInternalServerError)
			}
			return
		}
		userId = id
	}
	err := api.SetSessionCookie(w, userId, SessionCookieTTL)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
}

func (api *Api) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Path:     "/",
		HttpOnly: true,
	})
}

func (api *Api) SetSessionCookie(w http.ResponseWriter, userId int, ttl time.Duration) error {
	data, err := json.Marshal(Session{Id: userId})
	if err != nil {
		return err
	}
	h := hmac.New(HashFunc.New, []byte(api.authSecret))
	h.Write(data)
	value := base64.URLEncoding.EncodeToString(data) + "." + base64.URLEncoding.EncodeToString(h.Sum(nil))
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    value,
		Path:     "/",
		Expires:  time.Now().Add(ttl),
		HttpOnly: true,
	})
	return nil
}

func (api *Api) GetUser(r *http.Request) *db.User {
	if api.authAnonymousRole != "" {
		return db.AnonymousUser(api.authAnonymousRole)
	}

	c, _ := r.Cookie(SessionCookieName)
	if c == nil {
		return nil
	}
	parts := strings.Split(c.Value, ".")
	if len(parts) != 2 {
		klog.Errorln("invalid session")
		return nil
	}
	data, err := base64.URLEncoding.DecodeString(parts[0])
	if err != nil {
		klog.Errorln(err)
		return nil
	}
	h := hmac.New(HashFunc.New, []byte(api.authSecret))
	h.Write(data)
	if parts[1] != base64.URLEncoding.EncodeToString(h.Sum(nil)) {
		klog.Errorln("invalid session")
		return nil
	}
	var sess Session
	err = json.Unmarshal(data, &sess)
	if err != nil {
		klog.Errorln(err)
		return nil
	}

	user, err := api.db.GetUser(sess.Id)
	if err != nil {
		klog.Errorln(err)
		return nil
	}
	return user
}

func (api *Api) IsAllowed(u *db.User, actions ...rbac.Action) bool {
	roles, err := api.roles.GetRoles()
	if err != nil {
		klog.Errorln(err)
		return false
	}

	for _, rn := range u.Roles {
		for _, r := range roles {
			if r.Name != rn {
				continue
			}
			for _, action := range actions {
				if r.Permissions.Allows(action) {
					return true
				}
			}
		}
	}
	return false
}
