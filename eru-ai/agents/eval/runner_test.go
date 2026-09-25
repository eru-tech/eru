package eval

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	models "github.com/eru-tech/eru/eru-ai/models"
)

// stubAgent stands in for eru-ai: it records what the runner sent and replies
// with a trajectory of its own choosing. The runner under test is the real one,
// so the request shape, the raw=true requirement, the decoding and the scoring
// are all exercised for real - only the model is absent.
func stubAgent(t *testing.T, reply agents.AgentMessage, seen *http.Request, body *agents.AgentMessage) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = *r
		_ = json.NewDecoder(r.Body).Decode(body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(reply)
	}))
}

func TestRunnerCallsTheProductEndpointAndScoresTheReply(t *testing.T) {
	reply := agents.AgentMessage{
		Actions: []agents.AgentOutputAction{{ActionType: "question"}},
		Traces: []models.StepTrace{
			{Iteration: 1, ToolName: "get_processo_context", Agent: "processo_builder"},
			{Iteration: 2, ToolName: "get_field_spec", Agent: "processo_builder"},
		},
	}
	var seen http.Request
	var sent agents.AgentMessage
	server := stubAgent(t, reply, &seen, &sent)
	defer server.Close()

	runner := &Runner{BaseURL: server.URL, Project: "processo", Tenant: "t-1", Claims: `{"sub":"u"}`, Client: server.Client()}
	fixture, _ := Scenarios().ByName("attachment_without_storage_asks")

	traj, err := runner.Run(context.Background(), fixture)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	// The endpoint the product itself calls, with the tenant and agent in it.
	if !strings.HasSuffix(seen.URL.Path, "/processo/t-1/execute/agent/processo_builder") {
		t.Errorf("called %q", seen.URL.Path)
	}
	// Without raw=true the traces never come back and every assertion about what
	// the agent DID would silently have nothing to read.
	if seen.URL.Query().Get("raw") != "true" {
		t.Error("raw=true was not requested")
	}
	if seen.Header.Get("claims") == "" {
		t.Error("the claims header was not sent")
	}
	// The prompt is the fixture's, with any {{sample}} resolved for this attempt -
	// what the agent is actually asked, not the template.
	if sent.Content != fixture.PromptFor(1) {
		t.Errorf("prompt sent = %q, want %q", sent.Content, fixture.PromptFor(1))
	}
	if strings.Contains(sent.Content, "{{sample}}") {
		t.Error("the placeholder reached the agent unsubstituted")
	}

	if len(traj.ToolCalls) != 2 || traj.Iterations != 2 {
		t.Errorf("trajectory = %d calls, %d iterations", len(traj.ToolCalls), traj.Iterations)
	}
	if result := fixture.Score(traj); !result.OK() {
		t.Errorf("this stub asked and wrote nothing, so it should pass: %s", result)
	}
}

// A run that wrote instead of asking must fail, through the real runner.
func TestRunnerScoresAFailingRun(t *testing.T) {
	reply := agents.AgentMessage{
		Actions: []agents.AgentOutputAction{{ActionType: agents.ActionTypeAnswer, Action: map[string]interface{}{"summary": "done"}}},
		Traces: []models.StepTrace{{Iteration: 3, ToolName: "save_field", Agent: "processo_builder",
			ToolInput: map[string]interface{}{"field": map[string]interface{}{"storage_name": "default"}}}},
	}
	var seen http.Request
	var sent agents.AgentMessage
	server := stubAgent(t, reply, &seen, &sent)
	defer server.Close()

	runner := &Runner{BaseURL: server.URL, Project: "processo", Tenant: "t-1", Claims: "{}", Client: server.Client()}
	fixture, _ := Scenarios().ByName("attachment_without_storage_asks")

	traj, err := runner.Run(context.Background(), fixture)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	result := fixture.Score(traj)
	if result.OK() {
		t.Fatal("a run that invented a storage name must fail")
	}
	if !strings.Contains(result.String(), "save_field") {
		t.Errorf("the report should name the write: %s", result)
	}
}

