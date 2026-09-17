package orchestrator

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func planWith(t *testing.T, body string) map[string]interface{} {
	t.Helper()
	var plan map[string]interface{}
	if err := json.Unmarshal([]byte(body), &plan); err != nil {
		t.Fatalf("bad test plan: %v", err)
	}
	return plan
}

func templateOf(t *testing.T, plan map[string]interface{}, step string) string {
	t.Helper()
	steps, _ := plan["func_steps"].(map[string]interface{})
	entry, ok := steps[step].(map[string]interface{})
	if !ok {
		t.Fatalf("step %q missing", step)
	}
	template, _ := entry["transform_request"].(string)
	return template
}

// The content that broke three plan-repair rounds: long, multi-line, full of
// quotes. Written as a literal it needs no escaping by the model at all, and
// what comes out has to parse.
func TestALongInstructionCompilesAndParses(t *testing.T) {
	plan := planWith(t, `{"func_steps": {"eru_studio": {"agent_name": "eru_studio", "request": {
	  "content": "Build a modern analytics dashboard.\n\nLAYOUT:\n- filters on top (\"from\" and \"to\")\n- a chart below\n\nUse the 'er' entity."
	}}}}`)

	if issues := compileStepRequests(plan); len(issues) != 0 {
		t.Fatalf("compile reported issues: %v", issues)
	}
	template := templateOf(t, plan, "eru_studio")
	if template == "" {
		t.Fatal("no transform_request was produced")
	}
	if issues := validatePlanTemplates(context.Background(), plan); len(issues) != 0 {
		t.Fatalf("the generated template does not parse: %v", issues)
	}
	// It must still be the body the agent expects once rendered.
	if !strings.HasPrefix(template, `{"content": "Build a modern analytics dashboard.`) {
		t.Errorf("unexpected template: %s", template[:80])
	}
	if strings.Contains(template, "dict") {
		t.Error("the generated form should not use dict - there is nothing to balance")
	}
}

func TestRequestIsRemovedOnceCompiled(t *testing.T) {
	plan := planWith(t, `{"func_steps": {"a": {"agent_name": "x", "request": {"content": "hi"}}}}`)
	compileStepRequests(plan)
	steps := plan["func_steps"].(map[string]interface{})
	if _, present := steps["a"].(map[string]interface{})["request"]; present {
		t.Error("request should be consumed, not left for the executor to trip over")
	}
}

func TestReferencesCompileToTheRightPaths(t *testing.T) {
	plan := planWith(t, `{"func_steps": {
	  "first":  {"agent_name": "first", "request": {"content": {"from": "user.content"}}},
	  "second": {"agent_name": "second", "request": {"content": {"from": "first.sql"}, "params": {"scope": {"from": "user.params.scope"}}}}
	}}`)
	if issues := compileStepRequests(plan); len(issues) != 0 {
		t.Fatalf("issues: %v", issues)
	}
	first := templateOf(t, plan, "first")
	if !strings.Contains(first, "{{stringify .Vars.Body.content}}") {
		t.Errorf("user.content compiled to %s", first)
	}
	second := templateOf(t, plan, "second")
	if !strings.Contains(second, "(index .ResVars.first.Body.actions 0).action.sql") {
		t.Errorf("step reference compiled to %s", second)
	}
	if !strings.Contains(second, `"params": {"scope": {{stringify .Vars.Body.params.scope}}}`) {
		t.Errorf("params compiled to %s", second)
	}
	if issues := validatePlanTemplates(context.Background(), plan); len(issues) != 0 {
		t.Fatalf("generated templates do not parse: %v", issues)
	}
}

