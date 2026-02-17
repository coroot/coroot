package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

type AlertingRule model.AlertingRule

func (r *AlertingRule) Migrate(m *Migrator) error {
	err := m.Exec(`
	CREATE TABLE IF NOT EXISTS alerting_rule (
		id TEXT NOT NULL,
		project_id TEXT NOT NULL REFERENCES project(id),
		config TEXT NOT NULL,
		enabled INT NOT NULL DEFAULT 1,
		builtin INT NOT NULL DEFAULT 0,
		created_at INT NOT NULL,
		updated_at INT NOT NULL,
		PRIMARY KEY (id, project_id)
	)`)
	if err != nil {
		return err
	}
	err = m.Exec(`CREATE INDEX IF NOT EXISTS alerting_rule_project ON alerting_rule (project_id)`)
	if err != nil {
		return err
	}
	projects, err := m.db.GetProjects()
	if err != nil {
		return err
	}
	for _, project := range projects {
		_ = m.db.InitBuiltinAlertingRules(project.Id)
	}
	return nil
}

type alertingRuleConfig struct {
	Name                 string                    `json:"name"`
	Source               model.AlertSource         `json:"source"`
	Selector             model.AppSelector         `json:"selector"`
	Severity             model.Status              `json:"severity"`
	For                  timeseries.Duration       `json:"for"`
	KeepFiringFor        timeseries.Duration       `json:"keep_firing_for"`
	Templates            model.AlertTemplates      `json:"templates"`
	NotificationCategory model.ApplicationCategory `json:"notification_category,omitempty"`
	Readonly             bool                      `json:"readonly,omitempty"`
}

type Alert model.Alert

func (a *Alert) Migrate(m *Migrator) error {
	err := m.Exec(`
	CREATE TABLE IF NOT EXISTS alert (
		id TEXT PRIMARY KEY,
		fingerprint TEXT NOT NULL,
		rule_id TEXT NOT NULL,
		project_id TEXT NOT NULL REFERENCES project(id),
		application_id TEXT NOT NULL,
		application_category TEXT NOT NULL DEFAULT '',
		severity TEXT NOT NULL,
		summary TEXT NOT NULL,
		details TEXT,
		opened_at INT NOT NULL,
		resolved_at INT NOT NULL DEFAULT 0,
		updated_at INT NOT NULL,
		suppressed INT NOT NULL DEFAULT 0,
		resolved_by TEXT NOT NULL DEFAULT '',
		report TEXT NOT NULL DEFAULT '',
		pattern_words TEXT NOT NULL DEFAULT '',
		manually_resolved_at INT NOT NULL DEFAULT 0
	)`)
	if err != nil {
		return err
	}
	if err = m.Exec(`CREATE INDEX IF NOT EXISTS alert_project_resolved ON alert (project_id, resolved_at)`); err != nil {
		return err
	}
	if err = m.Exec(`CREATE INDEX IF NOT EXISTS alert_fingerprint_resolved ON alert (fingerprint, resolved_at)`); err != nil {
		return err
	}
	if err = m.Exec(`CREATE INDEX IF NOT EXISTS alert_fingerprint_suppressed ON alert (fingerprint, suppressed)`); err != nil {
		return err
	}
	return m.Exec(`CREATE INDEX IF NOT EXISTS alert_project_rule_resolved ON alert (project_id, rule_id, resolved_at)`)
}

