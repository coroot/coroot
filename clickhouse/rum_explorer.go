package clickhouse

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/ClickHouse/clickhouse-go/v2"
)

type RumExplorerOp string

const (
	RumExplorerOpEq   RumExplorerOp = ":"
	RumExplorerOpLike RumExplorerOp = "~"
	RumExplorerOpGt   RumExplorerOp = ">"
	RumExplorerOpGte  RumExplorerOp = ">="
	RumExplorerOpLt   RumExplorerOp = "<"
	RumExplorerOpLte  RumExplorerOp = "<="
)

// RumExplorerScope indicates which Explorer tab a field applies to.
type RumExplorerScope int

const (
	RumExplorerScopeBoth RumExplorerScope = iota
	RumExplorerScopeSessions
	RumExplorerScopePageViews
)

type RumExplorerTerm struct {
	Field  string // empty for free-text
	Op     RumExplorerOp
	Value  string
	Raw    string // original token for UI chips
	Number float64
	HasNum bool
	Scope  RumExplorerScope
}

type RumExplorerFilter struct {
	Terms []RumExplorerTerm
	Text  []string // free-text tokens
}

type rumExplorerFieldMeta struct {
	Scope      RumExplorerScope
	Numeric    bool
	SessionCol string // HAVING column alias
	PageView   string // outer WHERE expression (aliases s / e)
}

var rumExplorerFields = map[string]rumExplorerFieldMeta{
	"duration": {Scope: RumExplorerScopeSessions, Numeric: true, SessionCol: "DurMs"},
	"pages":    {Scope: RumExplorerScopeSessions, Numeric: true, SessionCol: "PageViews"},
	"path":     {Scope: RumExplorerScopePageViews, PageView: "s.PagePath"},
	"load":     {Scope: RumExplorerScopePageViews, Numeric: true, PageView: "s.DurationMs"},
	"lcp":      {Scope: RumExplorerScopePageViews, Numeric: true, PageView: "ifNull(e.Lcp, 0)"},
	"inp":      {Scope: RumExplorerScopePageViews, Numeric: true, PageView: "ifNull(e.Inp, 0)"},
	"ttfb":     {Scope: RumExplorerScopePageViews, Numeric: true, PageView: "ifNull(e.Ttfb, 0)"},
	"cls":      {Scope: RumExplorerScopePageViews, Numeric: true, PageView: "ifNull(e.Cls, 0)"},
	"trace":    {Scope: RumExplorerScopePageViews, PageView: "s.TraceId"},
	"errors":   {Scope: RumExplorerScopeBoth, Numeric: true, SessionCol: "Errors", PageView: "errors"},
	"browser":  {Scope: RumExplorerScopeBoth, SessionCol: "Browser", PageView: "ifNull(e.BrowserName, s.BrowserName)"},
	"os":       {Scope: RumExplorerScopeBoth, SessionCol: "Os", PageView: "s.OsName"},
	"device":   {Scope: RumExplorerScopeBoth, SessionCol: "Device", PageView: "ifNull(e.DeviceType, s.DeviceType)"},
	"country":  {Scope: RumExplorerScopeBoth, SessionCol: "Country", PageView: "s.Country"},
	"version":  {Scope: RumExplorerScopeBoth, SessionCol: "Version", PageView: "s.Version"},
	"session":  {Scope: RumExplorerScopeBoth, SessionCol: "SessionId", PageView: "s.SessionId"},
}

// ParseRumExplorerQuery parses a space-separated query into terms.
// Syntax: field:value | field~value | field>N | free-text
// Quoted values: path~"my page". Durations: 30s, 2.5s, 4000ms, or bare number (ms for durations, unitless for pages/cls/errors).
func ParseRumExplorerQuery(s string) (RumExplorerFilter, error) {
	var out RumExplorerFilter
	tokens, err := tokenizeRumExplorer(s)
	if err != nil {
		return out, err
	}
	for _, tok := range tokens {
		term, err := parseRumExplorerToken(tok)
		if err != nil {
			return RumExplorerFilter{}, err
		}
		if term.Field == "" {
			out.Text = append(out.Text, term.Value)
			continue
		}
		out.Terms = append(out.Terms, term)
	}
	return out, nil
}

func (f RumExplorerFilter) Empty() bool {
	return len(f.Terms) == 0 && len(f.Text) == 0
}

