package db

import (
	"fmt"
	"net/url"
	"strings"
)

// NormalizeAllowedOrigin normalizes a RUM allowlist entry to host[:port][/path].
// Accepts bare hosts, host/path, wildcards (*.example.com, *), or full URLs.
func NormalizeAllowedOrigin(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("empty allowed origin")
	}
	if s == "*" {
		return "*", nil
	}

	var host, path string
	if strings.Contains(s, "://") {
		u, err := url.Parse(s)
		if err != nil || u.Host == "" {
			return "", fmt.Errorf("invalid allowed origin: %s", s)
		}
		host = u.Host
		path = u.EscapedPath()
	} else {
		// host[:port][/path] — split on first '/'
		if i := strings.IndexByte(s, '/'); i >= 0 {
			host = s[:i]
			path = s[i:]
		} else {
			host = s
		}
	}

	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return "", fmt.Errorf("invalid allowed origin: missing host")
	}
	if host != "*" && !strings.HasPrefix(host, "*.") {
		// basic sanity: hostname / host:port
		if strings.ContainsAny(host, " \t\r\n") {
			return "", fmt.Errorf("invalid allowed origin host: %s", host)
		}
	}

	path = normalizeAllowPath(path)
	if path == "" {
		return host, nil
	}
	return host + path, nil
}

func normalizeAllowPath(path string) string {
	if path == "" || path == "/" {
		return ""
	}
	if q := strings.IndexAny(path, "?#"); q >= 0 {
		path = path[:q]
	}
	path = strings.ToLower(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	for strings.HasSuffix(path, "/") && path != "/" {
		path = strings.TrimSuffix(path, "/")
	}
	if path == "/" {
		return ""
	}
	return path
}

// splitAllowedPattern splits a normalized pattern into host and optional path prefix.
func splitAllowedPattern(pattern string) (host, path string) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return "", ""
	}
	if pattern == "*" {
		return "*", ""
	}
	if i := strings.IndexByte(pattern, '/'); i >= 0 {
		return pattern[:i], pattern[i:]
	}
	return pattern, ""
}

func hostFromOrigin(originHeader string) string {
	originHeader = strings.TrimSpace(originHeader)
	if originHeader == "" {
		return ""
	}
	u, err := url.Parse(originHeader)
	if err != nil || u.Host == "" {
		// Origin is typically scheme://host[:port] without path
		if strings.Contains(originHeader, "://") {
			return ""
		}
		return strings.ToLower(originHeader)
	}
	return strings.ToLower(u.Host)
}

func matchHostPattern(patternHost, reqHost string) bool {
	if patternHost == "" || reqHost == "" {
		return false
	}
	if patternHost == "*" {
		return true
	}
	if strings.HasPrefix(patternHost, "*.") {
		suffix := patternHost[1:] // ".example.com"
		return strings.HasSuffix(reqHost, suffix) && reqHost != strings.TrimPrefix(patternHost, "*.")
	}
	return patternHost == reqHost
}

func matchPathPrefix(patternPath, reqPath string) bool {
	if patternPath == "" {
		return true
	}
	reqPath = normalizeAllowPath(reqPath)
	if reqPath == "" {
		return false
	}
	if reqPath == patternPath {
		return true
	}
	return strings.HasPrefix(reqPath, patternPath+"/")
}

// MatchAllowedOrigin reports whether originHeader (and optional reqPath) match the allowlist.
// When reqPath is empty, only the host part is checked (CORS preflight / host-only mode).
// When reqPath is non-empty, path-scoped patterns must match the path prefix.
// When reqPath is empty and the only host-matching patterns are path-scoped, still allow (preflight).
func MatchAllowedOrigin(originHeader, reqPath string, allowed []string) bool {
	if originHeader == "" || len(allowed) == 0 {
		return false
	}
	reqHost := hostFromOrigin(originHeader)
	if reqHost == "" {
		return false
	}

	hostOnly := strings.TrimSpace(reqPath) == ""
	normalizedReqPath := ""
	if !hostOnly {
		normalizedReqPath = strings.ToLower(strings.TrimSpace(reqPath))
		if q := strings.IndexAny(normalizedReqPath, "?#"); q >= 0 {
			normalizedReqPath = normalizedReqPath[:q]
		}
		if normalizedReqPath != "" && !strings.HasPrefix(normalizedReqPath, "/") {
			normalizedReqPath = "/" + normalizedReqPath
		}
		for strings.HasSuffix(normalizedReqPath, "/") && normalizedReqPath != "/" {
			normalizedReqPath = strings.TrimSuffix(normalizedReqPath, "/")
		}
	}

	for _, a := range allowed {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		// Compat: normalize on the fly if still stored as full URL / mixed case
		if norm, err := NormalizeAllowedOrigin(a); err == nil {
			a = norm
		}
		if a == "*" {
			return true
		}
		pHost, pPath := splitAllowedPattern(a)
		if !matchHostPattern(pHost, reqHost) {
			continue
		}
		if hostOnly {
			return true
		}
		if pPath == "" {
			return true
		}
		if matchPathPrefix(pPath, normalizedReqPath) {
			return true
		}
	}
	return false
}

// PathRequiredForOrigin reports whether all host-matching patterns are path-scoped
// (so a missing request path must be rejected on POST).
func PathRequiredForOrigin(originHeader string, allowed []string) bool {
	reqHost := hostFromOrigin(originHeader)
	if reqHost == "" || len(allowed) == 0 {
		return false
	}
	sawHostMatch := false
	for _, a := range allowed {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if norm, err := NormalizeAllowedOrigin(a); err == nil {
			a = norm
		}
		if a == "*" {
			return false
		}
		pHost, pPath := splitAllowedPattern(a)
		if !matchHostPattern(pHost, reqHost) {
			continue
		}
		sawHostMatch = true
		if pPath == "" {
			return false
		}
	}
	return sawHostMatch
}
