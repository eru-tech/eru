package module_server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/eru-tech/eru/eru-ai/agents/a2a"
	module_store "github.com/eru-tech/eru/eru-ai/module_store"
)

func buildA2ARequest(method string, id interface{}, params interface{}) []byte {
	req := a2a.JSONRPCRequest{JSONRPCVersion: "2.0", ID: id, Method: method}
	if params != nil {
		b, _ := json.Marshal(params)
		req.Params = b
	}
	data, _ := json.Marshal(req)
	return data
}

func parseA2AResponse(t *testing.T, body []byte) a2a.JSONRPCResponse {
	t.Helper()
	var resp a2a.JSONRPCResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("failed to parse A2A response: %v\nbody: %s", err, string(body))
	}
	return resp
}

func extractTask(t *testing.T, resp a2a.JSONRPCResponse) a2a.Task {
	t.Helper()
	b, _ := json.Marshal(resp.Result)
	var task a2a.Task
	if err := json.Unmarshal(b, &task); err != nil {
		t.Fatalf("failed to extract task: %v", err)
	}
	return task
}

func TestA2AGetAgentCard(t *testing.T) {
	sh := newTestStoreHolder()
	s := NewEruAIA2AServer(sh)
	ctx := context.Background()
	projectId := "processo"
	tenantId := "39acd634-577e-41ba-aa56-df5695208696"
	card := s.GetAgentCard(ctx, "https://example.com", projectId, tenantId)
	if card.Name == "" {
		t.Error("expected non-empty agent card name")
	}
	if card.URL != "https://example.com/a2a" {
		t.Errorf("expected URL https://example.com/a2a, got %s", card.URL)
	}
	if len(card.Skills) == 0 {
		t.Error("expected at least one skill")
	}
	if !card.Capabilities.Streaming {
		t.Error("expected streaming capability")
	}
	if len(card.DefaultInputModes) == 0 {
		t.Error("expected default input modes")
	}

	found := false
	for _, skill := range card.Skills {
		if strings.Contains(skill.Id, "chatbot") {
			found = true
			if skill.Name != "chatbot" {
				t.Errorf("expected skill name chatbot, got %s", skill.Name)
			}
		}
	}
	if !found {
		t.Errorf("expected chatbot skill, got: %v", card.Skills)
	}
}

func TestA2AGetAgentCardEmptyStore(t *testing.T) {
	sh := newEmptyStoreHolder()
	s := NewEruAIA2AServer(sh)
	projectId := "processo"
	tenantId := "39acd634-577e-41ba-aa56-df5695208696"
	card := s.GetAgentCard(context.Background(), "https://example.com", projectId, tenantId)
	if len(card.Skills) != 0 {
		t.Errorf("expected no skills for empty store, got %d", len(card.Skills))
	}
}

func TestA2AAgentCardHandler(t *testing.T) {
	sh := newTestStoreHolder()
	s := NewEruAIA2AServer(sh)
	handler := s.CreateAgentCardHandler()

	req := httptest.NewRequest(http.MethodGet, "/.well-known/agent.json", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("expected application/json content type, got %s", ct)
	}
	var card a2a.AgentCard
	if err := json.NewDecoder(w.Body).Decode(&card); err != nil {
		t.Fatalf("failed to decode agent card: %v", err)
	}
	if card.Name == "" {
		t.Error("expected non-empty card name")
	}
}

func TestA2AMessageSend(t *testing.T) {
	sh := newTestStoreHolder()
	s := NewEruAIA2AServer(sh)

	msg := a2a.Message{
		Kind:      "message",
		MessageId: "msg-1",
		Role:      "user",
		Parts:     []a2a.Part{{Kind: "text", Text: "hello"}},
		Metadata: map[string]interface{}{
			"skillId": "test-project__tenant-abc123__chatbot",
		},
	}
	params := a2a.MessageSendParams{Message: msg}
	body := buildA2ARequest("message/send", 1, params)

	w := httptest.NewRecorder()
	s.HandleMessage(context.Background(), body, w)

	resp := parseA2AResponse(t, w.Body.Bytes())
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	task := extractTask(t, resp)
	if task.Kind != "task" {
		t.Errorf("expected kind=task, got %s", task.Kind)
	}
	if task.Id == "" {
		t.Error("expected non-empty task ID")
	}
	if task.Status.State != a2a.TaskStateCompleted {
		t.Errorf("expected completed state, got %s", task.Status.State)
	}
	if len(task.History) < 2 {
		t.Errorf("expected at least 2 messages in history (user + agent), got %d", len(task.History))
	}
}

