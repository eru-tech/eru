package module_server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/eru-tech/eru/eru-functions/module_store"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/gorilla/mux"
)

func TestMain(m *testing.M) {
	logs.LogInit("test", "eru-functions-routes-test")
	os.Exit(m.Run())
}

// the schedule routes have to be registered before /{project}/{tenant}/func/{funcname},
// which would otherwise swallow /{project}/schedule/func/{funcname} with tenant="schedule".
func TestScheduleRouteMatching(t *testing.T) {
	router := mux.NewRouter()
	AddModuleRoutes(router, &module_store.StoreHolder{})

	tests := []struct {
		name     string
		method   string
		path     string
		wantVars map[string]string
	}{
		{"schedule for a tenant", http.MethodPost, "/myproj/acme/schedule/func/my_func",
			map[string]string{"project": "myproj", "tenant": "acme", "funcname": "my_func"}},
		{"schedule for a default tenant route", http.MethodPost, "/myproj/global___acme/schedule/func/my_func",
			map[string]string{"project": "myproj", "tenant": "global___acme", "funcname": "my_func"}},
		{"schedule at project level", http.MethodPost, "/myproj/schedule/func/my_func",
			map[string]string{"project": "myproj", "funcname": "my_func"}},
		{"unschedule for a tenant", http.MethodDelete, "/myproj/acme/unschedule/func/job-1",
			map[string]string{"project": "myproj", "tenant": "acme", "jobid": "job-1"}},
		{"unschedule at project level", http.MethodDelete, "/myproj/unschedule/func/job-1",
			map[string]string{"project": "myproj", "jobid": "job-1"}},
		{"a plain tenant func call still matches the func route", http.MethodPost, "/myproj/acme/func/my_func",
			map[string]string{"project": "myproj", "tenant": "acme", "funcname": "my_func"}},
	}

	for _, tt := range tests {
		var match mux.RouteMatch
		req := httptest.NewRequest(tt.method, tt.path, nil)
		if !router.Match(req, &match) {
			t.Errorf("%s: %s did not match any route", tt.name, tt.path)
			continue
		}
		for k, want := range tt.wantVars {
			if got := match.Vars[k]; got != want {
				t.Errorf("%s: %s var %q = %q, want %q (vars %v)", tt.name, tt.path, k, got, want, match.Vars)
			}
		}
		if _, ok := tt.wantVars["tenant"]; !ok {
			if got, present := match.Vars["tenant"]; present && got != "" {
				t.Errorf("%s: %s bound tenant=%q, expected no tenant", tt.name, tt.path, got)
			}
		}
	}
}
