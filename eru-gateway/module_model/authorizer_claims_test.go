package module_model

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestTokenPrefersConfiguredHeader(t *testing.T) {
	authorizer := Authorizer{TokenHeaderKey: "x-access-token"}
	r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	r.Header.Set("x-access-token", "from-header")
	r.Header.Set("Authorization", "Bearer from-bearer")

	if got := authorizer.RequestToken(r); got != "from-header" {
		t.Errorf("the configured header must win so existing deployments are unaffected, got %q", got)
	}
}

func TestRequestTokenFallsBackToBearer(t *testing.T) {
	authorizer := Authorizer{TokenHeaderKey: "x-access-token"}
	r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	r.Header.Set("Authorization", "Bearer from-bearer")

	if got := authorizer.RequestToken(r); got != "from-bearer" {
		t.Errorf("expected the bearer token, got %q", got)
	}
}

func TestBearerTokenParsing(t *testing.T) {
	cases := map[string]string{
		"Bearer abc":  "abc",
		"bearer abc":  "abc",
		"BEARER  abc": "abc",
		"Basic abc":   "",
		"Bearer":      "",
		"":            "",
	}
	for header, want := range cases {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if header != "" {
			r.Header.Set("Authorization", header)
		}
		if got := BearerToken(r); got != want {
			t.Errorf("BearerToken(%q) = %q, want %q", header, got, want)
		}
	}
}

func TestClaimsRestrictionsOffByDefault(t *testing.T) {
	authorizer := Authorizer{Audience: []string{"https://eruai.example/mcp"}, Issuer: []string{"https://auth.example"}}
	claims := map[string]interface{}{"aud": "someone-else", "iss": "https://other.example"}

	if err := authorizer.VerifyClaimsRestrictions(context.Background(), claims); err != nil {
		t.Errorf("audience and issuer must stay unenforced until verify_claims is set, got %v", err)
	}
}

func TestClaimsRestrictionsEnforceAudience(t *testing.T) {
	authorizer := Authorizer{VerifyClaims: true, Audience: []string{"https://eruai.example/mcp"}}

	if err := authorizer.VerifyClaimsRestrictions(context.Background(),
		map[string]interface{}{"aud": []interface{}{"https://other", "https://eruai.example/mcp"}}); err != nil {
		t.Errorf("a matching audience in a list must pass, got %v", err)
	}
	if err := authorizer.VerifyClaimsRestrictions(context.Background(),
		map[string]interface{}{"aud": "https://eruai.example/mcp"}); err != nil {
		t.Errorf("a matching audience as a string must pass, got %v", err)
	}
	if err := authorizer.VerifyClaimsRestrictions(context.Background(),
		map[string]interface{}{"aud": "https://elsewhere"}); err == nil {
		t.Error("a token minted for another resource must be rejected")
	}
	if err := authorizer.VerifyClaimsRestrictions(context.Background(), map[string]interface{}{}); err == nil {
		t.Error("a token with no audience must be rejected once the audience is enforced")
	}
}

func TestClaimsRestrictionsEnforceIssuer(t *testing.T) {
	authorizer := Authorizer{VerifyClaims: true, Issuer: []string{"https://auth.example"}}
	if err := authorizer.VerifyClaimsRestrictions(context.Background(),
		map[string]interface{}{"iss": "https://auth.example"}); err != nil {
		t.Errorf("unexpected error %v", err)
	}
	if err := authorizer.VerifyClaimsRestrictions(context.Background(),
		map[string]interface{}{"iss": "https://evil.example"}); err == nil {
		t.Error("a token from another issuer must be rejected")
	}
}

func TestClaimsRestrictionsEnforceScope(t *testing.T) {
	authorizer := Authorizer{RequiredScope: []string{"openid"}}

	if err := authorizer.VerifyClaimsRestrictions(context.Background(),
		map[string]interface{}{"scope": "openid offline_access"}); err != nil {
		t.Errorf("space delimited scope must be read, got %v", err)
	}
	if err := authorizer.VerifyClaimsRestrictions(context.Background(),
		map[string]interface{}{"scp": []interface{}{"openid"}}); err != nil {
		t.Errorf("scp as a list must be read, got %v", err)
	}
	if err := authorizer.VerifyClaimsRestrictions(context.Background(),
		map[string]interface{}{"scope": "offline_access"}); err == nil {
		t.Error("a token missing the required scope must be rejected")
	}
	if err := authorizer.VerifyClaimsRestrictions(context.Background(), map[string]interface{}{}); err == nil {
		t.Error("a token with no scope must be rejected when a scope is required")
	}
}

func TestRequiredScopeEnforcedWithoutVerifyClaims(t *testing.T) {
	authorizer := Authorizer{RequiredScope: []string{"openid"}}
	if err := authorizer.VerifyClaimsRestrictions(context.Background(),
		map[string]interface{}{"scope": "other"}); err == nil {
		t.Error("required_scope is new config, so it applies as soon as it is set")
	}
}
