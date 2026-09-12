package tools

import (
	"context"
	"testing"
)

func TestClaimsFromContext(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		want string
	}{
		{"claims present", context.WithValue(context.Background(), ClaimsHeaderKey, `{"sub":"u1"}`), `{"sub":"u1"}`},
		{"claims set but empty", context.WithValue(context.Background(), ClaimsHeaderKey, ""), ""},
		// a background or relayed context never went through the http middleware
		{"no claims on the context", context.Background(), ""},
		{"claims of an unexpected type", context.WithValue(context.Background(), ClaimsHeaderKey, 42), ""},
		{"nil context", nil, ""},
	}
	for _, tt := range tests {
		if got := ClaimsFromContext(tt.ctx); got != tt.want {
			t.Errorf("%s: ClaimsFromContext() = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestClaimsKeyFromContext(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		want string
	}{
		{"project configures a key", WithClaimsKey(context.Background(), "X-Token"), "X-Token"},
		{"project configures nothing", WithClaimsKey(context.Background(), ""), ClaimsHeaderKey},
		{"key never set", context.Background(), ClaimsHeaderKey},
		{"nil context", nil, ClaimsHeaderKey},
	}
	for _, tt := range tests {
		if got := ClaimsKeyFromContext(tt.ctx); got != tt.want {
			t.Errorf("%s: ClaimsKeyFromContext() = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// the claims arrive under one fixed header and go out under whatever the callee expects
func TestClaimsHeader(t *testing.T) {
	ctx := context.WithValue(context.Background(), ClaimsHeaderKey, `{"sub":"u1"}`)
	ctx = WithClaimsKey(ctx, "X-Token")
	claimsKey, claims, ok := ClaimsHeader(ctx)
	if !ok || claimsKey != "X-Token" || claims != `{"sub":"u1"}` {
		t.Errorf("ClaimsHeader() = (%q, %q, %v), want (X-Token, {\"sub\":\"u1\"}, true)", claimsKey, claims, ok)
	}

	// nothing to forward when the caller carried no claims
	if _, _, ok := ClaimsHeader(context.Background()); ok {
		t.Error("ClaimsHeader() reported claims to forward when there are none")
	}
}

// a hook goroutine gets a fresh context - without the copy it runs unauthenticated, or
// forwards the token on the wrong header
func TestCopyClaims(t *testing.T) {
	from := context.WithValue(context.Background(), ClaimsHeaderKey, `{"sub":"u1"}`)
	from = WithClaimsKey(from, "X-Token")

	to := CopyClaims(from, context.Background())
	claimsKey, claims, ok := ClaimsHeader(to)
	if !ok || claims != `{"sub":"u1"}` || claimsKey != "X-Token" {
		t.Errorf("CopyClaims() gave (%q, %q, %v), want (X-Token, {\"sub\":\"u1\"}, true)", claimsKey, claims, ok)
	}

	// nothing to copy leaves the target as it was
	if _, _, ok := ClaimsHeader(CopyClaims(context.Background(), context.Background())); ok {
		t.Error("CopyClaims() invented claims from a context that had none")
	}
	if got := CopyClaims(nil, context.Background()); got == nil {
		t.Error("CopyClaims() returned a nil context")
	}
}
