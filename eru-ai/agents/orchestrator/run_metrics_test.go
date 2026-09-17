package orchestrator

import (
	"strings"
	"testing"
	"time"
)

func TestMetricsCountWhatARunCost(t *testing.T) {
	m := newRunMetrics()
	m.countPlanAttempt()
	m.recordPlanInvalid([]planIssue{{Field: "transform_request"}, {Field: "transform_request"}, {Field: "agent_name"}})
	m.countPlanAttempt()
	m.countReplan()
	m.countExecAttempt(3)
	m.countExecAttempt(3)

	if m.PlanAttempts != 2 || m.PlanRepairs != 1 || m.Replans != 1 || m.ExecAttempts != 2 || m.StepsPlanned != 3 {
		t.Fatalf("wrong tallies: %+v", *m)
	}
	fields := dedupeCounted(m.PlanInvalid)
	joined := strings.Join(fields, " ")
	if !strings.Contains(joined, "transform_request x2") {
		t.Errorf("repeated fields should collapse with a count, got %q", joined)
	}
	if !strings.Contains(joined, "agent_name") {
		t.Errorf("a single field should still appear, got %q", joined)
	}
}

func TestStageDurationsAccumulate(t *testing.T) {
	m := newRunMetrics()
	done := m.stage("planning")
	time.Sleep(5 * time.Millisecond)
	done()
	again := m.stage("planning")
	time.Sleep(5 * time.Millisecond)
	again()
	if m.StageDuration["planning"] < 8*time.Millisecond {
		t.Errorf("two spells in the same stage should add up, got %v", m.StageDuration["planning"])
	}
}

// Every method is used from a context that may carry no metrics at all.
func TestMetricsAreNilSafe(t *testing.T) {
	var m *runMetrics
	m.countPlanAttempt()
	m.countReplan()
	m.countExecAttempt(1)
	m.recordPlanInvalid([]planIssue{{Field: "x"}})
	m.stage("planning")()
}

func TestCountPlanStepsIncludesNestedSteps(t *testing.T) {
	plan := planWith(t, `{"func_steps": {
	  "a": {"agent_name": "a", "func_steps": {"b": {"agent_name": "b"}, "c": {"agent_name": "c"}}},
	  "d": {"agent_name": "d"}
	}}`)
	if got := countPlanSteps(plan); got != 4 {
		t.Errorf("counted %d steps, want 4", got)
	}
}
