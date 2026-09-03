package user_events

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

func init() {
	logs.LogInit("eru-gateway-test", "test")
}

type fakeSink struct {
	mu       sync.Mutex
	batches  [][]UserEvent
	failNext int
}

func (f *fakeSink) Write(ctx context.Context, events []UserEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failNext > 0 {
		f.failNext--
		return errors.New("simulated sink failure")
	}
	batch := make([]UserEvent, len(events))
	copy(batch, events)
	f.batches = append(f.batches, batch)
	return nil
}

func (f *fakeSink) Name() string { return "fake" }

func (f *fakeSink) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	total := 0
	for _, b := range f.batches {
		total += len(b)
	}
	return total
}

func (f *fakeSink) batchSizes() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	sizes := make([]int, 0, len(f.batches))
	for _, b := range f.batches {
		sizes = append(sizes, len(b))
	}
	return sizes
}

func testConfig() Config {
	return Config{
		BatchSize:      200,
		FlushInterval:  25 * time.Millisecond,
		BufferSize:     2000,
		MaxBufferBytes: 4 * 1024 * 1024,
		AllowedHeaders: []string{"user-agent", "content-type"},
		ExcludedPaths:  []string{"/hello", "/registry"},
	}
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met before deadline")
}

func TestNilLoggerIsSafe(t *testing.T) {
	var l *Logger
	if l.Enabled() {
		t.Error("nil logger should not be enabled")
	}
	if l.ShouldLog(httptest.NewRequest(http.MethodGet, "/anything", nil)) {
		t.Error("nil logger should not log")
	}
	l.Log(UserEvent{})
	l.Close(context.Background())
}