func tokenizeRumExplorer(s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	var tokens []string
	var b strings.Builder
	inQuote := false
	escape := false
	flush := func() {
		if b.Len() == 0 {
			return
		}
		tokens = append(tokens, b.String())
		b.Reset()
	}
	for _, r := range s {
		if escape {
			b.WriteRune(r)
			escape = false
			continue
		}
		if r == '\\' {
			escape = true
			continue
		}
		if r == '"' {
			inQuote = !inQuote
			b.WriteRune(r)
			continue
		}
		if unicode.IsSpace(r) && !inQuote {
			flush()
			continue
		}
		b.WriteRune(r)
	}
	if inQuote {
		return nil, fmt.Errorf("unclosed quote in query")
	}
	flush()
	return tokens, nil
}

func parseRumExplorerToken(tok string) (RumExplorerTerm, error) {
	term := RumExplorerTerm{Raw: tok}
	// Find operator after field name.
	fieldEnd := -1
	op := RumExplorerOp("")
	for i := 0; i < len(tok); i++ {
		c := tok[i]
		if c == ':' || c == '~' {
			fieldEnd = i
			op = RumExplorerOp(string(c))
			break
		}
		if c == '>' || c == '<' {
			fieldEnd = i
			if i+1 < len(tok) && tok[i+1] == '=' {
				op = RumExplorerOp(tok[i : i+2])
			} else {
				op = RumExplorerOp(string(c))
			}
			break
		}
		if !isRumExplorerFieldChar(c) {
			// free text (or invalid field start)
			break
		}
	}
	if fieldEnd <= 0 || op == "" {
		term.Value = unquoteRumExplorer(tok)
		return term, nil
	}
	field := strings.ToLower(tok[:fieldEnd])
	meta, ok := rumExplorerFields[field]
	if !ok {
		return term, fmt.Errorf("unknown field %q", field)
	}
	valStart := fieldEnd + len(op)
	if valStart >= len(tok) {
		return term, fmt.Errorf("missing value for %s%s", field, op)
	}
	val := unquoteRumExplorer(tok[valStart:])
	term.Field = field
	term.Op = op
	term.Value = val
	term.Scope = meta.Scope
	if meta.Numeric {
		n, err := parseRumExplorerNumber(val, field)
		if err != nil {
			return term, err
		}
		term.Number = n
		term.HasNum = true
		if op == RumExplorerOpEq || op == RumExplorerOpLike {
			// allow errors:0 style equality for numeric fields
			if op == RumExplorerOpLike {
				return term, fmt.Errorf("field %q does not support ~", field)
			}
		}
	} else if op != RumExplorerOpEq && op != RumExplorerOpLike {
		return term, fmt.Errorf("field %q only supports : and ~", field)
	}
	return term, nil
}

func isRumExplorerFieldChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

func unquoteRumExplorer(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		inner := s[1 : len(s)-1]
		return strings.ReplaceAll(inner, `\"`, `"`)
	}
	return s
}

func parseRumExplorerNumber(val, field string) (float64, error) {
	v := strings.TrimSpace(strings.ToLower(val))
	mult := 1.0
	switch {
	case strings.HasSuffix(v, "ms"):
		v = strings.TrimSuffix(v, "ms")
		mult = 1
	case strings.HasSuffix(v, "s") && !strings.HasSuffix(v, "ms"):
		// bare "s" suffix — seconds → ms for duration-like fields
		v = strings.TrimSuffix(v, "s")
		if field == "cls" || field == "pages" || field == "errors" {
			return 0, fmt.Errorf("invalid unit for %s: %s", field, val)
		}
		mult = 1000
	}
	n, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number for %s: %s", field, val)
	}
	return n * mult, nil
}

func (f RumExplorerFilter) AppliesToSessions(t RumExplorerTerm) bool {
	return t.Scope == RumExplorerScopeBoth || t.Scope == RumExplorerScopeSessions
}

func (f RumExplorerFilter) AppliesToPageViews(t RumExplorerTerm) bool {
	return t.Scope == RumExplorerScopeBoth || t.Scope == RumExplorerScopePageViews
}

