package reasoning_agents

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/eru-tech/eru/eru-ai/agents"
	"github.com/eru-tech/eru/eru-ai/models"
	"github.com/eru-tech/eru/eru-ai/tools"
	eru_models "github.com/eru-tech/eru/eru-models"
)

// The gate changes the inner loop's control flow, and control flow is not
// provable from the helper alone. These drive the real Execute against a model
// that answers from a script, which is the only way to see how the two budgets
// interact across attempts.

type scriptedModel struct {
	models.Model
	answers []string
	// callTool makes the model use a tool on its first turn, which is the only
	// way to exercise anything collected at the tool boundary.
	callTool   bool
	toolAction string
	toolArgs   map[string]interface{}
	// prompts is every user message the loop appended before each call, so a
	// test can assert the agent was actually TOLD what to fix.
	prompts []string
	calls   int
}

func (m *scriptedModel) RunToolLoop(ctx context.Context, chatRequest models.ChatRequest, toolsMap map[string]tools.Tooling, agentPrompt models.AgentPrompt, maxIterations int, thinkingBudget int, toolExecutor models.ToolExecutor) (models.Message, []models.StepTrace, error) {
	if len(chatRequest.Messages) > 0 {
		m.prompts = append(m.prompts, chatRequest.Messages[len(chatRequest.Messages)-1].Content)
	}
	if m.callTool && m.calls == 0 {
		if tool, ok := toolsMap[m.toolAction]; ok {
			_, _, _ = tool.Execute(ctx, "p", "t", m.toolAction, m.toolArgs)
		}
	}
	if m.calls >= len(m.answers) {
		return models.Message{}, nil, errors.New("the loop asked for more answers than the script has - it is not terminating")
	}
	answer := m.answers[m.calls]
	m.calls++
	return models.Message{Role: "assistant", Content: answer}, nil, nil
}

// scriptedProvider validates and judges from a script keyed on the answer's
// "v" field, so a test states what each attempt is worth rather than counting.
type scriptedProvider struct {
	// invalid are the answer tags ValidateOutput rejects.
	invalid map[string]bool
	// poor are the answer tags the judge rates poor.
	poor        map[string]bool
	judgeCalls  int
	validations int
}

func (scriptedProvider) GetSystemPrompt() string { return "" }
func (scriptedProvider) GetOutputSchema(ctx context.Context) eru_models.JSONSchema {
	return eru_models.JSONSchema{}
}

func tagOf(output map[string]interface{}) string {
	tag, _ := output["v"].(string)
	return tag
}

func (p *scriptedProvider) ValidateOutput(ctx context.Context, output map[string]interface{}) error {
	p.validations++
	if p.invalid[tagOf(output)] {
		return fmt.Errorf("component type %q is not in the library", tagOf(output))
	}
	return nil
}

func (p *scriptedProvider) JudgeQuality(ctx context.Context, output map[string]interface{}) (agents.QualityVerdict, error) {
	p.judgeCalls++
	if p.poor[tagOf(output)] {
		return agents.QualityVerdict{Rating: agents.QualityPoor, Reason: "the grid headings are raw column names"}, nil
	}
	return agents.QualityVerdict{Rating: agents.QualityGood}, nil
}

func runScript(t *testing.T, retryCount int, provider agents.SystemPromptProvider, answers ...string) (agents.AgentMessage, *scriptedModel, error) {
	t.Helper()
	model := &scriptedModel{answers: answers}
	ra := &ReasoningAgent{}
	ra.AgentName = "scripted"
	ra.RetryCount = retryCount
	ra.Model = model
	ra.SetProvider(provider)
	out, err := ra.Execute(context.Background(), agents.AgentMessage{Content: "build it"}, "", "p", "t")
	return out, model, err
}

func answeredWith(out agents.AgentMessage) string {
	if len(out.Actions) == 0 {
		return ""
	}
	return tagOf(out.Actions[0].Action)
}

// The gate's basic contract, end to end: a poor first answer is sent back and
// the improved one is what the caller receives.
func TestAPoorAnswerIsRegeneratedAndTheBetterOneDelivered(t *testing.T) {
	provider := &scriptedProvider{poor: map[string]bool{"first": true}}
	out, model, err := runScript(t, 1, provider, `{"v":"first"}`, `{"v":"second"}`)
	if err != nil {
		t.Fatal(err)
	}
	if model.calls != 2 {
		t.Errorf("the model should have been called twice, was called %d", model.calls)
	}
	if answeredWith(out) != "second" {
		t.Errorf("the improved answer must be the one delivered, got %q", answeredWith(out))
	}
	if len(model.prompts) < 2 || !strings.Contains(model.prompts[1], "not a rejection") {
		t.Errorf("the second call must carry the quality feedback, got %q", model.prompts)
	}
}

