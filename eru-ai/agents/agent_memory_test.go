package agents

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	models "github.com/eru-tech/eru/eru-ai/models"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	vectorstore "github.com/eru-tech/eru/eru-vectorstore/vectorstore"
)

func TestMain(m *testing.M) {
	logs.LogInit("test", "test-instance")
	os.Exit(m.Run())
}

type mockVectorStore struct {
	vectorstore.VectorStore
	saved   []vectorstore.VectorRecords
	results vectorstore.VectorResults
}

func (m *mockVectorStore) SaveVectors(ctx context.Context, records vectorstore.VectorRecords) error {
	m.saved = append(m.saved, records)
	return nil
}

func (m *mockVectorStore) SearchVectors(ctx context.Context, search vectorstore.VectorRecordsSearch) (vectorstore.VectorResults, error) {
	return m.results, nil
}

func TestParentConversationIdField(t *testing.T) {
	conv := Conversation{
		ConversationId:       "child-1",
		ParentConversationId: "parent-1",
	}
	if conv.ParentConversationId != "parent-1" {
		t.Errorf("expected parent-1, got %s", conv.ParentConversationId)
	}
}

func TestParentConversationIdEmpty(t *testing.T) {
	conv := Conversation{ConversationId: "conv-1"}
	if conv.ParentConversationId != "" {
		t.Errorf("expected empty parent, got %s", conv.ParentConversationId)
	}
}

func TestSetSemanticMemory(t *testing.T) {
	agent := &Agent{AgentName: "test_agent"}
	mock := &mockVectorStore{}
	agent.SetSemanticMemory(mock)
	if agent.SemanticMemory == nil {
		t.Fatal("expected semantic memory to be set")
	}
}