func TestA2AMessageSendFallbackSkill(t *testing.T) {
	sh := newTestStoreHolder()
	s := NewEruAIA2AServer(sh)

	msg := a2a.Message{
		Kind:      "message",
		MessageId: "msg-2",
		Role:      "user",
		Parts:     []a2a.Part{{Kind: "text", Text: "test"}},
	}
	params := a2a.MessageSendParams{Message: msg}
	body := buildA2ARequest("message/send", 2, params)

	w := httptest.NewRecorder()
	s.HandleMessage(context.Background(), body, w)

	resp := parseA2AResponse(t, w.Body.Bytes())
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	task := extractTask(t, resp)
	if task.Status.State != a2a.TaskStateCompleted {
		t.Errorf("expected completed, got %s", task.Status.State)
	}
}

func TestA2AMessageSendContextIdPreserved(t *testing.T) {
	sh := newTestStoreHolder()
	s := NewEruAIA2AServer(sh)

	msg := a2a.Message{
		Kind:      "message",
		MessageId: "msg-3",
		ContextId: "ctx-fixed-id",
		Role:      "user",
		Parts:     []a2a.Part{{Kind: "text", Text: "hello"}},
	}
	body := buildA2ARequest("message/send", 3, a2a.MessageSendParams{Message: msg})
	w := httptest.NewRecorder()
	s.HandleMessage(context.Background(), body, w)

	resp := parseA2AResponse(t, w.Body.Bytes())
	task := extractTask(t, resp)
	if task.ContextId != "ctx-fixed-id" {
		t.Errorf("expected contextId ctx-fixed-id, got %s", task.ContextId)
	}
}

func TestA2AMessageSendNoSkillNoAgents(t *testing.T) {
	sh := newEmptyStoreHolder()
	s := NewEruAIA2AServer(sh)

	msg := a2a.Message{
		Kind:      "message",
		MessageId: "msg-4",
		Role:      "user",
		Parts:     []a2a.Part{{Kind: "text", Text: "test"}},
	}
	body := buildA2ARequest("message/send", 4, a2a.MessageSendParams{Message: msg})
	w := httptest.NewRecorder()
	s.HandleMessage(context.Background(), body, w)

	resp := parseA2AResponse(t, w.Body.Bytes())
	if resp.Error == nil {
		t.Error("expected error when no agents available")
	}
}

func TestA2AMessageSendInvalidSkillId(t *testing.T) {
	sh := newTestStoreHolder()
	s := NewEruAIA2AServer(sh)

	msg := a2a.Message{
		Kind:      "message",
		MessageId: "msg-5",
		Role:      "user",
		Parts:     []a2a.Part{{Kind: "text", Text: "hello"}},
		Metadata: map[string]interface{}{
			"skillId": "test-project__tenant-abc123__nonexistent-agent",
		},
	}
	body := buildA2ARequest("message/send", 5, a2a.MessageSendParams{Message: msg})
	w := httptest.NewRecorder()
	s.HandleMessage(context.Background(), body, w)

	resp := parseA2AResponse(t, w.Body.Bytes())
	if resp.Error == nil {
		t.Error("expected error for nonexistent agent")
	}
}

func TestA2ATasksGet(t *testing.T) {
	sh := newTestStoreHolder()
	s := NewEruAIA2AServer(sh)

	msg := a2a.Message{
		Kind:      "message",
		MessageId: "msg-6",
		Role:      "user",
		Parts:     []a2a.Part{{Kind: "text", Text: "hello"}},
	}
	sendBody := buildA2ARequest("message/send", 1, a2a.MessageSendParams{Message: msg})
	w1 := httptest.NewRecorder()
	s.HandleMessage(context.Background(), sendBody, w1)
	sendResp := parseA2AResponse(t, w1.Body.Bytes())
	createdTask := extractTask(t, sendResp)

	getBody := buildA2ARequest("tasks/get", 2, a2a.TaskGetParams{ID: createdTask.Id})
	w2 := httptest.NewRecorder()
	s.HandleMessage(context.Background(), getBody, w2)
	getResp := parseA2AResponse(t, w2.Body.Bytes())
	if getResp.Error != nil {
		t.Fatalf("unexpected error: %+v", getResp.Error)
	}
	fetchedTask := extractTask(t, getResp)
	if fetchedTask.Id != createdTask.Id {
		t.Errorf("expected task ID %s, got %s", createdTask.Id, fetchedTask.Id)
	}
}

func TestA2ATasksGetNotFound(t *testing.T) {
	sh := newTestStoreHolder()
	s := NewEruAIA2AServer(sh)

	body := buildA2ARequest("tasks/get", 1, a2a.TaskGetParams{ID: "nonexistent-id"})
	w := httptest.NewRecorder()
	s.HandleMessage(context.Background(), body, w)
	resp := parseA2AResponse(t, w.Body.Bytes())
	if resp.Error == nil {
		t.Error("expected error for nonexistent task")
	}
	if resp.Error.Code != -32001 {
		t.Errorf("expected error code -32001, got %d", resp.Error.Code)
	}
}

