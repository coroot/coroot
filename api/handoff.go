package api

import (
	"crypto/subtle"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/coroot/coroot/api/forms"
	"github.com/coroot/coroot/rbac"
	"github.com/coroot/coroot/utils"
	"k8s.io/klog"
)

const (
	HandoffSecretHeader  = "X-Handoff-Secret"
	DefaultHandoffTTL    = 5 * time.Minute
	MaxHandoffTTL        = 10 * time.Minute
	MinHandoffTTL        = time.Minute
	handoffTokenBytes    = 32
	handoffSweepInterval = time.Minute
)

type handoffToken struct {
	UserId   int
	Redirect string
	Expires  time.Time
}

type handoffStore struct {
	mu     sync.Mutex
	tokens map[string]handoffToken
	once   sync.Once
}

func (s *handoffStore) init() {
	s.once.Do(func() {
		s.tokens = map[string]handoffToken{}
		go func() {
			t := time.NewTicker(handoffSweepInterval)
			defer t.Stop()
			for range t.C {
				s.sweep()
			}
		}()
	})
}

func (s *handoffStore) put(token string, ht handoffToken) {
	s.init()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[token] = ht
}

func (s *handoffStore) consume(token string) (handoffToken, bool) {
	s.init()
	s.mu.Lock()
	defer s.mu.Unlock()
	ht, ok := s.tokens[token]
	if !ok {
		return handoffToken{}, false
	}
	delete(s.tokens, token)
	if time.Now().After(ht.Expires) {
		return handoffToken{}, false
	}
	return ht, true
}

func (s *handoffStore) sweep() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, v := range s.tokens {
		if now.After(v.Expires) {
			delete(s.tokens, k)
		}
	}
}

// CreateHandoff mints a single-use OTT for Kubero (or other trusted callers).
// Protected by AUTH_HANDOFF_SECRET via X-Handoff-Secret or Authorization: Bearer.
func (api *Api) CreateHandoff(w http.ResponseWriter, r *http.Request) {
	if !api.checkHandoffSecret(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var form forms.HandoffCreateForm
	if err := forms.ReadAndValidate(r, &form); err != nil {
		klog.Warningln("bad handoff create:", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	role := form.Role
	if role == "" {
		role = rbac.RoleViewer
	}

	if len(form.Permissions) > 0 {
		if role.Builtin() {
			http.Error(w, "cannot attach custom permissions to a builtin role", http.StatusBadRequest)
			return
		}
		mgr, ok := api.roles.(rbac.MutableRoleManager)
		if !ok {
			http.Error(w, "custom roles are not supported", http.StatusInternalServerError)
			return
		}
		if err := mgr.SaveRole(role, form.Permissions); err != nil {
			klog.Errorln(err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	} else {
		roles, err := api.roles.GetRoles()
		if err != nil {
			klog.Errorln(err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		if !role.Valid(roles) {
			http.Error(w, "unknown role", http.StatusBadRequest)
			return
		}
	}

	user, err := api.db.EnsureUser(form.Email, form.Name, role)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	ttl := DefaultHandoffTTL
	if form.TTLSeconds > 0 {
		ttl = time.Duration(form.TTLSeconds) * time.Second
	}
	if ttl < MinHandoffTTL {
		ttl = MinHandoffTTL
	}
	if ttl > MaxHandoffTTL {
		ttl = MaxHandoffTTL
	}

	token := utils.RandomString(handoffTokenBytes)
	redirect := sanitizeHandoffRedirect(form.Redirect)
	api.handoffs.put(token, handoffToken{
		UserId:   user.Id,
		Redirect: redirect,
		Expires:  time.Now().Add(ttl),
	})

	handoffPath := path.Join(api.cfg.UrlBasePath, "api/auth/handoff")
	q := url.Values{"token": {token}}
	handoffURL := handoffPath + "?" + q.Encode()

	utils.WriteJson(w, map[string]any{
		"handoff_url": handoffURL,
		"expires_in":  int(ttl.Seconds()),
		"user_id":     user.Id,
		"email":       user.Email,
		"role":        role,
	})
}

// ConsumeHandoff exchanges a one-time token for a coroot_session cookie and redirects.
func (api *Api) ConsumeHandoff(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		http.Error(w, "token is required", http.StatusBadRequest)
		return
	}
	ht, ok := api.handoffs.consume(token)
	if !ok {
		http.Error(w, "invalid or expired token", http.StatusUnauthorized)
		return
	}
	if err := api.SetSessionCookie(w, ht.UserId, SessionCookieTTL); err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	target := ht.Redirect
	if target == "" {
		target = api.cfg.UrlBasePath
		if target == "" {
			target = "/"
		}
	} else if api.cfg.UrlBasePath != "/" && !strings.HasPrefix(target, api.cfg.UrlBasePath) {
		joined := path.Join(api.cfg.UrlBasePath, strings.TrimPrefix(target, "/"))
		if strings.HasSuffix(ht.Redirect, "/") && !strings.HasSuffix(joined, "/") {
			joined += "/"
		}
		target = joined
	}
	http.Redirect(w, r, target, http.StatusFound)
}

func (api *Api) checkHandoffSecret(r *http.Request) bool {
	secret := strings.TrimSpace(api.cfg.Auth.HandoffSecret)
	if secret == "" {
		klog.Warningln("AUTH_HANDOFF_SECRET is not configured")
		return false
	}
	provided := strings.TrimSpace(r.Header.Get(HandoffSecretHeader))
	if provided == "" {
		auth := r.Header.Get("Authorization")
		if len(auth) > 7 && strings.EqualFold(auth[:7], "bearer ") {
			provided = strings.TrimSpace(auth[7:])
		}
	}
	if provided == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(secret)) == 1
}

func sanitizeHandoffRedirect(redirect string) string {
	redirect = strings.TrimSpace(redirect)
	if redirect == "" {
		return ""
	}
	// Block open redirects to external hosts.
	if strings.HasPrefix(redirect, "//") {
		return ""
	}
	if u, err := url.Parse(redirect); err == nil && u.IsAbs() {
		return ""
	}
	if !strings.HasPrefix(redirect, "/") {
		redirect = "/" + redirect
	}
	return redirect
}