func TestJoinConcatenatesWithoutTheModelWritingPrintf(t *testing.T) {
	plan := planWith(t, `{"func_steps": {
	  "gen": {"agent_name": "gen", "request": {"content": "x"}},
	  "use": {"agent_name": "use", "request": {"content": {"join": ["Here is the sql: ", {"from": "gen.sql"}]}}}
	}}`)
	if issues := compileStepRequests(plan); len(issues) != 0 {
		t.Fatalf("issues: %v", issues)
	}
	if issues := validatePlanTemplates(context.Background(), plan); len(issues) != 0 {
		t.Fatalf("generated templates do not parse: %v", issues)
	}
	if !strings.Contains(templateOf(t, plan, "use"), "printf") {
		t.Errorf("join should concatenate: %s", templateOf(t, plan, "use"))
	}
}

// A percent sign in literal text must not be read as a format verb.
func TestJoinDoesNotTreatLiteralsAsFormatVerbs(t *testing.T) {
	plan := planWith(t, `{"func_steps": {
	  "gen": {"agent_name": "gen", "request": {"content": "x"}},
	  "use": {"agent_name": "use", "request": {"content": {"join": ["100% of rows: ", {"from": "gen.sql"}]}}}
	}}`)
	compileStepRequests(plan)
	template := templateOf(t, plan, "use")
	if strings.Contains(template, `printf "100%`) {
		t.Errorf("a literal was inlined into the format string: %s", template)
	}
	if issues := validatePlanTemplates(context.Background(), plan); len(issues) != 0 {
		t.Fatalf("generated template does not parse: %v", issues)
	}
}

// A hand-written template is not silently rewritten under its author.
func TestAHandWrittenTemplateWins(t *testing.T) {
	plan := planWith(t, `{"func_steps": {"a": {"agent_name": "x",
	  "transform_request": "{{stringify (dict \"content\" .Vars.Body.content)}}",
	  "request": {"content": "ignored"}}}}`)
	compileStepRequests(plan)
	if got := templateOf(t, plan, "a"); !strings.Contains(got, "dict") {
		t.Errorf("the hand-written template was replaced: %s", got)
	}
}

func TestABadReferenceIsReportedNotCompiled(t *testing.T) {
	plan := planWith(t, `{"func_steps": {"a": {"agent_name": "x", "request": {"content": {"from": "nonsense"}}}}}`)
	issues := compileStepRequests(plan)
	if len(issues) == 0 {
		t.Fatal("a reference with no step and field should be rejected")
	}
	if !strings.Contains(issues[0].Err, "not a reference") {
		t.Errorf("unhelpful message: %s", issues[0].Err)
	}
}

// An agent step needs content; a tool step is all params. What is never useful
// is a request that carries neither.
func TestRequestNeedsContentOrParams(t *testing.T) {
	empty := planWith(t, `{"func_steps": {"a": {"agent_name": "a", "request": {}}}}`)
	if issues := compileStepRequests(empty); len(issues) == 0 {
		t.Fatal("a request with neither content nor params should be rejected")
	}

	toolStep := planWith(t, `{"func_steps": {"a": {"tool_name": "a", "request": {"params": {"query": "select 1", "vars": {}}}}}}`)
	if issues := compileStepRequests(toolStep); len(issues) != 0 {
		t.Fatalf("a tool step is all params and must compile: %v", issues)
	}
	if got := templateOf(t, toolStep, "a"); got != `{"params": {"query": "select 1", "vars": {}}}` {
		t.Errorf("params-only body compiled to %s", got)
	}
}

// A tool answers with a plain body, so there is no action field to name.
func TestFromBodyReadsAWholeToolResult(t *testing.T) {
	plan := planWith(t, `{"func_steps": {
	  "run_sql": {"tool_name": "run_sql", "request": {"params": {"query": "select 1"}}},
	  "summarise": {"agent_name": "summarise", "request": {"content": {"from_body": "run_sql"}}}
	}}`)
	if issues := compileStepRequests(plan); len(issues) != 0 {
		t.Fatalf("issues: %v", issues)
	}
	if got := templateOf(t, plan, "summarise"); !strings.Contains(got, "{{stringify .ResVars.run_sql.Body}}") {
		t.Errorf("from_body compiled to %s", got)
	}
}

