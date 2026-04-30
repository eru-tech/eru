package memory

import (
	"context"
	"encoding/json"
	"os"
	"testing"

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
	deleted []vectorstore.VectorRecordsDelete
	results vectorstore.VectorResults
}

func (m *mockVectorStore) SaveVectors(ctx context.Context, records vectorstore.VectorRecords) error {
	m.saved = append(m.saved, records)
	return nil
}

func (m *mockVectorStore) SearchVectors(ctx context.Context, search vectorstore.VectorRecordsSearch) (vectorstore.VectorResults, error) {
	return m.results, nil
}

func (m *mockVectorStore) DeleteVectors(ctx context.Context, del vectorstore.VectorRecordsDelete) error {
	m.deleted = append(m.deleted, del)
	return nil
}

func TestMemoryToolGetActions(t *testing.T) {
	mt := &MemoryTool{}
	actions := mt.GetActions()
	if len(actions) != 3 {
		t.Fatalf("expected 3 actions, got %d", len(actions))
	}

	names := map[string]bool{}
	for _, a := range actions {
		names[a.ActionName] = true
	}
	for _, expected := range []string{Remember, Recall, Forget} {
		if !names[expected] {
			t.Errorf("missing action: %s", expected)
		}
	}
}

func TestMemoryToolGetActionsList(t *testing.T) {
	mt := &MemoryTool{}
	infos := mt.GetActionsList()
	if len(infos) != 3 {
		t.Fatalf("expected 3 action infos, got %d", len(infos))
	}
}

func TestMemoryToolGetSpec(t *testing.T) {
	mt := &MemoryTool{}
	if mt.GetSpec() != mt {
		t.Error("GetSpec should return self")
	}
}

func TestMemoryToolExecuteNoVectorStore(t *testing.T) {
	mt := &MemoryTool{}
	ctx := context.Background()
	_, _, err := mt.Execute(ctx, "proj", "tenant", Remember, map[string]interface{}{"content": "test", "namespace": "ns"})
	if err == nil {
		t.Fatal("expected error when vector store not configured")
	}
}