// SessionHavingSQL returns HAVING predicates for GetRumSessions aggregates.
func (f RumExplorerFilter) SessionHavingSQL() (string, []any) {
	if f.Empty() {
		return "", nil
	}
	var parts []string
	var args []any
	idx := 0
	next := func(prefix string) string {
		idx++
		return fmt.Sprintf("%s%d", prefix, idx)
	}
	for _, t := range f.Terms {
		if !f.AppliesToSessions(t) {
			continue
		}
		meta := rumExplorerFields[t.Field]
		col := meta.SessionCol
		if col == "" {
			continue
		}
		name := next("exs_")
		switch {
		case t.Field == "errors" && t.HasNum && t.Op == RumExplorerOpGt && t.Number == 0:
			parts = append(parts, col+" > 0")
		case t.HasNum:
			op := string(t.Op)
			if op == ":" {
				op = "="
			}
			parts = append(parts, fmt.Sprintf("%s %s @%s", col, op, name))
			args = append(args, clickhouse.Named(name, t.Number))
		case t.Op == RumExplorerOpLike:
			parts = append(parts, fmt.Sprintf("%s ILIKE @%s", col, name))
			args = append(args, clickhouse.Named(name, "%"+t.Value+"%"))
		default: // eq
			parts = append(parts, fmt.Sprintf("%s = @%s", col, name))
			args = append(args, clickhouse.Named(name, t.Value))
		}
	}
	for _, text := range f.Text {
		name := next("ext_")
		parts = append(parts, fmt.Sprintf("(SessionId ILIKE @%s OR Browser ILIKE @%s OR Country ILIKE @%s)", name, name, name))
		args = append(args, clickhouse.Named(name, "%"+text+"%"))
	}
	if len(parts) == 0 {
		return "", nil
	}
	return " HAVING " + strings.Join(parts, " AND "), args
}

// PageViewWhereSQL returns outer WHERE predicates for GetRumPageViewsFiltered (aliases s, e).
func (f RumExplorerFilter) PageViewWhereSQL() (string, []any) {
	if f.Empty() {
		return "", nil
	}
	var parts []string
	var args []any
	idx := 0
	next := func(prefix string) string {
		idx++
		return fmt.Sprintf("%s%d", prefix, idx)
	}
	for _, t := range f.Terms {
		if !f.AppliesToPageViews(t) {
			continue
		}
		meta := rumExplorerFields[t.Field]
		expr := meta.PageView
		if expr == "" {
			continue
		}
		name := next("exp_")
		if t.Field == "errors" {
			if t.HasNum && ((t.Op == RumExplorerOpGt && t.Number == 0) || (t.Op == RumExplorerOpGte && t.Number >= 1)) {
				parts = append(parts, "s.StatusCode = 'STATUS_CODE_ERROR'")
				continue
			}
			if t.HasNum && ((t.Op == RumExplorerOpEq && t.Number == 0) || (t.Op == RumExplorerOpLte && t.Number == 0) || (t.Op == RumExplorerOpLt && t.Number <= 1)) {
				parts = append(parts, "s.StatusCode != 'STATUS_CODE_ERROR'")
				continue
			}
			// other error comparisons not meaningful for page views — skip
			continue
		}
		switch {
		case t.HasNum:
			op := string(t.Op)
			if op == ":" {
				op = "="
			}
			parts = append(parts, fmt.Sprintf("%s %s @%s", expr, op, name))
			args = append(args, clickhouse.Named(name, t.Number))
		case t.Op == RumExplorerOpLike:
			parts = append(parts, fmt.Sprintf("%s ILIKE @%s", expr, name))
			args = append(args, clickhouse.Named(name, "%"+t.Value+"%"))
		default:
			if t.Field == "session" || t.Field == "trace" {
				// prefix match is useful for short ids
				parts = append(parts, fmt.Sprintf("%s ILIKE @%s", expr, name))
				args = append(args, clickhouse.Named(name, t.Value+"%"))
			} else {
				parts = append(parts, fmt.Sprintf("%s = @%s", expr, name))
				args = append(args, clickhouse.Named(name, t.Value))
			}
		}
	}
	for _, text := range f.Text {
		name := next("ept_")
		parts = append(parts, fmt.Sprintf("(s.PagePath ILIKE @%s OR s.SessionId ILIKE @%s OR s.TraceId ILIKE @%s)", name, name, name))
		args = append(args, clickhouse.Named(name, "%"+text+"%"))
	}
	if len(parts) == 0 {
		return "", nil
	}
	return " AND " + strings.Join(parts, " AND "), args
}
