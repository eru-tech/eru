package handlers

import (
	"net/http/httptest"
	"testing"
)

func TestConversationWindowDefaults(t *testing.T) {
	limit, skip := conversationWindow(httptest.NewRequest("GET", "/list/conversations/agent", nil))
	if limit != defaultConversationPageSize || skip != 0 {
		t.Errorf("got limit=%d skip=%d, want %d/0", limit, skip, defaultConversationPageSize)
	}
}

func TestConversationWindowReadsLimitAndSkip(t *testing.T) {
	limit, skip := conversationWindow(httptest.NewRequest("GET", "/list/conversations/agent?limit=10&skip=30", nil))
	if limit != 10 || skip != 30 {
		t.Errorf("got limit=%d skip=%d, want 10/30", limit, skip)
	}
}

// A page number is only limit and skip with arithmetic in between, and the
// client already knows how many rows it holds - so it is not a parameter.
func TestPageIsIgnored(t *testing.T) {
	limit, skip := conversationWindow(httptest.NewRequest("GET", "/list/conversations/agent?page=5", nil))
	if limit != defaultConversationPageSize || skip != 0 {
		t.Errorf("page should have no effect, got limit=%d skip=%d", limit, skip)
	}
}

func TestNonsenseWindowFallsBackToTheDefault(t *testing.T) {
	for _, q := range []string{"?limit=0", "?limit=-5", "?limit=abc", "?skip=-1", "?skip=xyz"} {
		limit, skip := conversationWindow(httptest.NewRequest("GET", "/list/conversations/agent"+q, nil))
		if limit <= 0 || skip < 0 {
			t.Errorf("%s produced limit=%d skip=%d", q, limit, skip)
		}
	}
}