func TestMemoryToolRemember(t *testing.T) {
	mock := &mockVectorStore{}
	mt := &MemoryTool{VectorStore: mock}
	ctx := context.Background()

	params := map[string]interface{}{
		"content":   "the capital of France is Paris",
		"namespace": "knowledge",
		"tags":      []string{"geography", "europe"},
		"metadata":  map[string]interface{}{"source": "user"},
	}

	result, _, err := mt.Execute(ctx, "proj", "tenant", Remember, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["message"] != "memory saved successfully" {
		t.Errorf("unexpected message: %v", result["message"])
	}
	if result["id"] == nil || result["id"] == "" {
		t.Error("expected non-empty id")
	}

	if len(mock.saved) != 1 {
		t.Fatalf("expected 1 save call, got %d", len(mock.saved))
	}
	if mock.saved[0].Namespace != "knowledge" {
		t.Errorf("expected namespace knowledge, got %s", mock.saved[0].Namespace)
	}
	if len(mock.saved[0].Vectors) != 1 {
		t.Fatalf("expected 1 vector, got %d", len(mock.saved[0].Vectors))
	}

	meta := mock.saved[0].Vectors[0].Metadata
	if meta["content"] != "the capital of France is Paris" {
		t.Errorf("unexpected content in metadata: %v", meta["content"])
	}
	if meta["source"] != "user" {
		t.Errorf("unexpected source in metadata: %v", meta["source"])
	}
	if meta["created_at"] == nil {
		t.Error("expected created_at in metadata")
	}
}

func TestMemoryToolRememberMissingContent(t *testing.T) {
	mock := &mockVectorStore{}
	mt := &MemoryTool{VectorStore: mock}
	ctx := context.Background()

	_, _, err := mt.Execute(ctx, "proj", "tenant", Remember, map[string]interface{}{"namespace": "ns"})
	if err == nil {
		t.Fatal("expected error for missing content")
	}
}

func TestMemoryToolRememberMissingNamespace(t *testing.T) {
	mock := &mockVectorStore{}
	mt := &MemoryTool{VectorStore: mock}
	ctx := context.Background()

	_, _, err := mt.Execute(ctx, "proj", "tenant", Remember, map[string]interface{}{"content": "test"})
	if err == nil {
		t.Fatal("expected error for missing namespace")
	}
}

func TestMemoryToolRecall(t *testing.T) {
	mock := &mockVectorStore{
		results: vectorstore.VectorResults{
			Records: []vectorstore.VectorResult{
				{
					Id:       "mem_1",
					Metadata: map[string]interface{}{"content": "Paris is the capital of France"},
				},
				{
					Id:       "mem_2",
					Metadata: map[string]interface{}{"content": "Berlin is the capital of Germany"},
				},
			},
		},
	}
	mt := &MemoryTool{VectorStore: mock}
	ctx := context.Background()

	params := map[string]interface{}{
		"query":     "what is the capital of France?",
		"namespace": "knowledge",
		"top_k":     3,
	}

	result, _, err := mt.Execute(ctx, "proj", "tenant", Recall, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	count, ok := result["count"]
	if !ok {
		t.Fatal("expected count in result")
	}
	if count != 2 {
		t.Errorf("expected count 2, got %v", count)
	}

	memories, ok := result["memories"].([]map[string]interface{})
	if !ok {
		t.Fatal("expected memories array in result")
	}
	if len(memories) != 2 {
		t.Fatalf("expected 2 memories, got %d", len(memories))
	}
	if memories[0]["content"] != "Paris is the capital of France" {
		t.Errorf("unexpected first memory content: %v", memories[0]["content"])
	}
}

func TestMemoryToolRecallMissingQuery(t *testing.T) {
	mock := &mockVectorStore{}
	mt := &MemoryTool{VectorStore: mock}
	ctx := context.Background()

	_, _, err := mt.Execute(ctx, "proj", "tenant", Recall, map[string]interface{}{"namespace": "ns"})
	if err == nil {
		t.Fatal("expected error for missing query")
	}
}

func TestMemoryToolRecallDefaultTopK(t *testing.T) {
	mock := &mockVectorStore{results: vectorstore.VectorResults{}}
	mt := &MemoryTool{VectorStore: mock}
	ctx := context.Background()

	_, _, err := mt.Execute(ctx, "proj", "tenant", Recall, map[string]interface{}{
		"query":     "test",
		"namespace": "ns",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMemoryToolForget(t *testing.T) {
	mock := &mockVectorStore{}
	mt := &MemoryTool{VectorStore: mock}
	ctx := context.Background()

	params := map[string]interface{}{
		"ids":       []string{"mem_1", "mem_2"},
		"namespace": "knowledge",
	}

	result, _, err := mt.Execute(ctx, "proj", "tenant", Forget, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["message"] != "memory entries deleted successfully" {
		t.Errorf("unexpected message: %v", result["message"])
	}

	if len(mock.deleted) != 1 {
		t.Fatalf("expected 1 delete call, got %d", len(mock.deleted))
	}
	if mock.deleted[0].Namespace != "knowledge" {
		t.Errorf("expected namespace knowledge, got %s", mock.deleted[0].Namespace)
	}
}

func TestMemoryToolForgetMissingIdsAndFilter(t *testing.T) {
	mock := &mockVectorStore{}
	mt := &MemoryTool{VectorStore: mock}
	ctx := context.Background()

	_, _, err := mt.Execute(ctx, "proj", "tenant", Forget, map[string]interface{}{"namespace": "ns"})
	if err == nil {
		t.Fatal("expected error for missing ids and filter")
	}
}

func TestMemoryToolForgetWithFilter(t *testing.T) {
	mock := &mockVectorStore{}
	mt := &MemoryTool{VectorStore: mock}
	ctx := context.Background()

	params := map[string]interface{}{
		"namespace": "knowledge",
		"filter":    map[string]interface{}{"source": "outdated"},
	}

	_, _, err := mt.Execute(ctx, "proj", "tenant", Forget, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMemoryToolUnknownAction(t *testing.T) {
	mock := &mockVectorStore{}
	mt := &MemoryTool{VectorStore: mock}
	ctx := context.Background()

	_, _, err := mt.Execute(ctx, "proj", "tenant", "unknown", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestMemoryToolMakeFromJson(t *testing.T) {
	data := map[string]interface{}{
		"tool_name":        "memory_1",
		"tool_type":        "Memory",
		"vectorstore_name": "my_vs",
		"description":      "memory tool",
	}
	b, _ := json.Marshal(data)
	raw := json.RawMessage(b)

	mt := &MemoryTool{}
	ctx := context.Background()
	if err := mt.MakeFromJson(ctx, &raw); err != nil {
		t.Fatalf("MakeFromJson failed: %v", err)
	}

	if mt.ToolName != "memory_1" {
		t.Errorf("expected tool_name memory_1, got %s", mt.ToolName)
	}
	if mt.VectorStoreName != "my_vs" {
		t.Errorf("expected vectorstore_name my_vs, got %s", mt.VectorStoreName)
	}
}

func TestMemoryToolBytesToTool(t *testing.T) {
	data := map[string]interface{}{
		"tool_name":        "mem_tool",
		"tool_type":        "Memory",
		"vectorstore_name": "vs1",
	}
	b, _ := json.Marshal(data)

	mt := &MemoryTool{}
	ctx := context.Background()
	result, err := mt.BytesToTool(ctx, b)
	if err != nil {
		t.Fatalf("BytesToTool failed: %v", err)
	}

	memTool, ok := result.(*MemoryTool)
	if !ok {
		t.Fatal("expected *MemoryTool type")
	}
	if memTool.ToolName != "mem_tool" {
		t.Errorf("expected tool_name mem_tool, got %s", memTool.ToolName)
	}
}

func TestMemoryToolGetSetAttribute(t *testing.T) {
	mt := &MemoryTool{}
	ctx := context.Background()

	mt.SetAttribute(ctx, "tool_name", "test_mem")
	mt.SetAttribute(ctx, "description", "test desc")

	val, err := mt.GetAttribute(ctx, "tool_name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "test_mem" {
		t.Errorf("expected test_mem, got %v", val)
	}

	val, err = mt.GetAttribute(ctx, "description")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "test desc" {
		t.Errorf("expected test desc, got %v", val)
	}

	_, err = mt.GetAttribute(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent attribute")
	}
}
