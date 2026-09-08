package orchestrator

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	eru_models "github.com/eru-tech/eru/eru-models"
)

func studioAgentSpec() agents.DiscoveredAgent {
	return agents.DiscoveredAgent{
		AgentName:   "eru_studio",
		AgentType:   "ERU_STUDIO",
		Description: "Builds EruPage JSON",
		TenantId:    "processo",
		InputSchema: agents.AgentInputSchema(map[string]eru_models.JSONSchema{
			"code":     {Type: "string"},
			"context":  {Type: "string"},
			"entities": {Type: "string"},
			"apis":     {Type: "string"},
		}, nil),
		OutputSchema: eru_models.JSONSchema{
			Type:       "object",
			Properties: map[string]eru_models.JSONSchema{"page": {Type: "object"}},
		},
		Tools: []string{"processo.fetch_page"},
	}
}

func sqlToolSpec() agents.DiscoveredTool {
	return agents.DiscoveredTool{
		ToolName:   "eruql_processo",
		ActionName: "execute_sql",
		TenantId:   "processo",
		InputSchema: eru_models.JSONSchema{
			Type: "object",
			Properties: map[string]eru_models.JSONSchema{
				"query":      {Type: "string"},
				"project_id": {Type: "string"},
				"vars":       {Type: "object"},
			},
			Required: []string{"query", "project_id"},
		},
	}
}

func planWithSteps(t *testing.T, steps map[string]interface{}) map[string]interface{} {
	t.Helper()
	plan := map[string]interface{}{
		"func_category_name": "scf",
		"func_group_name":    "scf_pie",
		"func_steps":         steps,
	}
	if _, err := json.Marshal(plan); err != nil {
		t.Fatal(err)
	}
	return plan
}

func agentStepPlan(t *testing.T, transformRequest string) map[string]interface{} {
	return planWithSteps(t, map[string]interface{}{
		"eru_studio": map[string]interface{}{
			"agent_name":        "eru_studio",
			"tenant_id":         "processo",
			"transform_request": transformRequest,
		},
	})
}

func toolStepPlan(t *testing.T, transformRequest string) map[string]interface{} {
	return planWithSteps(t, map[string]interface{}{
		"eruql_processo_execute_sql": map[string]interface{}{
			"tool_name":         "eruql_processo",
			"tool_action":       "execute_sql",
			"tenant_id":         "processo",
			"transform_request": transformRequest,
		},
	})
}

func validateAgainstSpecs(ctx context.Context, plan map[string]interface{}) []planIssue {
	return validatePlan(ctx, plan, []agents.DiscoveredAgent{studioAgentSpec()}, []agents.DiscoveredTool{sqlToolSpec()}, codeContext{})
}

func TestValidateStepPayloadAcceptsDeclaredParams(t *testing.T) {
	plan := agentStepPlan(t, `{{stringify (dict "content" .Vars.Body.content "params" (dict "context" (stringify .Vars.Body) "entities" "[]"))}}`)
	if issues := validateAgainstSpecs(context.Background(), plan); len(issues) != 0 {
		t.Fatalf("expected no issues, got %v", issues)
	}
}

func TestValidateStepPayloadRejectsUndeclaredParamKey(t *testing.T) {
	plan := agentStepPlan(t, `{{stringify (dict "content" .Vars.Body.content "params" (dict "rows" (stringify .Vars.Body)))}}`)
	issues := validateAgainstSpecs(context.Background(), plan)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d : %v", len(issues), issues)
	}
	if !strings.Contains(issues[0].Err, "does not read params.rows") {
		t.Errorf("unexpected issue : %s", issues[0].Err)
	}
	if !strings.Contains(issues[0].Err, "apis, code, context, entities") {
		t.Errorf("issue should list the accepted params keys : %s", issues[0].Err)
	}
}

func TestValidateStepPayloadRejectsUnknownTopLevelKey(t *testing.T) {
	plan := agentStepPlan(t, `{{stringify (dict "content" .Vars.Body.content "rows" (stringify .Vars.Body))}}`)
	issues := validateAgainstSpecs(context.Background(), plan)
	if len(issues) != 1 || !strings.Contains(issues[0].Err, "not part of the agent request body") {
		t.Fatalf("expected unknown top-level key issue, got %v", issues)
	}
}

func TestValidateStepPayloadRejectsMissingContent(t *testing.T) {
	plan := agentStepPlan(t, `{{.Vars.Body.content}}`)
	issues := validateAgainstSpecs(context.Background(), plan)
	if len(issues) != 1 || !strings.Contains(issues[0].Err, `without a "content" key`) {
		t.Fatalf("expected missing content issue, got %v", issues)
	}
}

