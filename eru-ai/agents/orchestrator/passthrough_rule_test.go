package orchestrator

import (
	"context"
	"strings"
	"testing"
)

func planIssuesFor(t *testing.T, body string, userMessage string) []planIssue {
	t.Helper()
	plan := planWith(t, body)
	compileStepRequests(plan)
	return validatePlan(context.Background(), plan, nil, nil, codeContext{UserMessage: userMessage})
}

func hasPassthroughIssue(issues []planIssue) bool {
	for _, issue := range issues {
		if issue.Field == "request" && strings.Contains(issue.Err, "forwarded as they wrote it") {
			return true
		}
	}
	return false
}

// The exact failure: a follow-up that leans on the previous turn, rewritten by
// the planner into a self-contained instruction that means something broader.
func TestARewrittenMessageOnALoneStepIsReported(t *testing.T) {
	issues := planIssuesFor(t, `{"func_steps": {"eru_studio": {"agent_name": "eru_studio",
	  "transform_request": "{{stringify (dict \"content\" \"The user wants to replace all blue colors with yellow throughout the page.\")}}"}}}`,
		"blue is not looking gd, lets change it to yellow")
	if !hasPassthroughIssue(issues) {
		t.Fatalf("a rewritten message should be reported: %v", issues)
	}
}

func TestForwardingTheMessageIsAccepted(t *testing.T) {
	issues := planIssuesFor(t, `{"func_steps": {"eru_studio": {"agent_name": "eru_studio",
	  "request": {"content": {"from": "user.content"}}}}}`,
		"blue is not looking gd, lets change it to yellow")
	if hasPassthroughIssue(issues) {
		t.Fatalf("forwarding the message is what we asked for: %v", issues)
	}
}

// Params carry context, not intent, so passing them alongside is fine.
func TestForwardingWithParamsIsAccepted(t *testing.T) {
	issues := planIssuesFor(t, `{"func_steps": {"eru_studio": {"agent_name": "eru_studio",
	  "request": {"content": {"from": "user.content"}, "params": {"code": {"from": "user.params.code"}}}}}}`,
		"change it to yellow")
	if hasPassthroughIssue(issues) {
		t.Fatalf("params alongside the message are fine: %v", issues)
	}
}

// A step that renders another step's output legitimately needs an instruction of
// the orchestrator's own.
func TestAStepThatConsumesAnotherIsNotAPassthrough(t *testing.T) {
	issues := planIssuesFor(t, `{"func_steps": {
	  "generate_sql": {"agent_name": "generate_sql", "request": {"content": {"from": "user.content"}}},
	  "eru_studio": {"agent_name": "eru_studio",
	    "transform_request": "{{stringify (dict \"content\" \"Render these rows as a table\" \"params\" (dict \"context\" (stringify .ResVars.generate_sql.Body)))}}"}
	}}`, "show me the invoices as a table")
	if hasPassthroughIssue(issues) {
		t.Fatalf("a multi-step plan is not a passthrough candidate: %v", issues)
	}
}

// Nested steps mean the plan is doing something more than relaying.
func TestANestedPlanIsNotAPassthrough(t *testing.T) {
	issues := planIssuesFor(t, `{"func_steps": {"outer": {"agent_name": "outer",
	  "transform_request": "{{stringify (dict \"content\" \"do the thing\")}}",
	  "func_steps": {"inner": {"agent_name": "inner", "transform_request": "{{stringify (dict \"content\" \"and this\")}}"}}}}}`,
		"do the thing")
	if hasPassthroughIssue(issues) {
		t.Fatalf("a nested plan is not a passthrough candidate: %v", issues)
	}
}

// With no user message there is nothing to compare against.
func TestNoUserMessageMeansNoOpinion(t *testing.T) {
	issues := planIssuesFor(t, `{"func_steps": {"eru_studio": {"agent_name": "eru_studio",
	  "transform_request": "{{stringify (dict \"content\" \"anything\")}}"}}}`, "")
	if hasPassthroughIssue(issues) {
		t.Fatalf("without the user's message the rule must stand down: %v", issues)
	}
}

// A tool step is not an agent step.
func TestALoneToolStepIsNotAPassthroughCandidate(t *testing.T) {
	issues := planIssuesFor(t, `{"func_steps": {"run_sql": {"tool_name": "run_sql", "tool_action": "execute_sql",
	  "request": {"params": {"query": "select 1"}}}}}`, "run this")
	if hasPassthroughIssue(issues) {
		t.Fatalf("a tool step takes params, not the user's prose: %v", issues)
	}
}
