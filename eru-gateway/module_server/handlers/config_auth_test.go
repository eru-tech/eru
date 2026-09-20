package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
}

func TestConfigApiKeyOpenWhenEnvUnset(t *testing.T) {
	t.Setenv(ConfigApiKeyEnvKey, "")

	w := httptest.NewRecorder()
	ConfigApiKeyMiddleware(okHandler()).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/store/config", nil))

	if w.Code != http.StatusOK {
		t.Errorf("with no key configured the routes must behave exactly as before, got %d", w.Code)
	}
}

func TestConfigApiKeyRejectsMissingAndWrong(t *testing.T) {
	t.Setenv(ConfigApiKeyEnvKey, "s3cret")

	for name, key := range map[string]string{"missing": "", "wrong": "nope", "prefix": "s3cre", "longer": "s3cret1"} {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/store/config", nil)
			if key != "" {
				r.Header.Set(ConfigApiKeyHeaderKey, key)
			}
			w := httptest.NewRecorder()
			ConfigApiKeyMiddleware(okHandler()).ServeHTTP(w, r)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("expected 401, got %d", w.Code)
			}
			if strings.Contains(w.Body.String(), "s3cret") {
				t.Error("the response must not echo the expected key")
			}
		})
	}
}

func TestConfigApiKeyAcceptsCorrect(t *testing.T) {
	t.Setenv(ConfigApiKeyEnvKey, "s3cret")

	r := httptest.NewRequest(http.MethodGet, "/store/config", nil)
	r.Header.Set(ConfigApiKeyHeaderKey, "s3cret")
	w := httptest.NewRecorder()
	ConfigApiKeyMiddleware(okHandler()).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected the request to pass, got %d %s", w.Code, w.Body.String())
	}
}

// The forwarding route must stay reachable without the config key - it is authorised by the
// listener rule's authorizer, not by this middleware.
func TestProxyRouteIsNotGuarded(t *testing.T) {
	t.Setenv(ConfigApiKeyEnvKey, "s3cret")

	router := mux.NewRouter()
	storeRouter := router.PathPrefix("/store").Subrouter()
	storeRouter.Use(ConfigApiKeyMiddleware)
	storeRouter.Methods(http.MethodGet).Path("/config").Handler(okHandler())

	proxied := false
	router.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxied = true
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/mcp", nil))

	if !proxied || w.Code != http.StatusOK {
		t.Errorf("the proxy route must not require the config key, got %d proxied=%v", w.Code, proxied)
	}

	// And the guarded route on the same router still rejects.
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/store/config", nil))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected the config route to still be guarded, got %d", w.Code)
	}
}

// A /store path with no matching config route falls through to the proxy, as it did before.
func TestUnmatchedStorePathStillFallsThrough(t *testing.T) {
	t.Setenv(ConfigApiKeyEnvKey, "s3cret")

	router := mux.NewRouter()
	storeRouter := router.PathPrefix("/store").Subrouter()
	storeRouter.Use(ConfigApiKeyMiddleware)
	storeRouter.Methods(http.MethodGet).Path("/config").Handler(okHandler())

	proxied := false
	router.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxied = true
		w.WriteHeader(http.StatusOK)
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/store/smartvalues/variables/list", nil))

	if !proxied {
		t.Error("a /store path the gateway does not own must still be forwarded")
	}
}
