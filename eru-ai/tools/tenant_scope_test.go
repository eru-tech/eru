package tools

import (
	"context"
	"os"
	"testing"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	utils "github.com/eru-tech/eru/eru-utils"
)

func TestMain(m *testing.M) {
	logs.LogInit("test", "tenant-scope-test")
	os.Exit(m.Run())
}

func TestWriteTenantRoute(t *testing.T) {
	withDefault := utils.WithDefaultTenant(context.Background(), "global")
	noDefault := context.Background()

	tests := []struct {
		name     string
		ctx      context.Context
		tenantId string
		scope    TenantScope
		want     string
		wantErr  bool
	}{
		{"tenant scope", withDefault, "acme", ScopeTenant, "acme", false},
		{"default scope", withDefault, "acme", ScopeDefaultTenant, "global", false},
		{"project scope", withDefault, "acme", ScopeProject, "", false},
		{"project scope without tenant", noDefault, "", ScopeProject, "", false},
		{"default scope without default", noDefault, "acme", ScopeDefaultTenant, "", true},
		{"tenant scope without tenant", noDefault, "", ScopeTenant, "", true},
		{"unknown scope", withDefault, "acme", TenantScope("nonsense"), "", true},
	}
	for _, tt := range tests {
		got, err := WriteTenantRoute(tt.ctx, tt.tenantId, tt.scope)
		if (err != nil) != tt.wantErr {
			t.Errorf("%s: WriteTenantRoute() err = %v, wantErr %v", tt.name, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("%s: WriteTenantRoute() = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestTenantPath(t *testing.T) {
	tests := []struct {
		name        string
		routeTenant string
		parts       []string
		want        string
	}{
		{"tenant level", "acme", []string{"myquery/save", "q1", "sql"}, "http://ql/store/myproj/acme/myquery/save/q1/sql"},
		{"project level", "", []string{"myquery/save", "q1", "sql"}, "http://ql/store/myproj/myquery/save/q1/sql"},
		{"empty part dropped", "acme", []string{"myquery/list", ""}, "http://ql/store/myproj/acme/myquery/list"},
	}
	for _, tt := range tests {
		if got := TenantPath("http://ql/store", "myproj", tt.routeTenant, tt.parts...); got != tt.want {
			t.Errorf("%s: TenantPath() = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestFetchTenantPathCarriesDefaultTenant(t *testing.T) {
	ctx := utils.WithDefaultTenant(context.Background(), "global")
	want := "http://ql/store/myproj/global___acme/myquery/fetch/q1"
	if got := FetchTenantPath(ctx, "http://ql/store", "myproj", "acme", "myquery/fetch", "q1"); got != want {
		t.Errorf("FetchTenantPath() = %q, want %q", got, want)
	}
	want = "http://ql/store/myproj/acme/myquery/fetch/q1"
	if got := FetchTenantPath(context.Background(), "http://ql/store", "myproj", "acme", "myquery/fetch", "q1"); got != want {
		t.Errorf("FetchTenantPath() without default = %q, want %q", got, want)
	}
}

func TestExpandScopedActions(t *testing.T) {
	base := []ToolAction{
		{ActionName: "save_thing", Description: "Save a thing."},
		{ActionName: "list_things", Description: "List things."},
	}
	expanded, scopes := ExpandScopedActions(base, map[string]bool{"save_thing": true})

	wantNames := []string{"save_thing", "save_thing_default", "save_thing_project", "list_things"}
	if len(expanded) != len(wantNames) {
		t.Fatalf("expanded %d actions, want %d", len(expanded), len(wantNames))
	}
	for i, want := range wantNames {
		if expanded[i].ActionName != want {
			t.Errorf("action %d = %q, want %q", i, expanded[i].ActionName, want)
		}
	}
	for name, wantScope := range map[string]TenantScope{
		"save_thing":         ScopeTenant,
		"save_thing_default": ScopeDefaultTenant,
		"save_thing_project": ScopeProject,
	} {
		if scopes[name].BaseName != "save_thing" || scopes[name].Scope != wantScope {
			t.Errorf("scopes[%q] = %+v, want base save_thing scope %q", name, scopes[name], wantScope)
		}
	}
	if scopes["list_things"].BaseName != "list_things" {
		t.Errorf("read action lost its base name: %+v", scopes["list_things"])
	}
	if _, ok := scopes["list_things_project"]; ok {
		t.Error("read action was expanded into scopes")
	}
}