func TestValidateStepPayloadRejectsEmptyTransformRequest(t *testing.T) {
	plan := agentStepPlan(t, "")
	issues := validateAgainstSpecs(context.Background(), plan)
	if len(issues) != 1 || !strings.Contains(issues[0].Err, "no transform_request") {
		t.Fatalf("expected missing transform_request issue, got %v", issues)
	}
}

func TestValidateStepPayloadToolRequiredFields(t *testing.T) {
	plan := toolStepPlan(t, `{{stringify (dict "params" (dict "query" "select 1"))}}`)
	issues := validateAgainstSpecs(context.Background(), plan)
	if len(issues) != 1 || !strings.Contains(issues[0].Err, "missing required field(s) project_id") {
		t.Fatalf("expected missing required field issue, got %v", issues)
	}
}

func TestValidateStepPayloadToolUnknownField(t *testing.T) {
	plan := toolStepPlan(t, `{{stringify (dict "params" (dict "query" "select 1" "project_id" "processo" "limit" 10))}}`)
	issues := validateAgainstSpecs(context.Background(), plan)
	if len(issues) != 1 || !strings.Contains(issues[0].Err, "does not accept") {
		t.Fatalf("expected unknown params field issue, got %v", issues)
	}
}

func TestValidateStepPayloadToolAcceptsValidParams(t *testing.T) {
	plan := toolStepPlan(t, `{{stringify (dict "params" (dict "query" "select 1" "project_id" "processo" "vars" (dict)))}}`)
	if issues := validateAgainstSpecs(context.Background(), plan); len(issues) != 0 {
		t.Fatalf("expected no issues, got %v", issues)
	}
}

func TestValidateStepPayloadToolRejectsAgentEnvelope(t *testing.T) {
	plan := toolStepPlan(t, `{{stringify (dict "content" .Vars.Body.content)}}`)
	issues := validateAgainstSpecs(context.Background(), plan)
	if len(issues) != 1 || !strings.Contains(issues[0].Err, `without a root "params" object`) {
		t.Fatalf("expected missing params issue, got %v", issues)
	}
}

func TestValidateStepPayloadSkipsDynamicParams(t *testing.T) {
	plan := toolStepPlan(t, `{{stringify (dict "params" .Vars.Body.params)}}`)
	if issues := validateAgainstSpecs(context.Background(), plan); len(issues) != 0 {
		t.Fatalf("expected no issues when params is passed by reference, got %v", issues)
	}
}

func TestValidateStepPayloadSkipsUnknownAgent(t *testing.T) {
	plan := planWithSteps(t, map[string]interface{}{
		"other": map[string]interface{}{
			"agent_name":        "other",
			"tenant_id":         "processo",
			"transform_request": `{{stringify (dict "content" .Vars.Body.content "params" (dict "rows" "x"))}}`,
		},
	})
	issues := validateAgainstSpecs(context.Background(), plan)
	for _, issue := range issues {
		if strings.Contains(issue.Err, "does not read params") {
			t.Fatalf("unknown agent must not get payload issues : %v", issues)
		}
	}
}

func TestBuildAgentDescriptionsIncludesContract(t *testing.T) {
	oa := &OrchestratorAgent{}
	oa.SetDiscoveredAgents([]agents.DiscoveredAgent{studioAgentSpec()})
	out := oa.buildAgentDescriptions()
	for _, want := range []string{
		"Params keys this agent READS: apis, code, context, entities",
		"Can call these tools itself: processo.fetch_page",
		"Params schema",
		"Type: ERU_STUDIO   Tenant: processo",
		"Response: structured",
		"Output fields (in actions[0].action): page",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("agent description missing %q :\n%s", want, out)
		}
	}
}

func TestBuildAgentDescriptionsFreeTextAgent(t *testing.T) {
	oa := &OrchestratorAgent{}
	oa.SetDiscoveredAgents([]agents.DiscoveredAgent{{
		AgentName:   "chatbot",
		TenantId:    "processo",
		Description: "answers questions",
		InputSchema: agents.AgentInputSchema(nil, nil),
	}})
	out := oa.buildAgentDescriptions()
	if !strings.Contains(out, "Response: free text") {
		t.Errorf("expected free text response note :\n%s", out)
	}
	if !strings.Contains(out, "Params keys this agent READS: none") {
		t.Errorf("expected empty params note :\n%s", out)
	}
}

