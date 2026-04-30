package orchestrator

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func makeTestJSON(t *testing.T) *json.RawMessage {
	t.Helper()
	data := map[string]interface{}{
		"agent_type":    "ORCHESTRATOR",
		"agent_name":    "test_orchestrator",
		"description":   "test orchestrator agent",
		"system_prompt": "custom prompt",
		"max_iterations":  5,
		"thinking_budget": 8000,
		"available_agents": []map[string]interface{}{
			{
				"agent_name":   "classifier",
				"agent_type":   "REASONING",
				"description":  "classifies documents",
				"capabilities": []string{"classification", "tagging"},
				"tenant_id":    "tenant1",
			},
			{
				"agent_name":  "summarizer",
				"agent_type":  "REASONING",
				"description": "summarizes text",
				"tenant_id":   "tenant1",
			},
		},
		"delegation_strategy": "sequential",
		"max_replans":         3,
		"synthesis_prompt":    "combine all results",
	}
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("failed to marshal test data: %v", err)
	}
	raw := json.RawMessage(b)
	return &raw
}

func TestUnmarshalJSON(t *testing.T) {
	data := map[string]interface{}{
		"agent_type":          "ORCHESTRATOR",
		"agent_name":          "orch1",
		"max_iterations":      7,
		"thinking_budget":     5000,
		"delegation_strategy": "parallel",
		"max_replans":         4,
		"synthesis_prompt":    "synth prompt",
		"available_agents": []map[string]interface{}{
			{"agent_name": "a1", "agent_type": "REASONING", "description": "desc1", "tenant_id": "t1"},
		},
	}
	b, _ := json.Marshal(data)

	var oa OrchestratorAgent
	if err := json.Unmarshal(b, &oa); err != nil {
		t.Fatalf("UnmarshalJSON failed: %v", err)
	}

	if oa.AgentName != "orch1" {
		t.Errorf("expected agent_name orch1, got %s", oa.AgentName)
	}
	if oa.AgentType != "ORCHESTRATOR" {
		t.Errorf("expected agent_type ORCHESTRATOR, got %s", oa.AgentType)
	}
	if oa.MaxIterations != 7 {
		t.Errorf("expected max_iterations 7, got %d", oa.MaxIterations)
	}
	if oa.ThinkingBudget != 5000 {
		t.Errorf("expected thinking_budget 5000, got %d", oa.ThinkingBudget)
	}
	if oa.DelegationStrategy != "parallel" {
		t.Errorf("expected delegation_strategy parallel, got %s", oa.DelegationStrategy)
	}
	if oa.MaxReplans != 4 {
		t.Errorf("expected max_replans 4, got %d", oa.MaxReplans)
	}
	if oa.SynthesisPrompt != "synth prompt" {
		t.Errorf("expected synthesis_prompt 'synth prompt', got %s", oa.SynthesisPrompt)
	}
	if len(oa.AvailableAgents) != 1 {
		t.Fatalf("expected 1 available agent, got %d", len(oa.AvailableAgents))
	}
	if oa.AvailableAgents[0].AgentName != "a1" {
		t.Errorf("expected available agent a1, got %s", oa.AvailableAgents[0].AgentName)
	}
}

func TestMakeFromJsonDefaults(t *testing.T) {
	data := map[string]interface{}{
		"agent_type": "ORCHESTRATOR",
		"agent_name": "orch_defaults",
	}
	b, _ := json.Marshal(data)
	raw := json.RawMessage(b)

	oa := &OrchestratorAgent{}
	ctx := context.Background()
	if err := oa.MakeFromJson(ctx, &raw); err != nil {
		t.Fatalf("MakeFromJson failed: %v", err)
	}

	if oa.MaxReplans != 2 {
		t.Errorf("expected default max_replans 2, got %d", oa.MaxReplans)
	}
	if oa.DelegationStrategy != "adaptive" {
		t.Errorf("expected default delegation_strategy adaptive, got %s", oa.DelegationStrategy)
	}
	if oa.MaxIterations != 10 {
		t.Errorf("expected default max_iterations 10, got %d", oa.MaxIterations)
	}
	if oa.ThinkingBudget != 10000 {
		t.Errorf("expected default thinking_budget 10000, got %d", oa.ThinkingBudget)
	}
}

