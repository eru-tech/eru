package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

func TestMain(m *testing.M) {
	logs.LogInit("test", "test-instance")
	os.Exit(m.Run())
}

type mockMCPServer struct {
	initResult MCPInitializeResult
	listResult MCPListToolsResult
	callResult MCPCallToolResult
	callErr    error
}

func newMockMCPServer() *mockMCPServer {
	return &mockMCPServer{
		initResult: MCPInitializeResult{
			ProtocolVersion: "2025-03-26",
			Capabilities:    MCPCapabilities{Tools: &MCPToolsCapability{ListChanged: true}},
			ServerInfo:      MCPServerInfo{Name: "test-server", Version: "1.0.0"},
		},
		listResult: MCPListToolsResult{
			Tools: []MCPTool{
				{Name: "tool__tenant1__search", Description: "Search tool", InputSchema: map[string]interface{}{"type": "object"}},
				{Name: "agent__tenant1__chatbot", Description: "Chat agent", InputSchema: map[string]interface{}{"type": "object"}},
			},
		},
		callResult: MCPCallToolResult{
			Content: []MCPContent{{Type: "text", Text: "result output"}},
		},
	}
}

func (m *mockMCPServer) Initialize(_ context.Context, _ MCPInitializeParams) (MCPInitializeResult, error) {
	return m.initResult, nil
}
func (m *mockMCPServer) ListTools(_ context.Context, _, _ string) (MCPListToolsResult, error) {
	return m.listResult, nil
}
func (m *mockMCPServer) CallTool(_ context.Context, _ string, _ MCPCallToolParams, _, _ string) (MCPCallToolResult, error) {
	return m.callResult, m.callErr
}
func (m *mockMCPServer) GetCapabilities() MCPCapabilities {
	return m.initResult.Capabilities
}
func (m *mockMCPServer) GetServerInfo() MCPServerInfo {
	return m.initResult.ServerInfo
}

func buildRequest(method string, id interface{}, params interface{}) []byte {
	req := MCPMessage{JSONRPCVersion: "2.0", ID: id, Method: method}
	if params != nil {
		b, _ := json.Marshal(params)
		req.Params = b
	}
	data, _ := json.Marshal(req)
	return data
}

func parseResponse(t *testing.T, data []byte) MCPMessage {
	t.Helper()
	var msg MCPMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	return msg
}

