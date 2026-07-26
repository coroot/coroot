package api

import (
	"fmt"
	"sort"
	"strings"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/rbac"
)

// restrictedNamespaces returns the application namespaces a handoff / scoped
// user may manage alerting rules for. restricted is false for project-wide
// roles (Admin, Editor, Viewer, etc.).
func (api *Api) restrictedNamespaces(u *db.User, projectId string, w *model.World) (namespaces []string, restricted bool) {
	if u == nil || w == nil {
		return nil, false
	}
	if !api.hasRestrictedAppAccess(u, projectId, w) {
		return nil, false
	}
	set := map[string]struct{}{}
	for _, app := range w.Applications {
		if api.canViewApplication(u, projectId, app) {
			set[app.Id.Namespace] = struct{}{}
		}
	}
	// Supplement from role permissions so users can create rules before any
	// matching apps are currently running.
	if roles, err := api.roles.GetRoles(); err == nil {
		for _, rn := range u.Roles {
			for _, role := range roles {
				if role.Name != rn {
					continue
				}
				for _, p := range role.Permissions {
					if p.Scope != rbac.ScopeApplication {
						continue
					}
					ns := p.Object["application_namespace"]
					if ns == "" || ns == "*" || strings.ContainsAny(ns, "*?[") {
						continue
					}
					if pid := p.Object["project_id"]; pid != "" && pid != "*" && pid != projectId {
						continue
					}
					set[ns] = struct{}{}
				}
			}
		}
	}
	out := make([]string, 0, len(set))
	for ns := range set {
		out = append(out, ns)
	}
	sort.Strings(out)
	return out, true
}

// applicationIdPatternConfined is true when pattern is namespace:kind:name and
// the namespace part is a literal match for one of the allowed namespaces.
func applicationIdPatternConfined(pattern string, namespaces []string) bool {
	parts := strings.SplitN(pattern, ":", 3)
	if len(parts) != 3 {
		return false
	}
	ns := parts[0]
	if ns == "" || strings.ContainsAny(ns, "*?[") {
		return false
	}
	for _, allowed := range namespaces {
		if ns == allowed {
			return true
		}
	}
	return false
}

// canAccessAlertingRule reports whether a restricted user may see / mutate the
// rule. Project-wide users always get true. Restricted users may only access
// non-builtin, applications-selector rules whose patterns stay inside their
// namespaces and do not match any app they cannot view.
func (api *Api) canAccessAlertingRule(u *db.User, projectId string, w *model.World, rule *model.AlertingRule) bool {
	if rule == nil {
		return false
	}
	namespaces, restricted := api.restrictedNamespaces(u, projectId, w)
	if !restricted {
		return true
	}
	if rule.Builtin || rule.Readonly {
		return false
	}
	if rule.Selector.Type != model.AppSelectorTypeApplications {
		return false
	}
	if len(rule.Selector.ApplicationIdPatterns) == 0 {
		return false
	}
	for _, p := range rule.Selector.ApplicationIdPatterns {
		if !applicationIdPatternConfined(p, namespaces) {
			return false
		}
	}
	for _, app := range w.Applications {
		if rule.Matches(app) && !api.canViewApplication(u, projectId, app) {
			return false
		}
	}
	return true
}

func (api *Api) filterAlertingRules(u *db.User, projectId string, w *model.World, rules []*model.AlertingRule) []*model.AlertingRule {
	if w == nil {
		return rules
	}
	if _, restricted := api.restrictedNamespaces(u, projectId, w); !restricted {
		return rules
	}
	out := make([]*model.AlertingRule, 0, len(rules))
	for _, r := range rules {
		if api.canAccessAlertingRule(u, projectId, w, r) {
			out = append(out, r)
		}
	}
	return out
}

// validateAlertingRuleForUser enforces namespace confinement on create/update.
// It mutates rule to clear project-wide fields that scoped users must not set.
func (api *Api) validateAlertingRuleForUser(u *db.User, projectId string, w *model.World, rule *model.AlertingRule) error {
	if rule == nil {
		return fmt.Errorf("rule is required")
	}
	namespaces, restricted := api.restrictedNamespaces(u, projectId, w)
	if !restricted {
		return nil
	}
	if len(namespaces) == 0 {
		return fmt.Errorf("no application namespaces available for alerting rules")
	}
	if rule.Source.Type == model.AlertSourceTypePromQL {
		return fmt.Errorf("promql alerting rules are not allowed for namespace-scoped users")
	}
	if rule.Selector.Type != model.AppSelectorTypeApplications {
		return fmt.Errorf("namespace-scoped users must select specific applications (namespace:kind:name patterns)")
	}
	if len(rule.Selector.ApplicationIdPatterns) == 0 {
		return fmt.Errorf("at least one application id pattern is required")
	}
	for _, p := range rule.Selector.ApplicationIdPatterns {
		if !applicationIdPatternConfined(p, namespaces) {
			return fmt.Errorf("application pattern %q is outside your namespaces (%s)", p, strings.Join(namespaces, ", "))
		}
	}
	rule.Selector.Categories = nil
	rule.NotificationCategory = ""
	rule.Builtin = false
	rule.Readonly = false
	return nil
}
