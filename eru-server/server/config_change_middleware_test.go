package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/eru-tech/eru/eru-store/store"
)

func TestMiddlewareStampsHeadersWhenConfigChanged(t *testing.T) {
	handlers.ServerName = "eru-ql"
	handlers.InstanceId = "eru-ql-abc"

	h := configChangeMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		store.MarkConfigChanged(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/store/proj/save", nil))

	if got := rec.Header().Get(ConfigUpdatedHeader); got != "eru-ql" {
		t.Errorf("expected %s=eru-ql, got %q", ConfigUpdatedHeader, got)
	}
	if got := rec.Header().Get(ConfigInstanceHeader); got != "eru-ql-abc" {
		t.Errorf("expected %s=eru-ql-abc, got %q", ConfigInstanceHeader, got)
	}
}

func TestMiddlewareOmitsHeadersWhenNothingChanged(t *testing.T) {
	handlers.ServerName = "eru-ql"
	handlers.InstanceId = "eru-ql-abc"

	h := configChangeMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sql/proj/execute", nil))

	if rec.Header().Get(ConfigUpdatedHeader) != "" {
		t.Error("a read-only request must not be flagged as a config change")
	}
}

func TestMiddlewareStampsOnImplicitWriteHeader(t *testing.T) {
	handlers.ServerName = "eru-ql"
	handlers.InstanceId = "eru-ql-abc"

	h := configChangeMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		store.MarkConfigChanged(r.Context())
		_, _ = w.Write([]byte(`{"msg":"saved"}`))
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/store/proj/save", nil))

	if rec.Header().Get(ConfigUpdatedHeader) != "eru-ql" {
		t.Error("expected the header to be stamped when the handler writes without WriteHeader")
	}
}

func TestMiddlewareIgnoresMarkAfterResponseStarted(t *testing.T) {
	handlers.ServerName = "eru-ql"
	handlers.InstanceId = "eru-ql-abc"

	h := configChangeMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		store.MarkConfigChanged(r.Context())
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/store/proj/save", nil))

	if rec.Header().Get(ConfigUpdatedHeader) != "" {
		t.Error("headers cannot be stamped after the response has started")
	}
}

func TestMiddlewarePreservesFlusher(t *testing.T) {
	h := configChangeMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := w.(http.Flusher); !ok {
			t.Error("wrapped writer must still implement http.Flusher")
		}
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}
