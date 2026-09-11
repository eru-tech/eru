package orchestrator

import (
	"context"
	"strings"
	"testing"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	eru_models "github.com/eru-tech/eru/eru-models"
)

// The three templates the planner actually produced, in order.
const attempt1 = "{{stringify (dict \"content\" \"Build a page:\n\n1. BASIC INFORMATION\n   - First Name\" \"params\" (dict \"entities\" \"[]\")})}}"
const attempt2 = `{{stringify (dict "content" "Build a page:\n\n1. BASIC" "params" (dict "entities" "[]")))}}`
const attempt3 = `{{stringify (dict "content" "Build a page:\n\n1. BASIC" "params" (dict "entities" "[]")})}}`

func TestRealFailuresAreDiagnosed(t *testing.T) {
	for name, tmpl := range map[string]string{"attempt1": attempt1, "attempt2": attempt2, "attempt3": attempt3} {
		issue, ok := validateStepTemplate(context.Background(), "eru_studio", "transform_request", tmpl)
		if ok {
			t.Errorf("%s: template unexpectedly parsed", name)
			continue
		}
		t.Logf("--- %s ---\n%s", name, issue.Err)
		if !strings.Contains(issue.Err, "JSON") {
			t.Errorf("%s: no JSON-form remedy", name)
		}
	}
}

func TestJSONBodyFormIsAcceptedToday(t *testing.T) {
	jsonForm := `{"content": "Build a page:\n\n1. BASIC INFORMATION\n   - First Name (text)", "params": {"code": {{stringify .Vars.Body.params.code}}, "entities": "[{\"name\":\"business_card\"}]"}}`
	if issue, ok := validateStepTemplate(context.Background(), "eru_studio", "transform_request", jsonForm); !ok {
		t.Fatalf("the JSON body form was rejected: %s", issue.Err)
	}
	t.Log("JSON body form parses")
}

// The pie-chart run: the UI set output_mode and inline_nested_pages, the plan
// forwarded only code, and the studio agent answered in single-page mode.
func TestDroppedShapingParamIsCaught(t *testing.T) {
	studio := discoveredWithParams("eru_studio", "code", "output_mode", "inline_nested_pages")
	cc := describeCodeParam(map[string]interface{}{
		"code":                map[string]interface{}{"id": "p1", "components": []interface{}{}},
		"output_mode":         "auto",
		"inline_nested_pages": "false",
	})
	if len(cc.ForwardParams) != 2 {
		t.Fatalf("ForwardParams = %v", cc.ForwardParams)
	}

	// The plan the orchestrator actually produced.
	dropped := planWithTransform("eru_studio", `{{stringify (dict "content" "Change the pie chart title." "params" (dict "code" .Vars.Body.params.code))}}`)
	issues := validatePlan(context.Background(), dropped, []agents.DiscoveredAgent{studio}, nil, cc)
	joined := ""
	for _, issue := range issues {
		joined += issue.Err + " "
	}
	for _, want := range []string{"output_mode", "inline_nested_pages"} {
		if !strings.Contains(joined, want) {
			t.Errorf("dropping params.%s was not caught: %v", want, issues)
		}
	}

	// Forwarding them passes.
	forwarded := planWithTransform("eru_studio", `{"content": "Change the pie chart title.", "params": {"code": {{stringify .Vars.Body.params.code}}, "output_mode": {{stringify .Vars.Body.params.output_mode}}, "inline_nested_pages": {{stringify .Vars.Body.params.inline_nested_pages}}}}`)
	if issues := validatePlan(context.Background(), forwarded, []agents.DiscoveredAgent{studio}, nil, cc); len(issues) > 0 {
		t.Errorf("a plan that forwards the params was rejected: %v", issues)
	}
}

func TestParamsTheAgentDoesNotReadAreNotDemanded(t *testing.T) {
	// An agent with no output_mode param must not be asked to take one.
	plain := discoveredWithParams("processo_generate_sql", "context")
	cc := describeCodeParam(map[string]interface{}{"output_mode": "auto"})
	plan := planWithTransform("processo_generate_sql", `{{stringify (dict "content" .Vars.Body.content)}}`)
	if issues := validatePlan(context.Background(), plan, []agents.DiscoveredAgent{plain}, nil, cc); len(issues) > 0 {
		t.Errorf("an agent that does not read the param was asked to forward it: %v", issues)
	}
}

func discoveredWithParams(name string, params ...string) agents.DiscoveredAgent {
	props := map[string]eru_models.JSONSchema{}
	for _, param := range params {
		props[param] = eru_models.JSONSchema{Type: "string", Description: param}
	}
	return agents.DiscoveredAgent{
		AgentName:   name,
		InputSchema: agents.AgentInputSchema(props, nil),
	}
}

func planWithTransform(agentName string, transform string) map[string]interface{} {
	return map[string]interface{}{
		"func_category_name": "page_edit",
		"func_group_name":    "edit",
		"func_steps": map[string]interface{}{
			agentName: map[string]interface{}{
				"agent_name":        agentName,
				"transform_request": transform,
			},
		},
	}
}
