package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/utils"
	"github.com/golang-jwt/jwt/v5"
	"k8s.io/klog"
)

const (
	mcpOAuthCodeTTL          = 30 * time.Second
	mcpOAuthAccessTokenTTL   = time.Hour
	mcpOAuthRefreshTokenTTL  = 30 * 24 * time.Hour
	mcpOAuthScope            = "mcp"
	mcpBearerPrefix          = "Bearer "
	mcpOAuthIssuerPath       = "/"
	mcpOAuthAuthorizePath    = "/oauth/authorize"
	mcpOAuthTokenPath        = "/oauth/token"
	mcpOAuthRegisterPath     = "/oauth/register"
	mcpOAuthRevokePath       = "/oauth/revoke"
	mcpOAuthResourcePath     = "/mcp"
	mcpOAuthMaxClientNameLen = 200
	mcpOAuthMaxRedirectURIs  = 5
)

const (
	mcpAudClient  = "mcp:client"
	mcpAudCode    = "mcp:code"
	mcpAudAccess  = "mcp:access"
	mcpAudRefresh = "mcp:refresh"
)

const (
	oauthErrInvalidRequest          = "invalid_request"
	oauthErrInvalidGrant            = "invalid_grant"
	oauthErrUnauthorizedClient      = "unauthorized_client"
	oauthErrUnsupportedGrantType    = "unsupported_grant_type"
	oauthErrUnsupportedResponseType = "unsupported_response_type"
	oauthErrAccessDenied            = "access_denied"
	oauthErrInvalidClientMetadata   = "invalid_client_metadata"
	oauthErrInvalidRedirectURI      = "invalid_redirect_uri"
)

type mcpClientClaims struct {
	Name         string   `json:"client_name"`
	RedirectURIs []string `json:"redirect_uris"`
	jwt.RegisteredClaims
}

type mcpCodeClaims struct {
	ClientId      string `json:"client_id"`
	RedirectURI   string `json:"redirect_uri"`
	CodeChallenge string `json:"code_challenge"`
	jwt.RegisteredClaims
}

type mcpTokenClaims struct {
	ClientId string `json:"client_id"`
	jwt.RegisteredClaims
}

func (api *Api) mcpAuthSecret() ([]byte, error) {
	var secret string
	if err := api.db.GetSetting(AuthSecretSettingName, &secret); err != nil {
		return nil, fmt.Errorf("failed to get auth secret: %w", err)
	}
	return []byte(secret), nil
}

