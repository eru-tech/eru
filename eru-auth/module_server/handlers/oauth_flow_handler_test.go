package handlers

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/eru-tech/eru/eru-auth/auth"
)

func brandedServer() auth.OAuthServerConfig {
	return auth.OAuthServerConfig{
		Enabled: true,
		Ui: auth.OAuthServerUi{
			Branding: auth.OAuthServerBranding{
				AppName:      "Smartvalues",
				LogoUrl:      "https://cdn.example/logo.png",
				PrimaryColor: "#123456",
				PrivacyUrl:   "https://example/privacy",
			},
		},
	}
}

func TestLoginPageRendersBranding(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, auth.OAuthLoginPath+"?login_challenge=abc", nil)

	renderOAuthPage(w, r, "login.html", oauthPageData{
		Branding:   brandingWithDefaults(brandedServer()),
		Challenge:  "abc",
		CsrfToken:  "tok",
		FormAction: auth.OAuthLoginPath,
		ClientName: "Claude",
	})

	body := w.Body.String()
	for _, want := range []string{"Smartvalues", "https://cdn.example/logo.png", "#123456", "Claude",
		`name="login_challenge" value="abc"`, `name="csrf_token" value="tok"`, "https://example/privacy"} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered login page is missing %q", want)
		}
	}
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("login page must refuse to be framed")
	}
	if !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
		t.Error("login page must not be cached")
	}
}

func TestOAuthPageEscapesInjectedClientName(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, auth.OAuthConsentPath, nil)

	renderOAuthPage(w, r, "consent.html", oauthPageData{
		Branding:   brandingWithDefaults(brandedServer()),
		ClientName: `<script>alert(1)</script>`,
		Scopes:     scopeViews([]string{"openid"}),
	})

	if strings.Contains(w.Body.String(), "<script>alert(1)</script>") {
		t.Error("a self registered client name must not be rendered as markup")
	}
}

func TestConsentPageNamesEveryScope(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, auth.OAuthConsentPath, nil)

	renderOAuthPage(w, r, "consent.html", oauthPageData{
		Branding: brandingWithDefaults(brandedServer()),
		Scopes:   scopeViews([]string{"openid", "offline_access", "custom.scope"}),
	})

	body := w.Body.String()
	for _, want := range []string{"Confirm who you are", "Stay signed in when you are away", "custom.scope"} {
		if !strings.Contains(body, want) {
			t.Errorf("consent screen must name %q", want)
		}
	}
}

func TestBrandingFallsBackToDefaults(t *testing.T) {
	if got := brandingWithDefaults(auth.OAuthServerConfig{}); got.AppName != defaultOAuthAppName {
		t.Errorf("expected a default app name, got %q", got.AppName)
	}
}

func TestCsrfTokenRoundTrip(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, auth.OAuthLoginPath, nil)
	token := issueCsrfToken(w, r)
	if token == "" {
		t.Fatal("expected a csrf token to be issued")
	}

	post := httptest.NewRequest(http.MethodPost, auth.OAuthLoginPath, nil)
	for _, cookie := range w.Result().Cookies() {
		post.AddCookie(cookie)
	}

	if !csrfTokenValid(post, token) {
		t.Error("the issued token must validate against its cookie")
	}
	if csrfTokenValid(post, "another-token") {
		t.Error("a mismatched token must be rejected")
	}
	if csrfTokenValid(httptest.NewRequest(http.MethodPost, "/", nil), token) {
		t.Error("a request with no cookie must be rejected")
	}
}

func TestCsrfCookieIsSecureBehindTls(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, auth.OAuthLoginPath, nil)
	r.Header.Set("X-Forwarded-Proto", "https")
	issueCsrfToken(w, r)

	cookie := w.Result().Cookies()[0]
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("csrf cookie must be secure, httponly and samesite lax, got %+v", cookie)
	}
}

func TestExternalUiUrlKeepsExistingQuery(t *testing.T) {
	if got := externalUiUrl("https://app.example/login", "login_challenge", "abc"); got != "https://app.example/login?login_challenge=abc" {
		t.Errorf("unexpected url %s", got)
	}
	if got := externalUiUrl("https://app.example/login?theme=dark", "login_challenge", "a b"); got != "https://app.example/login?theme=dark&login_challenge=a+b" {
		t.Errorf("unexpected url %s", got)
	}
}

// The rest of eru's login api takes the password base64 encoded, and EruAuth.Login decodes it.
// A browser form cannot do that itself, so the handler must.
func TestFormPasswordIsBase64Encoded(t *testing.T) {
	form := httptest.NewRequest(http.MethodPost, auth.OAuthLoginPath,
		strings.NewReader("login_challenge=abc&username=u&password=p%40ssw0rd%21"))
	form.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	credentials, err := readLoginSubmission(form)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	want := base64.StdEncoding.EncodeToString([]byte("p@ssw0rd!"))
	if credentials.Password != want {
		t.Errorf("form password must be base64 encoded, got %q want %q", credentials.Password, want)
	}
}

// A json body already carries the encoding the login api expects, so it is passed through as is.
func TestJsonPasswordIsPassedThrough(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("p@ssw0rd!"))
	jsonReq := httptest.NewRequest(http.MethodPost, auth.OAuthLoginPath,
		strings.NewReader(`{"login_challenge":"abc","username":"u","password":"`+encoded+`"}`))
	jsonReq.Header.Set("Content-Type", "application/json")

	credentials, err := readLoginSubmission(jsonReq)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if credentials.Password != encoded {
		t.Errorf("json password must not be re-encoded, got %q", credentials.Password)
	}
}

func TestReadLoginSubmissionAcceptsFormAndJson(t *testing.T) {
	form := httptest.NewRequest(http.MethodPost, auth.OAuthLoginPath,
		strings.NewReader("login_challenge=abc&username=u&password=p&csrf_token=t"))
	form.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	credentials, err := readLoginSubmission(form)
	if err != nil || credentials.Username != "u" || credentials.Challenge != "abc" {
		t.Errorf("form submission not read: %+v %v", credentials, err)
	}

	jsonReq := httptest.NewRequest(http.MethodPost, auth.OAuthLoginPath,
		strings.NewReader(`{"login_challenge":"abc","username":"u","password":"p"}`))
	jsonReq.Header.Set("Content-Type", "application/json")
	credentials, err = readLoginSubmission(jsonReq)
	if err != nil || credentials.Username != "u" {
		t.Errorf("json submission not read: %+v %v", credentials, err)
	}

	missing := httptest.NewRequest(http.MethodPost, auth.OAuthLoginPath, strings.NewReader("username=u"))
	missing.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if _, err = readLoginSubmission(missing); err == nil {
		t.Error("expected a missing login_challenge to be rejected")
	}
}

func TestRespondWithRedirect(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, auth.OAuthLoginPath, nil)
	respondWithRedirect(w, r, "https://hydra.example/oauth2/auth?x=1")
	if w.Code != http.StatusFound || w.Header().Get("Location") != "https://hydra.example/oauth2/auth?x=1" {
		t.Errorf("a browser must be redirected, got %d %s", w.Code, w.Header().Get("Location"))
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, auth.OAuthLoginPath, nil)
	r.Header.Set("Accept", "application/json")
	respondWithRedirect(w, r, "https://hydra.example/cb")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "redirect_to") {
		t.Errorf("an external ui must be handed the url, got %d %s", w.Code, w.Body.String())
	}
}