func (db *DB) GetAlertingRules(projectId ProjectId) ([]*model.AlertingRule, error) {
	rows, err := db.db.Query(
		"SELECT id, config, enabled, builtin, created_at, updated_at FROM alerting_rule WHERE project_id = $1",
		projectId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*model.AlertingRule
	for rows.Next() {
		var r model.AlertingRule
		var configJSON string
		if err := rows.Scan(&r.Id, &configJSON, &r.Enabled, &r.Builtin, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		rule, err := unmarshalAlertingRule(&r, configJSON, projectId)
		if err != nil {
			return nil, err
		}
		res = append(res, rule)
	}
	return res, nil
}

func (db *DB) GetAlertingRule(projectId ProjectId, id model.AlertingRuleId) (*model.AlertingRule, error) {
	var r model.AlertingRule
	var configJSON string
	err := db.db.QueryRow(
		"SELECT id, config, enabled, builtin, created_at, updated_at FROM alerting_rule WHERE project_id = $1 AND id = $2",
		projectId, id).Scan(&r.Id, &configJSON, &r.Enabled, &r.Builtin, &r.CreatedAt, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return unmarshalAlertingRule(&r, configJSON, projectId)
}

func (db *DB) CreateAlertingRule(projectId ProjectId, r *model.AlertingRule) error {
	now := timeseries.Now()
	r.CreatedAt = now
	r.UpdatedAt = now
	config, _ := json.Marshal(alertingRuleConfig{
		Name:                 r.Name,
		Source:               r.Source,
		Selector:             r.Selector,
		Severity:             r.Severity,
		For:                  r.For,
		KeepFiringFor:        r.KeepFiringFor,
		Templates:            r.Templates,
		NotificationCategory: r.NotificationCategory,
		Readonly:             r.Readonly,
	})

	_, err := db.db.Exec(
		"INSERT INTO alerting_rule (id, project_id, config, enabled, builtin, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		r.Id, projectId, string(config), r.Enabled, r.Builtin, r.CreatedAt, r.UpdatedAt)
	return err
}

func (db *DB) UpdateAlertingRule(projectId ProjectId, r *model.AlertingRule) error {
	r.UpdatedAt = timeseries.Now()
	config, _ := json.Marshal(alertingRuleConfig{
		Name:                 r.Name,
		Source:               r.Source,
		Selector:             r.Selector,
		Severity:             r.Severity,
		For:                  r.For,
		KeepFiringFor:        r.KeepFiringFor,
		Templates:            r.Templates,
		NotificationCategory: r.NotificationCategory,
		Readonly:             r.Readonly,
	})

	_, err := db.db.Exec(
		"UPDATE alerting_rule SET config = $1, enabled = $2, updated_at = $3 WHERE project_id = $4 AND id = $5",
		string(config), r.Enabled, r.UpdatedAt, projectId, r.Id)
	return err
}

func (db *DB) DeleteAlertingRule(projectId ProjectId, id model.AlertingRuleId) error {
	rule, err := db.GetAlertingRule(projectId, id)
	if err != nil {
		return err
	}
	if rule.Readonly {
		return ErrReadonly
	}
	result, err := db.db.Exec("DELETE FROM alerting_rule WHERE project_id = $1 AND id = $2 AND builtin = 0", projectId, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func unmarshalAlertingRule(r *model.AlertingRule, configJSON string, projectId ProjectId) (*model.AlertingRule, error) {
	r.ProjectId = string(projectId)
	var cfg alertingRuleConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return nil, err
	}
	r.Name = cfg.Name
	r.Source = cfg.Source
	r.Selector = cfg.Selector
	r.Severity = cfg.Severity
	r.For = cfg.For
	r.KeepFiringFor = cfg.KeepFiringFor
	r.Templates = cfg.Templates
	r.NotificationCategory = cfg.NotificationCategory
	r.Readonly = cfg.Readonly
	return r, nil
}

func (db *DB) GetFiringAlertCountsByRule(projectId ProjectId) (map[string]int, error) {
	rows, err := db.db.Query(
		"SELECT rule_id, COUNT(*) FROM alert WHERE project_id = $1 AND resolved_at = 0 AND manually_resolved_at = 0 AND suppressed = 0 GROUP BY rule_id",
		projectId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string]int)
	for rows.Next() {
		var ruleId string
		var count int
		if err := rows.Scan(&ruleId, &count); err != nil {
			return nil, err
		}
		res[ruleId] = count
	}
	return res, nil
}

func (db *DB) GetFiringAlertCountsBySeverity(projectId ProjectId) (map[string]int, error) {
	rows, err := db.db.Query(
		"SELECT severity, COUNT(*) FROM alert WHERE project_id = $1 AND resolved_at = 0 AND manually_resolved_at = 0 AND suppressed = 0 GROUP BY severity",
		projectId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string]int)
	for rows.Next() {
		var severity string
		var count int
		if err := rows.Scan(&severity, &count); err != nil {
			return nil, err
		}
		res[severity] = count
	}
	return res, nil
}

type AlertsQuery struct {
	IncludeResolved bool
	Search          string
	SortBy          string // "opened_at" or "duration"
	SortDesc        bool
	Offset          int
	Limit           int
}

type AlertsResult struct {
	Alerts   []*model.Alert `json:"alerts"`
	Total    int            `json:"total"`
	Firing   int            `json:"firing"`
	Resolved int            `json:"resolved"`
}

func (db *DB) QueryAlerts(projectId ProjectId, q AlertsQuery) (*AlertsResult, error) {
	args := []any{projectId}
	argIndex := 2

	baseWhere := "alert.project_id = $1"
	join := " LEFT JOIN alerting_rule ON alert.rule_id = alerting_rule.id AND alerting_rule.project_id = alert.project_id"
	if q.Search != "" {
		searchPattern := "%" + strings.ToLower(q.Search) + "%"
		baseWhere += fmt.Sprintf(" AND (LOWER(alert.summary) LIKE $%d OR LOWER(alert.application_id) LIKE $%d OR LOWER(alert.rule_id) LIKE $%d OR LOWER(alerting_rule.config) LIKE $%d)", argIndex, argIndex, argIndex, argIndex)
		args = append(args, searchPattern)
		argIndex++
	}
	var result AlertsResult
	var firing, resolved sql.NullInt64
	countQuery := fmt.Sprintf(`
		SELECT
			SUM(CASE WHEN alert.resolved_at = 0 AND alert.manually_resolved_at = 0 AND alert.suppressed = 0 THEN 1 ELSE 0 END),
			SUM(CASE WHEN alert.resolved_at > 0 OR alert.manually_resolved_at > 0 OR alert.suppressed = 1 THEN 1 ELSE 0 END)
		FROM alert%s WHERE %s`, join, baseWhere)
	err := db.db.QueryRow(countQuery, args...).Scan(&firing, &resolved)
	if err != nil {
		return nil, err
	}
	result.Firing = int(firing.Int64)
	result.Resolved = int(resolved.Int64)

	if q.IncludeResolved {
		result.Total = result.Firing + result.Resolved
	} else {
		result.Total = result.Firing
	}
	where := baseWhere
	if !q.IncludeResolved {
		where += " AND alert.resolved_at = 0 AND alert.manually_resolved_at = 0 AND alert.suppressed = 0"
	}

	orderBy := "alert.opened_at"
	switch q.SortBy {
	case "duration":
		orderBy = "CASE WHEN alert.resolved_at > 0 THEN alert.resolved_at ELSE strftime('%s', 'now') END - alert.opened_at"
	case "application":
		orderBy = "alert.application_id"
	case "summary":
		orderBy = "alert.summary"
	case "rule":
		orderBy = "alert.rule_id"
	case "resolved_at":
		orderBy = "alert.resolved_at"
	}
	if q.SortDesc {
		orderBy += " DESC"
	} else {
		orderBy += " ASC"
	}

	if q.Limit <= 0 {
		q.Limit = 1000
	}
	if q.Offset < 0 {
		q.Offset = 0
	}

	query := fmt.Sprintf(`
		SELECT alert.id, alert.fingerprint, alert.rule_id, alert.application_id, alert.application_category, alert.severity, alert.summary, alert.details, alert.opened_at, alert.resolved_at, alert.updated_at, alert.suppressed, alert.resolved_by, alert.report, alert.pattern_words, alert.manually_resolved_at
		FROM alert%s
		WHERE %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d`, join, where, orderBy, argIndex, argIndex+1)
	args = append(args, q.Limit, q.Offset)

	rows, err := db.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		a, err := scanAlert(rows, projectId)
		if err != nil {
			return nil, err
		}
		result.Alerts = append(result.Alerts, a)
	}
	if result.Alerts == nil {
		result.Alerts = []*model.Alert{}
	}
	return &result, nil
}

func (db *DB) GetAlert(projectId ProjectId, id string) (*model.Alert, error) {
	row := db.db.QueryRow(
		"SELECT id, fingerprint, rule_id, application_id, application_category, severity, summary, details, opened_at, resolved_at, updated_at, suppressed, resolved_by, report, pattern_words, manually_resolved_at FROM alert WHERE project_id = $1 AND id = $2",
		projectId, id)

	a, err := scanAlertRow(row, projectId)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return a, err
}

func (db *DB) GetActiveOrSuppressedAlertByFingerprint(projectId ProjectId, fingerprint string) (*model.Alert, error) {
	row := db.db.QueryRow(
		"SELECT id, fingerprint, rule_id, application_id, application_category, severity, summary, details, opened_at, resolved_at, updated_at, suppressed, resolved_by, report, pattern_words, manually_resolved_at FROM alert WHERE project_id = $1 AND fingerprint = $2 AND (resolved_at = 0 OR suppressed = 1) ORDER BY opened_at DESC LIMIT 1",
		projectId, fingerprint)

	a, err := scanAlertRow(row, projectId)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return a, err
}

func (db *DB) CreateAlert(projectId ProjectId, a *model.Alert) error {
	now := timeseries.Now()
	a.OpenedAt = now
	a.UpdatedAt = now
	detailsJSON, _ := json.Marshal(a.Details)
	_, err := db.db.Exec(
		"INSERT INTO alert (id, fingerprint, rule_id, project_id, application_id, application_category, severity, summary, details, opened_at, resolved_at, updated_at, suppressed, resolved_by, report, pattern_words, manually_resolved_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)",
		a.Id, a.Fingerprint, a.RuleId, projectId, a.ApplicationId.String(), a.ApplicationCategory, a.Severity.String(), a.Summary, string(detailsJSON), a.OpenedAt, a.ResolvedAt, a.UpdatedAt, a.Suppressed, a.ResolvedBy, a.Report, a.PatternWords, a.ManuallyResolvedAt)
	return err
}

func (db *DB) UpdateAlert(projectId ProjectId, a *model.Alert) error {
	a.UpdatedAt = timeseries.Now()
	detailsJSON, _ := json.Marshal(a.Details)
	_, err := db.db.Exec(
		"UPDATE alert SET severity = $1, summary = $2, details = $3, resolved_at = $4, updated_at = $5 WHERE project_id = $6 AND id = $7",
		a.Severity.String(), a.Summary, string(detailsJSON), a.ResolvedAt, a.UpdatedAt, projectId, a.Id)
	return err
}

func (db *DB) ResolveAlert(projectId ProjectId, id string, resolvedAt timeseries.Time) error {
	_, err := db.db.Exec(
		"UPDATE alert SET resolved_at = $1, updated_at = $1 WHERE project_id = $2 AND id = $3",
		resolvedAt, projectId, id)
	return err
}

func (db *DB) ResolveAlertsByRule(projectId ProjectId, ruleId string) ([]*model.Alert, error) {
	now := timeseries.Now()
	rows, err := db.db.Query(`
		SELECT id, fingerprint, rule_id, application_id, application_category, severity, summary, details, opened_at, resolved_at, updated_at, suppressed, resolved_by, report, pattern_words, manually_resolved_at
		FROM alert
		WHERE project_id = $1 AND rule_id = $2 AND resolved_at = 0
	`, projectId, ruleId)
	if err != nil {
		return nil, err
	}
	var alerts []*model.Alert
	for rows.Next() {
		alert, err := scanAlert(rows, projectId)
		if err != nil {
			rows.Close()
			return nil, err
		}
		alert.ResolvedAt = now
		alerts = append(alerts, alert)
	}
	rows.Close()
	_, err = db.db.Exec(
		"UPDATE alert SET resolved_at = $1, updated_at = $1 WHERE project_id = $2 AND rule_id = $3 AND resolved_at = 0",
		now, projectId, ruleId)
	if err != nil {
		return nil, err
	}

	return alerts, nil
}

func (db *DB) ResolveAlerts(projectId ProjectId, ids []string, resolvedBy string) error {
	if len(ids) == 0 {
		return nil
	}
	now := timeseries.Now()
	placeholders := make([]string, len(ids))
	args := []any{now, now, resolvedBy, projectId}
	for i, id := range ids {
		args = append(args, id)
		placeholders[i] = fmt.Sprintf("$%d", len(args))
	}
	_, err := db.db.Exec(
		"UPDATE alert SET manually_resolved_at = $1, updated_at = $2, resolved_by = $3 WHERE project_id = $4 AND resolved_at = 0 AND id IN ("+strings.Join(placeholders, ", ")+")",
		args...)
	return err
}

func (db *DB) SuppressAlerts(projectId ProjectId, ids []string, suppressedBy string) error {
	if len(ids) == 0 {
		return nil
	}
	now := timeseries.Now()
	placeholders := make([]string, len(ids))
	args := []any{suppressedBy, now, projectId}
	for i, id := range ids {
		args = append(args, id)
		placeholders[i] = fmt.Sprintf("$%d", len(args))
	}
	_, err := db.db.Exec(
		"UPDATE alert SET suppressed = 1, resolved_at = 0, manually_resolved_at = 0, resolved_by = $1, updated_at = $2 WHERE project_id = $3 AND id IN ("+strings.Join(placeholders, ", ")+")",
		args...)
	return err
}

func (db *DB) ClearSuppression(projectId ProjectId, fingerprint string) error {
	_, err := db.db.Exec(
		"UPDATE alert SET suppressed = 0, resolved_by = '' WHERE project_id = $1 AND fingerprint = $2 AND suppressed = 1",
		projectId, fingerprint)
	return err
}

func (db *DB) ReopenAlerts(projectId ProjectId, ids []string) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	now := timeseries.Now()
	placeholders := make([]string, len(ids))
	args := []any{now, projectId}
	for i, id := range ids {
		args = append(args, id)
		placeholders[i] = fmt.Sprintf("$%d", len(args))
	}
	result, err := db.db.Exec(
		"UPDATE alert SET manually_resolved_at = 0, resolved_by = '', suppressed = 0, updated_at = $1 WHERE project_id = $2 AND (manually_resolved_at > 0 OR suppressed = 1) AND id IN ("+strings.Join(placeholders, ", ")+")",
		args...)
	if err != nil {
		return 0, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(rows), nil
}

func scanAlert(rows *sql.Rows, projectId ProjectId) (*model.Alert, error) {
	var a model.Alert
	var severityStr string
	var detailsJSON sql.NullString
	err := rows.Scan(&a.Id, &a.Fingerprint, &a.RuleId, &a.ApplicationId, &a.ApplicationCategory, &severityStr, &a.Summary, &detailsJSON, &a.OpenedAt, &a.ResolvedAt, &a.UpdatedAt, &a.Suppressed, &a.ResolvedBy, &a.Report, &a.PatternWords, &a.ManuallyResolvedAt)
	if err != nil {
		return nil, err
	}
	a.ProjectId = string(projectId)
	a.Severity = parseSeverity(severityStr)
	if detailsJSON.String != "" {
		_ = json.Unmarshal([]byte(detailsJSON.String), &a.Details)
	}
	return &a, nil
}

func scanAlertRow(row *sql.Row, projectId ProjectId) (*model.Alert, error) {
	var a model.Alert
	var severityStr string
	var detailsJSON sql.NullString
	err := row.Scan(&a.Id, &a.Fingerprint, &a.RuleId, &a.ApplicationId, &a.ApplicationCategory, &severityStr, &a.Summary, &detailsJSON, &a.OpenedAt, &a.ResolvedAt, &a.UpdatedAt, &a.Suppressed, &a.ResolvedBy, &a.Report, &a.PatternWords, &a.ManuallyResolvedAt)
	if err != nil {
		return nil, err
	}
	a.ProjectId = string(projectId)
	a.Severity = parseSeverity(severityStr)
	if detailsJSON.String != "" {
		_ = json.Unmarshal([]byte(detailsJSON.String), &a.Details)
	}
	return &a, nil
}

func parseSeverity(s string) model.Status {
	switch s {
	case "warning":
		return model.WARNING
	case "critical":
		return model.CRITICAL
	default:
		return model.UNKNOWN
	}
}

func (db *DB) InitBuiltinAlertingRules(projectId ProjectId) error {
	for _, builtin := range model.BuiltinAlertingRules() {
		rule := builtin
		rule.ProjectId = string(projectId)
		err := db.CreateAlertingRule(projectId, &rule)
		if err != nil {
			if db.IsUniqueViolationError(err) {
				continue
			}
			return err
		}
	}
	return nil
}

func (db *DB) DisableBuiltinAlertingRules(projectId ProjectId) error {
	_, err := db.db.Exec(
		"UPDATE alerting_rule SET enabled = 0, updated_at = $1 WHERE project_id = $2 AND builtin = 1 AND enabled = 1",
		timeseries.Now(), projectId)
	return err
}

func (db *DB) ClearAlertingRulesReadonly(projectId ProjectId) error {
	rules, err := db.GetAlertingRules(projectId)
	if err != nil {
		return err
	}
	for _, r := range rules {
		if r.Readonly {
			r.Readonly = false
			if err := db.UpdateAlertingRule(projectId, r); err != nil {
				return err
			}
		}
	}
	return nil
}

func (db *DB) GetLatestAlertsByRule(projectId ProjectId, ruleId string) ([]*model.Alert, error) {
	rows, err := db.db.Query(`
		SELECT id, fingerprint, rule_id, application_id, application_category, severity, summary, details, opened_at, resolved_at, updated_at, suppressed, resolved_by, report, pattern_words, manually_resolved_at
		FROM alert
		WHERE project_id = $1 AND rule_id = $2 AND resolved_at = 0
		ORDER BY opened_at DESC
	`, projectId, ruleId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var alerts []*model.Alert
	for rows.Next() {
		alert, err := scanAlert(rows, projectId)
		if err != nil {
			return nil, err
		}
		alerts = append(alerts, alert)
	}
	return alerts, nil
}

func (db *DB) HasAlerts(projectId ProjectId) (bool, error) {
	var exists int
	err := db.db.QueryRow(
		"SELECT 1 FROM alert WHERE project_id = $1 LIMIT 1",
		projectId).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
