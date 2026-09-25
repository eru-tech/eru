package eval

import (
	"strings"
	"testing"
)

func withCalls(calls ...ToolCall) Trajectory { return Trajectory{ToolCalls: calls} }

func tc(name string, args map[string]interface{}) ToolCall {
	return ToolCall{Name: name, Input: args}
}

// The failure this exists for: an agent that answers perfectly while asking the
// same question eleven times passes every correctness assertion here.
func TestRepeatedLookupsAreCounted(t *testing.T) {
	same := map[string]interface{}{"entity_names": "invoices"}
	effort := EffortOf(withCalls(
		tc("get_entity_metadata", same),
		tc("get_entity_metadata", same),
		tc("get_entity_metadata", same),
		tc("run_query", map[string]interface{}{"query_name": "a"}),
	))
	if effort.Calls != 4 || effort.Distinct != 2 || effort.Wasted() != 2 {
		t.Fatalf("%+v", effort)
	}
	out := effort.String()
	if !strings.Contains(out, "already answered") {
		t.Errorf("the repeats must be named:\n%s", out)
	}
}

// Different arguments are different questions, not repeats.
func TestDifferentArgumentsAreNotRepeats(t *testing.T) {
	effort := EffortOf(withCalls(
		tc("run_query", map[string]interface{}{"query_name": "a"}),
		tc("run_query", map[string]interface{}{"query_name": "b"}),
	))
	if effort.Wasted() != 0 {
		t.Errorf("two different queries are two pieces of work: %+v", effort)
	}
}

// Argument order must not make one call look like two.
func TestArgumentOrderDoesNotMatter(t *testing.T) {
	effort := EffortOf(withCalls(
		tc("save", map[string]interface{}{"a": 1, "b": 2}),
		tc("save", map[string]interface{}{"b": 2, "a": 1}),
	))
	if effort.Distinct != 1 {
		t.Errorf("the same call written two ways is one call: %+v", effort)
	}
}

// A run that called nothing has nothing to report, and must not print an empty
// heading into the suite output.
func TestNoCallsRendersNothing(t *testing.T) {
	if out := EffortOf(Trajectory{}).String(); out != "" {
		t.Errorf("expected silence, got %q", out)
	}
}

func TestBusiestActionComesFirst(t *testing.T) {
	effort := EffortOf(withCalls(
		tc("run_query", map[string]interface{}{"q": "1"}),
		tc("run_query", map[string]interface{}{"q": "2"}),
		tc("get_page", map[string]interface{}{"id": "p"}),
	))
	out := effort.String()
	if strings.Index(out, "run_query") > strings.Index(out, "get_page") {
		t.Errorf("the busiest action should lead:\n%s", out)
	}
}
