package models

import (
	"context"
	"os"
	"testing"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

func TestMain(m *testing.M) {
	logs.LogInit("test", "test-instance")
	os.Exit(m.Run())
}

func TestStreamEventTypes(t *testing.T) {
	types := []StreamEventType{
		StreamThinking,
		StreamToolUse,
		StreamToolResult,
		StreamTextDelta,
		StreamDone,
	}
	seen := map[StreamEventType]bool{}
	for _, typ := range types {
		if seen[typ] {
			t.Errorf("duplicate stream event type: %s", typ)
		}
		seen[typ] = true
		if typ == "" {
			t.Error("stream event type should not be empty")
		}
	}
}

func TestModelStreamEventStruct(t *testing.T) {
	event := ModelStreamEvent{
		Type:      StreamToolUse,
		ToolName:  "search",
		ToolInput: map[string]interface{}{"query": "test"},
		Iteration: 2,
	}
	if event.Type != StreamToolUse {
		t.Errorf("unexpected type: %s", event.Type)
	}
	if event.ToolName != "search" {
		t.Errorf("unexpected tool_name: %s", event.ToolName)
	}
	if event.Iteration != 2 {
		t.Errorf("unexpected iteration: %d", event.Iteration)
	}
}

func TestStreamEventCallbackNil(t *testing.T) {
	var cb StreamEventCallback
	if cb != nil {
		t.Fatal("nil callback should be nil")
	}
}

func TestTokenUsageStruct(t *testing.T) {
	usage := TokenUsage{
		InputTokens:     100,
		OutputTokens:    50,
		ReasoningTokens: 500,
		CachedTokens:    20,
		TotalTokens:     650,
	}
	if usage.ReasoningTokens != 500 {
		t.Errorf("expected 500, got %d", usage.ReasoningTokens)
	}
}

func TestOpenAIIsReasoningModel(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"o1-preview", true},
		{"o1-mini", true},
		{"o3-mini", true},
		{"o4-mini", true},
		{"gpt-4o", false},
		{"gpt-4-turbo", false},
		{"gpt-3.5-turbo", false},
	}
	for _, tc := range tests {
		m := &OpenAIModel{}
		m.LLMName = tc.name
		got := m.isReasoningModel()
		if got != tc.expected {
			t.Errorf("isReasoningModel(%s) = %v, want %v", tc.name, got, tc.expected)
		}
	}
}

func TestOpenAIReasoningEffortInRequest(t *testing.T) {
	m := &OpenAIModel{}
	m.LLMName = "o1-preview"
	m.ReasoningEffort = "high"
	m.Temprature = 0.7

	req, _ := m.makeOpenAIChatRequest(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "test"}},
	})

	if req.ReasoningEffort != "high" {
		t.Errorf("expected reasoning_effort high, got %s", req.ReasoningEffort)
	}
	if req.Temperature != 0 {
		t.Error("temperature should be 0 for reasoning models")
	}
	if req.MaxCompletionTokens != 16000 {
		t.Errorf("expected max_completion_tokens 16000, got %d", req.MaxCompletionTokens)
	}
}

func TestOpenAIReasoningEffortDefault(t *testing.T) {
	m := &OpenAIModel{}
	m.LLMName = "o3-mini"

	req, _ := m.makeOpenAIChatRequest(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "test"}},
	})

	if req.ReasoningEffort != "medium" {
		t.Errorf("expected default reasoning_effort medium, got %s", req.ReasoningEffort)
	}
}

func TestOpenAINonReasoningModel(t *testing.T) {
	m := &OpenAIModel{}
	m.LLMName = "gpt-4o"
	m.Temprature = 0.7

	req, _ := m.makeOpenAIChatRequest(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "test"}},
	})

	if req.ReasoningEffort != "" {
		t.Errorf("expected empty reasoning_effort for non-reasoning model, got %s", req.ReasoningEffort)
	}
	if req.Temperature != 0.7 {
		t.Errorf("expected temperature 0.7, got %f", req.Temperature)
	}
}

func TestStreamEventCallbackInvocation(t *testing.T) {
	var received []ModelStreamEvent
	cb := StreamEventCallback(func(event ModelStreamEvent) {
		received = append(received, event)
	})

	cb(ModelStreamEvent{Type: StreamThinking, Content: "thinking...", Iteration: 1})
	cb(ModelStreamEvent{Type: StreamTextDelta, Content: "hello", Iteration: 1})
	cb(ModelStreamEvent{Type: StreamDone, Content: "final", Iteration: 1})

	if len(received) != 3 {
		t.Fatalf("expected 3 events, got %d", len(received))
	}
	if received[0].Type != StreamThinking {
		t.Errorf("expected thinking, got %s", received[0].Type)
	}
	if received[1].Content != "hello" {
		t.Errorf("expected hello, got %s", received[1].Content)
	}
	if received[2].Type != StreamDone {
		t.Errorf("expected done, got %s", received[2].Type)
	}
}