// Recording then replaying must score identically, or the offline suite is lying.
func TestRecordedRunReplaysToTheSameScore(t *testing.T) {
	reply := agents.AgentMessage{
		Actions: []agents.AgentOutputAction{{ActionType: "question"}},
		Traces:  []models.StepTrace{{Iteration: 1, ToolName: "get_field_spec", Agent: "processo_builder"}},
	}
	var seen http.Request
	var sent agents.AgentMessage
	server := stubAgent(t, reply, &seen, &sent)
	defer server.Close()

	dir := t.TempDir()
	runner := &Runner{BaseURL: server.URL, Project: "processo", Tenant: "t-1", Claims: "{}", Client: server.Client()}
	fixture, _ := Scenarios().ByName("attachment_without_storage_asks")

	live, err := runner.RunAll(context.Background(), Suite{fixture}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, fixture.Name+".json")); err != nil {
		t.Fatalf("the run was not recorded: %v", err)
	}

	replayed, err := ScoreRecorded(dir, Scenarios())
	if err != nil {
		t.Fatal(err)
	}
	if len(replayed) != 1 || replayed[0].Passed != live[0].Passed || !replayed[0].OK() == live[0].OK() {
		t.Errorf("replay scored %+v, live scored %+v", replayed, live)
	}
}

// A reply with nothing in it is reported as such, rather than scoring green by
// virtue of having no evidence against it.
func TestEmptyReplyIsAnError(t *testing.T) {
	var seen http.Request
	var sent agents.AgentMessage
	server := stubAgent(t, agents.AgentMessage{}, &seen, &sent)
	defer server.Close()

	runner := &Runner{BaseURL: server.URL, Project: "processo", Tenant: "t-1", Claims: "{}", Client: server.Client()}
	fixture, _ := Scenarios().ByName("attachment_without_storage_asks")
	if _, err := runner.Run(context.Background(), fixture); err == nil {
		t.Error("a reply with no traces and no actions should be an error, not a pass")
	}
}

// The claims header is the id_token's payload, which is what the gateway would
// have written.
func TestClaimsAreTheIdTokenPayload(t *testing.T) {
	// {"sub":"u1","identity":{"attributes":{"email":"a@b.c"}}}
	payload := "eyJzdWIiOiJ1MSIsImlkZW50aXR5Ijp7ImF0dHJpYnV0ZXMiOnsiZW1haWwiOiJhQGIuYyJ9fX0="
	claims, err := claimsFromIdToken("header." + strings.TrimRight(payload, "=") + ".signature")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(claims), &parsed); err != nil {
		t.Fatalf("claims are not JSON: %v", err)
	}
	if parsed["sub"] != "u1" {
		t.Errorf("claims = %s", claims)
	}
}

func TestLoginReportsRefusalPlainly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid credentials - please try again"}`))
	}))
	defer server.Close()

	_, err := Login(context.Background(), server.Client(), server.URL, "processo", "someone@example.com", "wrong")
	if err == nil || !strings.Contains(err.Error(), "refused") {
		t.Errorf("a refusal should say so plainly, got %v", err)
	}
}

// The password is hashed before it leaves the client. Sending the raw one is
// refused as "invalid credentials" with no hint as to why, which cost an
// afternoon once.
func TestLoginSendsTheHashedPassword(t *testing.T) {
	var sent struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&sent)
		// header.{"sub":"u1"}.signature
		_, _ = w.Write([]byte(`{"id_token":"h.eyJzdWIiOiJ1MSJ9.s"}`))
	}))
	defer server.Close()

	claims, err := Login(context.Background(), server.Client(), server.URL, "processo", "a@b.c", "Eru@123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if sent.Password == "Eru@123" {
		t.Fatal("the raw password was sent")
	}
	// SHA-512 hex is 128 characters.
	if len(sent.Password) != 128 {
		t.Errorf("password length = %d, want a 128-char SHA-512 hex digest", len(sent.Password))
	}
	if !strings.Contains(claims, `"sub":"u1"`) {
		t.Errorf("claims = %s", claims)
	}
}
