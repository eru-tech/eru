package orchestrator

import (
	"strings"
	"testing"

	functions "github.com/eru-tech/eru/eru-functions/functions"
)

func varsFor(body interface{}) *functions.TemplateVars {
	return &functions.TemplateVars{Body: body}
}

func funcVars(steps map[string]*functions.TemplateVars) map[string]functions.FuncTemplateVars {
	return map[string]functions.FuncTemplateVars{"group": {ResVars: steps}}
}

func TestCompletedWorkKeepsStepsThatSucceeded(t *testing.T) {
	plan := planWith(t, `{"func_steps": {"generate_sql": {"agent_name": "generate_sql"}, "eru_studio": {"agent_name": "eru_studio"}}}`)
	done := collectCompletedWork(plan, funcVars(map[string]*functions.TemplateVars{
		"generate_sql": varsFor(map[string]interface{}{"actions": []interface{}{map[string]interface{}{"action": map[string]interface{}{"sql": "select 1"}}}}),
	}))
	if !done.any() || len(done.Steps) != 1 || done.Steps[0] != "generate_sql" {
		t.Fatalf("expected generate_sql to be kept, got %v", done.Steps)
	}
	brief := done.briefing()
	for _, want := range []string{"ALREADY RAN SUCCESSFULLY", "generate_sql", ".ResVars.<step>"} {
		if !strings.Contains(brief, want) {
			t.Errorf("briefing missing %q:\n%s", want, brief)
		}
	}
}

// A step that returned an error did no work worth keeping.
func TestAFailedStepIsNotTreatedAsCompleted(t *testing.T) {
	plan := planWith(t, `{"func_steps": {"a": {"agent_name": "a"}}}`)
	done := collectCompletedWork(plan, funcVars(map[string]*functions.TemplateVars{
		"a": varsFor(map[string]interface{}{"error": "boom"}),
	}))
	if done.any() {
		t.Errorf("a step whose body is an error should not be kept: %v", done.Steps)
	}
}

// A paused step belongs to the clarification path, not to the replan.
func TestAPausedStepIsNotTreatedAsCompleted(t *testing.T) {
	plan := planWith(t, `{"func_steps": {"a": {"agent_name": "a"}}}`)
	done := collectCompletedWork(plan, funcVars(map[string]*functions.TemplateVars{
		"a": varsFor(map[string]interface{}{"actions": []interface{}{map[string]interface{}{
			"action_type": "question",
			"action":      map[string]interface{}{"questions": []interface{}{map[string]interface{}{"id": "q1", "question": "which?"}}},
		}}}),
	}))
	if done.any() {
		t.Errorf("a paused step should not be counted as completed: %v", done.Steps)
	}
}

// Something the executor left behind that was never a step in this plan is not
// ours to carry forward.
func TestOnlyPlanStepsAreCarried(t *testing.T) {
	plan := planWith(t, `{"func_steps": {"a": {"agent_name": "a"}}}`)
	done := collectCompletedWork(plan, funcVars(map[string]*functions.TemplateVars{
		"somewhere_else": varsFor(map[string]interface{}{"ok": true}),
	}))
	if done.any() {
		t.Errorf("a non-plan step was carried: %v", done.Steps)
	}
}

func TestNothingCompletedProducesNoBriefing(t *testing.T) {
	if (completedWork{}).briefing() != "" {
		t.Error("an empty briefing should stay empty so the replan prompt is unchanged")
	}
}

func TestLargeResultsArePreviewedNotPasted(t *testing.T) {
	plan := planWith(t, `{"func_steps": {"a": {"agent_name": "a"}}}`)
	huge := strings.Repeat("x", 5000)
	done := collectCompletedWork(plan, funcVars(map[string]*functions.TemplateVars{
		"a": varsFor(map[string]interface{}{"page": huge}),
	}))
	brief := done.briefing()
	if strings.Contains(brief, huge) {
		t.Error("a whole result was pasted into the prompt")
	}
	if !strings.Contains(brief, "truncated") {
		t.Errorf("a long result should be marked truncated:\n%s", brief[:200])
	}
}