func TestMakeFromJsonFull(t *testing.T) {
	rj := makeTestJSON(t)
	oa := &OrchestratorAgent{}
	ctx := context.Background()

	if err := oa.MakeFromJson(ctx, rj); err != nil {
		t.Fatalf("MakeFromJson failed: %v", err)
	}

	if oa.AgentName != "test_orchestrator" {
		t.Errorf("expected agent_name test_orchestrator, got %s", oa.AgentName)
	}
	if oa.MaxIterations != 5 {
		t.Errorf("expected max_iterations 5, got %d", oa.MaxIterations)
	}
	if oa.ThinkingBudget != 8000 {
		t.Errorf("expected thinking_budget 8000, got %d", oa.ThinkingBudget)
	}
	if oa.DelegationStrategy != "sequential" {
		t.Errorf("expected delegation_strategy sequential, got %s", oa.DelegationStrategy)
	}
	if oa.MaxReplans != 3 {
		t.Errorf("expected max_replans 3, got %d", oa.MaxReplans)
	}
	if oa.SynthesisPrompt != "combine all results" {
		t.Errorf("expected synthesis_prompt, got %s", oa.SynthesisPrompt)
	}
	if len(oa.AvailableAgents) != 2 {
		t.Fatalf("expected 2 available agents, got %d", len(oa.AvailableAgents))
	}
	if oa.AvailableAgents[0].AgentName != "classifier" {
		t.Errorf("expected first agent classifier, got %s", oa.AvailableAgents[0].AgentName)
	}
	if len(oa.AvailableAgents[0].Capabilities) != 2 {
		t.Errorf("expected 2 capabilities, got %d", len(oa.AvailableAgents[0].Capabilities))
	}
}

func TestMakeFromJsonSetsProvider(t *testing.T) {
	rj := makeTestJSON(t)
	oa := &OrchestratorAgent{}
	ctx := context.Background()

	if err := oa.MakeFromJson(ctx, rj); err != nil {
		t.Fatalf("MakeFromJson failed: %v", err)
	}

	if oa.GetProvider() == nil {
		t.Fatal("expected Provider to be set after MakeFromJson")
	}
}

func TestGetSpec(t *testing.T) {
	oa := &OrchestratorAgent{}
	spec := oa.GetSpec()
	if spec != oa {
		t.Error("GetSpec should return self")
	}
}

func TestGetOutputSchema(t *testing.T) {
	oa := &OrchestratorAgent{}
	ctx := context.Background()
	schema := oa.GetOutputSchema(ctx)

	if schema.Type != "object" {
		t.Errorf("expected schema type object, got %s", schema.Type)
	}
	if len(schema.Properties) == 0 {
		t.Error("expected schema to have properties")
	}
	if _, ok := schema.Properties["func_steps"]; !ok {
		t.Error("expected schema to contain func_steps property")
	}
	if _, ok := schema.Properties["func_group_name"]; !ok {
		t.Error("expected schema to contain func_group_name property")
	}
	if _, ok := schema.Properties["func_category_name"]; !ok {
		t.Error("expected schema to contain func_category_name property")
	}
}

func TestBuildDecompositionTools(t *testing.T) {
	oa := &OrchestratorAgent{}
	ctx := context.Background()
	toolsMap := oa.buildDecompositionTools(ctx)

	if len(toolsMap) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(toolsMap))
	}
	if _, ok := toolsMap["structured_output"]; !ok {
		t.Error("expected structured_output tool in map")
	}

	toolNameI, _ := toolsMap["structured_output"].GetAttribute(ctx, "tool_name")
	if toolNameI != "structured_output" {
		t.Errorf("expected tool_name structured_output, got %v", toolNameI)
	}
}

func TestSystemPromptContainsRequiredSections(t *testing.T) {
	oa := &OrchestratorAgent{
		AvailableAgents: []AgentDescriptor{
			{
				AgentName:    "data_extractor",
				AgentType:    "REASONING",
				Description:  "extracts structured data from documents",
				Capabilities: []string{"extraction", "parsing"},
				TenantId:     "t1",
			},
			{
				AgentName:   "summarizer",
				AgentType:   "REFLEX",
				Description: "summarizes text content",
				TenantId:    "t2",
			},
		},
	}

	prompt := oa.GetSystemPrompt()

	requiredSections := []string{
		"AVAILABLE AGENTS",
		"RULE #1",
		"STEP KEY = AGENT NAME",
		"FUNCGROUP STRUCTURE",
		"STEP TYPE — AGENT ONLY",
		"EXECUTION MODEL",
		"TEMPLATE VARIABLES",
		"OPTIONAL STEP FIELDS",
		"CHECKLIST",
	}
	for _, section := range requiredSections {
		if !strings.Contains(prompt, section) {
			t.Errorf("prompt missing required section: %s", section)
		}
	}
}

