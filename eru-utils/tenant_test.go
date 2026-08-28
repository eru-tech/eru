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
