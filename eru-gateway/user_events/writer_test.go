package user_events

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type plainWriter struct {
	header http.Header
	status int
	body   strings.Builder
}

func (p *plainWriter) Header() http.Header {
	if p.header == nil {
		p.header = http.Header{}
	}
	return p.header
}
func (p *plainWriter) Write(b []byte) (int, error) { return p.body.Write(b) }
func (p *plainWriter) WriteHeader(code int)        { p.status = code }

func TestRecorderDefaultsToOkOnWrite(t *testing.T) {
	rec := NewRecorder(httptest.NewRecorder())
	if _, err := rec.Write([]byte("body")); err != nil {
		t.Fatal(err)
	}
	if rec.Status() != http.StatusOK {
		t.Errorf("expected 200 for an implicit header, got %d", rec.Status())
	}
}

func TestRecorderCapturesExplicitStatus(t *testing.T) {
	inner := httptest.NewRecorder()
	rec := NewRecorder(inner)
	rec.WriteHeader(http.StatusUnauthorized)
	if rec.Status() != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Status())
	}
	if inner.Code != http.StatusUnauthorized {
		t.Errorf("expected the status to reach the inner writer, got %d", inner.Code)
	}
}

func TestRecorderKeepsFirstStatus(t *testing.T) {
	rec := NewRecorder(httptest.NewRecorder())
	rec.WriteHeader(http.StatusBadGateway)
	rec.WriteHeader(http.StatusOK)
	if rec.Status() != http.StatusBadGateway {
		t.Errorf("expected the first status to win, got %d", rec.Status())
	}
}

func TestRecorderPreservesFlusher(t *testing.T) {
	inner := httptest.NewRecorder()
	var w http.ResponseWriter = NewRecorder(inner)

	flusher, ok := w.(http.Flusher)
	if !ok {
		t.Fatal("recorder must implement http.Flusher or SSE proxying breaks")
	}
	flusher.Flush()
	if !inner.Flushed {
		t.Error("expected Flush to reach the inner writer")
	}
}

func TestRecorderFlushIsSafeWithoutInnerFlusher(t *testing.T) {
	var w http.ResponseWriter = NewRecorder(&plainWriter{})
	flusher, ok := w.(http.Flusher)
	if !ok {
		t.Fatal("expected the recorder to implement http.Flusher")
	}
	flusher.Flush()
}

func TestRecorderHijackErrorsWithoutInnerHijacker(t *testing.T) {
	var w http.ResponseWriter = NewRecorder(httptest.NewRecorder())
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		t.Fatal("expected the recorder to implement http.Hijacker")
	}
	if _, _, err := hijacker.Hijack(); err == nil {
		t.Error("expected an error when the inner writer cannot hijack")
	}
}

func TestRecorderPassesThroughHeaders(t *testing.T) {
	inner := httptest.NewRecorder()
	rec := NewRecorder(inner)
	rec.Header().Set("Content-Type", "text/event-stream")
	if inner.Header().Get("Content-Type") != "text/event-stream" {
		t.Error("expected header writes to reach the inner writer")
	}
}

func TestBuildInsertShape(t *testing.T) {
	now := time.Now()
	events := []UserEvent{
		{RequestId: "r1", Host: "a.example.com", Path: "/one", Method: "GET", Status: 200, DurationMs: 5, RequestTime: now},
		{RequestId: "r2", Host: "b.example.com", Path: "/two", Method: "POST", Status: 502, DurationMs: 9, RequestTime: now,
			Headers: map[string]string{"user-agent": "curl/8.0"}},
	}

	query, vals := buildInsert("erugateway_user_events", events)

	if len(vals) != len(events)*eventColumnCount {
		t.Fatalf("expected %d bind values, got %d", len(events)*eventColumnCount, len(vals))
	}
	if strings.Count(query, "(") != len(events)+1 {
		t.Errorf("expected one value tuple per event, query was %q", query)
	}
	if !strings.Contains(query, "$11::jsonb") || !strings.Contains(query, "$23::jsonb") {
		t.Errorf("expected the headers column to be cast to jsonb, query was %q", query)
	}
	if !strings.HasPrefix(query, "insert into erugateway_user_events (") {
		t.Errorf("unexpected query prefix: %q", query)
	}
	if vals[10] != "{}" {
		t.Errorf("expected empty headers to serialise as {}, got %v", vals[10])
	}
	if vals[22] != `{"user-agent":"curl/8.0"}` {
		t.Errorf("expected headers json, got %v", vals[22])
	}
}

func TestBuildInsertNullsEmptyStrings(t *testing.T) {
	query, vals := buildInsert("t", []UserEvent{{Status: 200, RequestTime: time.Now()}})
	if query == "" {
		t.Fatal("expected a query")
	}
	if vals[0] != nil {
		t.Errorf("expected an empty request_id to bind as NULL, got %v", vals[0])
	}
	if vals[6] != 200 {
		t.Errorf("expected status 200, got %v", vals[6])
	}
}
