package reasoning_agents

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/eru-tech/eru/eru-ai/agents"
	eru_models "github.com/eru-tech/eru/eru-models"
)

// The exact A-B-A-B from the two live dashboard runs, as a test.

// alternatingProvider rejects with fault A, then B, then A, then B - the shape
// the page agent produced twice while burning its whole budget.
type alternatingProvider struct {
	calls  int
	faults []string
}

func (alternatingProvider) GetSystemPrompt() string { return "" }
func (alternatingProvider) GetOutputSchema(ctx context.Context) eru_models.JSONSchema {
	return eru_models.JSONSchema{}
}
func (p *alternatingProvider) ValidateOutput(ctx context.Context, output map[string]interface{}) error {
	fault := p.faults[p.calls%len(p.faults)]
	p.calls++
	if fault == "" {
		return nil
	}
	return errors.New(fault)
}

func alternating(t *testing.T, retryCount int, faults []string, answers ...string) (agents.AgentMessage, *scriptedModel, error) {
	t.Helper()
	model := &scriptedModel{answers: answers}
	ra := &ReasoningAgent{}
	ra.AgentName = "cycler"
	ra.RetryCount = retryCount
	ra.Model = model
	ra.SetProvider(&alternatingProvider{faults: faults})
	out, err := ra.Execute(context.Background(), agents.AgentMessage{Content: "build it"}, "", "p", "t")
	return out, model, err
}

func repeated(answer string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = answer
	}
	return out
}

func TestAnAlternatingAgentIsStoppedRatherThanBurningTheBudget(t *testing.T) {
	faults := []string{
		"the page binds no queries",
		`tile "a" has no property query_result_path`,
	}
	// RetryCount 6 would allow seven attempts; the cycle is recognised long
	// before that.
	_, model, err := alternating(t, 6, faults, repeated(`{"v":"x"}`, 8)...)
	if err == nil {
		t.Fatal("an agent that cannot satisfy both faults must stop, not succeed")
	}
	if !strings.Contains(err.Error(), "alternating") {
		t.Errorf("the error must say what happened: %v", err)
	}
	// A-B-A-B-A: the third sighting of fault A is the second repeat, and stops it.
	if model.calls > 5 {
		t.Errorf("the cycle must be caught early; it took %d attempts", model.calls)
	}
	if model.calls < 3 {
		t.Errorf("two different faults are not yet a cycle; stopped after %d", model.calls)
	}
}

// The feedback has to change, not just the count. A fourth "this is wrong" is
// the one thing that demonstrably does not work.
func TestTheAgentIsToldItIsTradingOneFaultForTheOther(t *testing.T) {
	faults := []string{"fault A here", "fault B here"}
	_, model, _ := alternating(t, 6, faults, repeated(`{"v":"x"}`, 8)...)

	var noted string
	for _, prompt := range model.prompts {
		if strings.Contains(prompt, "READ THIS FIRST") {
			noted = prompt
			break
		}
	}
	if noted == "" {
		t.Fatalf("no cycle note was ever sent; prompts: %d", len(model.prompts))
	}
	for _, want := range []string{
		"already been given this exact rejection",
		"trading one fault for another",
		"Do NOT simply undo your last change",
	} {
		if !strings.Contains(noted, want) {
			t.Errorf("the note must say %q:\n%s", want, noted)
		}
	}
}

// Being stuck on ONE fault is not cycling - it is an agent failing to fix
// something, which the ordinary retry budget already handles. Treating it as a
// cycle would cut its attempts short for no reason.
func TestRepeatingASingleFaultIsNotACycle(t *testing.T) {
	_, model, err := alternating(t, 3, []string{"the same fault every time"}, repeated(`{"v":"x"}`, 6)...)
	if err == nil {
		t.Fatal("it never succeeded, so it must fail")
	}
	if strings.Contains(err.Error(), "alternating") {
		t.Errorf("one fault repeated is not a cycle: %v", err)
	}
	if model.calls != 4 {
		t.Errorf("the full retry budget must be used, got %d attempts", model.calls)
	}
}

// The same complaint about a different number of components is the same
// complaint, not progress.
func TestACountChangeIsNotADifferentFault(t *testing.T) {
	a := fingerprintFault(`- 7 tiles have no property "query_result_path"`)
	b := fingerprintFault(`- 6 tiles have no property "query_result_path"`)
	if a != b {
		t.Error("a changed count must not read as a new fault")
	}
	c := fingerprintFault(`- 7 tiles have no property "entity_name"`)
	if a == c {
		t.Error("a different property is a different fault")
	}
}

// Order within one rejection must not matter either.
func TestFaultOrderDoesNotMatter(t *testing.T) {
	one := fingerprintFault("- alpha is wrong\n- beta is wrong")
	two := fingerprintFault("- beta is wrong\n- alpha is wrong")
	if one != two {
		t.Error("the same set of faults reported in another order is the same fault")
	}
}

// A run that recovers after one repeat must not be cut short: the note is a
// nudge, and the nudge is allowed to work.
func TestOneRepeatIsANudgeNotAStop(t *testing.T) {
	faults := []string{"fault A", "fault B", "fault A", ""}
	out, model, err := alternating(t, 6, faults, repeated(`{"v":"x"}`, 6)...)
	if err != nil {
		t.Fatalf("the fourth attempt was accepted and must be delivered: %v", err)
	}
	if model.calls != 4 {
		t.Errorf("expected four attempts, got %d", model.calls)
	}
	if out.StopReason != agents.StopEndTurn {
		t.Errorf("StopReason = %q, want end_turn", out.StopReason)
	}
}

func TestDescribeAttemptsReadsAsEnglish(t *testing.T) {
	for attempts, want := range map[string]string{
		describeAttempts([]int{2}):       "2",
		describeAttempts([]int{2, 4}):    "2 and 4",
		describeAttempts([]int{1, 3, 5}): "1, 3 and 5",
	} {
		if attempts != want {
			t.Errorf("got %q, want %q", attempts, want)
		}
	}
	_ = fmt.Sprint()
}
