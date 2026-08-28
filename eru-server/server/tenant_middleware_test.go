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
		header          string
		tenantId        string
		defaultTenantId string
	}{
		{"tenant only", "/store/myproj/ACME/func/list", "", "acme", ""},
		{"default and tenant", "/store/myproj/Global___ACME/func/list", "", "acme", "global"},
		{"header tenant", "/store/myproj/func/list", "Global___ACME", "", "global"},
	}
	for _, tt := range tests {
		var gotTenantId, gotDefaultTenantId, gotHeaderTenantId string
		router := mux.NewRouter()
		router.Use(tenantMiddleWare)
		storeRouter := router.PathPrefix("/store").Subrouter()
		handler := func(w http.ResponseWriter, r *http.Request) {
			gotTenantId = mux.Vars(r)["tenant"]
			gotDefaultTenantId = eru_utils.DefaultTenant(r.Context())
			gotHeaderTenantId = r.Header.Get("tenant_id")
		}
		storeRouter.Path("/{project}/{tenant}/func/list").HandlerFunc(handler)
		storeRouter.Path("/{project}/func/list").HandlerFunc(handler)

		req := httptest.NewRequest(http.MethodGet, tt.path, nil)
		if tt.header != "" {
			req.Header.Set("tenant_id", tt.header)
		}
		router.ServeHTTP(httptest.NewRecorder(), req)

		if gotTenantId != tt.tenantId {
			t.Errorf("%s: tenant var = %q, want %q", tt.name, gotTenantId, tt.tenantId)
		}
		if gotDefaultTenantId != tt.defaultTenantId {
			t.Errorf("%s: default tenant = %q, want %q", tt.name, gotDefaultTenantId, tt.defaultTenantId)
		}
		if tt.header != "" && gotHeaderTenantId != "acme" {
			t.Errorf("%s: tenant_id header = %q, want %q", tt.name, gotHeaderTenantId, "acme")
		}
	}
}
