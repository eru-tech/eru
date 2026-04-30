package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	"github.com/eru-tech/eru/eru-ai/agents/reasoning_agents"
	models "github.com/eru-tech/eru/eru-ai/models"
	tools "github.com/eru-tech/eru/eru-ai/tools"
	utility "github.com/eru-tech/eru/eru-ai/tools/utility"
	functions "github.com/eru-tech/eru/eru-functions/functions"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	eru_utils "github.com/eru-tech/eru/eru-utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type AgentDescriptor struct {
	AgentName    string   `json:"agent_name"`
	AgentType    string   `json:"agent_type"`
	Description  string   `json:"description"`
	Capabilities []string `json:"capabilities"`
	TenantId     string   `json:"tenant_id"`
}

type OrchestratorAgent struct {
	reasoning_agents.ReasoningAgent
	AvailableAgents    []AgentDescriptor `json:"available_agents"`
	DelegationStrategy string            `json:"delegation_strategy"`
	MaxReplans         int               `json:"max_replans"`
	SynthesisPrompt    string            `json:"synthesis_prompt"`
}

func (oa *OrchestratorAgent) GetSpec() agents.AgentI {
	return oa
}

func (oa *OrchestratorAgent) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &oa.ReasoningAgent); err != nil {
		return err
	}
	type orchestratorFields struct {
		AvailableAgents    []AgentDescriptor `json:"available_agents"`
		DelegationStrategy string            `json:"delegation_strategy"`
		MaxReplans         int               `json:"max_replans"`
		SynthesisPrompt    string            `json:"synthesis_prompt"`
	}
	var of orchestratorFields
	if err := json.Unmarshal(b, &of); err != nil {
		return err
	}
	oa.AvailableAgents = of.AvailableAgents
	oa.DelegationStrategy = of.DelegationStrategy
	oa.MaxReplans = of.MaxReplans
	oa.SynthesisPrompt = of.SynthesisPrompt
	return nil
}

func (oa *OrchestratorAgent) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("OrchestratorAgent MakeFromJson - Start")
	err := oa.ReasoningAgent.MakeFromJson(ctx, rj)
	if err != nil {
		return err
	}

	type orchestratorFields struct {
		AvailableAgents    []AgentDescriptor `json:"available_agents"`
		DelegationStrategy string            `json:"delegation_strategy"`
		MaxReplans         int               `json:"max_replans"`
		SynthesisPrompt    string            `json:"synthesis_prompt"`
	}
	var of orchestratorFields
	if err := json.Unmarshal(*rj, &of); err != nil {
		return err
	}
	oa.AvailableAgents = of.AvailableAgents
	oa.DelegationStrategy = of.DelegationStrategy
	oa.MaxReplans = of.MaxReplans
	oa.SynthesisPrompt = of.SynthesisPrompt

	if oa.MaxReplans <= 0 {
		oa.MaxReplans = 2
	}
	if oa.DelegationStrategy == "" {
		oa.DelegationStrategy = "adaptive"
	}
	oa.ReasoningAgent.Agent.Provider = oa
	return nil
}

