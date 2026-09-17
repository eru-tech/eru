package agents

import (
	"context"
	"encoding/json"
	"testing"

	models "github.com/eru-tech/eru/eru-ai/models"
	cache "github.com/eru-tech/eru/eru-cache/cache"
)

// fakeCache is a cache that remembers, so what SaveConversation writes can be
// read back exactly as loadMessages would read it.
type fakeCache struct {
	cache.CacheStore
	values map[string]string
}

func newFakeCache() *fakeCache { return &fakeCache{values: map[string]string{}} }

func (f *fakeCache) Get(ctx context.Context, key string) (string, error) {
	if v, ok := f.values[key]; ok {
		return v, nil
	}
	return "", errNotFound{}
}
func (f *fakeCache) Set(ctx context.Context, key string, value interface{}) error {
	if text, ok := value.(string); ok {
		f.values[key] = text
		return nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	f.values[key] = string(encoded)
	return nil
}
func (f *fakeCache) GetAttribute(ctx context.Context, name string) (interface{}, error) {
	if name == "persist_enabled" {
		return false, nil
	}
	return nil, nil
}

type errNotFound struct{}

func (errNotFound) Error() string { return "not found" }

func agentWithCache(c cache.CacheStoreI) *Agent {
	return &Agent{AgentName: "test_agent", ChatMemory: c}
}

// The bug this exists for: the cache used to be written in the []CacheData shape
// and read back as []AgentMessage, which share no field names - so a second turn
// loaded a list of blank messages and the conversation was effectively gone.
func TestASavedConversationReadsBackAsItself(t *testing.T) {
	c := newFakeCache()
	agent := agentWithCache(c)
	conversation := &Conversation{
		ConversationId: "conv-1",
		NewMessages: []AgentMessage{
			{Role: "user", Content: "make me a page"},
			{Role: "assistant", Content: "here it is", Actions: []AgentOutputAction{{ActionName: "eru_studio"}}},
		},
	}
	if err := agent.SaveConversation(context.Background(), conversation, "p", "t"); err != nil {
		t.Fatal(err)
	}

	loaded, err := agent.loadMessages(context.Background(), "conv-1", "p", "t")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected both messages back, got %d", len(loaded))
	}
	if loaded[0].Role != "user" || loaded[0].Content != "make me a page" {
		t.Errorf("the user message came back empty: %+v", loaded[0])
	}
	if loaded[1].Content != "here it is" || len(loaded[1].Actions) != 1 {
		t.Errorf("the answer came back without its content or actions: %+v", loaded[1])
	}
}

// A second turn must extend the conversation, not replace it.
func TestASecondTurnKeepsTheFirst(t *testing.T) {
	c := newFakeCache()
	agent := agentWithCache(c)
	ctx := context.Background()

	first := &Conversation{ConversationId: "conv-1", NewMessages: []AgentMessage{
		{Role: "user", Content: "turn one"},
		{Role: "assistant", Content: "answer one"},
	}}
	if err := agent.SaveConversation(ctx, first, "p", "t"); err != nil {
		t.Fatal(err)
	}

	history, _ := agent.loadMessages(ctx, "conv-1", "p", "t")
	second := &Conversation{ConversationId: "conv-1", Messages: history, NewMessages: []AgentMessage{
		{Role: "user", Content: "turn two"},
		{Role: "assistant", Content: "answer two"},
	}}
	if err := agent.SaveConversation(ctx, second, "p", "t"); err != nil {
		t.Fatal(err)
	}

	loaded, _ := agent.loadMessages(ctx, "conv-1", "p", "t")
	if len(loaded) != 4 {
		t.Fatalf("expected all four messages, got %d", len(loaded))
	}
	if loaded[0].Content != "turn one" || loaded[3].Content != "answer two" {
		t.Errorf("history is not in order: %v", []string{loaded[0].Content, loaded[3].Content})
	}
}

// Thinking and tool payloads drive the live activity log and are not worth
// remembering: nothing reads them back, and one trace can carry a whole page.
func TestTracesAreNotRemembered(t *testing.T) {
	c := newFakeCache()
	agent := agentWithCache(c)
	conversation := &Conversation{ConversationId: "conv-1", NewMessages: []AgentMessage{
		{Role: "assistant", Content: "done", Traces: []models.StepTrace{
			{Thinking: "a long internal monologue", ToolResult: map[string]interface{}{"page": "...huge..."}},
		}},
	}}
	if err := agent.SaveConversation(context.Background(), conversation, "p", "t"); err != nil {
		t.Fatal(err)
	}

	// Cache keys are scoped to the tenant; the conversation id alone is not a key.
	raw := c.values[memoryKey("p", "t", "conv-1")]
	if raw == "" {
		t.Fatal("nothing was cached")
	}
	if len(raw) > 0 && json.Valid([]byte(raw)) == false {
		t.Fatal("what was cached is not valid JSON")
	}
	loaded, _ := agent.loadMessages(context.Background(), "conv-1", "p", "t")
	if len(loaded) != 1 {
		t.Fatalf("expected the message, got %d", len(loaded))
	}
	if len(loaded[0].Traces) != 0 {
		t.Error("traces should not be remembered")
	}
	if loaded[0].Content != "done" {
		t.Error("the answer itself must survive")
	}
}

// The live response keeps its traces - they drive the activity log.
func TestTheLiveMessageKeepsItsTraces(t *testing.T) {
	msg := AgentMessage{Content: "x", Traces: []models.StepTrace{{Thinking: "y"}}}
	_ = msg.forStorage()
	if len(msg.Traces) != 1 {
		t.Error("forStorage must not strip the caller's own message")
	}
}
