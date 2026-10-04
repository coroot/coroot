package clickhouse

import (
	"strings"
	"testing"
)

func TestParseRumExplorerQuery_Basic(t *testing.T) {
	f, err := ParseRumExplorerQuery(`errors>0 lcp>4000 browser:Chrome path~checkout`)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Terms) != 4 {
		t.Fatalf("terms=%d want 4: %+v", len(f.Terms), f.Terms)
	}
	if f.Terms[0].Field != "errors" || !f.Terms[0].HasNum || f.Terms[0].Number != 0 || f.Terms[0].Op != RumExplorerOpGt {
		t.Fatalf("errors term: %+v", f.Terms[0])
	}
	if f.Terms[1].Field != "lcp" || f.Terms[1].Number != 4000 {
		t.Fatalf("lcp term: %+v", f.Terms[1])
	}
	if f.Terms[2].Field != "browser" || f.Terms[2].Value != "Chrome" || f.Terms[2].Op != RumExplorerOpEq {
		t.Fatalf("browser term: %+v", f.Terms[2])
	}
	if f.Terms[3].Field != "path" || f.Terms[3].Op != RumExplorerOpLike || f.Terms[3].Value != "checkout" {
		t.Fatalf("path term: %+v", f.Terms[3])
	}
}

func TestParseRumExplorerQuery_UnitsAndQuotes(t *testing.T) {
	f, err := ParseRumExplorerQuery(`duration>30s lcp>=2.5s path~"my page" load<3000ms`)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Terms) != 4 {
		t.Fatalf("terms=%d: %+v", len(f.Terms), f.Terms)
	}
	if f.Terms[0].Number != 30000 {
		t.Fatalf("duration 30s -> %v", f.Terms[0].Number)
	}
	if f.Terms[1].Number != 2500 || f.Terms[1].Op != RumExplorerOpGte {
		t.Fatalf("lcp 2.5s: %+v", f.Terms[1])
	}
	if f.Terms[2].Value != "my page" {
		t.Fatalf("quoted path: %+v", f.Terms[2])
	}
	if f.Terms[3].Number != 3000 {
		t.Fatalf("load 3000ms: %+v", f.Terms[3])
	}
}

func TestParseRumExplorerQuery_FreeText(t *testing.T) {
	f, err := ParseRumExplorerQuery(`checkout abc123`)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Terms) != 0 || len(f.Text) != 2 {
		t.Fatalf("got terms=%v text=%v", f.Terms, f.Text)
	}
}

func TestParseRumExplorerQuery_Errors(t *testing.T) {
	cases := []string{
		`unknown:foo`,
		`lcp>abc`,
		`browser>1`,
		`"unclosed`,
		`lcp~4000`,
	}
	for _, c := range cases {
		if _, err := ParseRumExplorerQuery(c); err == nil {
			t.Errorf("expected error for %q", c)
		}
	}
}

func TestParseRumExplorerQuery_Scope(t *testing.T) {
	f, err := ParseRumExplorerQuery(`duration>1000 path~/ lcp>100 browser:Chrome`)
	if err != nil {
		t.Fatal(err)
	}
	if !f.AppliesToSessions(f.Terms[0]) || f.AppliesToPageViews(f.Terms[0]) {
		t.Fatalf("duration scope: %+v", f.Terms[0])
	}
	if f.AppliesToSessions(f.Terms[1]) || !f.AppliesToPageViews(f.Terms[1]) {
		t.Fatalf("path scope: %+v", f.Terms[1])
	}
	if !f.AppliesToSessions(f.Terms[3]) || !f.AppliesToPageViews(f.Terms[3]) {
		t.Fatalf("browser scope: %+v", f.Terms[3])
	}
}

func TestSessionHavingSQL(t *testing.T) {
	f, err := ParseRumExplorerQuery(`errors>0 browser:Chrome duration>=5000`)
	if err != nil {
		t.Fatal(err)
	}
	sql, args := f.SessionHavingSQL()
	if !strings.Contains(sql, "HAVING") {
		t.Fatalf("sql=%q", sql)
	}
	if !strings.Contains(sql, "Errors > 0") {
		t.Fatalf("missing Errors>0: %s", sql)
	}
	if !strings.Contains(sql, "Browser = @") {
		t.Fatalf("missing Browser: %s", sql)
	}
	if !strings.Contains(sql, "DurMs >=") {
		t.Fatalf("missing DurMs: %s", sql)
	}
	if len(args) != 2 { // errors>0 has no arg
		t.Fatalf("args=%d want 2", len(args))
	}
	// path-only term should not appear in HAVING
	f2, _ := ParseRumExplorerQuery(`path~/checkout`)
	sql2, args2 := f2.SessionHavingSQL()
	if sql2 != "" || args2 != nil {
		t.Fatalf("path should not affect sessions: %q %v", sql2, args2)
	}
}

func TestPageViewWhereSQL(t *testing.T) {
	f, err := ParseRumExplorerQuery(`errors>0 lcp>4000 path~shop browser:Chrome`)
	if err != nil {
		t.Fatal(err)
	}
	sql, args := f.PageViewWhereSQL()
	if !strings.HasPrefix(sql, " AND ") {
		t.Fatalf("sql=%q", sql)
	}
	if !strings.Contains(sql, "STATUS_CODE_ERROR") {
		t.Fatalf("missing status: %s", sql)
	}
	if !strings.Contains(sql, "ifNull(e.Lcp, 0) > @") {
		t.Fatalf("missing lcp: %s", sql)
	}
	if !strings.Contains(sql, "s.PagePath ILIKE @") {
		t.Fatalf("missing path: %s", sql)
	}
	if len(args) != 3 {
		t.Fatalf("args=%d want 3", len(args))
	}
	// duration-only should not affect page views
	f2, _ := ParseRumExplorerQuery(`duration>10s pages>2`)
	sql2, _ := f2.PageViewWhereSQL()
	if sql2 != "" {
		t.Fatalf("session-only fields leaked: %q", sql2)
	}
}

func TestEmptyQuery(t *testing.T) {
	f, err := ParseRumExplorerQuery("  ")
	if err != nil {
		t.Fatal(err)
	}
	if !f.Empty() {
		t.Fatal("expected empty")
	}
	sql, args := f.SessionHavingSQL()
	if sql != "" || args != nil {
		t.Fatal(sql, args)
	}
}
