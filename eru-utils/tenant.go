package eru_utils

import (
	"context"
	"strings"
)

const TenantSeparator = "___"

type defaultTenantKeyType struct{}

var defaultTenantKey = defaultTenantKeyType{}

func ParseTenantRoute(routeTenant string) (tenantId string, defaultTenantId string) {
	routeTenant = strings.ToLower(strings.TrimSpace(routeTenant))
	if routeTenant == "" {
		return "", ""
	}
	defaultTenantId, tenantId, found := strings.Cut(routeTenant, TenantSeparator)
	if !found {
		return routeTenant, ""
	}
	if tenantId == "" {
		return defaultTenantId, ""
	}
	if defaultTenantId == tenantId {
		return tenantId, ""
	}
	return tenantId, defaultTenantId
}

func WithDefaultTenant(ctx context.Context, defaultTenantId string) context.Context {
	if defaultTenantId == "" {
		return ctx
	}
	return context.WithValue(ctx, defaultTenantKey, defaultTenantId)
}

func DefaultTenant(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	defaultTenantId, _ := ctx.Value(defaultTenantKey).(string)
	return defaultTenantId
}

func TenantLookupOrder(ctx context.Context, tenantId string, extraFallbacks ...string) []string {
	candidates := make([]string, 0, 2+len(extraFallbacks))
	seen := make(map[string]bool)
	for _, candidate := range append([]string{tenantId, DefaultTenant(ctx)}, extraFallbacks...) {
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		candidates = append(candidates, candidate)
	}
	return candidates
}
