package collector

import (
	"net/http"
	"strings"

	"github.com/coroot/coroot/db"
)

const (
	CorootSignalHeader = "X-Coroot-Signal"
	maxRumBodyBytes    = 2 << 20 // 2 MiB
)

func writeCORSHeaders(w http.ResponseWriter, origin string, allowCredentials bool) {
	if origin == "" {
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Encoding, X-API-Key, X-Coroot-Signal")
	w.Header().Set("Access-Control-Max-Age", "86400")
	if allowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	w.Header().Add("Vary", "Origin")
}

func matchOrigin(origin string, allowed []string) bool {
	if origin == "" || len(allowed) == 0 {
		return false
	}
	for _, a := range allowed {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if a == "*" || strings.EqualFold(a, origin) {
			return true
		}
		// allow trailing /* for origin prefixes like https://*.example.com — only exact * or exact match for stage 1
		if strings.HasSuffix(a, "/*") {
			prefix := strings.TrimSuffix(a, "/*")
			if strings.HasPrefix(origin, prefix) {
				return true
			}
		}
	}
	return false
}

func resolveCORSOrigin(r *http.Request, key *db.ApiKey) (string, bool) {
	origin := r.Header.Get("Origin")
	if key == nil || !key.IsRum() {
		return "", false
	}
	if matchOrigin(origin, key.AllowedOrigins) {
		return origin, true
	}
	return "", false
}

func handleCORSPreflight(w http.ResponseWriter, r *http.Request, key *db.ApiKey) bool {
	if r.Method != http.MethodOptions {
		return false
	}
	origin, ok := resolveCORSOrigin(r, key)
	if !ok {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return true
	}
	writeCORSHeaders(w, origin, false)
	w.WriteHeader(http.StatusNoContent)
	return true
}
