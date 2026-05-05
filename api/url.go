package api

import (
	"net/http"
	"net/url"
	"strings"
)

func (api *Api) GetAbsoluteUrl(r *http.Request, uri string) *url.URL {
	callbackURL := fullURL(r)
	callbackURL.RawQuery = ""
	callbackURL.Fragment = ""
	if uri == "/" && api.cfg.UrlBasePath != "/" {
		uri = api.cfg.UrlBasePath
	}
	if parsed, err := url.Parse(uri); err == nil {
		callbackURL.Path = parsed.Path
		callbackURL.RawQuery = parsed.RawQuery
	} else {
		callbackURL.Path = uri
	}
	return callbackURL
}

func fullURL(r *http.Request) *url.URL {
	u := *r.URL

	u.Scheme = "http"
	u.Host = r.Host

	// RFC 7239 Forwarded header
	if fwd := r.Header.Get("Forwarded"); fwd != "" {
		parts := strings.Split(fwd, ";")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if strings.HasPrefix(p, "proto=") {
				u.Scheme = strings.TrimPrefix(p, "proto=")
			} else if strings.HasPrefix(p, "host=") {
				u.Host = strings.TrimPrefix(p, "host=")
			}
		}
	}

	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		u.Scheme = strings.Split(proto, ",")[0]
	}

	if h := r.Header.Get("X-Forwarded-Host"); h != "" {
		u.Host = strings.Split(h, ",")[0]
	}

	if port := r.Header.Get("X-Forwarded-Port"); port != "" && port != "443" && port != "80" && !strings.Contains(u.Host, ":") {
		u.Host = u.Host + ":" + port
	}

	if r.TLS != nil {
		u.Scheme = "https"
	}

	return &u
}