func TestNestedStepsAreCompiledToo(t *testing.T) {
	plan := planWith(t, `{"func_steps": {"outer": {"agent_name": "x", "request": {"content": "a"},
	  "func_steps": {"inner": {"agent_name": "y", "request": {"content": "b"}}}}}}`)
	if issues := compileStepRequests(plan); len(issues) != 0 {
		t.Fatalf("issues: %v", issues)
	}
	steps := plan["func_steps"].(map[string]interface{})
	inner := steps["outer"].(map[string]interface{})["func_steps"].(map[string]interface{})["inner"].(map[string]interface{})
	if _, ok := inner["transform_request"].(string); !ok {
		t.Error("a nested step's request was not compiled")
	}
}

func TestAttachmentsCanBeNamedByAPlan(t *testing.T) {
	// The attachments are already in the request body beside content and params.
	// Without a way to name them, an orchestrator could see an attached image
	// and had no way to hand it to the agent that needed it.
	expr, err := referenceExpression("user.files")
	if err != nil {
		t.Fatalf("user.files is not a reference: %v", err)
	}
	if expr != ".Vars.Body.files" {
		t.Fatalf("user.files resolved to %q", expr)
	}
}

func TestASingleAttachmentCanBeNamed(t *testing.T) {
	expr, err := referenceExpression("user.files.0")
	if err != nil {
		t.Fatalf("user.files.0 is not a reference: %v", err)
	}
	if expr != "(index .Vars.Body.files 0)" {
		t.Fatalf("user.files.0 resolved to %q", expr)
	}
}

func TestAnAttachmentReferenceMustNameAnIndex(t *testing.T) {
	if _, err := referenceExpression("user.files.first"); err == nil {
		t.Fatal("accepted a non-numeric attachment index")
	}
}

func TestAStepCanBeGivenTheUsersAttachments(t *testing.T) {
	// files is a top-level key of the request body, beside content - every
	// agent decodes into the same message shape, so this is how an attachment
	// reaches any of them.
	body, err := buildTransformRequest(map[string]interface{}{
		"content": map[string]interface{}{"from": "user.content"},
		"files":   map[string]interface{}{"from": "user.files"},
	})
	if err != nil {
		t.Fatalf("buildTransformRequest: %v", err)
	}
	if !strings.Contains(body, `"files": `) {
		t.Fatalf("the compiled body carries no files key:\n%s", body)
	}
	if !strings.Contains(body, ".Vars.Body.files") {
		t.Fatalf("files does not resolve to the request's attachments:\n%s", body)
	}
	if !strings.Contains(body, `"content": `) {
		t.Fatalf("content was lost:\n%s", body)
	}
}

func TestFilesIsNotNestedUnderParams(t *testing.T) {
	body, err := buildTransformRequest(map[string]interface{}{
		"content": "do the thing",
		"params":  map[string]interface{}{"code": "x"},
		"files":   map[string]interface{}{"from": "user.files"},
	})
	if err != nil {
		t.Fatalf("buildTransformRequest: %v", err)
	}
	paramsAt := strings.Index(body, `"params"`)
	filesAt := strings.Index(body, `"files"`)
	if paramsAt < 0 || filesAt < 0 {
		t.Fatalf("missing a key:\n%s", body)
	}
	if filesAt < paramsAt {
		t.Fatalf("files was emitted before params, which would put it inside the params object:\n%s", body)
	}
	var decoded map[string]interface{}
	probe := strings.ReplaceAll(body, "{{stringify .Vars.Body.files}}", `"FILES"`)
	if err := json.Unmarshal([]byte(probe), &decoded); err != nil {
		t.Fatalf("the compiled body is not valid JSON: %v\n%s", err, probe)
	}
	if _, ok := decoded["files"]; !ok {
		t.Fatalf("files is not a top-level key:\n%s", probe)
	}
}

func TestAStepThatOnlyForwardsFilesIsStillARequest(t *testing.T) {
	if _, err := buildTransformRequest(map[string]interface{}{
		"files": map[string]interface{}{"from": "user.files"},
	}); err != nil {
		t.Fatalf("a request carrying only attachments was rejected: %v", err)
	}
}
