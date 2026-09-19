package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthChallenge(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Host = "eruai.app.smartvalues.co.in"
	req.Header.Set("X-Forwarded-Proto", "https")

	want := `Bearer resource_metadata="https://eruai.app.smartvalues.co.in/.well-known/oauth-protected-resource"`
	if got := authChallenge(req, ""); got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	if got := authChallenge(req, "invalid_token"); got != want+`, error="invalid_token"` {
		t.Errorf("unexpected challenge with error code: %s", got)
	}
}

func TestRespondUnauthorizedSetsChallenge(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Host = "eruai.app.smartvalues.co.in"
	req.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()

	respondUnauthorized(w, req, "", "Unauthorized Request")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
	if w.Header().Get("WWW-Authenticate") == "" {
		t.Error("expected WWW-Authenticate header on 401")
	}
}
