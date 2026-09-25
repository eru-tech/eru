package reasoning_agents

import (
	"context"
	"testing"

	"github.com/eru-tech/eru/eru-ai/agents"
)

// Every exit the loop has, named. Unit-testing Budget proves the arithmetic;
// these prove the loop actually reaches for it, which is the part that rots.

func stopReasonOf(t *testing.T, retryCount int, budget agents.Budget, provider agents.SystemPromptProvider, answers ...string) (agents.AgentMessage, error) {
	t.Helper()
	ra := &ReasoningAgent{}
	ra.AgentName = "stopper"
	ra.RetryCount = retryCount
	ra.Budget = budget
	ra.Model = &scriptedModel{answers: answers}
	if provider != nil {
		ra.SetProvider(provider)
	}
	return ra.Execute(context.Background(), agents.AgentMessage{Content: "go"}, "", "p", "t")
}

func TestAFinishedRunSaysEndTurn(t *testing.T) {
	out, err := stopReasonOf(t, 1, agents.Budget{}, nil, `{"v":"ok"}`)
	if err != nil {
		t.Fatal(err)
	}
	if out.StopReason != agents.StopEndTurn {
		t.Errorf("StopReason = %q, want end_turn", out.StopReason)
	}
}

func TestAnAttemptLimitStopsTheLoopAndSaysSo(t *testing.T) {
	provider := &scriptedProvider{invalid: map[string]bool{"a": true, "b": true, "c": true}}
	// RetryCount would allow four attempts; the budget allows two.
	out, err := stopReasonOf(t, 3, agents.Budget{MaxAttempts: 2}, provider,
		`{"v":"a"}`, `{"v":"b"}`, `{"v":"c"}`, `{"v":"d"}`)
	if err == nil {
		t.Fatal("nothing was ever valid, so there is nothing to deliver")
	}
	if out.StopReason != "" {
		t.Errorf("a failed run returns no message to carry a reason, got %q", out.StopReason)
	}
	// The reason still has to reach the caller somehow.
	if err.Error() == "" {
		t.Error("the error must say why")
	}
}

// A budget that bites after something deliverable exists must deliver it, and
// must not call the result a success.
func TestALimitAfterAGoodAnswerDeliversItAndSaysItWasCutShort(t *testing.T) {
	provider := &scriptedProvider{poor: map[string]bool{"first": true}}
	// Attempt 1 is valid but poor, so the gate sends it back and spends the
	// attempt; the budget then stops the loop before attempt 2 can be judged.
	out, err := stopReasonOf(t, 3, agents.Budget{MaxAttempts: 1}, provider, `{"v":"first"}`, `{"v":"second"}`)
	if err != nil {
		t.Fatalf("an answer existed and must be delivered: %v", err)
	}
	if out.StopReason != agents.StopLimitAttempts {
		t.Errorf("StopReason = %q, want limit_attempts", out.StopReason)
	}
	if answeredWith(out) != "first" {
		t.Errorf("the remembered answer must be the one delivered, got %q", answeredWith(out))
	}
	if final := out.Metrics.Quality[len(out.Metrics.Quality)-1]; final.Enforced {
		t.Error("the verdict could not be acted on and must not claim it was")
	}
}

// Falling back to an answer the gate rejected is not the same as finishing, and
// a report that calls it end_turn is flattering.
func TestRecoveringAnEarlierAnswerHasItsOwnReason(t *testing.T) {
	provider := &scriptedProvider{
		poor:    map[string]bool{"valid_but_plain": true},
		invalid: map[string]bool{"worse": true, "worse2": true},
	}
	out, err := stopReasonOf(t, 1, agents.Budget{}, provider,
		`{"v":"valid_but_plain"}`, `{"v":"worse"}`, `{"v":"worse2"}`)
	if err != nil {
		t.Fatal(err)
	}
	if out.StopReason != agents.StopRecoveredEarlierAnswer {
		t.Errorf("StopReason = %q, want recovered_earlier_answer", out.StopReason)
	}
	if answeredWith(out) != "valid_but_plain" {
		t.Errorf("delivered %q", answeredWith(out))
	}
}

// A run nobody is waiting for must stop calling the model.
func TestACancelledCallerStopsTheLoop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	model := &scriptedModel{answers: []string{`{"v":"ok"}`}}
	ra := &ReasoningAgent{}
	ra.AgentName = "stopper"
	ra.Model = model
	if _, err := ra.Execute(ctx, agents.AgentMessage{Content: "go"}, "", "p", "t"); err == nil {
		t.Fatal("a cancelled run must not report success")
	}
	if model.calls != 0 {
		t.Errorf("the model must not be called for a caller that went away; called %d times", model.calls)
	}
}

// An agent that declares no budget behaves exactly as it did before budgets
// existed - the regression that would be easiest to ship and hardest to notice.
func TestNoBudgetChangesNothing(t *testing.T) {
	provider := &scriptedProvider{invalid: map[string]bool{"a": true}}
	out, err := stopReasonOf(t, 2, agents.Budget{}, provider, `{"v":"a"}`, `{"v":"b"}`)
	if err != nil {
		t.Fatal(err)
	}
	if out.StopReason != agents.StopEndTurn || out.RetryCount != 1 {
		t.Errorf("StopReason %q, RetryCount %d; want end_turn and 1", out.StopReason, out.RetryCount)
	}
}
