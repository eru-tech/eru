package models

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func collectSSE(t *testing.T, body string, contentType string, status int) ([]string, error) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	defer server.Close()

	req, err := newJsonRequest(context.Background(), http.MethodPost, server.URL, http.Header{}, map[string]string{"hello": "world"})
	if err != nil {
		t.Fatalf("newJsonRequest failed: %v", err)
	}

	var received []string
	err = streamSSE(context.Background(), req, func(data []byte) (bool, error) {
		received = append(received, string(data))
		return false, nil
	})
	return received, err
}

func TestStreamSSEParsesEvents(t *testing.T) {
	body := "data: {\"a\":1}\n\ndata: {\"b\":2}\n\ndata: [DONE]\n\n"

	received, err := collectSSE(t, body, "text/event-stream", http.StatusOK)
	if err != nil {
		t.Fatalf("streamSSE returned error: %v", err)
	}
	if len(received) != 2 {
		t.Fatalf("expected 2 events, got %d: %v", len(received), received)
	}
	if received[0] != `{"a":1}` || received[1] != `{"b":2}` {
		t.Errorf("unexpected payloads: %v", received)
	}
}

func TestStreamSSEStopsAtDone(t *testing.T) {
	body := "data: {\"a\":1}\n\ndata: [DONE]\n\ndata: {\"never\":true}\n\n"

	received, err := collectSSE(t, body, "text/event-stream", http.StatusOK)
	if err != nil {
		t.Fatalf("streamSSE returned error: %v", err)
	}
	if len(received) != 1 {
		t.Fatalf("expected to stop at [DONE], got %d events: %v", len(received), received)
	}
}

func TestStreamSSEIgnoresCommentsAndBlankLines(t *testing.T) {
	body := ": keep-alive\n\n\ndata: {\"a\":1}\n\n: another comment\n\ndata: [DONE]\n\n"

	received, err := collectSSE(t, body, "text/event-stream", http.StatusOK)
	if err != nil {
		t.Fatalf("streamSSE returned error: %v", err)
	}
	if len(received) != 1 || received[0] != `{"a":1}` {
		t.Fatalf("comments should be skipped, got: %v", received)
	}
}

func TestStreamSSEJoinsMultilineData(t *testing.T) {
	body := "data: {\"a\":\ndata: 1}\n\ndata: [DONE]\n\n"

	received, err := collectSSE(t, body, "text/event-stream", http.StatusOK)
	if err != nil {
		t.Fatalf("streamSSE returned error: %v", err)
	}
	if len(received) != 1 || received[0] != "{\"a\":\n1}" {
		t.Fatalf("multiline data should be joined with newline, got: %q", received)
	}
}

func TestStreamSSEFlushesTrailingEventWithoutBlankLine(t *testing.T) {
	body := "data: {\"a\":1}"

	received, err := collectSSE(t, body, "text/event-stream", http.StatusOK)
	if err != nil {
		t.Fatalf("streamSSE returned error: %v", err)
	}
	if len(received) != 1 || received[0] != `{"a":1}` {
		t.Fatalf("trailing event should be flushed at EOF, got: %v", received)
	}
}

func TestStreamSSEReturnsErrorOnHttpFailure(t *testing.T) {
	received, err := collectSSE(t, `{"error":{"message":"invalid api key"}}`, "application/json", http.StatusUnauthorized)
	if err == nil {
		t.Fatal("expected an error for a 401 response")
	}
	if len(received) != 0 {
		t.Errorf("no events should be delivered on failure, got: %v", received)
	}
	if !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "invalid api key") {
		t.Errorf("error should carry status and body, got: %v", err)
	}
}

func TestStreamSSEPropagatesHandlerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"a\":1}\n\n")
	}))
	defer server.Close()

	req, err := newJsonRequest(context.Background(), http.MethodPost, server.URL, http.Header{}, map[string]string{})
	if err != nil {
		t.Fatalf("newJsonRequest failed: %v", err)
	}

	handlerErr := fmt.Errorf("handler exploded")
	err = streamSSE(context.Background(), req, func(data []byte) (bool, error) {
		return false, handlerErr
	})
	if err != handlerErr {
		t.Fatalf("expected handler error to propagate, got: %v", err)
	}
}

func TestStreamSSERespectsCancelledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"a\":1}\n\n")
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	req, err := newJsonRequest(ctx, http.MethodPost, server.URL, http.Header{}, map[string]string{})
	if err != nil {
		t.Fatalf("newJsonRequest failed: %v", err)
	}
	cancel()

	if err = streamSSE(ctx, req, func(data []byte) (bool, error) { return false, nil }); err == nil {
		t.Fatal("expected an error once the context is cancelled")
	}
}
