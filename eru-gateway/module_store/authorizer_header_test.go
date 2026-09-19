package module_store

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eru-tech/eru/eru-gateway/module_model"
	"github.com/eru-tech/eru/eru-server/server"
)

func storeWithAuthorizer(authorizer module_model.Authorizer) *ModuleStore {
	return &ModuleStore{Authorizers: map[string]module_model.Authorizer{authorizer.AuthorizerName: authorizer}}
}

func TestAuthNameHeaderDefaultsToAuthorizerName(t *testing.T) {
	ms := storeWithAuthorizer(module_model.Authorizer{AuthorizerName: "smartvalues"})
	r := httptest.NewRequest(http.MethodGet, "/mcp", nil)

	ms.setAuthNameHeader(context.Background(), r, "smartvalues")

	if got := r.Header.Get(server.AuthNameHeaderKey); got != "smartvalues" {
		t.Errorf("expected the authorizer name to be used as the auth name, got %q", got)
	}
}

func TestAuthNameHeaderUsesExplicitAuthName(t *testing.T) {
	ms := storeWithAuthorizer(module_model.Authorizer{AuthorizerName: "sv_authorizer", AuthName: "smartvalues"})
	r := httptest.NewRequest(http.MethodGet, "/mcp", nil)

	ms.setAuthNameHeader(context.Background(), r, "sv_authorizer")

	if got := r.Header.Get(server.AuthNameHeaderKey); got != "smartvalues" {
		t.Errorf("expected the explicit auth name to win, got %q", got)
	}
}

func TestAuthNameHeaderCannotBeSpoofed(t *testing.T) {
	ms := storeWithAuthorizer(module_model.Authorizer{AuthorizerName: "smartvalues"})
	r := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	r.Header.Set(server.AuthNameHeaderKey, "someone-elses-auth")

	ms.setAuthNameHeader(context.Background(), r, "smartvalues")

	if got := r.Header.Get(server.AuthNameHeaderKey); got != "smartvalues" {
		t.Errorf("a caller supplied auth name must be overwritten, got %q", got)
	}
}

func TestAuthNameHeaderClearedWhenRuleHasNoAuthorizer(t *testing.T) {
	ms := storeWithAuthorizer(module_model.Authorizer{AuthorizerName: "smartvalues"})
	r := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	r.Header.Set(server.AuthNameHeaderKey, "someone-elses-auth")

	ms.setAuthNameHeader(context.Background(), r, "")

	if got := r.Header.Get(server.AuthNameHeaderKey); got != "" {
		t.Errorf("an unguarded rule must not forward an auth name, got %q", got)
	}
}