// The regression this whole two-counter change exists for.
//
// RetryCount is 1. The first answer is valid but poor, so the gate spends its
// own budget. The second answer then fails VALIDATION - and the conformance
// budget must still be untouched, because taste did not consume it. A single
// shared counter fails the run here.
func TestAQualityRetryDoesNotConsumeTheConformanceBudget(t *testing.T) {
	provider := &scriptedProvider{
		poor:    map[string]bool{"first": true},
		invalid: map[string]bool{"second": true},
	}
	out, model, err := runScript(t, 1, provider, `{"v":"first"}`, `{"v":"second"}`, `{"v":"third"}`)
	if err != nil {
		t.Fatalf("the conformance retry was still owed and must have been granted: %v", err)
	}
	if model.calls != 3 {
		t.Errorf("expected 3 attempts (poor, invalid, good), got %d", model.calls)
	}
	if answeredWith(out) != "third" {
		t.Errorf("delivered %q, want third", answeredWith(out))
	}
	if out.RetryCount != 2 {
		t.Errorf("both retries should be reported to the caller, got RetryCount %d", out.RetryCount)
	}
}

// And the converse: conformance keeps its own ceiling. Taste cannot lend it an
// extra attempt.
func TestTheConformanceBudgetIsStillEnforced(t *testing.T) {
	provider := &scriptedProvider{invalid: map[string]bool{"first": true, "second": true}}
	_, model, err := runScript(t, 1, provider, `{"v":"first"}`, `{"v":"second"}`, `{"v":"third"}`)
	if err == nil {
		t.Fatal("two validation failures with RetryCount 1 must fail the run")
	}
	if model.calls != 2 {
		t.Errorf("it must stop after 2 attempts, made %d", model.calls)
	}
}

// One extra attempt, not N. A judge that is never satisfied must not be able to
// bill for it.
func TestAJudgeThatIsNeverSatisfiedStillStops(t *testing.T) {
	provider := &scriptedProvider{poor: map[string]bool{"a": true, "b": true, "c": true}}
	out, model, err := runScript(t, 3, provider, `{"v":"a"}`, `{"v":"b"}`, `{"v":"c"}`)
	if err != nil {
		t.Fatalf("the answer must still be delivered: %v", err)
	}
	if model.calls != 2 {
		t.Errorf("the gate gets exactly one extra attempt regardless of RetryCount, took %d", model.calls)
	}
	if answeredWith(out) != "b" {
		t.Errorf("the second answer ships even though it is still poor, got %q", answeredWith(out))
	}
}

// An answer that shipped below the bar is the case worth finding later, so the
// unenforced verdict has to survive into the metrics.
func TestEveryVerdictReachesTheMetrics(t *testing.T) {
	provider := &scriptedProvider{poor: map[string]bool{"a": true, "b": true}}
	out, _, err := runScript(t, 3, provider, `{"v":"a"}`, `{"v":"b"}`)
	if err != nil {
		t.Fatal(err)
	}
	if out.Metrics == nil || len(out.Metrics.Quality) != 2 {
		t.Fatalf("both verdicts must be recorded: %+v", out.Metrics)
	}
	first, second := out.Metrics.Quality[0], out.Metrics.Quality[1]
	if !first.Enforced || first.Attempt != 1 {
		t.Errorf("the first verdict was acted on: %+v", first)
	}
	if second.Enforced {
		t.Error("the second could not be acted on and must not claim it was")
	}
	if second.Rating != agents.QualityPoor || second.Attempt != 2 {
		t.Errorf("the shipped-below-bar verdict must survive intact: %+v", second)
	}
}

// The gate is opt-in: an agent type that does not judge must behave exactly as
// it did before this existed.
func TestAnUnjudgedAgentRunsExactlyOnce(t *testing.T) {
	out, model, err := runScript(t, 1, plainProvider{}, `{"v":"only"}`)
	if err != nil {
		t.Fatal(err)
	}
	if model.calls != 1 {
		t.Errorf("no judge means no extra attempt, took %d", model.calls)
	}
	if out.Metrics != nil && len(out.Metrics.Quality) != 0 {
		t.Errorf("and nothing to record: %+v", out.Metrics.Quality)
	}
}

// A good answer is not judged twice and not regenerated.
func TestAGoodAnswerIsDeliveredOnTheFirstAttempt(t *testing.T) {
	provider := &scriptedProvider{}
	out, model, err := runScript(t, 2, provider, `{"v":"good"}`)
	if err != nil {
		t.Fatal(err)
	}
	if model.calls != 1 || provider.judgeCalls != 1 {
		t.Errorf("model calls %d, judge calls %d; want 1 and 1", model.calls, provider.judgeCalls)
	}
	if out.RetryCount != 0 {
		t.Errorf("RetryCount = %d, want 0", out.RetryCount)
	}
}

// An invalid answer must never reach the judge: judging malformed work spends a
// model call to be told what validation already knows.
func TestTheJudgeNeverSeesAnAnswerThatFailedValidation(t *testing.T) {
	provider := &scriptedProvider{invalid: map[string]bool{"bad": true}}
	if _, _, err := runScript(t, 2, provider, `{"v":"bad"}`, `{"v":"ok"}`); err != nil {
		t.Fatal(err)
	}
	if provider.validations != 2 {
		t.Errorf("both answers should have been validated, got %d", provider.validations)
	}
	if provider.judgeCalls != 1 {
		t.Errorf("only the valid answer should have been judged, judge ran %d times", provider.judgeCalls)
	}
}

