package api

import (
	"testing"
)

func testApp(id, namespace, category, status string) mcpAppInfo {
	return mcpAppInfo{Id: id, Namespace: namespace, Category: category, Status: status}
}

// On a large hub list_applications returned the whole registry (tens of
// thousands of applications, megabytes of JSON) with no way to narrow it, which
// no caller can use. These cover the filter/order/cap that replaced it.
func TestMCPSelectApplicationsOrdersBySeverity(t *testing.T) {
	apps := []mcpAppInfo{
		testApp("c:ns:Deployment:ok-app", "ns", "application", "ok"),
		testApp("c:ns:Deployment:unknown-app", "ns", "application", ""),
		testApp("c:ns:Deployment:critical-app", "ns", "application", "critical"),
		testApp("c:ns:Deployment:warning-app", "ns", "application", "warning"),
	}

	got := mcpSelectApplications(apps, mcpAppsFilter{})

	want := []string{
		"c:ns:Deployment:critical-app",
		"c:ns:Deployment:warning-app",
		"c:ns:Deployment:ok-app",
		"c:ns:Deployment:unknown-app",
	}
	for i, app := range got {
		if app.Id != want[i] {
			t.Fatalf("position %d: got %s, want %s", i, app.Id, want[i])
		}
	}
}

func TestMCPSelectApplicationsTieBreaksById(t *testing.T) {
	apps := []mcpAppInfo{
		testApp("c:ns:Deployment:b", "ns", "application", "ok"),
		testApp("c:ns:Deployment:a", "ns", "application", "ok"),
	}

	got := mcpSelectApplications(apps, mcpAppsFilter{})

	if got[0].Id != "c:ns:Deployment:a" || got[1].Id != "c:ns:Deployment:b" {
		t.Fatalf("same-severity applications must be ordered by id, got %s then %s", got[0].Id, got[1].Id)
	}
}

func TestMCPSelectApplicationsLimitKeepsMostSevere(t *testing.T) {
	apps := []mcpAppInfo{
		testApp("c:ns:Deployment:ok-1", "ns", "application", "ok"),
		testApp("c:ns:Deployment:critical-1", "ns", "application", "critical"),
		testApp("c:ns:Deployment:ok-2", "ns", "application", "ok"),
		testApp("c:ns:Deployment:critical-2", "ns", "application", "critical"),
		testApp("c:ns:Deployment:warning-1", "ns", "application", "warning"),
	}

	got := mcpSelectApplications(apps, mcpAppsFilter{limit: 3})

	if len(got) != 3 {
		t.Fatalf("expected 3 applications, got %d", len(got))
	}
	want := []string{
		"c:ns:Deployment:critical-1",
		"c:ns:Deployment:critical-2",
		"c:ns:Deployment:warning-1",
	}
	for i, app := range got {
		if app.Id != want[i] {
			t.Fatalf("the cap must keep the most severe applications: position %d is %s, want %s", i, app.Id, want[i])
		}
	}
}

func TestMCPSelectApplicationsLimitZeroMeansNoCap(t *testing.T) {
	apps := []mcpAppInfo{
		testApp("c:ns:Deployment:a", "ns", "application", "ok"),
		testApp("c:ns:Deployment:b", "ns", "application", "ok"),
	}

	if got := mcpSelectApplications(apps, mcpAppsFilter{limit: 0}); len(got) != 2 {
		t.Fatalf("limit 0 must not cap the result, got %d applications", len(got))
	}
}

func TestMCPSelectApplicationsFilters(t *testing.T) {
	apps := []mcpAppInfo{
		testApp("c:team-a:Deployment:checkout", "team-a", "application", "ok"),
		testApp("c:team-b:Deployment:checkout", "team-b", "application", "critical"),
		testApp("c:team-b:Deployment:payments", "team-b", "application", "ok"),
		testApp("c:team-b:DaemonSet:node-exporter", "team-b", "monitoring", "ok"),
	}

	for name, tc := range map[string]struct {
		filter mcpAppsFilter
		want   []string
	}{
		"namespace": {
			filter: mcpAppsFilter{namespace: "team-a"},
			want:   []string{"c:team-a:Deployment:checkout"},
		},
		"category": {
			filter: mcpAppsFilter{category: "monitoring"},
			want:   []string{"c:team-b:DaemonSet:node-exporter"},
		},
		"status": {
			filter: mcpAppsFilter{status: "critical"},
			want:   []string{"c:team-b:Deployment:checkout"},
		},
		"search is case-insensitive and matches a substring": {
			filter: mcpAppsFilter{search: "CHECKout"},
			// severity first: team-b's checkout is critical, team-a's is ok
			want: []string{"c:team-b:Deployment:checkout", "c:team-a:Deployment:checkout"},
		},
		"search matches the kind": {
			filter: mcpAppsFilter{search: "daemonset"},
			want:   []string{"c:team-b:DaemonSet:node-exporter"},
		},
		"combined": {
			filter: mcpAppsFilter{namespace: "team-b", status: "ok"},
			want:   []string{"c:team-b:DaemonSet:node-exporter", "c:team-b:Deployment:payments"},
		},
		"no match": {
			filter: mcpAppsFilter{namespace: "team-c"},
			want:   []string{},
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := mcpSelectApplications(apps, tc.filter)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d applications, want %d: %v", len(got), len(tc.want), got)
			}
			for i, app := range got {
				if app.Id != tc.want[i] {
					t.Fatalf("position %d: got %s, want %s", i, app.Id, tc.want[i])
				}
			}
		})
	}
}
