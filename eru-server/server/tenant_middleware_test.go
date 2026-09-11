package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	eru_utils "github.com/eru-tech/eru/eru-utils"
	"github.com/gorilla/mux"
)

func TestTenantMiddleWare(t *testing.T) {
	tests := []struct {
		name            string
		path            string
		headerKey       string
		header          string
		tenantId        string
		defaultTenantId string
	}{
		{"tenant only", "/store/myproj/ACME/func/list", "", "", "acme", ""},
		{"default and tenant", "/store/myproj/Global___ACME/func/list", "", "", "acme", "global"},
		{"header tenant", "/store/myproj/func/list", TenantHeaderKey, "Global___ACME", "", "global"},
		{"header tenant lower case", "/store/myproj/func/list", "x-tenant-id", "Global___ACME", "", "global"},
		{"legacy header tenant", "/store/myproj/func/list", "tenant_id", "Global___ACME", "", "global"},
		{"header tenant without a default", "/store/myproj/func/list", TenantHeaderKey, "ACME", "", ""},
	}
	for _, tt := range tests {
		var gotTenantId, gotDefaultTenantId, gotHeaderTenantId string
		router := mux.NewRouter()
		router.Use(tenantMiddleWare)
		storeRouter := router.PathPrefix("/store").Subrouter()
		handler := func(w http.ResponseWriter, r *http.Request) {
			gotTenantId = mux.Vars(r)["tenant"]
			gotDefaultTenantId = eru_utils.DefaultTenant(r.Context())
			gotHeaderTenantId = RequestTenantRoute(r)
		}
		storeRouter.Path("/{project}/{tenant}/func/list").HandlerFunc(handler)
		storeRouter.Path("/{project}/func/list").HandlerFunc(handler)

		req := httptest.NewRequest(http.MethodGet, tt.path, nil)
		if tt.header != "" {
			req.Header.Set(tt.headerKey, tt.header)
		}
		router.ServeHTTP(httptest.NewRecorder(), req)

		if gotTenantId != tt.tenantId {
			t.Errorf("%s: tenant var = %q, want %q", tt.name, gotTenantId, tt.tenantId)
		}
		if gotDefaultTenantId != tt.defaultTenantId {
			t.Errorf("%s: default tenant = %q, want %q", tt.name, gotDefaultTenantId, tt.defaultTenantId)
		}
		// the header has to reach the handler exactly as the caller sent it, because the
		// gateway forwards this same request on to the service that will use it.
		if gotHeaderTenantId != tt.header {
			t.Errorf("%s: tenant header = %q, want it left as %q", tt.name, gotHeaderTenantId, tt.header)
		}
	}
}

func TestRequestTenant(t *testing.T) {
	tests := []struct {
		name            string
		headers         map[string]string
		tenantId        string
		defaultTenantId string
	}{
		{"canonical header", map[string]string{TenantHeaderKey: "Global___ACME"}, "acme", "global"},
		{"any casing", map[string]string{"x-tenant-id": "global___acme"}, "acme", "global"},
		{"legacy header", map[string]string{"tenant_id": "global___acme"}, "acme", "global"},
		{"canonical wins over legacy", map[string]string{TenantHeaderKey: "acme", "tenant_id": "globex"}, "acme", ""},
		{"no header", map[string]string{}, "", ""},
	}
	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		for k, v := range tt.headers {
			req.Header.Set(k, v)
		}
		tenantId, defaultTenantId := RequestTenant(req)
		if tenantId != tt.tenantId || defaultTenantId != tt.defaultTenantId {
			t.Errorf("%s: RequestTenant() = (%q, %q), want (%q, %q)", tt.name, tenantId, defaultTenantId, tt.tenantId, tt.defaultTenantId)
		}
	}
}

func TestRequestProject(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("x-project-id", "myproj")
	if got := RequestProject(req); got != "myproj" {
		t.Errorf("RequestProject() = %q, want myproj", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("project_id", "myproj")
	if got := RequestProject(req); got != "myproj" {
		t.Errorf("RequestProject() from the legacy header = %q, want myproj", got)
	}
}
