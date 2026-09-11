package module_store

import (
	"context"
	"os"
	"strings"
	"testing"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_utils "github.com/eru-tech/eru/eru-utils"
)

func TestMain(m *testing.M) {
	logs.LogInit("test", "eru-functions-store-test")
	os.Exit(m.Run())
}

func TestAsyncTenantContext(t *testing.T) {
	tests := []struct {
		name              string
		rowProjectId      string
		rowRouteTenantId  string
		fallbackProjectId string
		wantProjectId     string
		wantTenantId      string
		wantDefaultTenant string
	}{
		{"tenant with default", "myproj", "global___acme", "pollproj", "myproj", "acme", "global"},
		{"tenant only", "myproj", "acme", "pollproj", "myproj", "acme", ""},
		{"project level row", "myproj", "", "pollproj", "myproj", "", ""},
		{"row written before project was recorded", "", "acme", "pollproj", "pollproj", "acme", ""},
	}
	for _, tt := range tests {
		asyncFuncData := AsyncFuncData{ProjectId: tt.rowProjectId, RouteTenantId: tt.rowRouteTenantId}
		ctx, projectId, tenantId := AsyncTenantContext(context.Background(), asyncFuncData, tt.fallbackProjectId)
		if projectId != tt.wantProjectId {
			t.Errorf("%s: projectId = %q, want %q", tt.name, projectId, tt.wantProjectId)
		}
		if tenantId != tt.wantTenantId {
			t.Errorf("%s: tenantId = %q, want %q", tt.name, tenantId, tt.wantTenantId)
		}
		if got := eru_utils.DefaultTenant(ctx); got != tt.wantDefaultTenant {
			t.Errorf("%s: default tenant = %q, want %q", tt.name, got, tt.wantDefaultTenant)
		}
	}
}

// an async row raised for a tenant has to resolve its function through the same fallback
// order the original call had - tenant first, then the default tenant.
func TestAsyncTenantContextRestoresLookupOrder(t *testing.T) {
	asyncFuncData := AsyncFuncData{ProjectId: "myproj", RouteTenantId: "global___acme"}
	ctx, projectId, tenantId := AsyncTenantContext(context.Background(), asyncFuncData, "")
	order := eru_utils.TenantLookupOrder(ctx, tenantId)
	if len(order) != 2 || order[0] != "acme" || order[1] != "global" {
		t.Errorf("lookup order = %v, want [acme global]", order)
	}
	if projectId != "myproj" {
		t.Errorf("projectId = %q, want myproj", projectId)
	}
}

func TestScheduleProcedureCall(t *testing.T) {
	got := scheduleProcedureCall("my_func", `{"a":1}`, "my_scheduler", "myproj", "global___acme")
	want := `CALL schedule_procedure('my_func','{"a":1}','my_scheduler','myproj','global___acme')`
	if got != want {
		t.Errorf("scheduleProcedureCall() = %s, want %s", got, want)
	}
	// a project level schedule leaves the tenant empty rather than dropping the argument
	got = scheduleProcedureCall("my_func", "{}", "my_scheduler", "myproj", "")
	if !strings.HasSuffix(got, `,'myproj','')`) {
		t.Errorf("project level call = %s, want it to end with the project and an empty tenant", got)
	}
}

func TestCanUnscheduleJob(t *testing.T) {
	tests := []struct {
		name             string
		callerDefault    string
		callerTenantId   string
		jobRouteTenantId string
		want             bool
	}{
		{"own tenant", "", "acme", "acme", true},
		{"another tenant", "", "acme", "globex", false},
		{"default tenant of the caller", "global", "acme", "global", true},
		{"default tenant without the caller having one", "", "acme", "global", false},
		{"job stored in route form", "global", "acme", "global___acme", true},
		{"project level job from a tenant", "global", "acme", "", false},
		{"project level job from project level", "", "", "", true},
		{"tenant job from project level", "", "", "acme", false},
	}
	for _, tt := range tests {
		ctx := eru_utils.WithDefaultTenant(context.Background(), tt.callerDefault)
		if got := canUnscheduleJob(ctx, tt.callerTenantId, tt.jobRouteTenantId); got != tt.want {
			t.Errorf("%s: canUnscheduleJob(%q, %q) = %v, want %v", tt.name, tt.callerTenantId, tt.jobRouteTenantId, got, tt.want)
		}
	}
}
