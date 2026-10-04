package clickhouse

import (
	"net"
	"strings"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/utils"
)

// NormalizeRumHost strips scheme/port/path from a browser-observed server address.
func NormalizeRumHost(serverService string) string {
	host := strings.TrimSpace(serverService)
	if host == "" {
		return ""
	}
	host = strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://")
	if i := strings.IndexAny(host, "/"); i >= 0 {
		host = host[:i]
	}
	// host:port — keep IP literals with brackets carefully
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	} else if i := strings.LastIndex(host, ":"); i > 0 && !strings.Contains(host, "]") {
		// likely host:port without IPv6
		if !strings.Contains(host[:i], ":") {
			host = host[:i]
		}
	}
	return strings.Trim(host, "[]")
}

// GuessBackendAppForRumServer resolves browser http.host / peer.service to a backend Application.
func GuessBackendAppForRumServer(w *model.World, serverService string) *model.Application {
	return GuessBackendAppForRumServerWithProject(w, nil, serverService)
}

// GuessBackendAppForRumServerWithProject also applies project-level host mappings.
func GuessBackendAppForRumServerWithProject(w *model.World, project *db.Project, serverService string) *model.Application {
	if serverService == "" || w == nil {
		return nil
	}
	host := NormalizeRumHost(serverService)
	if host == "" {
		host = serverService
	}

	if project != nil && project.Settings.Rum != nil {
		for _, m := range project.Settings.Rum.HostMappings {
			if hostMatchesPattern(host, m.Pattern) || hostMatchesPattern(serverService, m.Pattern) {
				if id, err := model.NewApplicationIdFromString(m.AppId, ""); err == nil {
					// Prefer an existing metrics-backed app with the same name (CPU/Mem) over recreating ExternalService.
					for _, existing := range w.Applications {
						if existing == nil || existing.Id.Kind == model.ApplicationKindRumClient {
							continue
						}
						if strings.EqualFold(existing.Id.Name, id.Name) && existing.Id.Kind != model.ApplicationKindExternalService {
							if existing.Settings == nil {
								existing.Settings = &model.ApplicationSettings{}
							}
							if existing.Settings.Tracing == nil {
								existing.Settings.Tracing = &model.ApplicationSettingsTracing{Service: id.Name}
							}
							return existing
						}
					}
					app := w.GetOrCreateApplication(id, true)
					if app.Settings == nil {
						app.Settings = &model.ApplicationSettings{}
					}
					if app.Settings.Tracing == nil && id.Name != "" {
						app.Settings.Tracing = &model.ApplicationSettingsTracing{Service: id.Name}
					}
					if app.Category == "" {
						app.Category = model.ApplicationCategoryApplication
					}
					return app
				}
			}
		}
	}

	// Per-app HostPatterns on Rum settings (used when a backend opts into mapping).
	for _, app := range w.Applications {
		if app.Settings == nil || app.Settings.Rum == nil {
			continue
		}
		for _, p := range app.Settings.Rum.HostPatterns {
			if hostMatchesPattern(host, p) || hostMatchesPattern(serverService, p) {
				return app
			}
		}
	}

	// Match OTEL service names (ExternalService apps created from traces).
	for _, app := range w.Applications {
		if app.Id.Kind == model.ApplicationKindRumClient {
			continue
		}
		if app.Settings != nil && app.Settings.Tracing != nil && app.Settings.Tracing.Service != "" {
			svc := app.Settings.Tracing.Service
			if strings.EqualFold(svc, serverService) || strings.EqualFold(svc, host) ||
				strings.EqualFold(app.Id.Name, serverService) || strings.EqualFold(app.Id.Name, host) {
				return app
			}
		}
	}

	// K8s Service / Ingress-style names and ClusterIP / LB IPs.
	for _, app := range w.Applications {
		if app.Id.Kind == model.ApplicationKindRumClient || app.Id.Kind == model.ApplicationKindExternalService {
			continue
		}
		for _, svc := range app.KubernetesServices {
			if svc == nil {
				continue
			}
			if strings.EqualFold(svc.Name, host) || strings.EqualFold(svc.Name, serverService) {
				return app
			}
			fqdn := svc.Name + "." + svc.Namespace
			if strings.EqualFold(fqdn, host) || strings.HasPrefix(strings.ToLower(host), strings.ToLower(fqdn+".")) {
				return app
			}
			if svc.ClusterIP != "" && (svc.ClusterIP == host || svc.ClusterIP == serverService) {
				return app
			}
			if svc.LoadBalancerIPs != nil {
				for _, ip := range svc.LoadBalancerIPs.Items() {
					if ip == host {
						return app
					}
				}
			}
		}
		// App name / short DNS label match
		if strings.EqualFold(app.Id.Name, serverService) || strings.EqualFold(app.Id.Name, host) {
			return app
		}
		if strings.EqualFold(app.Id.Name+"."+app.Id.Namespace, host) {
			return app
		}
		if app.Settings != nil && app.Settings.Tracing != nil && app.Settings.Tracing.Service != "" {
			if strings.EqualFold(app.Settings.Tracing.Service, serverService) || strings.EqualFold(app.Settings.Tracing.Service, host) {
				return app
			}
		}
		// Listen IP match
		for _, instance := range app.Instances {
			for listen := range instance.TcpListens {
				if listen.IP == host {
					return app
				}
			}
		}
	}
	return nil
}

func hostMatchesPattern(host, pattern string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	if strings.EqualFold(host, pattern) {
		return true
	}
	return utils.GlobMatch(host, pattern)
}