func TestHandleInitialize(t *testing.T) {
	h := NewMCPMessageHandler(newMockMCPServer(), "sess-1")
	ctx := context.Background()

	params := MCPInitializeParams{
		ProtocolVersion: "2025-03-26",
		ClientInfo:      MCPClientInfo{Name: "test-client", Version: "1.0"},
	}
	data := buildRequest("initialize", 1, params)
	resp, err := h.HandleMessage(ctx, data, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	msg := parseResponse(t, resp)
	if msg.Error != nil {
		t.Fatalf("expected no error, got: %v", msg.Error)
	}
	var result MCPInitializeResult
	if err := json.Unmarshal(msg.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if result.ProtocolVersion != "2025-03-26" {
		t.Errorf("expected protocol version 2025-03-26, got %s", result.ProtocolVersion)
	}
	if result.ServerInfo.Name != "test-server" {
		t.Errorf("expected server name test-server, got %s", result.ServerInfo.Name)
	}
}

func TestHandleInitializedNotification(t *testing.T) {
	h := NewMCPMessageHandler(newMockMCPServer(), "sess-1")
	ctx := context.Background()

	data := buildRequest("initialized", nil, nil)
	resp, err := h.HandleMessage(ctx, data, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != nil {
		t.Errorf("expected nil response for notification, got: %s", string(resp))
	}
}

func TestHandlePing(t *testing.T) {
	h := NewMCPMessageHandler(newMockMCPServer(), "sess-1")
	ctx := context.Background()

	data := buildRequest("ping", 42, nil)
	resp, err := h.HandleMessage(ctx, data, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	msg := parseResponse(t, resp)
	if msg.Error != nil {
		t.Fatalf("ping returned error: %v", msg.Error)
	}
	if string(msg.Result) != "{}" {
		t.Errorf("expected ping result {}, got %s", string(msg.Result))
	}
}

func TestHandleListToolsBeforeInit(t *testing.T) {
	h := NewMCPMessageHandler(newMockMCPServer(), "sess-1")
	ctx := context.Background()

	data := buildRequest("tools/list", 1, nil)
	resp, err := h.HandleMessage(ctx, data, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	msg := parseResponse(t, resp)
	if msg.Error == nil {
		t.Fatal("expected error when listing tools before init")
	}
	if msg.Error.Code != -32002 {
		t.Errorf("expected error code -32002, got %d", msg.Error.Code)
	}
}

func TestHandleListToolsAfterInit(t *testing.T) {
	h := NewMCPMessageHandler(newMockMCPServer(), "sess-1")
	ctx := context.Background()

	initData := buildRequest("initialize", 1, MCPInitializeParams{
		ProtocolVersion: "2025-03-26",
		ClientInfo:      MCPClientInfo{Name: "test", Version: "1.0"},
	})
	h.HandleMessage(ctx, initData, "", "")

	data := buildRequest("tools/list", 2, nil)
	resp, err := h.HandleMessage(ctx, data, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	msg := parseResponse(t, resp)
	if msg.Error != nil {
		t.Fatalf("unexpected error: %v", msg.Error)
	}
	var result MCPListToolsResult
	if err := json.Unmarshal(msg.Result, &result); err != nil {
		t.Fatalf("failed to parse list result: %v", err)
	}
	if len(result.Tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(result.Tools))
	}
}

func TestHandleCallTool(t *testing.T) {
	h := NewMCPMessageHandler(newMockMCPServer(), "sess-1")
	ctx := context.Background()

	h.HandleMessage(ctx, buildRequest("initialize", 1, MCPInitializeParams{
		ProtocolVersion: "2025-03-26",
		ClientInfo:      MCPClientInfo{Name: "test", Version: "1.0"},
	}), "", "")

	params := MCPCallToolParams{Name: "tool__tenant1__search", Arguments: map[string]interface{}{"query": "hello"}}
	data := buildRequest("tools/call", 3, params)
	resp, err := h.HandleMessage(ctx, data, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	msg := parseResponse(t, resp)
	if msg.Error != nil {
		t.Fatalf("unexpected error: %v", msg.Error)
	}
	var result MCPCallToolResult
	if err := json.Unmarshal(msg.Result, &result); err != nil {
		t.Fatalf("failed to parse call result: %v", err)
	}
	if len(result.Content) == 0 || result.Content[0].Text != "result output" {
		t.Errorf("unexpected result content: %v", result.Content)
	}
}

func TestHandleResourcesList(t *testing.T) {
	h := NewMCPMessageHandler(newMockMCPServer(), "sess-1")
	ctx := context.Background()

	data := buildRequest("resources/list", 1, nil)
	resp, err := h.HandleMessage(ctx, data, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	msg := parseResponse(t, resp)
	if msg.Error != nil {
		t.Fatalf("unexpected error: %v", msg.Error)
	}
	var result map[string]interface{}
	json.Unmarshal(msg.Result, &result)
	if _, ok := result["resources"]; !ok {
		t.Error("expected resources key in result")
	}
}

func TestHandlePromptsList(t *testing.T) {
	h := NewMCPMessageHandler(newMockMCPServer(), "sess-1")
	ctx := context.Background()

	data := buildRequest("prompts/list", 1, nil)
	resp, err := h.HandleMessage(ctx, data, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	msg := parseResponse(t, resp)
	if msg.Error != nil {
		t.Fatalf("unexpected error: %v", msg.Error)
	}
	var result map[string]interface{}
	json.Unmarshal(msg.Result, &result)
	if _, ok := result["prompts"]; !ok {
		t.Error("expected prompts key in result")
	}
}

func TestHandleUnknownMethodWithId(t *testing.T) {
	h := NewMCPMessageHandler(newMockMCPServer(), "sess-1")
	ctx := context.Background()

	data := buildRequest("unknown/method", 99, nil)
	resp, err := h.HandleMessage(ctx, data, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	msg := parseResponse(t, resp)
	if msg.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if msg.Error.Code != -32601 {
		t.Errorf("expected -32601, got %d", msg.Error.Code)
	}
}

func TestHandleUnknownNotification(t *testing.T) {
	h := NewMCPMessageHandler(newMockMCPServer(), "sess-1")
	ctx := context.Background()

	data := buildRequest("notifications/unknown", nil, nil)
	resp, err := h.HandleMessage(ctx, data, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != nil {
		t.Errorf("expected nil for unknown notification, got: %s", string(resp))
	}
}

func TestHandleInvalidJSON(t *testing.T) {
	h := NewMCPMessageHandler(newMockMCPServer(), "sess-1")
	ctx := context.Background()

	resp, err := h.HandleMessage(ctx, []byte("{invalid json}"), "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	msg := parseResponse(t, resp)
	if msg.Error == nil || msg.Error.Code != -32700 {
		t.Errorf("expected parse error -32700, got: %v", msg.Error)
	}
}

func TestHTTPHandlerInitialize(t *testing.T) {
	srv := newMockMCPServer()
	handler := CreateMCPHttpHandler(srv)

	params := MCPInitializeParams{
		ProtocolVersion: "2025-03-26",
		ClientInfo:      MCPClientInfo{Name: "test", Version: "1.0"},
	}
	body := MCPMessage{JSONRPCVersion: "2.0", ID: 1, Method: "initialize"}
	b, _ := json.Marshal(params)
	body.Params = b
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	sessionId := w.Header().Get("Mcp-Session-Id")
	if sessionId == "" {
		t.Error("expected Mcp-Session-Id header in response")
	}

	var msg MCPMessage
	json.NewDecoder(w.Body).Decode(&msg)
	if msg.Error != nil {
		t.Fatalf("unexpected error: %v", msg.Error)
	}
}

func TestHTTPHandlerNotificationReturnsEmpty(t *testing.T) {
	srv := newMockMCPServer()
	handler := CreateMCPHttpHandler(srv)

	body := MCPMessage{JSONRPCVersion: "2.0", Method: "initialized"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	bodyStr := strings.TrimSpace(w.Body.String())
	if bodyStr != "{}" {
		t.Errorf("expected empty {} for notification, got: %s", bodyStr)
	}
}

func TestHTTPHandlerDeleteSession(t *testing.T) {
	srv := newMockMCPServer()
	handler := CreateMCPHttpHandler(srv)

	req := httptest.NewRequest(http.MethodDelete, "/mcp", nil)
	req.Header.Set("Mcp-Session-Id", "test-session-123")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestHTTPHandlerSessionPersistsAcrossRequests(t *testing.T) {
	srv := newMockMCPServer()
	handler := CreateMCPHttpHandler(srv)

	sessionId := "persistent-session"
	initBody := MCPMessage{JSONRPCVersion: "2.0", ID: 1, Method: "initialize"}
	b, _ := json.Marshal(MCPInitializeParams{
		ProtocolVersion: "2025-03-26",
		ClientInfo:      MCPClientInfo{Name: "test", Version: "1.0"},
	})
	initBody.Params = b
	initBytes, _ := json.Marshal(initBody)

	req1 := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(initBytes))
	req1.Header.Set("Mcp-Session-Id", sessionId)
	w1 := httptest.NewRecorder()
	handler(w1, req1)

	listBody := MCPMessage{JSONRPCVersion: "2.0", ID: 2, Method: "tools/list"}
	listBytes, _ := json.Marshal(listBody)
	req2 := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(listBytes))
	req2.Header.Set("Mcp-Session-Id", sessionId)
	w2 := httptest.NewRecorder()
	handler(w2, req2)

	var msg MCPMessage
	json.NewDecoder(w2.Body).Decode(&msg)
	if msg.Error != nil {
		t.Errorf("expected session to persist init state, got error: %v", msg.Error)
	}
}

func TestMCPSessionManager(t *testing.T) {
	srv := newMockMCPServer()
	mgr := NewMCPSessionManager(srv)

	h1 := mgr.GetOrCreate("session-1")
	h2 := mgr.GetOrCreate("session-1")
	if h1 != h2 {
		t.Error("expected same handler for same session ID")
	}

	h3 := mgr.GetOrCreate("session-2")
	if h1 == h3 {
		t.Error("expected different handlers for different session IDs")
	}

	mgr.Delete("session-1")
	h4 := mgr.GetOrCreate("session-1")
	if h4 == h1 {
		t.Error("expected new handler after session deletion")
	}
}

func TestHealthHandler(t *testing.T) {
	srv := newMockMCPServer()
	handler := CreateMCPHealthHandler(srv)

	req := httptest.NewRequest(http.MethodGet, "/mcp/health", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var result map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result)
	if result["status"] != "healthy" {
		t.Errorf("expected status healthy, got %v", result["status"])
	}
}
