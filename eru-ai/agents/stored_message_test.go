package agents

import (
	"strings"
	"testing"
)

func TestAssistantTurnKeepsItsProseWhenRemembered(t *testing.T) {
	msg := AgentMessage{
		Role: "assistant",
		Actions: []AgentOutputAction{{
			ActionType: ActionTypeAnswer,
			ActionName: "processo_scf",
			Action:     map[string]interface{}{"response": "Done! The title is yellow now."},
		}},
	}

	stored := msg.forStorage()
	if stored.Content != "Done! The title is yellow now." {
		t.Fatalf("Content = %q - a remembered assistant turn with no Content renders as a blank bubble and reaches the model as a blank line", stored.Content)
	}
}


func TestBothHalvesOfAnExchangeKeepDistinctIds(t *testing.T) {
	seen := map[string]int{}

	user := AgentMessage{Role: "user", MessageId: "m1"}
	assistant := AgentMessage{Role: "assistant", MessageId: "m1"}

	userId := storageMessageId(user, seen)
	assistantId := storageMessageId(assistant, seen)

	if userId != "m1" {
		t.Fatalf("the first message should keep its id, got %q", userId)
	}
	if assistantId == userId {
		t.Fatalf("both halves stored under %q - a reader that keys on the id keeps the question and drops the answer", userId)
	}
	if assistantId != "m1#assistant" {
		t.Fatalf("assistant id = %q", assistantId)
	}
}

func TestAMessageWithNoIdIsLeftAlone(t *testing.T) {
	if got := storageMessageId(AgentMessage{Role: "user"}, map[string]int{}); got != "" {
		t.Fatalf("invented an id: %q", got)
	}
}

func TestArtifactPrefersTheCodeTheClientSent(t *testing.T) {
	msg := AgentMessage{
		Code:    `{"page":1}`,
		Actions: []AgentOutputAction{{ActionType: ActionTypeAnswer, Action: map[string]interface{}{"response": "ok"}}},
	}
	if got := artifactOf(msg); got != `{"page":1}` {
		t.Fatalf("artifactOf = %q", got)
	}
}

func TestArtifactIsFoundWhereTheOrchestratorReceivesIt(t *testing.T) {
	// The orchestrator is handed the baseline as params.code and its own Code is
	// empty. If the working set only looks at Code, the orchestrator remembers
	// nothing and asks the user for a value it was already sent.
	msg := AgentMessage{
		Role: "user",
		Params: map[string]interface{}{
			"code": map[string]interface{}{"components": []interface{}{map[string]interface{}{"id": "tb-title"}}},
		},
	}
	got := artifactOf(msg)
	if got == "" {
		t.Fatal("the baseline in params.code was not recognised as the turn's artifact")
	}
	if !strings.Contains(got, "tb-title") {
		t.Fatalf("artifactOf = %q", got)
	}
}

func TestArtifactAcceptsParamsCodeAsAString(t *testing.T) {
	msg := AgentMessage{Role: "user", Params: map[string]interface{}{"code": `{"components":[]}`}}
	if got := artifactOf(msg); got != `{"components":[]}` {
		t.Fatalf("artifactOf = %q", got)
	}
}
