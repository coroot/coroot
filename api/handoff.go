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
	UserId    int
	Redirect  string
	Theme     string // light|dark|auto — from Kubero (or redirect ?theme=)
	ReturnUrl string // absolute dashboard URL for the "Back to main panel" link
	Workspace string // workspace/tenant display name for the sidebar header
	Expires   time.Time
}

const (
	corootThemeCookie     = "coroot_theme"
	corootReturnUrlCookie = "coroot_return_url"
	corootWorkspaceCookie = "coroot_workspace"
	// Short-lived: the SPA persists these to localStorage on first paint, so
	// the cookies only need to survive the base-path redirect.
	handoffSignalTTL = 600 * time.Second
)

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
	theme := normalizeHandoffTheme(form.Theme)
	if theme == "" {
		theme = themeFromRedirect(redirect)
	}
	returnUrl := strings.TrimSpace(form.ReturnUrl)
	workspace := strings.TrimSpace(form.Workspace)
	api.handoffs.put(token, handoffToken{
		UserId:    user.Id,
		Redirect:  redirect,
		Theme:     theme,
		ReturnUrl: returnUrl,
		Workspace: workspace,
		Expires:   time.Now().Add(ttl),
	})

	handoffPath := path.Join(api.cfg.UrlBasePath, "api/auth/handoff")
	q := url.Values{"token": {token}}
	if theme != "" {
		q.Set("theme", theme)
	}
	if returnUrl != "" {
		q.Set("return_url", returnUrl)
	}
	if workspace != "" {
		q.Set("workspace", workspace)
	}
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
	theme := normalizeHandoffTheme(ht.Theme)
	if theme == "" {
		theme = normalizeHandoffTheme(r.URL.Query().Get("theme"))
	}
	if theme != "" {
		api.setThemeCookie(w, theme)
	}
	returnUrl := strings.TrimSpace(ht.ReturnUrl)
	if returnUrl == "" {
		returnUrl = strings.TrimSpace(r.URL.Query().Get("return_url"))
	}
	if returnUrl != "" {
		api.setHandoffSignalCookie(w, corootReturnUrlCookie, returnUrl)
	}
	workspace := strings.TrimSpace(ht.Workspace)
	if workspace == "" {
		workspace = strings.TrimSpace(r.URL.Query().Get("workspace"))
	}
	if workspace != "" {
		api.setHandoffSignalCookie(w, corootWorkspaceCookie, workspace)
	}
	target := ht.Redirect
	if target == "" {
		target = api.cfg.UrlBasePath
		if target == "" {
			target = "/"
		}
	} else if api.cfg.UrlBasePath != "/" && !strings.HasPrefix(target, api.cfg.UrlBasePath) {
		target = joinBasePathPreserveQuery(api.cfg.UrlBasePath, target)
	}
	if theme != "" {
		target = appendQueryParam(target, "theme", theme)
	}
	if returnUrl != "" {
		target = appendQueryParam(target, "return_url", returnUrl)
	}
	if workspace != "" {
		target = appendQueryParam(target, "workspace", workspace)
	}
	http.Redirect(w, r, target, http.StatusFound)
}

func (api *Api) setThemeCookie(w http.ResponseWriter, theme string) {
	http.SetCookie(w, &http.Cookie{
		Name:     corootThemeCookie,
		Value:    theme,
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60,
		SameSite: http.SameSiteLaxMode,
		// Readable by the SPA so theme can apply before Vue mounts.
		HttpOnly: false,
	})
}

// setHandoffSignalCookie writes a short-lived, SPA-readable cookie (return_url
// or workspace) that survives the base-path redirect. Mirrors setThemeCookie
// but with a short Max-Age since the SPA persists the value to localStorage.
func (api *Api) setHandoffSignalCookie(w http.ResponseWriter, name, value string) {
	if value == "" {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    url.QueryEscape(value),
		Path:     "/",
		MaxAge:   int(handoffSignalTTL.Seconds()),
		SameSite: http.SameSiteLaxMode,
		HttpOnly: false,
	})
}

func normalizeHandoffTheme(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "light", "dark", "auto":
		return strings.ToLower(strings.TrimSpace(t))
	default:
		return ""
	}
}

func themeFromRedirect(redirect string) string {
	u, err := url.Parse(redirect)
	if err != nil {
		return ""
	}
	return normalizeHandoffTheme(u.Query().Get("theme"))
}

// joinBasePathPreserveQuery joins UrlBasePath with a relative redirect without
// dropping ?query or #fragment (path.Join alone is easy to misuse here).
func joinBasePathPreserveQuery(base, target string) string {
	frag := ""
	if i := strings.Index(target, "#"); i >= 0 {
		frag = target[i:]
		target = target[:i]
	}
	query := ""
	if i := strings.Index(target, "?"); i >= 0 {
		query = target[i:]
		target = target[:i]
	}
	joined := path.Join(base, strings.TrimPrefix(target, "/"))
	if strings.HasSuffix(target, "/") && !strings.HasSuffix(joined, "/") {
		joined += "/"
	}
	return joined + query + frag
}

func appendQueryParam(raw, key, value string) string {
	if value == "" {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Path == "" && !strings.HasPrefix(raw, "/") {
		sep := "?"
		if strings.Contains(raw, "?") {
			sep = "&"
		}
		return raw + sep + url.QueryEscape(key) + "=" + url.QueryEscape(value)
	}
	q := u.Query()
	q.Set(key, value)
	u.RawQuery = q.Encode()
	// url.Parse("/p/x?a=1") keeps Path; String() is fine for relative refs.
	if u.Scheme == "" && u.Host == "" {
		out := u.Path
		if u.RawQuery != "" {
			out += "?" + u.RawQuery
		}
		if u.Fragment != "" {
			out += "#" + u.Fragment
		}
		return out
	}
	return u.String()
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