// The failure the first live run produced, as a test.
//
// Attempt 1 was a valid dashboard. The gate sent it back over one missing chart
// title. The regenerated answer dropped required keys, then invented properties,
// then unbound every query - four conformance attempts, all failed, and the
// request returned an error. The user asked for a dashboard, we HAD one, and the
// gate threw it away for a chart title.
//
// A gate that cannot veto must not be able to do that by the back door.
func TestTheGateNeverTurnsADeliverableAnswerIntoAFailure(t *testing.T) {
	provider := &scriptedProvider{
		poor:    map[string]bool{"valid_but_plain": true},
		invalid: map[string]bool{"worse": true, "worse2": true, "worse3": true},
	}
	out, model, err := runScript(t, 2, provider,
		`{"v":"valid_but_plain"}`, `{"v":"worse"}`, `{"v":"worse2"}`, `{"v":"worse3"}`)
	if err != nil {
		t.Fatalf("the request must not fail when a valid answer was already in hand: %v", err)
	}
	if answeredWith(out) != "valid_but_plain" {
		t.Errorf("the remembered answer must be delivered, got %q", answeredWith(out))
	}
	if model.calls != 4 {
		t.Errorf("one quality attempt plus three conformance attempts, got %d", model.calls)
	}
	if out.Metrics == nil || len(out.Metrics.Quality) == 0 {
		t.Fatal("the verdict must still be on the record")
	}
	final := out.Metrics.Quality[len(out.Metrics.Quality)-1]
	if final.Rating != agents.QualityPoor {
		t.Errorf("the answer shipped below the bar and must say so: %+v", final)
	}
	if final.Enforced {
		t.Error("and the verdict was NOT acted on in the end, so it must not claim it was")
	}
}

// Without a gate in the picture there is nothing to fall back to, and an
// exhausted conformance budget must still fail the run.
func TestAnExhaustedBudgetStillFailsWhenNothingValidWasEverProduced(t *testing.T) {
	provider := &scriptedProvider{invalid: map[string]bool{"a": true, "b": true}}
	if _, _, err := runScript(t, 1, provider, `{"v":"a"}`, `{"v":"b"}`); err == nil {
		t.Fatal("with no valid answer to fall back on the run must fail")
	}
}

// The fallback is for the gate's own gamble, not a general safety net: an answer
// that was never accepted is not something to fall back to.
func TestTheFallbackOnlyAppliesToAnAnswerTheGateRejected(t *testing.T) {
	provider := &scriptedProvider{
		poor:    map[string]bool{},
		invalid: map[string]bool{"b": true, "c": true},
	}
	// "a" is valid and good, so the loop breaks on it immediately and never
	// reaches the conformance path at all.
	out, model, err := runScript(t, 2, provider, `{"v":"a"}`, `{"v":"b"}`, `{"v":"c"}`)
	if err != nil {
		t.Fatal(err)
	}
	if model.calls != 1 || answeredWith(out) != "a" {
		t.Errorf("a good answer ends the loop on the first attempt: %d calls, delivered %q", model.calls, answeredWith(out))
	}
}

// repairingProvider answers a quality verdict with a patch, like eru_studio.
type repairingProvider struct {
	scriptedProvider
	repairCalls int
	prompt      string
	decline     bool
}

func (p *repairingProvider) QualityRepairTurn(ctx context.Context, accepted map[string]interface{}, verdict agents.QualityVerdict) (agents.RepairTurn, bool) {
	p.repairCalls++
	if p.decline {
		return agents.RepairTurn{}, false
	}
	return agents.RepairTurn{Prompt: "PATCH ONLY: " + verdict.Reason}, true
}

// The loop has to ASK. Everything about the patch path is worthless if the only
// thing between the verdict and the agent is a type assertion nobody exercised.
func TestTheLoopAsksForAPatchWhenTheAgentTypeCanGiveOne(t *testing.T) {
	provider := &repairingProvider{scriptedProvider: scriptedProvider{poor: map[string]bool{"first": true}}}
	_, model, err := runScript(t, 1, provider, `{"v":"first"}`, `{"v":"second"}`)
	if err != nil {
		t.Fatal(err)
	}
	if provider.repairCalls != 1 {
		t.Fatalf("the loop must offer the patch path exactly once, offered %d", provider.repairCalls)
	}
	if len(model.prompts) < 2 || !strings.HasPrefix(model.prompts[1], "PATCH ONLY:") {
		t.Errorf("the agent type's prompt must be the one sent, got %q", model.prompts[1])
	}
}

// Declining is always safe: the loop falls back to asking for the whole answer.
func TestDecliningThePatchFallsBackToTheWholeAnswer(t *testing.T) {
	provider := &repairingProvider{
		scriptedProvider: scriptedProvider{poor: map[string]bool{"first": true}},
		decline:          true,
	}
	_, model, err := runScript(t, 1, provider, `{"v":"first"}`, `{"v":"second"}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(model.prompts) < 2 || !strings.Contains(model.prompts[1], "not a rejection") {
		t.Errorf("a declined patch must fall back to the standard quality prompt, got %q", model.prompts[1])
	}
}
