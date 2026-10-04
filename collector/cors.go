package collector

import (
	"net/http"

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

// matchOrigin checks the Origin header against the allowlist in host-only mode (CORS / preflight).
func matchOrigin(origin string, allowed []string) bool {
	return db.MatchAllowedOrigin(origin, "", allowed)
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

// allowRumRequest validates Origin (+ optional path) for a RUM key on POST ingest.
func allowRumRequest(r *http.Request, key *db.ApiKey, reqPath string) (origin string, ok bool) {
	if key == nil || !key.IsRum() {
		return "", false
	}
	origin = r.Header.Get("Origin")
	if origin == "" {
		return "", false
	}
	if reqPath == "" && db.PathRequiredForOrigin(origin, key.AllowedOrigins) {
		return "", false
	}
	if !db.MatchAllowedOrigin(origin, reqPath, key.AllowedOrigins) {
		return "", false
	}
	return origin, true
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