func TestRecallMemoryNilStore(t *testing.T) {
	agent := &Agent{AgentName: "test_agent"}
	ctx := context.Background()
	result, err := agent.RecallMemory(ctx, "query", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

func TestRecallMemory(t *testing.T) {
	mock := &mockVectorStore{
		results: vectorstore.VectorResults{
			Records: []vectorstore.VectorResult{
				{Id: "m1", Metadata: map[string]interface{}{"content": "fact one"}},
				{Id: "m2", Metadata: map[string]interface{}{"content": "fact two"}},
			},
		},
	}
	agent := &Agent{AgentName: "test_agent"}
	agent.SetSemanticMemory(mock)
	ctx := context.Background()

	memories, err := agent.RecallMemory(ctx, "query", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(memories) != 2 {
		t.Fatalf("expected 2 memories, got %d", len(memories))
	}
	if memories[0]["content"] != "fact one" {
		t.Errorf("unexpected content: %v", memories[0]["content"])
	}
}

func TestRecallMemoryDefaultTopK(t *testing.T) {
	mock := &mockVectorStore{results: vectorstore.VectorResults{}}
	agent := &Agent{AgentName: "test_agent"}
	agent.SetSemanticMemory(mock)
	ctx := context.Background()

	_, err := agent.RecallMemory(ctx, "query", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecallMemoryCustomNamespace(t *testing.T) {
	mock := &mockVectorStore{results: vectorstore.VectorResults{}}
	agent := &Agent{AgentName: "test_agent", MemoryNamespace: "custom_ns"}
	agent.SetSemanticMemory(mock)
	ctx := context.Background()

	_, err := agent.RecallMemory(ctx, "query", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSaveToMemoryNilStore(t *testing.T) {
	agent := &Agent{AgentName: "test_agent"}
	ctx := context.Background()
	err := agent.SaveToMemory(ctx, "content", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSaveToMemory(t *testing.T) {
	mock := &mockVectorStore{}
	agent := &Agent{AgentName: "test_agent"}
	agent.SetSemanticMemory(mock)
	ctx := context.Background()

	err := agent.SaveToMemory(ctx, "important fact", map[string]interface{}{"source": "user"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.saved) != 1 {
		t.Fatalf("expected 1 save, got %d", len(mock.saved))
	}
	if mock.saved[0].Namespace != "test_agent" {
		t.Errorf("expected namespace test_agent, got %s", mock.saved[0].Namespace)
	}
	meta := mock.saved[0].Vectors[0].Metadata
	if meta["content"] != "important fact" {
		t.Errorf("unexpected content: %v", meta["content"])
	}
	if meta["agent_name"] != "test_agent" {
		t.Errorf("unexpected agent_name: %v", meta["agent_name"])
	}
	if meta["source"] != "user" {
		t.Errorf("unexpected source: %v", meta["source"])
	}
	if meta["created_at"] == nil {
		t.Error("expected created_at")
	}
}

func TestSaveToMemoryCustomNamespace(t *testing.T) {
	mock := &mockVectorStore{}
	agent := &Agent{AgentName: "test_agent", MemoryNamespace: "shared"}
	agent.SetSemanticMemory(mock)
	ctx := context.Background()

	err := agent.SaveToMemory(ctx, "shared fact", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.saved[0].Namespace != "shared" {
		t.Errorf("expected namespace shared, got %s", mock.saved[0].Namespace)
	}
}

func TestSaveToMemoryNilMetadata(t *testing.T) {
	mock := &mockVectorStore{}
	agent := &Agent{AgentName: "test_agent"}
	agent.SetSemanticMemory(mock)
	ctx := context.Background()

	err := agent.SaveToMemory(ctx, "fact", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	meta := mock.saved[0].Vectors[0].Metadata
	if meta["content"] != "fact" {
		t.Errorf("unexpected content: %v", meta["content"])
	}
}

func newConversationManager() *ConversationManager {
	return &ConversationManager{
		Config: &ConversationConfig{
			MaxRecentMessages: 50,
			MaxTokens:         100000,
		},
	}
}

func TestConvertAssistantActionToContent(t *testing.T) {
	cm := newConversationManager()

	funcGroup := map[string]interface{}{
		"func_category_name": "reporting",
		"func_group_name":    "daily_report",
		"func_steps": map[string]interface{}{
			"get_data": map[string]interface{}{
				"query_name": "get_data",
			},
		},
	}

	agentMessages := []AgentMessage{
		{Role: "user", Content: "create a daily report function"},
		{
			Role: "assistant",
			Actions: []AgentOutputAction{{
				ActionName: "eru_func",
				Action:     funcGroup,
			}},
		},
	}

	msgs := cm.convertAgentMessagesToMessages(agentMessages, "test_agent")

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	if msgs[0].Content != "create a daily report function" {
		t.Errorf("user content mismatch: %s", msgs[0].Content)
	}

	if msgs[1].Content == "" {
		t.Fatal("assistant content should not be empty when Actions exist")
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(msgs[1].Content), &parsed); err != nil {
		t.Fatalf("assistant content should be valid JSON: %v", err)
	}
	if parsed["func_group_name"] != "daily_report" {
		t.Errorf("expected func_group_name daily_report, got %v", parsed["func_group_name"])
	}
}

func TestConvertAssistantWithContentPreferred(t *testing.T) {
	cm := newConversationManager()

	agentMessages := []AgentMessage{
		{
			Role:    "assistant",
			Content: "here is the answer",
			Actions: []AgentOutputAction{{
				ActionName: "test",
				Action:     map[string]interface{}{"key": "value"},
			}},
		},
	}

	msgs := cm.convertAgentMessagesToMessages(agentMessages, "test_agent")

	if msgs[0].Content != "here is the answer" {
		t.Errorf("should prefer Content over Actions when Content is set, got: %s", msgs[0].Content)
	}
}

func TestConvertAssistantNoContentNoActions(t *testing.T) {
	cm := newConversationManager()

	agentMessages := []AgentMessage{
		{Role: "assistant"},
	}

	msgs := cm.convertAgentMessagesToMessages(agentMessages, "test_agent")

	if msgs[0].Content != "" {
		t.Errorf("expected empty content, got: %s", msgs[0].Content)
	}
}

func TestConversationHistoryPreservesRoles(t *testing.T) {
	cm := newConversationManager()

	agentMessages := []AgentMessage{
		{Role: "user", Content: "build a query function"},
		{
			Role: "assistant",
			Actions: []AgentOutputAction{{
				ActionName: "eru_func",
				Action:     map[string]interface{}{"func_group_name": "v1"},
			}},
		},
		{Role: "user", Content: "now add email step after the query"},
	}

	msgs := cm.convertAgentMessagesToMessages(agentMessages, "test_agent")

	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" {
		t.Errorf("expected user role, got %s", msgs[0].Role)
	}
	if msgs[1].Role != "assistant" {
		t.Errorf("expected assistant role, got %s", msgs[1].Role)
	}
	if msgs[2].Role != "user" {
		t.Errorf("expected user role, got %s", msgs[2].Role)
	}

	var parsed map[string]interface{}
	json.Unmarshal([]byte(msgs[1].Content), &parsed)
	if parsed["func_group_name"] != "v1" {
		t.Errorf("expected func_group_name v1, got %v", parsed["func_group_name"])
	}
}

func TestBuildChatRequestIncludesActionHistory(t *testing.T) {
	cm := newConversationManager()

	funcGroup := map[string]interface{}{
		"func_category_name": "reporting",
		"func_group_name":    "report_v1",
	}

	conversation := &Conversation{
		ConversationId: "conv-1",
		Messages: []AgentMessage{
			{Role: "user", Content: "create a report function"},
			{
				Role: "assistant",
				Actions: []AgentOutputAction{{
					ActionName: "eru_func",
					Action:     funcGroup,
				}},
			},
		},
	}

	currentMsg := models.Message{
		Role:    "user",
		Content: "add an email step after the report",
	}

	ctx := context.Background()
	chatRequest, err := cm.BuildChatRequest(ctx, conversation, currentMsg, "test_agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(chatRequest.Messages) != 3 {
		t.Fatalf("expected 3 messages in chat request, got %d", len(chatRequest.Messages))
	}

	assistantMsg := chatRequest.Messages[1]
	if assistantMsg.Role != "assistant" {
		t.Errorf("expected assistant role, got %s", assistantMsg.Role)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(assistantMsg.Content), &parsed); err != nil {
		t.Fatalf("assistant message should contain valid JSON: %v", err)
	}
	if parsed["func_group_name"] != "report_v1" {
		t.Errorf("expected func_group_name report_v1, got %v", parsed["func_group_name"])
	}

	if chatRequest.Messages[2].Content != "add an email step after the report" {
		t.Errorf("current message not appended correctly")
	}
}

func TestBuildMetricsFromTraces(t *testing.T) {
	traces := []models.StepTrace{
		{Iteration: 1, ToolName: "search", Timestamp: time.Now()},
		{Iteration: 2, ToolName: "search", Timestamp: time.Now()},
		{Iteration: 3, ToolName: "structured_output", Timestamp: time.Now()},
	}

	startTime := time.Now().Add(-2 * time.Second)
	metrics := BuildMetrics(traces, startTime, nil)

	if metrics.TotalIterations != 3 {
		t.Errorf("expected 3 iterations, got %d", metrics.TotalIterations)
	}
	if metrics.DurationMs < 1000 {
		t.Errorf("expected duration >= 1000ms, got %d", metrics.DurationMs)
	}
	if metrics.Usage != nil {
		t.Error("expected nil usage when not provided")
	}

	toolMap := make(map[string]int)
	for _, tc := range metrics.ToolCalls {
		toolMap[tc.ToolName] = tc.CallCount
	}
	if toolMap["search"] != 2 {
		t.Errorf("expected search call count 2, got %d", toolMap["search"])
	}
	if toolMap["structured_output"] != 1 {
		t.Errorf("expected structured_output call count 1, got %d", toolMap["structured_output"])
	}
}

func TestBuildMetricsWithUsage(t *testing.T) {
	usage := &models.TokenUsage{
		InputTokens:     100,
		OutputTokens:    50,
		ReasoningTokens: 500,
		TotalTokens:     650,
	}

	metrics := BuildMetrics(nil, time.Now(), usage)

	if metrics.TotalIterations != 0 {
		t.Errorf("expected 0 iterations, got %d", metrics.TotalIterations)
	}
	if metrics.Usage == nil {
		t.Fatal("expected usage to be set")
	}
	if metrics.Usage.ReasoningTokens != 500 {
		t.Errorf("expected 500 reasoning tokens, got %d", metrics.Usage.ReasoningTokens)
	}
}

func TestBuildMetricsEmptyTraces(t *testing.T) {
	metrics := BuildMetrics([]models.StepTrace{}, time.Now(), nil)

	if metrics.TotalIterations != 0 {
		t.Errorf("expected 0 iterations, got %d", metrics.TotalIterations)
	}
	if len(metrics.ToolCalls) != 0 {
		t.Errorf("expected 0 tool calls, got %d", len(metrics.ToolCalls))
	}
}

func TestExecutionMetricsJSON(t *testing.T) {
	metrics := ExecutionMetrics{
		TotalIterations: 3,
		ToolCalls: []ToolCallMetric{
			{ToolName: "search", CallCount: 2},
		},
		Usage: &models.TokenUsage{
			InputTokens:     100,
			OutputTokens:    50,
			ReasoningTokens: 500,
		},
		DurationMs: 1500,
	}

	b, err := json.Marshal(metrics)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]interface{}
	json.Unmarshal(b, &parsed)

	if parsed["total_iterations"].(float64) != 3 {
		t.Errorf("unexpected total_iterations: %v", parsed["total_iterations"])
	}
	if parsed["duration_ms"].(float64) != 1500 {
		t.Errorf("unexpected duration_ms: %v", parsed["duration_ms"])
	}

	usage := parsed["usage"].(map[string]interface{})
	if usage["reasoning_tokens"].(float64) != 500 {
		t.Errorf("unexpected reasoning_tokens: %v", usage["reasoning_tokens"])
	}
}