func (api *Api) mcpSign(claims jwt.Claims) (string, error) {
	secret, err := api.mcpAuthSecret()
	if err != nil {
		return "", err
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}

func (api *Api) mcpVerify(audience string, token string, out jwt.Claims) error {
	secret, err := api.mcpAuthSecret()
	if err != nil {
		return err
	}
	_, err = jwt.ParseWithClaims(token, out, func(*jwt.Token) (any, error) { return secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithAudience(audience),
	)
	return err
}

func (api *Api) mcpIssueToken(audience string, userId int, clientId string, ttl time.Duration) (string, error) {
	return api.mcpSign(mcpTokenClaims{
		ClientId: clientId,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.Itoa(userId),
			Audience:  jwt.ClaimStrings{audience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
	})
}

func (api *Api) MCPUserFromBearer(r *http.Request) *db.User {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, mcpBearerPrefix) {
		return nil
	}
	token := strings.TrimPrefix(auth, mcpBearerPrefix)
	var claims mcpTokenClaims
	if err := api.mcpVerify(mcpAudAccess, token, &claims); err != nil {
		return nil
	}
	userId, err := strconv.Atoi(claims.Subject)
	if err != nil {
		return nil
	}
	user, err := api.db.GetUser(userId)
	if err != nil || user == nil {
		return nil
	}
	return user
}

func (api *Api) MCPOAuthProtectedResource(w http.ResponseWriter, r *http.Request) {
	issuer := api.mcpIssuerURL(r)
	resp := map[string]any{
		"resource":                 api.GetAbsoluteUrl(r, mcpOAuthResourcePath).String(),
		"authorization_servers":    []string{issuer},
		"scopes_supported":         []string{mcpOAuthScope},
		"bearer_methods_supported": []string{"header"},
	}
	utils.WriteJson(w, resp)
}

func (api *Api) MCPOAuthAuthorizationServer(w http.ResponseWriter, r *http.Request) {
	issuer := api.mcpIssuerURL(r)
	resp := map[string]any{
		"issuer":                                issuer,
		"authorization_endpoint":                api.GetAbsoluteUrl(r, mcpOAuthAuthorizePath).String(),
		"token_endpoint":                        api.GetAbsoluteUrl(r, mcpOAuthTokenPath).String(),
		"registration_endpoint":                 api.GetAbsoluteUrl(r, mcpOAuthRegisterPath).String(),
		"revocation_endpoint":                   api.GetAbsoluteUrl(r, mcpOAuthRevokePath).String(),
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"token_endpoint_auth_methods_supported": []string{"none"},
		"code_challenge_methods_supported":      []string{"S256"},
		"scopes_supported":                      []string{mcpOAuthScope},
	}
	utils.WriteJson(w, resp)
}

func (api *Api) mcpIssuerURL(r *http.Request) string {
	u := api.GetAbsoluteUrl(r, mcpOAuthIssuerPath)
	s := u.String()
	return strings.TrimRight(s, "/")
}

func (api *Api) MCPOAuthRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientName              string   `json:"client_name"`
		RedirectURIs            []string `json:"redirect_uris"`
		GrantTypes              []string `json:"grant_types"`
		ResponseTypes           []string `json:"response_types"`
		TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		mcpOAuthError(w, http.StatusBadRequest, oauthErrInvalidClientMetadata, "invalid request body")
		return
	}
	if len(req.RedirectURIs) == 0 {
		mcpOAuthError(w, http.StatusBadRequest, oauthErrInvalidRedirectURI, "redirect_uris required")
		return
	}
	if len(req.RedirectURIs) > mcpOAuthMaxRedirectURIs {
		mcpOAuthError(w, http.StatusBadRequest, oauthErrInvalidRedirectURI, "too many redirect_uris")
		return
	}
	for _, ru := range req.RedirectURIs {
		if _, err := url.Parse(ru); err != nil {
			mcpOAuthError(w, http.StatusBadRequest, oauthErrInvalidRedirectURI, "invalid redirect_uri")
			return
		}
	}
	if len(req.ClientName) > mcpOAuthMaxClientNameLen {
		req.ClientName = req.ClientName[:mcpOAuthMaxClientNameLen]
	}
	clientId, err := api.mcpSign(mcpClientClaims{
		Name:         req.ClientName,
		RedirectURIs: req.RedirectURIs,
		RegisteredClaims: jwt.RegisteredClaims{
			Audience: jwt.ClaimStrings{mcpAudClient},
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
	})
	if err != nil {
		klog.Errorln("mcp:", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	utils.WriteJson(w, map[string]any{
		"client_id":                  clientId,
		"client_id_issued_at":        time.Now().Unix(),
		"client_name":                req.ClientName,
		"redirect_uris":              req.RedirectURIs,
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none",
	})
}

type authorizeRequest struct {
	ClientId            string
	RedirectURI         string
	ResponseType        string
	CodeChallenge       string
	CodeChallengeMethod string
	State               string
	Scope               string
}

func parseAuthorizeRequest(r *http.Request) authorizeRequest {
	return authorizeRequest{
		ClientId:            r.Form.Get("client_id"),
		RedirectURI:         r.Form.Get("redirect_uri"),
		ResponseType:        r.Form.Get("response_type"),
		CodeChallenge:       r.Form.Get("code_challenge"),
		CodeChallengeMethod: r.Form.Get("code_challenge_method"),
		State:               r.Form.Get("state"),
		Scope:               r.Form.Get("scope"),
	}
}

func (api *Api) MCPOAuthAuthorize(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		mcpOAuthError(w, http.StatusBadRequest, oauthErrInvalidRequest, "cannot parse params")
		return
	}
	req := parseAuthorizeRequest(r)

	var client mcpClientClaims
	if err := api.mcpVerify(mcpAudClient, req.ClientId, &client); err != nil {
		mcpOAuthError(w, http.StatusBadRequest, oauthErrUnauthorizedClient, "unknown client")
		return
	}
	if !mcpRedirectURIAllowed(client.RedirectURIs, req.RedirectURI) {
		mcpOAuthError(w, http.StatusBadRequest, oauthErrInvalidRequest, "redirect_uri mismatch")
		return
	}
	if req.ResponseType != "code" {
		mcpRedirectErr(w, r, req.RedirectURI, req.State, oauthErrUnsupportedResponseType, "")
		return
	}
	if req.CodeChallenge == "" || req.CodeChallengeMethod != "S256" {
		mcpRedirectErr(w, r, req.RedirectURI, req.State, oauthErrInvalidRequest, "PKCE S256 required")
		return
	}

	user := api.GetUser(r)
	switch r.Method {
	case http.MethodGet:
		api.mcpAuthorizeShowConsent(w, r, client.Name, user)
	case http.MethodPost:
		api.mcpAuthorizeIssueCode(w, r, req, user)
	}
}

func (api *Api) mcpAuthorizeShowConsent(w http.ResponseWriter, r *http.Request, clientName string, user *db.User) {
	if user == nil {
		http.Redirect(w, r, api.mcpLoginRedirect(r), http.StatusFound)
		return
	}
	http.Redirect(w, r, api.mcpConsentRedirect(r, clientName, user), http.StatusFound)
}

func (api *Api) mcpAuthorizeIssueCode(w http.ResponseWriter, r *http.Request, req authorizeRequest, user *db.User) {
	if user == nil {
		mcpOAuthError(w, http.StatusUnauthorized, oauthErrAccessDenied, "not authenticated")
		return
	}
	if r.Form.Get("decision") != "allow" {
		mcpRedirectErr(w, r, req.RedirectURI, req.State, oauthErrAccessDenied, "")
		return
	}

	code, err := api.mcpSign(mcpCodeClaims{
		ClientId:      req.ClientId,
		RedirectURI:   req.RedirectURI,
		CodeChallenge: req.CodeChallenge,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.Itoa(user.Id),
			Audience:  jwt.ClaimStrings{mcpAudCode},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(mcpOAuthCodeTTL)),
		},
	})
	if err != nil {
		klog.Errorln("mcp:", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	u, _ := url.Parse(req.RedirectURI)
	q := u.Query()
	q.Set("code", code)
	if req.State != "" {
		q.Set("state", req.State)
	}
	if req.Scope != "" {
		q.Set("scope", req.Scope)
	}
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func (api *Api) mcpLoginRedirect(r *http.Request) string {
	u := url.URL{Path: path.Join(api.cfg.UrlBasePath, "login")}
	q := url.Values{}
	q.Set("next", r.URL.RequestURI())
	u.RawQuery = q.Encode()
	return u.String()
}

func (api *Api) mcpConsentRedirect(r *http.Request, clientName string, user *db.User) string {
	u := url.URL{Path: path.Join(api.cfg.UrlBasePath, "auth/mcp-consent")}
	q := url.Values{}
	for _, k := range []string{"client_id", "redirect_uri", "response_type", "code_challenge", "code_challenge_method", "state", "scope"} {
		if v := r.Form.Get(k); v != "" {
			q.Set(k, v)
		}
	}
	if clientName == "" {
		clientName = "An MCP client"
	}
	q.Set("client_name", clientName)
	q.Set("user_name", mcpDisplayUserName(user))
	u.RawQuery = q.Encode()
	return u.String()
}

func mcpDisplayUserName(u *db.User) string {
	if u.Name != "" {
		return u.Name
	}
	if u.Email != "" {
		return u.Email
	}
	return fmt.Sprintf("user #%d", u.Id)
}

func (api *Api) MCPOAuthToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		mcpOAuthError(w, http.StatusBadRequest, oauthErrInvalidRequest, "cannot parse params")
		return
	}

	switch r.Form.Get("grant_type") {
	case "authorization_code":
		api.mcpExchangeCode(w, r)
	case "refresh_token":
		api.mcpExchangeRefresh(w, r)
	default:
		mcpOAuthError(w, http.StatusBadRequest, oauthErrUnsupportedGrantType, "")
	}
}

func (api *Api) mcpExchangeCode(w http.ResponseWriter, r *http.Request) {
	codeStr := r.Form.Get("code")
	clientId := r.Form.Get("client_id")
	redirectURI := r.Form.Get("redirect_uri")
	verifier := r.Form.Get("code_verifier")

	var code mcpCodeClaims
	if err := api.mcpVerify(mcpAudCode, codeStr, &code); err != nil {
		mcpOAuthError(w, http.StatusBadRequest, oauthErrInvalidGrant, "invalid code")
		return
	}
	if code.ClientId != clientId {
		mcpOAuthError(w, http.StatusBadRequest, oauthErrInvalidGrant, "client_id mismatch")
		return
	}
	if code.RedirectURI != redirectURI {
		mcpOAuthError(w, http.StatusBadRequest, oauthErrInvalidGrant, "redirect_uri mismatch")
		return
	}
	if !mcpVerifyPKCE(verifier, code.CodeChallenge) {
		mcpOAuthError(w, http.StatusBadRequest, oauthErrInvalidGrant, "PKCE verification failed")
		return
	}
	userId, err := strconv.Atoi(code.Subject)
	if err != nil {
		mcpOAuthError(w, http.StatusBadRequest, oauthErrInvalidGrant, "invalid subject")
		return
	}
	api.mcpIssueTokenPair(w, userId, clientId)
}

func (api *Api) mcpExchangeRefresh(w http.ResponseWriter, r *http.Request) {
	refresh := r.Form.Get("refresh_token")
	clientId := r.Form.Get("client_id")

	var claims mcpTokenClaims
	if err := api.mcpVerify(mcpAudRefresh, refresh, &claims); err != nil {
		mcpOAuthError(w, http.StatusBadRequest, oauthErrInvalidGrant, "invalid refresh_token")
		return
	}
	if clientId != "" && claims.ClientId != clientId {
		mcpOAuthError(w, http.StatusBadRequest, oauthErrInvalidGrant, "client_id mismatch")
		return
	}
	userId, err := strconv.Atoi(claims.Subject)
	if err != nil {
		mcpOAuthError(w, http.StatusBadRequest, oauthErrInvalidGrant, "invalid subject")
		return
	}
	api.mcpIssueTokenPair(w, userId, claims.ClientId)
}

func (api *Api) mcpIssueTokenPair(w http.ResponseWriter, userId int, clientId string) {
	access, err := api.mcpIssueToken(mcpAudAccess, userId, clientId, mcpOAuthAccessTokenTTL)
	if err != nil {
		klog.Errorln("mcp:", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	refresh, err := api.mcpIssueToken(mcpAudRefresh, userId, clientId, mcpOAuthRefreshTokenTTL)
	if err != nil {
		klog.Errorln("mcp:", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	utils.WriteJson(w, map[string]any{
		"access_token":  access,
		"token_type":    "Bearer",
		"expires_in":    int(mcpOAuthAccessTokenTTL.Seconds()),
		"refresh_token": refresh,
		"scope":         mcpOAuthScope,
	})
}

func (api *Api) MCPOAuthRevoke(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func mcpVerifyPKCE(verifier, challenge string) bool {
	if verifier == "" || challenge == "" {
		return false
	}
	h := sha256.Sum256([]byte(verifier))
	got := base64.RawURLEncoding.EncodeToString(h[:])
	return subtle.ConstantTimeCompare([]byte(got), []byte(challenge)) == 1
}

func mcpRedirectURIAllowed(allowed []string, given string) bool {
	for _, a := range allowed {
		if a == given {
			return true
		}
	}
	return false
}

func mcpOAuthError(w http.ResponseWriter, status int, code, desc string) {
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	body := map[string]string{"error": code}
	if desc != "" {
		body["error_description"] = desc
	}
	_ = json.NewEncoder(w).Encode(body)
}

func mcpRedirectErr(w http.ResponseWriter, r *http.Request, redirectURI, state, code, desc string) {
	if redirectURI == "" {
		mcpOAuthError(w, http.StatusBadRequest, code, desc)
		return
	}
	u, err := url.Parse(redirectURI)
	if err != nil {
		mcpOAuthError(w, http.StatusBadRequest, code, desc)
		return
	}
	q := u.Query()
	q.Set("error", code)
	if desc != "" {
		q.Set("error_description", desc)
	}
	if state != "" {
		q.Set("state", state)
	}
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}
