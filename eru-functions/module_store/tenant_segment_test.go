package module_store

import "testing"

func TestTenantSegment(t *testing.T) {
	tests := []struct {
		name          string
		routeTenantId string
		want          string
	}{
		{"tenant", "acme", "/acme"},
		{"tenant with a default", "global___acme", "/global___acme"},
		{"no tenant addresses the project level route", "", ""},
	}
	for _, tt := range tests {
		if got := tenantSegment(tt.routeTenantId); got != tt.want {
			t.Errorf("%s: tenantSegment(%q) = %q, want %q", tt.name, tt.routeTenantId, got, tt.want)
		}
	}
}

func TestEruaiTenantId(t *testing.T) {
	tests := []struct {
		name          string
		routeTenantId string
		want          string
	}{
		{"tenant", "acme", "acme"},
		{"tenant with a default", "global___acme", "global___acme"},
		{"no tenant addresses the project as its own tenant", "", "myproj"},
	}
	for _, tt := range tests {
		if got := eruaiTenantId("myproj", tt.routeTenantId); got != tt.want {
			t.Errorf("%s: eruaiTenantId(%q) = %q, want %q", tt.name, tt.routeTenantId, got, tt.want)
		}
	}
}

// the step url is built from the tenant the function was called for. eru-ql has a project
// level route, so its tenant segment is dropped; eru-ai keys a project's own tools and agents
// under the project id, so the project id goes in the tenant position instead.
func TestStepUrlsFollowTheCallTenant(t *testing.T) {
	tests := []struct {
		name          string
		routeTenantId string
		query         string
		tool          string
		agent         string
	}{
		{
			"tenant call",
			"global___acme",
			"/store/myproj/global___acme/myquery/execute/q1",
			"/myproj/global___acme/execute/tool/t1",
			"/myproj/global___acme/execute/agent/a1",
		},
		{
			"project level call",
			"",
			"/store/myproj/myquery/execute/q1",
			"/myproj/myproj/execute/tool/t1",
			"/myproj/myproj/execute/agent/a1",
		},
	}
	for _, tt := range tests {
		if got := "/store/myproj" + tenantSegment(tt.routeTenantId) + "/myquery/execute/q1"; got != tt.query {
			t.Errorf("%s: query url = %q, want %q", tt.name, got, tt.query)
		}
		aiTenant := eruaiTenantId("myproj", tt.routeTenantId)
		if got := "/myproj/" + aiTenant + "/execute/tool/t1"; got != tt.tool {
			t.Errorf("%s: tool url = %q, want %q", tt.name, got, tt.tool)
		}
		if got := "/myproj/" + aiTenant + "/execute/agent/a1"; got != tt.agent {
			t.Errorf("%s: agent url = %q, want %q", tt.name, got, tt.agent)
		}
	}
}
