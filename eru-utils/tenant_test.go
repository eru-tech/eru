package eru_utils

import (
	"context"
	"reflect"
	"testing"
)

func TestParseTenantRoute(t *testing.T) {
	tests := []struct {
		routeTenant     string
		tenantId        string
		defaultTenantId string
	}{
		{"", "", ""},
		{"acme", "acme", ""},
		{"ACME", "acme", ""},
		{"Global___ACME", "acme", "global"},
		{"global___acme", "acme", "global"},
		{"global___acme___dev", "acme___dev", "global"},
		{"___acme", "acme", ""},
		{"global___", "global", ""},
		{"acme___acme", "acme", ""},
	}
	for _, tt := range tests {
		tenantId, defaultTenantId := ParseTenantRoute(tt.routeTenant)
		if tenantId != tt.tenantId || defaultTenantId != tt.defaultTenantId {
			t.Errorf("ParseTenantRoute(%q) = (%q, %q), want (%q, %q)", tt.routeTenant, tenantId, defaultTenantId, tt.tenantId, tt.defaultTenantId)
		}
	}
}

func TestTenantLookupOrder(t *testing.T) {
	tests := []struct {
		name            string
		defaultTenantId string
		tenantId        string
		extraFallbacks  []string
		want            []string
	}{
		{"no default", "", "acme", nil, []string{"acme"}},
		{"with default", "global", "acme", nil, []string{"acme", "global"}},
		{"with project fallback", "global", "acme", []string{"myproj"}, []string{"acme", "global", "myproj"}},
		{"default same as tenant", "acme", "acme", []string{"myproj"}, []string{"acme", "myproj"}},
		{"no tenant", "global", "", []string{"myproj"}, []string{"global", "myproj"}},
	}
	for _, tt := range tests {
		ctx := WithDefaultTenant(context.Background(), tt.defaultTenantId)
		if got := TenantLookupOrder(ctx, tt.tenantId, tt.extraFallbacks...); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: TenantLookupOrder() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestJoinTenantRoute(t *testing.T) {
	tests := []struct {
		defaultTenantId string
		tenantId        string
		want            string
	}{
		{"", "", ""},
		{"", "acme", "acme"},
		{"global", "", "global"},
		{"global", "acme", "global___acme"},
		{"Global", "ACME", "global___acme"},
		{"acme", "acme", "acme"},
	}
	for _, tt := range tests {
		if got := JoinTenantRoute(tt.defaultTenantId, tt.tenantId); got != tt.want {
			t.Errorf("JoinTenantRoute(%q, %q) = %q, want %q", tt.defaultTenantId, tt.tenantId, got, tt.want)
		}
	}
}

func TestJoinTenantRouteRoundTrip(t *testing.T) {
	routeTenants := []string{"acme", "global___acme", ""}
	for _, routeTenant := range routeTenants {
		tenantId, defaultTenantId := ParseTenantRoute(routeTenant)
		if got := JoinTenantRoute(defaultTenantId, tenantId); got != routeTenant {
			t.Errorf("round trip of %q = %q", routeTenant, got)
		}
	}
}

func TestFetchTenantRoute(t *testing.T) {
	ctx := WithDefaultTenant(context.Background(), "global")
	if got := FetchTenantRoute(ctx, "acme"); got != "global___acme" {
		t.Errorf("FetchTenantRoute() = %q, want %q", got, "global___acme")
	}
	if got := FetchTenantRoute(context.Background(), "acme"); got != "acme" {
		t.Errorf("FetchTenantRoute() without default = %q, want %q", got, "acme")
	}
}