func TestA2ATasksCancel(t *testing.T) {
	sh := newTestStoreHolder()
	s := NewEruAIA2AServer(sh)

	ts := s.taskStore
	task := &a2a.Task{
		Kind:      "task",
		Id:        "cancel-me",
		ContextId: "ctx-cancel",
		Status:    a2a.TaskStatus{State: a2a.TaskStateWorking, Timestamp: "2026-01-01T00:00:00Z"},
	}
	ts.Save(task)

	body := buildA2ARequest("tasks/cancel", 1, a2a.TaskCancelParams{ID: "cancel-me"})
	w := httptest.NewRecorder()
	s.HandleMessage(context.Background(), body, w)
	resp := parseA2AResponse(t, w.Body.Bytes())
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	cancelled := extractTask(t, resp)
	if cancelled.Status.State != a2a.TaskStateCanceled {
		t.Errorf("expected canceled state, got %s", cancelled.Status.State)
	}
}

func TestA2ATasksCancelNotFound(t *testing.T) {
	sh := newTestStoreHolder()
	s := NewEruAIA2AServer(sh)

	body := buildA2ARequest("tasks/cancel", 1, a2a.TaskCancelParams{ID: "nonexistent"})
	w := httptest.NewRecorder()
	s.HandleMessage(context.Background(), body, w)
	resp := parseA2AResponse(t, w.Body.Bytes())
	if resp.Error == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestA2AMessageStream(t *testing.T) {
	sh := newTestStoreHolder()
	s := NewEruAIA2AServer(sh)
	handler := s.CreateHttpHandler()

	msg := a2a.Message{
		Kind:      "message",
		MessageId: "msg-stream",
		Role:      "user",
		Parts:     []a2a.Part{{Kind: "text", Text: "hello"}},
	}
	params := a2a.MessageSendParams{Message: msg}
	body := buildA2ARequest("message/stream", 1, params)

	req := httptest.NewRequest(http.MethodPost, "/a2a", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Errorf("expected text/event-stream content type, got %s", ct)
	}

	var events []map[string]interface{}
	scanner := bufio.NewScanner(w.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(data), &event); err == nil {
				events = append(events, event)
			}
		}
	}

	if len(events) == 0 {
		t.Fatal("expected at least one SSE event")
	}

	var hasWorking, hasFinal bool
	for _, ev := range events {
		if status, ok := ev["status"].(map[string]interface{}); ok {
			if status["state"] == "working" {
				hasWorking = true
			}
			if status["state"] == "completed" {
				hasFinal = true
			}
		}
	}
	if !hasWorking {
		t.Error("expected working status event")
	}
	if !hasFinal {
		t.Error("expected final completed status event")
	}
}

func TestA2AUnknownMethod(t *testing.T) {
	sh := newTestStoreHolder()
	s := NewEruAIA2AServer(sh)

	body := buildA2ARequest("unknown/method", 99, nil)
	w := httptest.NewRecorder()
	s.HandleMessage(context.Background(), body, w)
	resp := parseA2AResponse(t, w.Body.Bytes())
	if resp.Error == nil {
		t.Error("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("expected -32601, got %d", resp.Error.Code)
	}
}

func TestA2AInvalidJSON(t *testing.T) {
	sh := newTestStoreHolder()
	s := NewEruAIA2AServer(sh)

	w := httptest.NewRecorder()
	s.HandleMessage(context.Background(), []byte("{invalid}"), w)
	resp := parseA2AResponse(t, w.Body.Bytes())
	if resp.Error == nil {
		t.Error("expected error for invalid JSON")
	}
	if resp.Error.Code != -32700 {
		t.Errorf("expected -32700, got %d", resp.Error.Code)
	}
}

func TestA2AInvalidProtocolVersion(t *testing.T) {
	sh := newTestStoreHolder()
	s := NewEruAIA2AServer(sh)

	req := a2a.JSONRPCRequest{JSONRPCVersion: "1.0", ID: 1, Method: "message/send"}
	body, _ := json.Marshal(req)
	w := httptest.NewRecorder()
	s.HandleMessage(context.Background(), body, w)
	resp := parseA2AResponse(t, w.Body.Bytes())
	if resp.Error == nil {
		t.Error("expected error for wrong JSON-RPC version")
	}
	if resp.Error.Code != -32600 {
		t.Errorf("expected -32600, got %d", resp.Error.Code)
	}
}

func TestA2AHttpHandler(t *testing.T) {
	sh := newTestStoreHolder()
	s := NewEruAIA2AServer(sh)
	handler := s.CreateHttpHandler()

	msg := a2a.Message{
		Kind:      "message",
		MessageId: "msg-http",
		Role:      "user",
		Parts:     []a2a.Part{{Kind: "text", Text: "hello"}},
	}
	body := buildA2ARequest("message/send", 1, a2a.MessageSendParams{Message: msg})
	req := httptest.NewRequest(http.MethodPost, "/a2a", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	resp := parseA2AResponse(t, w.Body.Bytes())
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
}

func newEmptyStoreHolder() *module_store.StoreHolder {
	ms := newMockModuleStore()
	return &module_store.StoreHolder{Store: ms}
}
