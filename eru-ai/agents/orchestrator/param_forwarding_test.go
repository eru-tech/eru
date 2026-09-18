package orchestrator

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/eru-tech/eru/eru-ai/agents"
	"github.com/eru-tech/eru/eru-functions/functions"
	eru_models "github.com/eru-tech/eru/eru-models"
)

func studioAgent() agents.DiscoveredAgent {
	return agents.DiscoveredAgent{
		AgentName: "eru_studio",
		InputSchema: eru_models.JSONSchema{
			Type: "object",
			Properties: map[string]eru_models.JSONSchema{
				"params": {
					Type: "object",
					Properties: map[string]eru_models.JSONSchema{
						"code":        {Type: "string"},
						"output_mode": {Type: "string"},
					},
				},
			},
		},
	}
}

func TestInjectParamIntoExistingParamsObject(t *testing.T) {
	template := `{"content": {{stringify .Vars.OrgBody.content}}, "params": {"code": {{stringify .Vars.OrgBody.params.code}}}}`
	got := injectParam(template, "output_mode")
	want := `{"content": {{stringify .Vars.OrgBody.content}}, "params": {"output_mode": {{stringify .Vars.OrgBody.params.output_mode}}, "code": {{stringify .Vars.OrgBody.params.code}}}}`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestInjectParamIntoEmptyParamsObject(t *testing.T) {
	template := `{"content": "do it", "params": {}}`
	got := injectParam(template, "output_mode")
	want := `{"content": "do it", "params": {"output_mode": {{stringify .Vars.OrgBody.params.output_mode}}}}`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestInjectParamWhenStepHasNoParams(t *testing.T) {
	template := `{"content": "do it"}`
	got := injectParam(template, "output_mode")
	want := `{"content": "do it", "params": {"output_mode": {{stringify .Vars.OrgBody.params.output_mode}}}}`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestInjectedTemplateStaysValidJSONShape(t *testing.T) {
	template := injectParam(`{"content": "do it"}`, "output_mode")
	rendered := strings.ReplaceAll(template, `{{stringify .Vars.OrgBody.params.output_mode}}`, `"auto"`)
	var body map[string]interface{}
	if err := json.Unmarshal([]byte(rendered), &body); err != nil {
		t.Fatalf("rendered template is not valid JSON: %v (%s)", err, rendered)
	}
	params, ok := body["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("params missing from %s", rendered)
	}
	if params["output_mode"] != "auto" {
		t.Fatalf("output_mode not forwarded: %v", params)
	}
}

func TestInjectableSkipsDictStyleTemplates(t *testing.T) {
	if injectable(`{{stringify (dict "content" .Vars.OrgBody.content)}}`) {
		t.Fatal("dict-style template must not be rewritten")
	}
	if injectable("") {
		t.Fatal("empty template must not be rewritten")
	}
	if !injectable(`{"content": "x"}`) {
		t.Fatal("json-object template should be injectable")
	}
}

func TestAutoForwardParamsFillsNestedStep(t *testing.T) {
	plan := map[string]interface{}{
		"func_steps": map[string]interface{}{
			"processo_generate_sql": map[string]interface{}{
				"agent_name":        "processo_generate_sql",
				"transform_request": `{"content": "write sql"}`,
				"func_steps": map[string]interface{}{
					"eru_studio": map[string]interface{}{
						"agent_name":        "eru_studio",
						"transform_request": `{"content": "fix the page", "params": {"code": {{stringify .Vars.OrgBody.params.code}}}}`,
					},
				},
			},
		},
	}
	cc := codeContext{ForwardParams: []string{"output_mode"}}
	autoForwardParams(plan, []agents.DiscoveredAgent{studioAgent()}, cc)

	steps := plan["func_steps"].(map[string]interface{})
	parent := steps["processo_generate_sql"].(map[string]interface{})
	nested := parent["func_steps"].(map[string]interface{})["eru_studio"].(map[string]interface{})
	template := nested["transform_request"].(string)
	if !strings.Contains(template, `"output_mode": {{stringify .Vars.OrgBody.params.output_mode}}`) {
		t.Fatalf("output_mode not forwarded into nested step: %s", template)
	}
	if strings.Contains(parent["transform_request"].(string), "output_mode") {
		t.Fatal("a step whose agent does not read the param must be left alone")
	}
}

func TestAutoForwardParamsLeavesExistingForwardAlone(t *testing.T) {
	template := `{"content": "fix", "params": {"output_mode": {{stringify .Vars.OrgBody.params.output_mode}}}}`
	plan := map[string]interface{}{
		"func_steps": map[string]interface{}{
			"eru_studio": map[string]interface{}{
				"agent_name":        "eru_studio",
				"transform_request": template,
			},
		},
	}
	autoForwardParams(plan, []agents.DiscoveredAgent{studioAgent()}, codeContext{ForwardParams: []string{"output_mode"}})
	got := plan["func_steps"].(map[string]interface{})["eru_studio"].(map[string]interface{})["transform_request"].(string)
	if got != template {
		t.Fatalf("template was rewritten: %s", got)
	}
}

func TestStaleRequestRootIsRejected(t *testing.T) {
	steps := map[string]*functions.FuncStep{
		"eru_studio": {
			AgentName:        "eru_studio",
			TransformRequest: `{"content": "fix", "params": {"code": {{stringify .Vars.Body.params.code}}}}`,
		},
	}
	issues := validateUserRequestRoot(steps)
	if len(issues) != 1 {
		t.Fatalf("expected the caller-request root to be faulted, got %v", issues)
	}
	if !strings.Contains(issues[0].Err, userRequestRoot) {
		t.Fatalf("the fix was not named: %s", issues[0].Err)
	}
}

func TestCurrentRequestRootIsAccepted(t *testing.T) {
	steps := map[string]*functions.FuncStep{
		"eru_studio": {
			AgentName:        "eru_studio",
			TransformRequest: `{"content": "fix", "params": {"code": {{stringify .Vars.OrgBody.params.code}}}}`,
		},
	}
	if issues := validateUserRequestRoot(steps); len(issues) != 0 {
		t.Fatalf("expected no issues, got %v", issues)
	}
}

func TestStepsOwnBodyIsNotFaultedOutsideTransformRequest(t *testing.T) {
	steps := map[string]*functions.FuncStep{
		"step": {
			AgentName: "eru_studio",
			Condition: `{{if eq .Vars.Body.status "active"}}true{{else}}false{{end}}`,
		},
	}
	if issues := validateUserRequestRoot(steps); len(issues) != 0 {
		t.Fatalf("a condition reading this step's own body is legitimate, got %v", issues)
	}
}
