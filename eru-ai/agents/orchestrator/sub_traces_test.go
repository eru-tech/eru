package orchestrator

import (
	"strings"
	"testing"
	"time"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	models "github.com/eru-tech/eru/eru-ai/models"
	functions "github.com/eru-tech/eru/eru-functions/functions"
)

func agentBody(actionName string, thinking ...string) map[string]interface{} {
	var traces []interface{}
	for i, text := range thinking {
		traces = append(traces, map[string]interface{}{
			"iteration": i + 1,
			"thinking":  text,
			"timestamp": time.Date(2026, 1, 1, 0, 0, i, 0, time.UTC).Format(time.RFC3339Nano),
		})
	}
	return map[string]interface{}{
		"actions": []interface{}{map[string]interface{}{"action_name": actionName, "action": map[string]interface{}{"sql": "select 1"}}},
		"traces":  traces,
	}
}

func varsWith(bodies map[string]interface{}) map[string]functions.FuncTemplateVars {
	resVars := map[string]*functions.TemplateVars{}
	for step, body := range bodies {
		resVars[step] = &functions.TemplateVars{Body: body}
	}
	// eru-functions gives every entry the full ResVars map, which is what makes
	// naive harvesting duplicate every trace once per step.
	out := map[string]functions.FuncTemplateVars{}
	for step := range bodies {
		out[step] = functions.FuncTemplateVars{ResVars: resVars}
	}
	return out
}

func TestCollectSubAgentTracesDeduplicatesAndLabels(t *testing.T) {
	traces := collectSubAgentTraces(varsWith(map[string]interface{}{
		"generate_sql": agentBody("generate_sql", "picking tables", "writing sql"),
		"execute_sql":  agentBody("execute_sql", "running"),
	}))
	if len(traces) != 3 {
		t.Fatalf("expected 3 traces (deduplicated), got %d", len(traces))
	}
	byStep := map[string]int{}
	for _, trace := range traces {
		byStep[trace.Step]++
		if trace.Agent == "" {
			t.Errorf("trace not attributed : %+v", trace)
		}
	}
	if byStep["generate_sql"] != 2 || byStep["execute_sql"] != 1 {
		t.Errorf("unexpected per-step counts : %v", byStep)
	}
}

func TestCollectSubAgentTracesOrdersByTime(t *testing.T) {
	traces := collectSubAgentTraces(varsWith(map[string]interface{}{
		"a": agentBody("a", "a1", "a2", "a3"),
		"b": agentBody("b", "b1", "b2"),
	}))
	for i := 1; i < len(traces); i++ {
		if traces[i].Timestamp.Before(traces[i-1].Timestamp) {
			t.Fatalf("traces out of order at %d : %v", i, traces)
		}
	}
}

func TestCollectSubAgentTracesFallsBackToStepName(t *testing.T) {
	body := agentBody("", "thinking")
	delete(body, "actions")
	traces := collectSubAgentTraces(varsWith(map[string]interface{}{"lonely_step": body}))
	if len(traces) != 1 || traces[0].Agent != "lonely_step" {
		t.Fatalf("expected the step name as a fallback label, got %+v", traces)
	}
}

func TestCollectSubAgentTracesIgnoresToolAndEmptyBodies(t *testing.T) {
	traces := collectSubAgentTraces(varsWith(map[string]interface{}{
		"tool_step": map[string]interface{}{"rows": []interface{}{}},
		"":          agentBody("ghost", "should be skipped"),
	}))
	if len(traces) != 0 {
		t.Fatalf("expected no traces, got %+v", traces)
	}
}

func TestLabelOwnTracesLeavesSubAgentLabelsAlone(t *testing.T) {
	traces := labelOwnTraces([]models.StepTrace{
		{Thinking: "orchestrator planning"},
		{Thinking: "sub reasoning", Agent: "generate_sql", Step: "generate_sql"},
	}, "planner")
	if traces[0].Agent != "planner" {
		t.Errorf("orchestrator trace not labelled : %+v", traces[0])
	}
	if traces[1].Agent != "generate_sql" {
		t.Errorf("sub-agent label overwritten : %+v", traces[1])
	}
}

func TestClientTracesTruncatesButKeepsAttribution(t *testing.T) {
	long := strings.Repeat("x", traceTextLimit+500)
	traces := clientTraces(t.Context(), []models.StepTrace{{Thinking: long, Content: long, Agent: "sub", Step: "s"}})
	if len(traces[0].Thinking) > traceTextLimit+len(traceTruncationNote) {
		t.Errorf("thinking not truncated : %d chars", len(traces[0].Thinking))
	}
	if !strings.Contains(traces[0].Thinking, "truncated") {
		t.Error("expected a truncation marker")
	}
	if traces[0].Agent != "sub" || traces[0].Step != "s" {
		t.Error("truncation must not lose attribution")
	}
}

func TestClientTracesKeepsFullTextForRawCallers(t *testing.T) {
	long := strings.Repeat("x", traceTextLimit+500)
	traces := clientTraces(agents.WithRawOutput(t.Context(), true), []models.StepTrace{{Thinking: long}})
	if len(traces[0].Thinking) != len(long) {
		t.Errorf("raw callers must get the untrimmed trace, got %d chars", len(traces[0].Thinking))
	}
}
