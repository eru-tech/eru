package auth

import (
	"context"
	"testing"
)

func TestGrantableScopeNarrowsToPolicy(t *testing.T) {
	oAuthServer := OAuthServerConfig{ClientPolicy: OAuthClientPolicy{AllowedScopes: []string{"openid", "offline_access"}}}

	granted := oAuthServer.GrantableScope([]string{"openid", "admin", "offline_access"})

	if len(granted) != 2 || granted[0] != "openid" || granted[1] != "offline_access" {
		t.Errorf("a client must not widen its grant by asking, got %v", granted)
	}
}

func TestGrantableScopeEmptyWhenNothingMatches(t *testing.T) {
	oAuthServer := OAuthServerConfig{ClientPolicy: OAuthClientPolicy{AllowedScopes: []string{"openid"}}}
	if granted := oAuthServer.GrantableScope([]string{"admin"}); len(granted) != 0 {
		t.Errorf("expected nothing to be grantable, got %v", granted)
	}
}

func TestAuthorizationFlowResolvesHydra(t *testing.T) {
	authObj := &Auth{Hydra: HydraConfig{PublicScheme: "https", PublicHost: "hydra.example"}}
	flow, err := authObj.AuthorizationFlow(context.Background())
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if _, ok := flow.(HydraAuthorizationFlow); !ok {
		t.Errorf("expected the hydra flow by default, got %T", flow)
	}
}

func TestAuthorizationFlowRejectsUnknownBackend(t *testing.T) {
	authObj := &Auth{OAuthServerConfig: OAuthServerConfig{Backend: "SOMETHING_ELSE"}}
	if _, err := authObj.AuthorizationFlow(context.Background()); err == nil {
		t.Error("expected an unknown backend to be rejected")
	}
}

func TestAcceptLoginRequiresSubject(t *testing.T) {
	flow := HydraAuthorizationFlow{Hydra: HydraConfig{PublicScheme: "https", PublicHost: "hydra.example"}}
	if _, err := flow.AcceptLogin(context.Background(), "challenge", "", true); err == nil {
		t.Error("accepting a login with no subject must fail before it reaches the authorization server")
	}
}

func TestFlowRequiresChallenge(t *testing.T) {
	flow := HydraAuthorizationFlow{Hydra: HydraConfig{PublicScheme: "https", PublicHost: "hydra.example"}}
	if _, err := flow.LoginRequest(context.Background(), ""); err == nil {
		t.Error("expected a missing login_challenge to fail")
	}
	if _, err := flow.ConsentRequest(context.Background(), ""); err == nil {
		t.Error("expected a missing consent_challenge to fail")
	}
}
