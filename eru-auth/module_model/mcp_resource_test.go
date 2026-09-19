package module_model

import (
	"net/http/httptest"
	"testing"
)

func TestMcpResourceUrlFromForwardedHost(t *testing.T) {
	req := httptest.NewRequest("GET", McpWellKnownPath+"/mcp", nil)
	req.Host = "eruauth-internal:8085"
	req.Header.Set("X-Forwarded-Host", "eruai.app.smartvalues.co.in")
	req.Header.Set("X-Forwarded-Proto", "https")

	if got := McpResourceUrl(req); got != "https://eruai.app.smartvalues.co.in/mcp" {
		t.Errorf("forwarded host must win over the proxied host, got %s", got)
	}
}

func TestMcpResourceUrlDefaultsResourcePath(t *testing.T) {
	req := httptest.NewRequest("GET", McpWellKnownPath, nil)
	req.Host = "eruai.app.smartvalues.co.in"
	req.Header.Set("X-Forwarded-Proto", "https")

	if got := McpResourceUrl(req); got != "https://eruai.app.smartvalues.co.in/mcp" {
		t.Errorf("bare well-known path must still describe the mcp endpoint, got %s", got)
	}
}

func TestMcpResourceUrlKeepsResourcePath(t *testing.T) {
	req := httptest.NewRequest("GET", McpWellKnownPath+"/agent/mcp", nil)
	req.Host = "eruai.app.smartvalues.co.in"
	req.Header.Set("X-Forwarded-Proto", "https")

	if got := McpResourceUrl(req); got != "https://eruai.app.smartvalues.co.in/agent/mcp" {
		t.Errorf("unexpected resource %s", got)
	}
}