func TestRenderSchemaJSONShrinksLargeSchema(t *testing.T) {
	deep := eru_models.JSONSchema{Type: "object", Properties: map[string]eru_models.JSONSchema{}}
	for i := 0; i < 40; i++ {
		leaf := eru_models.JSONSchema{Type: "object", Properties: map[string]eru_models.JSONSchema{}}
		for j := 0; j < 20; j++ {
			leaf.Properties[string(rune('a'+j))] = eru_models.JSONSchema{Type: "string", Description: strings.Repeat("x", 60)}
		}
		deep.Properties[string(rune('A'+i%26))+string(rune('a'+i/26))] = leaf
	}
	out := renderSchemaJSON(deep, 3000)
	if len(out) > 3200 {
		t.Fatalf("rendered schema not shrunk : %d chars", len(out))
	}
	if !strings.Contains(out, "omitted") && !strings.Contains(out, "too large") {
		t.Errorf("expected a truncation note : %s", out[:200])
	}
}

func TestBuildAgentDescriptionsSubOrchestrator(t *testing.T) {
	oa := &OrchestratorAgent{}
	oa.SetDiscoveredAgents([]agents.DiscoveredAgent{{
		AgentName:      "sub_planner",
		AgentType:      "ORCHESTRATOR",
		TenantId:       "processo",
		Description:    "plans sub tasks",
		InputSchema:    agents.AgentInputSchema(nil, nil),
		IsOrchestrator: true,
	}})
	out := oa.buildAgentDescriptions()
	if !strings.Contains(out, "one or more actions produced by its own sub-steps") {
		t.Errorf("expected sub-orchestrator response note :\n%s", out)
	}
	if strings.Contains(out, "func_steps") {
		t.Errorf("sub-orchestrator must not advertise its planning FuncGroup as its response :\n%s", out)
	}
}

func TestOrchestratorResponseSchemaIsNotThePlanSchema(t *testing.T) {
	ctx := context.Background()
	oa := &OrchestratorAgent{}
	if plan := oa.GetOutputSchema(ctx); plan.Type == "" {
		t.Fatal("expected a planning schema")
	}
	if response := oa.GetResponseSchema(ctx); response.Type != "" {
		t.Errorf("expected an unset response schema, got %+v", response)
	}
}

func TestValidateStepKeyUniquenessRejectsCrossBranchDuplicate(t *testing.T) {
	plan := planWithSteps(t, map[string]interface{}{
		"branch_a": map[string]interface{}{
			"agent_name":        "branch_a",
			"tenant_id":         "processo",
			"transform_request": `{{stringify (dict "content" .Vars.Body.content)}}`,
			"func_steps": map[string]interface{}{
				"generate_sql": map[string]interface{}{
					"agent_name":        "generate_sql",
					"tenant_id":         "processo",
					"transform_request": `{{stringify (dict "content" .Vars.Body.content)}}`,
				},
			},
		},
		"branch_b": map[string]interface{}{
			"agent_name":        "branch_b",
			"tenant_id":         "processo",
			"transform_request": `{{stringify (dict "content" .Vars.Body.content)}}`,
			"func_steps": map[string]interface{}{
				"generate_sql": map[string]interface{}{
					"agent_name":        "generate_sql",
					"tenant_id":         "processo",
					"transform_request": `{{stringify (dict "content" .Vars.Body.content)}}`,
				},
			},
		},
	})
	issues := validatePlanTemplates(context.Background(), plan)
	found := false
	for _, issue := range issues {
		if strings.Contains(issue.Err, `step key "generate_sql" is used 2 times`) {
			found = true
			if !strings.Contains(issue.StepPath, "branch_a.generate_sql") || !strings.Contains(issue.StepPath, "branch_b.generate_sql") {
				t.Errorf("issue should name both offending paths : %s", issue.StepPath)
			}
		}
	}
	if !found {
		t.Fatalf("expected a duplicate step key issue, got %v", issues)
	}
}

func TestValidateStepKeyUniquenessAcceptsSuffixedDuplicates(t *testing.T) {
	plan := planWithSteps(t, map[string]interface{}{
		"generate_sql": map[string]interface{}{
			"agent_name":        "generate_sql",
			"tenant_id":         "processo",
			"transform_request": `{{stringify (dict "content" .Vars.Body.content)}}`,
			"func_steps": map[string]interface{}{
				"generate_sql2": map[string]interface{}{
					"agent_name":        "generate_sql",
					"tenant_id":         "processo",
					"transform_request": `{{stringify (dict "content" .Vars.Body.content)}}`,
				},
			},
		},
	})
	for _, issue := range validatePlanTemplates(context.Background(), plan) {
		if strings.Contains(issue.Err, "is used") {
			t.Fatalf("suffixed duplicates are legal : %v", issue)
		}
	}
}