func TestShouldLogExcludesPaths(t *testing.T) {
	l := newLogger(&fakeSink{}, testConfig())
	defer l.Close(context.Background())

	cases := map[string]bool{
		"/hello":                     false,
		"/registry/services":         false,
		"/registry/heartbeat/abc":    false,
		"/store/proj/variables/list": true,
		"/api/v1/orders":             true,
	}
	for path, want := range cases {
		got := l.ShouldLog(httptest.NewRequest(http.MethodGet, path, nil))
		if got != want {
			t.Errorf("ShouldLog(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestFlushesOnBatchSize(t *testing.T) {
	sink := &fakeSink{}
	cfg := testConfig()
	cfg.BatchSize = 10
	cfg.FlushInterval = time.Hour
	l := newLogger(sink, cfg)
	defer l.Close(context.Background())

	for i := 0; i < 10; i++ {
		l.Log(UserEvent{Path: "/api", Status: 200, sizeBytes: 100})
	}
	waitFor(t, func() bool { return sink.count() == 10 })

	sizes := sink.batchSizes()
	if len(sizes) != 1 || sizes[0] != 10 {
		t.Errorf("expected a single batch of 10, got %v", sizes)
	}
}

func TestFlushesOnInterval(t *testing.T) {
	sink := &fakeSink{}
	l := newLogger(sink, testConfig())
	defer l.Close(context.Background())

	l.Log(UserEvent{Path: "/api", Status: 200, sizeBytes: 100})
	waitFor(t, func() bool { return sink.count() == 1 })
}

func TestDropsWhenByteCapExceeded(t *testing.T) {
	sink := &fakeSink{}
	cfg := testConfig()
	cfg.BatchSize = 100000
	cfg.FlushInterval = time.Hour
	cfg.MaxBufferBytes = 1000
	l := newLogger(sink, cfg)
	defer l.Close(context.Background())

	for i := 0; i < 100; i++ {
		l.Log(UserEvent{Path: "/api", sizeBytes: 100})
	}
	if dropped := l.dropped.Load(); dropped == 0 {
		t.Error("expected events to be dropped once the byte cap was exceeded")
	}
	if buffered := l.bufferedBytes.Load(); buffered > cfg.MaxBufferBytes {
		t.Errorf("buffered bytes %d exceeded cap %d", buffered, cfg.MaxBufferBytes)
	}
}

func TestLogNeverBlocksWhenBufferFull(t *testing.T) {
	sink := &fakeSink{}
	cfg := testConfig()
	cfg.BufferSize = 1
	cfg.BatchSize = 100000
	cfg.FlushInterval = time.Hour
	l := newLogger(sink, cfg)
	defer l.Close(context.Background())

	done := make(chan struct{})
	go func() {
		for i := 0; i < 5000; i++ {
			l.Log(UserEvent{Path: "/api", sizeBytes: 1})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Log blocked when the buffer was full")
	}
}

func TestCloseFlushesBufferedEvents(t *testing.T) {
	sink := &fakeSink{}
	cfg := testConfig()
	cfg.BatchSize = 100000
	cfg.FlushInterval = time.Hour
	l := newLogger(sink, cfg)

	for i := 0; i < 25; i++ {
		l.Log(UserEvent{Path: "/api", Status: 200, sizeBytes: 10})
	}
	l.Close(context.Background())

	if sink.count() != 25 {
		t.Errorf("expected 25 events flushed on close, got %d", sink.count())
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	l := newLogger(&fakeSink{}, testConfig())
	l.Close(context.Background())
	l.Close(context.Background())
}

func TestRetriesFailedFlush(t *testing.T) {
	sink := &fakeSink{failNext: 1}
	cfg := testConfig()
	cfg.BatchSize = 100000
	cfg.FlushInterval = time.Hour
	l := newLogger(sink, cfg)

	l.Log(UserEvent{Path: "/api", Status: 200, sizeBytes: 10})
	l.Close(context.Background())

	if sink.count() != 1 {
		t.Errorf("expected the retry to succeed, got %d events", sink.count())
	}
	if l.failed.Load() != 0 {
		t.Errorf("expected no permanent failures, got %d", l.failed.Load())
	}
}

func TestDropsBatchAfterRepeatedFailure(t *testing.T) {
	sink := &fakeSink{failNext: 2}
	cfg := testConfig()
	cfg.BatchSize = 100000
	cfg.FlushInterval = time.Hour
	l := newLogger(sink, cfg)

	l.Log(UserEvent{Path: "/api", Status: 200, sizeBytes: 10})
	l.Close(context.Background())

	if sink.count() != 0 {
		t.Errorf("expected the batch to be dropped, got %d events", sink.count())
	}
	if l.failed.Load() != 1 {
		t.Errorf("expected 1 failed event, got %d", l.failed.Load())
	}
}

func TestCaptureHeadersRedactsSensitiveHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("Authorization", "Bearer secret-token")
	h.Set("Cookie", "session=secret")
	h.Set("claims", `{"sub":"user-1"}`)
	h.Set("X-Api-Key", "secret-key")
	h.Set("User-Agent", "curl/8.0")

	captured := captureHeaders(h, []string{"authorization", "cookie", "claims", "x-api-key", "user-agent"})

	for _, denied := range []string{"authorization", "cookie", "claims", "x-api-key"} {
		if _, found := captured[denied]; found {
			t.Errorf("sensitive header %q was captured", denied)
		}
	}
	if captured["user-agent"] != "curl/8.0" {
		t.Errorf("expected user-agent to be captured, got %q", captured["user-agent"])
	}
}

func TestCaptureHeadersTruncatesLongValues(t *testing.T) {
	h := http.Header{}
	h.Set("User-Agent", strings.Repeat("a", maxHeaderValueLen+500))

	captured := captureHeaders(h, []string{"user-agent"})
	if len(captured["user-agent"]) != maxHeaderValueLen {
		t.Errorf("expected truncation to %d chars, got %d", maxHeaderValueLen, len(captured["user-agent"]))
	}
}

func TestCaptureHeadersOnlyAllowlisted(t *testing.T) {
	h := http.Header{}
	h.Set("User-Agent", "curl/8.0")
	h.Set("X-Internal-Secret", "nope")

	captured := captureHeaders(h, []string{"user-agent"})
	if len(captured) != 1 {
		t.Errorf("expected only allowlisted headers, got %v", captured)
	}
}

func TestClientIpPrefersForwardedFor(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api", nil)
	r.RemoteAddr = "10.0.0.1:5555"
	if got := clientIp(r); got != "10.0.0.1" {
		t.Errorf("expected RemoteAddr host, got %q", got)
	}

	r.Header.Set("X-Real-Ip", "203.0.113.9")
	if got := clientIp(r); got != "203.0.113.9" {
		t.Errorf("expected X-Real-Ip, got %q", got)
	}

	r.Header.Set("X-Forwarded-For", "198.51.100.7, 70.41.3.18")
	if got := clientIp(r); got != "198.51.100.7" {
		t.Errorf("expected first X-Forwarded-For entry, got %q", got)
	}
}

func TestNewEventCapturesRequestBeforeMutation(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "http://gateway.example.com:8086/api/v1/orders", nil)
	r.Host = "gateway.example.com:8086"
	r.Header.Set("User-Agent", "curl/8.0")
	w := httptest.NewRecorder()
	w.Header().Set("request_id", "req-1")
	w.Header().Set("trace_id", "trace-1")

	at := time.Now()
	e := NewEvent(r, w, at, []string{"user-agent"})

	r.Host = "upstream.internal"
	r.URL.Path = "/rewritten"
	r.Method = http.MethodGet

	if e.Host != "gateway.example.com" {
		t.Errorf("expected the original host without port, got %q", e.Host)
	}
	if e.Path != "/api/v1/orders" {
		t.Errorf("expected the original path, got %q", e.Path)
	}
	if e.Method != http.MethodPost {
		t.Errorf("expected the original method, got %q", e.Method)
	}
	if e.RequestId != "req-1" || e.TraceId != "trace-1" {
		t.Errorf("expected request/trace ids, got %q / %q", e.RequestId, e.TraceId)
	}
	if e.sizeBytes <= 0 {
		t.Error("expected a positive approximate size")
	}
}