func (oa *OrchestratorAgent) Execute(ctx context.Context, agentMessage agents.AgentMessage, conversationId string, projectId string, tenantId string) (agents.AgentMessage, error) {
	logs.WithContext(ctx).Debug("OrchestratorAgent Execute - Start")
	ctx, span := otel.Tracer("eru-ai").Start(ctx, "OrchestratorAgent.Execute",
		oteltrace.WithAttributes(attribute.String("agent_name", oa.AgentName), attribute.String("conversation_id", conversationId)),
	)
	defer span.End()

	_, conversation, err := oa.LoadConversations(ctx, conversationId, agentMessage, projectId, tenantId)
	if err != nil {
		return agents.AgentMessage{}, err
	}

	bb := NewBlackboard()
	ctx = WithBlackboard(ctx, bb)

	decompositionResult, traces, err := oa.decompose(ctx, agentMessage, projectId, tenantId)
	if err != nil {
		return agents.AgentMessage{}, err
	}

	var allTraces []models.StepTrace
	allTraces = append(allTraces, traces...)

	var executionResult map[string]interface{}
	var execErr error

	for attempt := 0; attempt <= oa.MaxReplans; attempt++ {
		executionResult, execErr = oa.executeFuncGroup(ctx, decompositionResult, agentMessage, projectId, tenantId)
		if execErr == nil {
			break
		}

		logs.WithContext(ctx).Info(fmt.Sprintf("Execution failed (attempt %d/%d): %v", attempt+1, oa.MaxReplans+1, execErr))

		if attempt < oa.MaxReplans {
			replanResult, replanTraces, replanErr := oa.replan(ctx, agentMessage, decompositionResult, execErr, projectId, tenantId)
			allTraces = append(allTraces, replanTraces...)
			if replanErr != nil {
				logs.WithContext(ctx).Error(fmt.Sprintf("Re-planning failed: %v", replanErr))
				break
			}
			decompositionResult = replanResult
		}
	}

	if execErr != nil {
		return agents.AgentMessage{}, fmt.Errorf("orchestration failed after %d attempts: %w", oa.MaxReplans+1, execErr)
	}

	synthesisResult, synthesisTraces, err := oa.synthesize(ctx, agentMessage, executionResult, projectId, tenantId)
	allTraces = append(allTraces, synthesisTraces...)
	if err != nil {
		return agents.AgentMessage{}, err
	}

	agentOutput := agents.AgentMessage{
		Role: "assistant",
		Actions: []agents.AgentOutputAction{{
			ActionName: oa.AgentName,
			Action:     synthesisResult,
		}},
		Traces:           allTraces,
		MessageId:        agentMessage.MessageId,
		MessageTimestamp: time.Now(),
	}

	conversation.Messages = append(conversation.Messages, agentOutput)
	conversation.NewMessages = append(conversation.NewMessages, agentOutput)
	err = oa.SaveConversation(ctx, conversation, projectId, tenantId)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to save conversation: %v", err))
		return agents.AgentMessage{}, err
	}

	return agentOutput, nil
}

func (oa *OrchestratorAgent) decompose(ctx context.Context, agentMessage agents.AgentMessage, projectId string, tenantId string) (map[string]interface{}, []models.StepTrace, error) {
	logs.WithContext(ctx).Debug("OrchestratorAgent decompose - Start")
	ctx, span := otel.Tracer("eru-ai").Start(ctx, "OrchestratorAgent.Decompose")
	defer span.End()

	chatRequest := models.ChatRequest{
		Messages: []models.Message{{
			Role:    "user",
			Content: agentMessage.Content,
			Files:   agentMessage.Files,
		}},
	}

	toolsMap := oa.buildDecompositionTools(ctx)

	sp := oa.GetSystemPrompt()
	if oa.GetProvider() != nil {
		providerPrompt := oa.GetProvider().GetSystemPrompt()
		if providerPrompt != "" {
			sp = providerPrompt
		}
	}

	toolExecutor := func(ctx context.Context, toolName string, input map[string]interface{}) (map[string]interface{}, error) {
		return nil, fmt.Errorf("tool %s not expected during decomposition", toolName)
	}

	response, traces, err := oa.Model.RunToolLoop(ctx, chatRequest, toolsMap, sp, oa.MaxIterations, oa.ThinkingBudget, toolExecutor)
	if err != nil {
		return nil, traces, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(response.Content), &result); err != nil {
		return nil, traces, fmt.Errorf("failed to parse decomposition result: %w", err)
	}

	return result, traces, nil
}

