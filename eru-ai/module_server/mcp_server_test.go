package module_server

import (
	"context"
	"strings"
	"testing"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	module_model "github.com/eru-tech/eru/eru-ai/module_model"
	module_store "github.com/eru-tech/eru/eru-ai/module_store"
	tools "github.com/eru-tech/eru/eru-ai/tools"
	server "github.com/eru-tech/eru/eru-server/server"
)

func TestEruAIMCPServerInitialize(t *testing.T) {
	sh := newTestStoreHolder()
	s := NewEruAIMCPServer(sh)
	ctx := context.Background()

	params := server.MCPInitializeParams{
		ProtocolVersion: "2025-03-26",
		ClientInfo:      server.MCPClientInfo{Name: "test-client", Version: "1.0"},
	}
	result, err := s.Initialize(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ProtocolVersion != MCPProtocolVersion {
		t.Errorf("expected %s, got %s", MCPProtocolVersion, result.ProtocolVersion)
	}
	if result.ServerInfo.Name != ServerName {
		t.Errorf("expected server name %s, got %s", ServerName, result.ServerInfo.Name)
	}
	if result.ServerInfo.Version != ServerVersion {
		t.Errorf("expected server version %s, got %s", ServerVersion, result.ServerInfo.Version)
	}
	if result.Capabilities.Tools == nil {
		t.Error("expected tools capability to be set")
	}
}

func TestEruAIMCPServerGetCapabilities(t *testing.T) {
	sh := newTestStoreHolder()
	s := NewEruAIMCPServer(sh)
	caps := s.GetCapabilities()
	if caps.Tools == nil {
		t.Error("expected tools capability")
	}
}

func TestEruAIMCPServerGetServerInfo(t *testing.T) {
	sh := newTestStoreHolder()
	s := NewEruAIMCPServer(sh)
	info := s.GetServerInfo()
	if info.Name == "" {
		t.Error("expected non-empty server name")
	}
	if info.Version == "" {
		t.Error("expected non-empty server version")
	}
}

func TestEruAIMCPServerListToolsAll(t *testing.T) {
	sh := newTestStoreHolder()
	s := NewEruAIMCPServer(sh)
	ctx := context.Background()

	result, err := s.ListTools(ctx, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Tools) == 0 {
		t.Fatal("expected at least one tool")
	}

	foundTool := false
	foundAgent := false
	for _, tool := range result.Tools {
		if tool.Name == "tool__search" {
			foundTool = true
		}
		if tool.Name == "agent__chatbot" {
			foundAgent = true
		}
	}
	if !foundTool {
		t.Errorf("expected tool 'tool__search' in results, got: %v", toolNames(result.Tools))
	}
	if !foundAgent {
		t.Errorf("expected agent 'agent__chatbot' in results, got: %v", toolNames(result.Tools))
	}
}

func TestEruAIMCPServerListToolsFilterByProject(t *testing.T) {
	sh := newTestStoreHolder()
	s := NewEruAIMCPServer(sh)
	ctx := context.Background()

	result, err := s.ListTools(ctx, "test-project", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Tools) == 0 {
		t.Error("expected tools when filtering by existing project")
	}

	result2, err := s.ListTools(ctx, "nonexistent-project", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result2.Tools) != 0 {
		t.Errorf("expected no tools for nonexistent project, got %d", len(result2.Tools))
	}
}

func TestEruAIMCPServerListToolsEmptyStore(t *testing.T) {
	sh := &mockModuleStore{
		projects: make(map[string]*module_model.Project),
		toolMap:  make(map[string]map[string]map[string]tools.Tooling),
		agentMap: make(map[string]map[string]map[string]agents.AgentI),
	}
	s := NewEruAIMCPServer(&module_store.StoreHolder{Store: sh})
	result, err := s.ListTools(context.Background(), "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Tools) != 0 {
		t.Errorf("expected empty tools list, got %d", len(result.Tools))
	}
}

func TestEruAIMCPServerCallToolExecute(t *testing.T) {
	sh := newTestStoreHolder()
	s := NewEruAIMCPServer(sh)
	ctx := context.Background()

	result, err := s.CallTool(ctx, "conv-1", server.MCPCallToolParams{
		Name:      "tool__search",
		Arguments: map[string]interface{}{"query": "test"},
	}, "test-project", "tenant-abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Content) == 0 {
		t.Error("expected content in result")
	}
	if result.IsError {
		t.Errorf("expected no error, content: %v", result.Content)
	}
}

func TestEruAIMCPServerCallAgentExecute(t *testing.T) {
	sh := newTestStoreHolder()
	s := NewEruAIMCPServer(sh)
	ctx := context.Background()

	result, err := s.CallTool(ctx, "conv-1", server.MCPCallToolParams{
		Name:      "agent__chatbot",
		Arguments: map[string]interface{}{"content": "hello"},
	}, "test-project", "tenant-abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Content) == 0 {
		t.Error("expected content in result")
	}
	if !strings.Contains(result.Content[0].Text, "mock agent response") {
		t.Errorf("expected mock agent response in result, got: %s", result.Content[0].Text)
	}
}

func TestEruAIMCPServerCallToolInvalidName(t *testing.T) {
	sh := newTestStoreHolder()
	s := NewEruAIMCPServer(sh)
	ctx := context.Background()

	_, err := s.CallTool(ctx, "conv-1", server.MCPCallToolParams{
		Name: "invalid",
	}, "test-project", "tenant-abc123")
	if err == nil {
		t.Error("expected error for invalid tool name format")
	}
}

func TestEruAIMCPServerCallNonExistentTool(t *testing.T) {
	sh := newTestStoreHolder()
	s := NewEruAIMCPServer(sh)
	ctx := context.Background()

	_, err := s.CallTool(ctx, "conv-1", server.MCPCallToolParams{
		Name:      "tool__nonexistent",
		Arguments: map[string]interface{}{},
	}, "test-project", "tenant-abc123")
	if err == nil {
		t.Error("expected error for nonexistent tool")
	}
}

func TestEruAIMCPServerCallAgentMissingContent(t *testing.T) {
	sh := newTestStoreHolder()
	s := NewEruAIMCPServer(sh)
	ctx := context.Background()

	_, err := s.CallTool(ctx, "conv-1", server.MCPCallToolParams{
		Name:      "agent__chatbot",
		Arguments: map[string]interface{}{},
	}, "test-project", "tenant-abc123")
	if err == nil {
		t.Error("expected error when content is missing")
	}
}


func TestParseToolName(t *testing.T) {
	sh := newTestStoreHolder()
	s := NewEruAIMCPServer(sh)

	tests := []struct {
		input    string
		expected []string
	}{
		{"tool__search", []string{"tool", "search"}},
		{"agent__chatbot", []string{"agent", "chatbot"}},
		{"tool__project__search", []string{"tool", "project", "search"}},
		{"single", []string{"single"}},
	}
	for _, tt := range tests {
		got := s.parseToolName(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("parseToolName(%q) length = %d, want %d", tt.input, len(got), len(tt.expected))
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("parseToolName(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.expected[i])
			}
		}
	}
}

func toolNames(tools []server.MCPTool) []string {
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name
	}
	return names
}
