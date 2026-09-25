package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
)

func publicClient() OAuthClient {
	return OAuthClient{
		ClientId:                "cid",
		RedirectURIs:            []string{"https://claude.ai/api/mcp/auth_callback"},
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		ResponseTypes:           []string{"code"},
		Scope:                   "openid offline_access",
		TokenEndpointAuthMethod: "none",
	}
}

func authorizeParams(overrides map[string]string) url.Values {
	params := url.Values{}
	params.Set("client_id", "cid")
	params.Set("response_type", "code")
	params.Set("redirect_uri", "https://claude.ai/api/mcp/auth_callback")
	params.Set("scope", "openid")
	params.Set("code_challenge", "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM")
	params.Set("code_challenge_method", "S256")
	for k, v := range overrides {
		if v == "" {
			params.Del(k)
		} else {
			params.Set(k, v)
		}
	}
	return params
}

func TestValidateAuthorizationRequestAccepts(t *testing.T) {
	authRequest, err := ValidateAuthorizationRequest(context.Background(), authorizeParams(nil), publicClient(), claudePolicy())
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if authRequest.RequestedScope != "openid" || authRequest.CodeChallengeMethod != "S256" {
		t.Errorf("unexpected request %+v", authRequest)
	}
}

func TestValidateRejectsUnregisteredRedirectUri(t *testing.T) {
	_, err := ValidateAuthorizationRequest(context.Background(),
		authorizeParams(map[string]string{"redirect_uri": "https://claude.ai/evil"}), publicClient(), claudePolicy())
	if err == nil {
		t.Fatal("an unregistered redirect_uri must be rejected")
	}
	if oAuthError, ok := err.(OAuthError); !ok || oAuthError.ErrorCode != "invalid_request" {
		t.Errorf("unexpected error %v", err)
	}
}

func TestValidateDefaultsRedirectUriOnlyWhenUnambiguous(t *testing.T) {
	authRequest, err := ValidateAuthorizationRequest(context.Background(),
		authorizeParams(map[string]string{"redirect_uri": ""}), publicClient(), claudePolicy())
	if err != nil || authRequest.RedirectUri != "https://claude.ai/api/mcp/auth_callback" {
		t.Errorf("a single registered uri should be used, got %s %v", authRequest.RedirectUri, err)
	}

	twoUris := publicClient()
	twoUris.RedirectURIs = append(twoUris.RedirectURIs, "https://claude.ai/other")
	if _, err = ValidateAuthorizationRequest(context.Background(),
		authorizeParams(map[string]string{"redirect_uri": ""}), twoUris, claudePolicy()); err == nil {
		t.Error("with two registered uris the parameter must be required")
	}
}

func TestValidateRequiresPkceForPublicClient(t *testing.T) {
	if _, err := ValidateAuthorizationRequest(context.Background(),
		authorizeParams(map[string]string{"code_challenge": ""}), publicClient(), claudePolicy()); err == nil {
		t.Error("a public client must not be able to skip pkce")
	}
	if _, err := ValidateAuthorizationRequest(context.Background(),
		authorizeParams(map[string]string{"code_challenge_method": "plain"}), publicClient(), claudePolicy()); err == nil {
		t.Error("plain pkce must be rejected")
	}
}

func TestValidateRejectsUnsupportedResponseTypeAndScope(t *testing.T) {
	if _, err := ValidateAuthorizationRequest(context.Background(),
		authorizeParams(map[string]string{"response_type": "token"}), publicClient(), claudePolicy()); err == nil {
		t.Error("implicit response types must be rejected")
	}
	if _, err := ValidateAuthorizationRequest(context.Background(),
		authorizeParams(map[string]string{"scope": "openid admin"}), publicClient(), claudePolicy()); err == nil {
		t.Error("a scope the client did not register must be rejected")
	}
}

func TestVerifyCodeChallenge(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	if !VerifyCodeChallenge(challenge, "S256", verifier) {
		t.Error("the matching verifier must pass")
	}
	if VerifyCodeChallenge(challenge, "S256", "wrong-verifier") {
		t.Error("a wrong verifier must fail")
	}
	if VerifyCodeChallenge(challenge, "plain", verifier) {
		t.Error("plain must not be accepted even if the verifier matches the challenge text")
	}
	if VerifyCodeChallenge(challenge, "S256", "") {
		t.Error("a missing verifier must fail when a challenge was set")
	}
}

func TestRedirectUrlsCarryStateAndEscape(t *testing.T) {
	got := CodeRedirectUrl("https://claude.ai/cb", "st ate", "the code")
	if !strings.Contains(got, "code=the+code") || !strings.Contains(got, "state=st+ate") {
		t.Errorf("unexpected redirect %s", got)
	}

	got = CodeRedirectUrl("https://claude.ai/cb?x=1", "s", "c")
	if !strings.Contains(got, "?x=1&") {
		t.Errorf("an existing query must be preserved, got %s", got)
	}

	got = ErrorRedirectUrl("https://claude.ai/cb", "s", "access_denied", "the user denied the request")
	if !strings.Contains(got, "error=access_denied") || !strings.Contains(got, "state=s") {
		t.Errorf("unexpected error redirect %s", got)
	}
}

func TestRandomTokensAreDistinctAndHashed(t *testing.T) {
	first, err := randomToken()
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	second, _ := randomToken()
	if first == second || len(first) < 40 {
		t.Errorf("tokens must be long and distinct, got %q and %q", first, second)
	}
	if hashToken(first) == first {
		t.Error("the stored value must be a digest, not the token itself")
	}
}