func (oa *OrchestratorAgent) executeFuncGroup(ctx context.Context, funcGroupMap map[string]interface{}, agentMessage agents.AgentMessage, projectId string, tenantId string) (map[string]interface{}, error) {
	logs.WithContext(ctx).Debug("OrchestratorAgent executeFuncGroup - Start")
	ctx, span := otel.Tracer("eru-ai").Start(ctx, "OrchestratorAgent.ExecuteFuncGroup")
	defer span.End()

	funcGroupJSON, err := json.Marshal(funcGroupMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal FuncGroup: %w", err)
	}

	var funcGroup functions.FuncGroup
	if err := json.Unmarshal(funcGroupJSON, &funcGroup); err != nil {
		return nil, fmt.Errorf("failed to unmarshal FuncGroup: %w", err)
	}

	oa.Function = funcGroup

	result, err := oa.ExecuteAgentFunction(ctx, agentMessage, projectId, tenantId)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (oa *OrchestratorAgent) replan(ctx context.Context, agentMessage agents.AgentMessage, previousPlan map[string]interface{}, previousErr error, projectId string, tenantId string) (map[string]interface{}, []models.StepTrace, error) {
	logs.WithContext(ctx).Debug("OrchestratorAgent replan - Start")

	previousPlanJSON, _ := json.Marshal(previousPlan)

	replanContent := fmt.Sprintf(
		"The previous plan failed with error: %s\n\nPrevious plan:\n%s\n\nOriginal request: %s\n\nPlease generate a corrected FuncGroup that avoids this error.",
		previousErr.Error(),
		string(previousPlanJSON),
		agentMessage.Content,
	)

	chatRequest := models.ChatRequest{
		Messages: []models.Message{{
			Role:    "user",
			Content: replanContent,
		}},
	}

	toolsMap := oa.buildDecompositionTools(ctx)

	sp := oa.GetSystemPrompt()
	if oa.GetProvider() != nil {
		providerPrompt := oa.GetProvider().GetSystemPrompt()
		if providerPrompt != "" {
			sp = providerPrompt
		}
	}

	toolExecutor := func(ctx context.Context, toolName string, input map[string]interface{}) (map[string]interface{}, error) {
		return nil, fmt.Errorf("tool %s not expected during replanning", toolName)
	}

	response, traces, err := oa.Model.RunToolLoop(ctx, chatRequest, toolsMap, sp, oa.MaxIterations, oa.ThinkingBudget, toolExecutor)
	if err != nil {
		return nil, traces, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(response.Content), &result); err != nil {
		return nil, traces, fmt.Errorf("failed to parse replan result: %w", err)
	}

	return result, traces, nil
}

func (oa *OrchestratorAgent) synthesize(ctx context.Context, agentMessage agents.AgentMessage, executionResult map[string]interface{}, projectId string, tenantId string) (map[string]interface{}, []models.StepTrace, error) {
	logs.WithContext(ctx).Debug("OrchestratorAgent synthesize - Start")
	ctx, span := otel.Tracer("eru-ai").Start(ctx, "OrchestratorAgent.Synthesize")
	defer span.End()

	resultJSON, _ := json.Marshal(executionResult)

	synthesisPrompt := oa.SynthesisPrompt
	if synthesisPrompt == "" {
		synthesisPrompt = "Synthesize the results from the sub-agents into a coherent, unified response that directly addresses the user's request."
	}

	var bbSection string
	if bb := GetBlackboard(ctx); bb != nil {
		bbState := bb.GetAll()
		if len(bbState) > 0 {
			bbJSON, _ := json.Marshal(bbState)
			bbSection = fmt.Sprintf("\n\nShared state (blackboard):\n%s", string(bbJSON))
		}
	}

	synthesisContent := fmt.Sprintf(
		"Original request: %s\n\nSub-agent results:\n%s%s\n\n%s",
		agentMessage.Content,
		string(resultJSON),
		bbSection,
		synthesisPrompt,
	)

	chatRequest := models.ChatRequest{
		Messages: []models.Message{{
			Role:    "user",
			Content: synthesisContent,
		}},
	}

	response, err := oa.Model.QueryModel(ctx, chatRequest)
	if err != nil {
		return nil, nil, err
	}

	trace := models.StepTrace{
		Iteration: 1,
		Content:   response.Content,
		Timestamp: time.Now(),
	}

	result := map[string]interface{}{
		"response": response.Content,
	}
	responseMap := map[string]interface{}{}
	if jsonErr := json.Unmarshal([]byte(response.Content), &responseMap); jsonErr == nil {
		result = responseMap
	}

	return result, []models.StepTrace{trace}, nil
}

func (oa *OrchestratorAgent) buildDecompositionTools(ctx context.Context) map[string]tools.Tooling {
	toolsMap := make(map[string]tools.Tooling)
	outputSchema := oa.GetOutputSchema(ctx)
	if outputSchema.Type != "" {
		outputTool := &utility.StructuredOutputTool{}
		outputTool.SetAttribute(ctx, "output_schema", outputSchema)
		outputTool.SetAttribute(ctx, "parameters", outputSchema)
		outputTool.SetAttribute(ctx, "description", "Output the final FuncGroup JSON. Call this tool when you have the complete orchestration plan ready.")
		outputTool.SetAttribute(ctx, "tool_name", "structured_output")
		outputTool.SetAttribute(ctx, "tool_type", "STRUCTURED_OUTPUT")
		outputTool.SetToolAction("structured_output")
		toolsMap["structured_output"] = outputTool
	}
	return toolsMap
}

func (oa *OrchestratorAgent) GetOutputSchema(ctx context.Context) eru_models.JSONSchema {
	sampleFuncGroup := functions.FuncGroup{
		FuncCategoryName:        "sample",
		FuncGroupName:           "sample",
		ResponseStatusCode:      200,
		ResponseStatusCondition: "ERROR",
		ResponseContentType:     "application/json",
		FuncSteps: map[string]*functions.FuncStep{
			"sample_agent": {
				AgentName: "sample",
				TenantId:  "sample",
				FuncSteps: map[string]*functions.FuncStep{},
			},
		},
	}
	return eru_utils.StructToJSONSchema(reflect.TypeOf(sampleFuncGroup), []string{})
}

func (oa *OrchestratorAgent) GetSystemPrompt() string {
	agentDescriptions := oa.buildAgentDescriptions()

	systemPrompt := `You are an expert orchestration engineer for the Eru platform.
Your job is to decompose complex user tasks into a FuncGroup that coordinates multiple sub-agents.
Output ONLY the FuncGroup JSON via the structured_output tool. No markdown, no explanations.

============================================================
AVAILABLE AGENTS
============================================================

` + agentDescriptions + `

============================================================
RULE #1 — STEP KEY = AGENT NAME (most common mistake)
============================================================

Every func_step key MUST exactly equal the agent_name value of that step.

CORRECT:
  {"classifier": {"agent_name": "classifier", "tenant_id": "t1"}}
  {"summarizer": {"agent_name": "summarizer", "tenant_id": "t1"}}

WRONG:
  {"step1": {"agent_name": "classifier", "tenant_id": "t1"}}
  {"classify_data": {"agent_name": "classifier", "tenant_id": "t1"}}

Duplicates at the same level: append numeric suffix — "classifier", "classifier2".

============================================================
FUNCGROUP STRUCTURE
============================================================

{
  "func_category_name": "<snake_case, MANDATORY>",
  "func_group_name":    "<snake_case, MANDATORY>",
  "response_status_code": 200,
  "response_status_condition": "ERROR",
  "response_content_type": "application/json",
  "func_steps": { ... }
}

============================================================
STEP TYPE — AGENT ONLY
============================================================

You may ONLY use agent steps. Each step requires:
  "agent_name": "<name from available agents above>"
  "tenant_id": "<tenant_id from available agents above>"

Do NOT use query_name, function_name, tool_name, or api steps.

============================================================
EXECUTION MODEL
============================================================

Two rules:
  - SIBLING steps (same func_steps map) → run in PARALLEL
  - NESTED steps (func_steps inside a parent) → run SEQUENTIALLY after parent

Sequential (A then B): nest B inside A's func_steps.
Parallel (A and B): place both as siblings.
Default to sequential (nesting) unless the task clearly benefits from parallelism.

If parent fails, nested children do NOT execute — no success-check conditions needed.

Example — sequential: extract data, then summarize it:
{
  "extractor": {
    "agent_name": "extractor",
    "tenant_id": "t1",
    "transform_request": "{{json .Vars.Body}}",
    "func_steps": {
      "summarizer": {
        "agent_name": "summarizer",
        "tenant_id": "t1",
        "transform_request": "{{json .ResVars.extractor.Body}}"
      }
    }
  }
}

Example — parallel: two independent agents, then merge:
{
  "sentiment_analyzer": {
    "agent_name": "sentiment_analyzer",
    "tenant_id": "t1",
    "transform_request": "{{json .Vars.Body}}"
  },
  "topic_classifier": {
    "agent_name": "topic_classifier",
    "tenant_id": "t1",
    "transform_request": "{{json .Vars.Body}}"
  }
}

Example — parallel then sequential merge:
{
  "sentiment_analyzer": {
    "agent_name": "sentiment_analyzer",
    "tenant_id": "t1",
    "transform_request": "{{json .Vars.Body}}"
  },
  "topic_classifier": {
    "agent_name": "topic_classifier",
    "tenant_id": "t1",
    "transform_request": "{{json .Vars.Body}}",
    "func_steps": {
      "report_generator": {
        "agent_name": "report_generator",
        "tenant_id": "t1",
        "wait_for": "sentiment_analyzer",
        "transform_request": "{{dict \"sentiment\" .ResVars.sentiment_analyzer.Body \"topics\" .ResVars.topic_classifier.Body \"original\" .Vars.Body}}"
      }
    }
  }
}

============================================================
TEMPLATE VARIABLES
============================================================

.Vars.Body              — original user input (request body)
.Vars.Headers           — original request headers
.Vars.Params            — original query params
.Vars.Token             — auth token
.Vars.LoopVar           — current loop item
.Vars.LoopVars          — full loop array
.ResVars.<step_key>.Body — response FROM a completed step
.ReqVars.<step_key>.Body — request sent TO a step

Syntax:
  {{.Vars.Body.field}}
  {{index .Vars.Body "field-with-dash"}}
  {{json .Vars.Body}}
  {{dict "key1" .ResVars.agent1.Body "key2" .ResVars.agent2.Body}}

Conditions:
  {{if eq .Vars.Body.status "active"}}true{{else}}false{{end}}

============================================================
OPTIONAL STEP FIELDS
============================================================

Transforms:
  "transform_request":  "<Go template for request body>"
  "transform_response": "<Go template for response body>"

Conditional:
  "condition": "<Go template → 'true' or 'false'>"
  "condition_fail_action": "ERROR | STOP | IGNORE"
  "condition_fail_message": "<Go template>"

Looping:
  "loop_variable": "<Go template → JSON array>"
  "loop_in_parallel": <bool>

Synchronization:
  "wait_for": "<sibling step key>" (ONLY for parallel siblings)

============================================================
CHECKLIST (verify before outputting)
============================================================

[ ] Every func_step key exactly matches agent_name (Rule #1)
[ ] func_category_name and func_group_name are set (snake_case)
[ ] Each step uses ONLY agent_name + tenant_id (no query/function/tool/api)
[ ] Sequential steps are NESTED, parallel steps are SIBLINGS
[ ] transform_request passes data correctly between agents
[ ] Only agents from the available agents list are used
[ ] wait_for only references sibling step keys, not nested ones

--- GUIDELINES ---
{{GUIDELINES_PLACEHOLDER}}

--- EXAMPLES ---
{{EXAMPLES_PLACEHOLDER}}
`
	return systemPrompt
}

func (oa *OrchestratorAgent) buildAgentDescriptions() string {
	if len(oa.AvailableAgents) == 0 {
		return "No agents configured."
	}

	var sb strings.Builder
	for _, ad := range oa.AvailableAgents {
		sb.WriteString(fmt.Sprintf("Agent: %s\n", ad.AgentName))
		sb.WriteString(fmt.Sprintf("  Type: %s\n", ad.AgentType))
		sb.WriteString(fmt.Sprintf("  Tenant: %s\n", ad.TenantId))
		sb.WriteString(fmt.Sprintf("  Description: %s\n", ad.Description))
		if len(ad.Capabilities) > 0 {
			sb.WriteString(fmt.Sprintf("  Capabilities: %s\n", strings.Join(ad.Capabilities, ", ")))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