func TestSystemPromptContainsAgentDescriptors(t *testing.T) {
	oa := &OrchestratorAgent{
		AvailableAgents: []AgentDescriptor{
			{
				AgentName:    "classifier",
				AgentType:    "REASONING",
				Description:  "classifies input",
				Capabilities: []string{"nlp", "classification"},
				TenantId:     "tenant_a",
			},
		},
	}

	prompt := oa.GetSystemPrompt()

	checks := []string{
		"Agent: classifier",
		"Type: REASONING",
		"Tenant: tenant_a",
		"Description: classifies input",
		"Capabilities: nlp, classification",
	}
	for _, check := range checks {
		if !strings.Contains(prompt, check) {
			t.Errorf("prompt missing agent descriptor: %s", check)
		}
	}
}

func TestSystemPromptNoAgents(t *testing.T) {
	oa := &OrchestratorAgent{}
	prompt := oa.GetSystemPrompt()

	if !strings.Contains(prompt, "No agents configured.") {
		t.Error("prompt should indicate no agents configured when list is empty")
	}
}

func TestSystemPromptAgentOnlyConstraint(t *testing.T) {
	oa := &OrchestratorAgent{}
	prompt := oa.GetSystemPrompt()

	if !strings.Contains(prompt, "Do NOT use query_name, function_name, tool_name, or api steps") {
		t.Error("prompt should explicitly prohibit non-agent step types")
	}
}

func TestSystemPromptExamples(t *testing.T) {
	oa := &OrchestratorAgent{}
	prompt := oa.GetSystemPrompt()

	if !strings.Contains(prompt, "sequential: extract data") {
		t.Error("prompt should contain sequential example")
	}
	if !strings.Contains(prompt, "parallel: two independent agents") {
		t.Error("prompt should contain parallel example")
	}
	if !strings.Contains(prompt, "parallel then sequential merge") {
		t.Error("prompt should contain parallel-then-merge example")
	}
	if !strings.Contains(prompt, "wait_for") {
		t.Error("prompt should contain wait_for in examples")
	}
}

func TestSystemPromptTemplateVariables(t *testing.T) {
	oa := &OrchestratorAgent{}
	prompt := oa.GetSystemPrompt()

	vars := []string{
		".Vars.Body",
		".ResVars.<step_key>.Body",
		".ReqVars.<step_key>.Body",
		"{{json .Vars.Body}}",
		"{{dict",
	}
	for _, v := range vars {
		if !strings.Contains(prompt, v) {
			t.Errorf("prompt missing template variable reference: %s", v)
		}
	}
}

func TestSystemPromptChecklist(t *testing.T) {
	oa := &OrchestratorAgent{}
	prompt := oa.GetSystemPrompt()

	checks := []string{
		"Every func_step key exactly matches agent_name",
		"func_category_name and func_group_name are set",
		"ONLY agent_name + tenant_id",
		"Sequential steps are NESTED, parallel steps are SIBLINGS",
		"wait_for only references sibling step keys",
	}
	for _, c := range checks {
		if !strings.Contains(prompt, c) {
			t.Errorf("prompt missing checklist item: %s", c)
		}
	}
}

func TestAgentDescriptorJSON(t *testing.T) {
	ad := AgentDescriptor{
		AgentName:    "test_agent",
		AgentType:    "REASONING",
		Description:  "does things",
		Capabilities: []string{"cap1", "cap2"},
		TenantId:     "t1",
	}

	b, err := json.Marshal(ad)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var ad2 AgentDescriptor
	if err := json.Unmarshal(b, &ad2); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if ad2.AgentName != "test_agent" || ad2.AgentType != "REASONING" || ad2.TenantId != "t1" {
		t.Errorf("roundtrip mismatch: %+v", ad2)
	}
	if len(ad2.Capabilities) != 2 || ad2.Capabilities[0] != "cap1" {
		t.Errorf("capabilities roundtrip mismatch: %v", ad2.Capabilities)
	}
}
