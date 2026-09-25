package reasoning_agents

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/eru-tech/eru/eru-ai/agents"
	eru_models "github.com/eru-tech/eru/eru-models"
)

type fakeJudge struct {
	verdicts []agents.QualityVerdict
	err      error
	calls    int
}

func (f *fakeJudge) GetSystemPrompt() string { return "" }
func (f *fakeJudge) GetOutputSchema(ctx context.Context) eru_models.JSONSchema {
	return eru_models.JSONSchema{}
}
func (f *fakeJudge) JudgeQuality(ctx context.Context, output map[string]interface{}) (agents.QualityVerdict, error) {
	f.calls++
	if f.err != nil {
		return agents.QualityVerdict{}, f.err
	}
	if len(f.verdicts) == 0 {
		return agents.QualityVerdict{Rating: agents.QualityGood}, nil
	}
	v := f.verdicts[0]
	if len(f.verdicts) > 1 {
		f.verdicts = f.verdicts[1:]
	}
	return v, nil
}

// A provider that is not a judge at all - the default, and it must be left
// completely alone.
type plainProvider struct{}

func (plainProvider) GetSystemPrompt() string { return "" }
func (plainProvider) GetOutputSchema(ctx context.Context) eru_models.JSONSchema {
	return eru_models.JSONSchema{}
}

func agentWith(provider agents.SystemPromptProvider) *ReasoningAgent {
	ra := &ReasoningAgent{}
	ra.AgentName = "test_agent"
	ra.SetProvider(provider)
	return ra
}

func TestAnAgentTypeWithNoJudgeIsNotGated(t *testing.T) {
	ra := agentWith(plainProvider{})
	verdict, judged := ra.judgeQuality(context.Background(), map[string]interface{}{}, 0, true)
	if judged {
		t.Error("an agent with no judge must not produce a verdict")
	}
	if verdict.Enforced {
		t.Error("and must never be sent back")
	}
}

func TestAPoorVerdictSendsTheAnswerBack(t *testing.T) {
	judge := &fakeJudge{verdicts: []agents.QualityVerdict{
		{Rating: agents.QualityPoor, Reason: "the grid headings are the raw column names an, cl - label them"},
	}}
	verdict, judged := agentWith(judge).judgeQuality(context.Background(), map[string]interface{}{}, 0, true)
	if !judged || !verdict.Enforced {
		t.Fatalf("a poor verdict with budget must be enforced: %+v", verdict)
	}
	if verdict.Attempt != 1 {
		t.Errorf("attempt = %d, want 1", verdict.Attempt)
	}
}

// The whole reason the rating is an enum and not a number: there has to be a
// state that means "imperfect but ship it", or the loop never settles.
func TestAcceptableDoesNotSpendAnAttempt(t *testing.T) {
	judge := &fakeJudge{verdicts: []agents.QualityVerdict{
		{Rating: agents.QualityAcceptable, Reason: "the tile order could be better"},
	}}
	verdict, judged := agentWith(judge).judgeQuality(context.Background(), map[string]interface{}{}, 0, true)
	if !judged {
		t.Fatal("the verdict must still be recorded")
	}
	if verdict.Enforced {
		t.Error("acceptable must not trigger a retry")
	}
}

func TestGoodShipsImmediately(t *testing.T) {
	verdict, judged := agentWith(&fakeJudge{}).judgeQuality(context.Background(), map[string]interface{}{}, 0, true)
	if !judged || verdict.Rating != agents.QualityGood || verdict.Enforced {
		t.Errorf("a good verdict ships as it is: %+v", verdict)
	}
}

// On the last attempt the gate annotates rather than vetoes. Delivering
// something imperfect beats failing the request outright.
func TestWithNoBudgetLeftAPoorVerdictIsRecordedAndNotEnforced(t *testing.T) {
	judge := &fakeJudge{verdicts: []agents.QualityVerdict{
		{Rating: agents.QualityPoor, Reason: "still no column labels"},
	}}
	verdict, judged := agentWith(judge).judgeQuality(context.Background(), map[string]interface{}{}, 1, false)
	if !judged {
		t.Fatal("the verdict must be recorded even when it cannot be acted on")
	}
	if verdict.Enforced {
		t.Error("with no budget left the answer must ship")
	}
	if verdict.Rating != agents.QualityPoor {
		t.Error("and it must still say the answer was poor - that is the point of recording it")
	}
}

// A check that could not run is not a check that failed.
func TestAJudgeThatErrorsLeavesTheAnswerAlone(t *testing.T) {
	judge := &fakeJudge{err: errors.New("model unreachable")}
	verdict, judged := agentWith(judge).judgeQuality(context.Background(), map[string]interface{}{}, 0, true)
	if judged {
		t.Error("a failed judge must not produce a verdict")
	}
	if verdict.Enforced {
		t.Error("and must never cost an attempt")
	}
}

// Sending "not good enough" back without naming the fault asks the model to
// guess, and spends the one attempt doing it.
func TestAPoorVerdictWithNoReasonIsNotEnforced(t *testing.T) {
	judge := &fakeJudge{verdicts: []agents.QualityVerdict{{Rating: agents.QualityPoor, Reason: "   "}}}
	verdict, _ := agentWith(judge).judgeQuality(context.Background(), map[string]interface{}{}, 0, true)
	if verdict.Enforced {
		t.Error("a verdict with nothing to act on must not trigger a retry")
	}
}

func TestTheRetryPromptTellsTheModelItsAnswerWasValid(t *testing.T) {
	prompt := agents.QualityRetryPrompt
	for _, phrase := range []string{
		"every structural check passed",
		"This is not a rejection",
		"ONE attempt",
	} {
		if !strings.Contains(prompt, phrase) {
			t.Errorf("the quality retry prompt no longer says %q - without it the model re-checks what already passed", phrase)
		}
	}
}

// The budget is the guard against an unbounded taste loop, so it is asserted
// rather than left as a convention.
func TestTheQualityBudgetIsOne(t *testing.T) {
	if agents.QualityGateBudget != 1 {
		t.Errorf("QualityGateBudget = %d; raising it reintroduces the unbounded retry the design rejects", agents.QualityGateBudget)
	}
}