func eruServerConfig() OAuthServerConfig {
	return OAuthServerConfig{
		Enabled: true, Backend: OAuthBackendEru,
		Issuer: "https://apiai.dev.smartvalues.co.in", SigningKid: "smartvalues",
		ClientPolicy: claudePolicy(),
	}
}

func TestTokenLifespanDefaults(t *testing.T) {
	config := eruServerConfig()
	if config.accessTokenLifespan() != 3600 || config.idTokenLifespan() != 3600 {
		t.Error("unexpected default token lifespans")
	}
	if config.refreshTokenLifespan() != 30*24*3600 {
		t.Error("unexpected default refresh lifespan")
	}
	config.AccessTokenLifespan = 60
	if config.accessTokenLifespan() != 60 {
		t.Error("a configured lifespan must win")
	}
}

// With the eru backend the metadata has to advertise our endpoints, not hydra's.
func TestMetadataAdvertisesOwnEndpointsForEruBackend(t *testing.T) {
	authObj := &Auth{AuthName: "eru-oauth", OAuthServerConfig: eruServerConfig()}
	metadata, err := authObj.AuthorizationServerMetadata(context.Background())
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if metadata.TokenEndpoint != "https://apiai.dev.smartvalues.co.in/oauth2/token" {
		t.Errorf("token endpoint must be ours, got %s", metadata.TokenEndpoint)
	}
	if metadata.AuthorizationEndpoint != "https://apiai.dev.smartvalues.co.in/oauth2/auth" {
		t.Errorf("authorization endpoint must be ours, got %s", metadata.AuthorizationEndpoint)
	}
}

// And with the hydra backend it must keep pointing at hydra.
func TestMetadataStillPointsAtHydraByDefault(t *testing.T) {
	authObj := testAuth()
	metadata, err := authObj.AuthorizationServerMetadata(context.Background())
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if !strings.HasPrefix(metadata.TokenEndpoint, "https://hydrapublic.app.smartvalues.co.in") {
		t.Errorf("hydra backend must keep hydra's endpoints, got %s", metadata.TokenEndpoint)
	}
}

func TestTokenServiceRejectsMissingInputs(t *testing.T) {
	service := TokenService{Flow: EruAuthorizationFlow{ProjectId: "p", AuthName: "a"}, Config: eruServerConfig()}
	if _, err := service.ExchangeCode(context.Background(), "cid", "", "", "v"); err == nil {
		t.Error("an empty code must be rejected before touching the store")
	}
	if _, err := service.RefreshToken(context.Background(), "cid", ""); err == nil {
		t.Error("an empty refresh token must be rejected before touching the store")
	}
}

func TestSigningRequiresASigner(t *testing.T) {
	service := TokenService{Flow: EruAuthorizationFlow{ProjectId: "p"}, Config: eruServerConfig()}
	if _, err := service.signToken(context.Background(), map[string]interface{}{"sub": "x"}); err == nil {
		t.Error("issuing a token with no signer configured must fail")
	}
}

func TestSessionLifespanDefault(t *testing.T) {
	config := eruServerConfig()
	if config.SessionLifespanSeconds() != 12*3600 {
		t.Errorf("unexpected default session lifespan %d", config.SessionLifespanSeconds())
	}
	config.SessionLifespan = 300
	if config.SessionLifespanSeconds() != 300 {
		t.Error("a configured session lifespan must win")
	}
}

// A missing session is the ordinary case and must read as "nobody", not as an error.
func TestSessionIdentityWithoutSessionOrConnection(t *testing.T) {
	flow := EruAuthorizationFlow{ProjectId: "p", AuthName: "a"}
	if got := flow.SessionIdentity(context.Background(), ""); got != "" {
		t.Errorf("an empty session id must resolve to nobody, got %q", got)
	}
	if got := flow.SessionIdentity(context.Background(), "some-session"); got != "" {
		t.Errorf("with no connection the answer must be nobody, got %q", got)
	}
}

func TestCreateSessionRequiresIdentityAndConnection(t *testing.T) {
	flow := EruAuthorizationFlow{ProjectId: "p", AuthName: "a"}
	if _, err := flow.CreateSession(context.Background(), "identity", 60); err == nil {
		t.Error("expected creating a session with no connection to fail")
	}
}

func TestRevokeSessionIsQuietWhenThereIsNothingToRevoke(t *testing.T) {
	flow := EruAuthorizationFlow{ProjectId: "p", AuthName: "a"}
	if err := flow.RevokeSession(context.Background(), ""); err != nil {
		t.Errorf("revoking nothing must succeed quietly, got %v", err)
	}
}

// Remembered consent must never widen a grant: a request for a scope that was not previously
// granted has to be shown to the user again.
func TestConsentCoversWithoutConnection(t *testing.T) {
	flow := EruAuthorizationFlow{ProjectId: "p", AuthName: "a"}
	if flow.ConsentCovers(context.Background(), "cid", "identity", []string{"openid"}) {
		t.Error("with no connection nothing can be known to be covered")
	}
	if flow.ConsentCovers(context.Background(), "", "identity", []string{"openid"}) {
		t.Error("a missing client must not be treated as covered")
	}
	if flow.ConsentCovers(context.Background(), "cid", "", []string{"openid"}) {
		t.Error("a missing identity must not be treated as covered")
	}
}

func TestRememberConsentRequiresConnection(t *testing.T) {
	flow := EruAuthorizationFlow{ProjectId: "p", AuthName: "a"}
	if err := flow.RememberConsent(context.Background(), "cid", "identity", []string{"openid"}); err == nil {
		t.Error("expected remembering consent with no connection to fail")
	}
}
