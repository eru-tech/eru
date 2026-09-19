package auth

import (
	"context"
	"strings"
	"testing"
)

func claudePolicy() OAuthClientPolicy {
	return OAuthClientPolicy{
		AllowDynamicRegistration: true,
		AllowedRedirectHosts:     []string{"claude.ai"},
		AllowedScopes:            []string{"openid", "offline_access"},
		PublicClientsOnly:        true,
	}
}

func TestApplyPolicyAcceptsClaudeCallback(t *testing.T) {
	client, err := claudePolicy().ApplyPolicy(context.Background(), OAuthClient{
		ClientName:   "Claude",
		RedirectURIs: []string{"https://claude.ai/api/mcp/auth_callback"},
	})
	if err != nil {
		t.Fatalf("expected the claude callback to be accepted, got %v", err)
	}
	if client.TokenEndpointAuthMethod != "none" {
		t.Errorf("a public client must register as none, got %s", client.TokenEndpointAuthMethod)
	}
	if len(client.GrantTypes) != 2 || client.GrantTypes[0] != "authorization_code" {
		t.Errorf("unexpected default grant types %v", client.GrantTypes)
	}
	if client.Scope != "openid offline_access" {
		t.Errorf("unexpected default scope %q", client.Scope)
	}
}

func TestApplyPolicyRejectsRedirectUris(t *testing.T) {
	cases := map[string]string{
		"unlisted host": "https://evil.example/cb",
		"plain http":    "http://claude.ai/cb",
		"fragment":      "https://claude.ai/cb#x",
		"relative":      "/cb",
	}
	for name, redirectUri := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := claudePolicy().ApplyPolicy(context.Background(), OAuthClient{RedirectURIs: []string{redirectUri}})
			if err == nil {
				t.Fatalf("expected %s to be rejected", redirectUri)
			}
			if oAuthError, ok := err.(OAuthError); !ok || oAuthError.ErrorCode != "invalid_redirect_uri" {
				t.Errorf("expected invalid_redirect_uri, got %v", err)
			}
		})
	}
}

func TestApplyPolicyFailsClosedWithNoAllowedHosts(t *testing.T) {
	_, err := OAuthClientPolicy{AllowDynamicRegistration: true}.ApplyPolicy(context.Background(),
		OAuthClient{RedirectURIs: []string{"https://claude.ai/api/mcp/auth_callback"}})
	if err == nil {
		t.Fatal("an empty allowed host list must reject every host, not allow every host")
	}
}

func TestApplyPolicyAllowsLoopbackForNativeClients(t *testing.T) {
	if _, err := claudePolicy().ApplyPolicy(context.Background(),
		OAuthClient{RedirectURIs: []string{"http://127.0.0.1:8976/callback"}}); err != nil {
		t.Errorf("loopback redirect must be allowed over http, got %v", err)
	}
}

func TestApplyPolicyRejectsUnlistedScopeAndGrant(t *testing.T) {
	if _, err := claudePolicy().ApplyPolicy(context.Background(), OAuthClient{
		RedirectURIs: []string{"https://claude.ai/cb"}, Scope: "openid admin",
	}); err == nil {
		t.Error("expected an unlisted scope to be rejected")
	}
	if _, err := claudePolicy().ApplyPolicy(context.Background(), OAuthClient{
		RedirectURIs: []string{"https://claude.ai/cb"}, GrantTypes: []string{"implicit"},
	}); err == nil {
		t.Error("expected an unlisted grant type to be rejected")
	}
}

func TestApplyPolicyStripsClientSecretForPublicClients(t *testing.T) {
	client, err := claudePolicy().ApplyPolicy(context.Background(), OAuthClient{
		RedirectURIs:            []string{"https://claude.ai/cb"},
		ClientSecret:            "smuggled",
		TokenEndpointAuthMethod: "client_secret_post",
	})
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if client.ClientSecret != "" || client.TokenEndpointAuthMethod != "none" {
		t.Error("a public client must not keep a secret or a secret based auth method")
	}
}

func testAuth() *Auth {
	return &Auth{
		AuthName: "smartvalues",
		Hydra:    HydraConfig{PublicScheme: "https", PublicHost: "hydrapublic.app.smartvalues.co.in"},
		OAuthServerConfig: OAuthServerConfig{
			Enabled:      true,
			ClientPolicy: claudePolicy(),
		},
	}
}

func TestAuthorizationServerMetadata(t *testing.T) {
	metadata, err := testAuth().AuthorizationServerMetadata(context.Background())
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if metadata.Issuer != "https://hydrapublic.app.smartvalues.co.in" {
		t.Errorf("issuer must fall back to the hydra public url, got %s", metadata.Issuer)
	}
	if metadata.TokenEndpoint != "https://hydrapublic.app.smartvalues.co.in/oauth2/token" {
		t.Errorf("unexpected token endpoint %s", metadata.TokenEndpoint)
	}
	if metadata.RegistrationEndpoint != "https://hydrapublic.app.smartvalues.co.in/oauth2/register" {
		t.Errorf("registration must be advertised on the issuer, got %s", metadata.RegistrationEndpoint)
	}
	if len(metadata.CodeChallengeMethodsSupported) != 1 || metadata.CodeChallengeMethodsSupported[0] != "S256" {
		t.Errorf("plain pkce must not be advertised, got %v", metadata.CodeChallengeMethodsSupported)
	}
	if strings.Join(metadata.TokenEndpointAuthMethodsSupported, ",") != "none" {
		t.Errorf("a public-clients-only policy must advertise none alone, got %v", metadata.TokenEndpointAuthMethodsSupported)
	}
}

func TestAuthorizationServerMetadataHonoursConfiguredIssuer(t *testing.T) {
	authObj := testAuth()
	authObj.OAuthServerConfig.Issuer = "https://auth.smartvalues.co.in/"
	metadata, err := authObj.AuthorizationServerMetadata(context.Background())
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if metadata.Issuer != "https://auth.smartvalues.co.in" {
		t.Errorf("trailing slash must be trimmed, got %s", metadata.Issuer)
	}
	if metadata.AuthorizationEndpoint != "https://hydrapublic.app.smartvalues.co.in/oauth2/auth" {
		t.Errorf("grant endpoints stay on hydra while it is the backend, got %s", metadata.AuthorizationEndpoint)
	}
}

func TestAuthorizationServerMetadataDisabled(t *testing.T) {
	authObj := testAuth()
	authObj.OAuthServerConfig.Enabled = false
	if _, err := authObj.AuthorizationServerMetadata(context.Background()); err == nil {
		t.Error("expected an error when the oauth server is not enabled")
	}
}

func TestRegistrationEndpointHiddenWhenDcrOff(t *testing.T) {
	authObj := testAuth()
	authObj.OAuthServerConfig.ClientPolicy.AllowDynamicRegistration = false
	metadata, _ := authObj.AuthorizationServerMetadata(context.Background())
	if metadata.RegistrationEndpoint != "" {
		t.Errorf("registration must not be advertised when it is off, got %s", metadata.RegistrationEndpoint)
	}
}
