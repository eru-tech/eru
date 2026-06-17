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

const orchestratorClarificationGuidance = `

HUMAN-IN-THE-LOOP CLARIFICATION:
If the task is too ambiguous or missing information you need to build a correct orchestration plan, call the ask_user tool instead of outputting a FuncGroup. Ask the fewest questions needed, give 2-4 concrete options per question, and allow free text when options may not be exhaustive. Calling ask_user ends your turn; the user's answers arrive as a follow-up message and you then produce the plan.`

type OrchestratorAgent struct {
	reasoning_agents.ReasoningAgent
	AllowedAgents      []string `json:"available_agents"`
	DelegationStrategy string   `json:"delegation_strategy"`
	MaxReplans         int      `json:"max_replans"`
	SynthesisPrompt    string   `json:"synthesis_prompt"`
	discoveredAgents   []agents.DiscoveredAgent
}

func (oa *OrchestratorAgent) AllowedAgentNames() []string {
	return oa.AllowedAgents
}

func (oa *OrchestratorAgent) SetDiscoveredAgents(discovered []agents.DiscoveredAgent) {
	oa.discoveredAgents = discovered
}

func (oa *OrchestratorAgent) GetSpec() agents.AgentI {
	return oa
}

func (oa *OrchestratorAgent) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &oa.ReasoningAgent); err != nil {
		return err
	}
	type orchestratorFields struct {
		AllowedAgents      []string `json:"available_agents"`
		DelegationStrategy string   `json:"delegation_strategy"`
		MaxReplans         int      `json:"max_replans"`
		SynthesisPrompt    string   `json:"synthesis_prompt"`
	}
	var of orchestratorFields
	if err := json.Unmarshal(b, &of); err != nil {
		return err
	}
	oa.AllowedAgents = of.AllowedAgents
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
		AllowedAgents      []string `json:"available_agents"`
		DelegationStrategy string   `json:"delegation_strategy"`
		MaxReplans         int      `json:"max_replans"`
		SynthesisPrompt    string   `json:"synthesis_prompt"`
	}
	var of orchestratorFields
	if err := json.Unmarshal(*rj, &of); err != nil {
		return err
	}
	oa.AllowedAgents = of.AllowedAgents
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

	if oa.EnableClarification {
		if answers, ok := agentMessage.ClarificationAnswers(); ok {
			pendingMsg, qa, found := agents.PendingQuestion(conversation)
			if pr := loadPendingResume(pendingMsg); found && pr != nil && len(pr.PausedBranches) > 0 {
				return oa.resumeOrchestration(ctx, pr, agentMessage, conversation, conversationId, projectId, tenantId)
			}
			var req agents.ClarificationRequest
			if found {
				req, _ = agents.ParseClarificationRequest(qa.Action)
			}
			answerText := agents.FormatAnswersForModel(req, answers)
			if strings.TrimSpace(agentMessage.Content) != "" {
				agentMessage.Content = agentMessage.Content + "\n\n" + answerText
			} else {
				agentMessage.Content = answerText
			}
		}
	}

	bb := NewBlackboard()
	ctx = WithBlackboard(ctx, bb)

	streamCb := agents.GetStreamCallback(ctx)
	emitStatus := func(stage string) {
		if streamCb != nil {
			streamCb(agents.StreamEvent{Event: agents.StreamEventStatus, Data: stage})
		}
	}

	emitStatus("planning")
	decompositionResult, decompositionQuestion, traces, err := oa.decompose(ctx, agentMessage, projectId, tenantId)
	if err != nil {
		return agents.AgentMessage{}, err
	}

	var allTraces []models.StepTrace
	allTraces = append(allTraces, traces...)

	if decompositionQuestion != nil {
		return oa.emitClarification(ctx, *decompositionQuestion, nil, allTraces, agentMessage, conversation, projectId, tenantId)
	}

	assignStableConversationIds(decompositionResult, conversationId)

	var executionResult map[string]interface{}
	var funcVarsMap map[string]functions.FuncTemplateVars
	var execErr error

	for attempt := 0; attempt <= oa.MaxReplans; attempt++ {
		emitStatus("executing")
		executionResult, funcVarsMap, execErr = oa.executeFuncGroup(ctx, decompositionResult, agentMessage, projectId, tenantId, "", "", nil, nil)
		if execErr == nil {
			break
		}

		logs.WithContext(ctx).Info(fmt.Sprintf("Execution failed (attempt %d/%d): %v", attempt+1, oa.MaxReplans+1, execErr))

		if attempt < oa.MaxReplans {
			replanResult, replanQuestion, replanTraces, replanErr := oa.replan(ctx, agentMessage, decompositionResult, execErr, projectId, tenantId)
			allTraces = append(allTraces, replanTraces...)
			if replanErr != nil {
				logs.WithContext(ctx).Error(fmt.Sprintf("Re-planning failed: %v", replanErr))
				break
			}
			if replanQuestion != nil {
				return oa.emitClarification(ctx, *replanQuestion, nil, allTraces, agentMessage, conversation, projectId, tenantId)
			}
			assignStableConversationIds(replanResult, conversationId)
			decompositionResult = replanResult
		}
	}

	if execErr != nil {
		return agents.AgentMessage{}, fmt.Errorf("orchestration failed after %d attempts: %w", oa.MaxReplans+1, execErr)
	}

	if oa.EnableClarification {
		if pr, merged, paused := buildPendingResume(decompositionResult, funcVarsMap, agentMessage.MessageId); paused {
			return oa.emitClarification(ctx, merged, &pr, allTraces, agentMessage, conversation, projectId, tenantId)
		}
	}

	emitStatus("synthesizing")
	synthesisResult, synthesisTraces, err := oa.synthesize(ctx, agentMessage, executionResult, projectId, tenantId)
	allTraces = append(allTraces, synthesisTraces...)
	if err != nil {
		return agents.AgentMessage{}, err
	}

	agentOutput := agents.AgentMessage{
		Role: "assistant",
		Actions: []agents.AgentOutputAction{{
			ActionType: agents.ActionTypeAnswer,
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

// emitClarification persists and returns a question action, pausing the
// orchestration until the user answers in the same conversation. When pending
// is non-nil the resume checkpoint is stored on the message so the next turn
// can resume only the remaining steps.
func (oa *OrchestratorAgent) emitClarification(ctx context.Context, req agents.ClarificationRequest, pending *PendingResume, traces []models.StepTrace, agentMessage agents.AgentMessage, conversation *agents.Conversation, projectId string, tenantId string) (agents.AgentMessage, error) {
	streamCb := agents.GetStreamCallback(ctx)
	if streamCb != nil {
		action := req.ToAction(oa.AgentName)
		streamCb(agents.StreamEvent{Event: agents.StreamEventQuestion, Data: action.Action})
	}

	agentOutput := agents.AgentMessage{
		Role:             "assistant",
		Actions:          []agents.AgentOutputAction{req.ToAction(oa.AgentName)},
		Traces:           traces,
		MessageId:        agentMessage.MessageId,
		MessageTimestamp: time.Now(),
	}
	if pending != nil {
		prBytes, _ := json.Marshal(pending)
		var prMap map[string]interface{}
		if json.Unmarshal(prBytes, &prMap) == nil {
			agentOutput.Params = map[string]interface{}{PendingResumeParamKey: prMap}
		}
	}

	conversation.Messages = append(conversation.Messages, agentOutput)
	conversation.NewMessages = append(conversation.NewMessages, agentOutput)
	if err := oa.SaveConversation(ctx, conversation, projectId, tenantId); err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to save conversation: %v", err))
		return agents.AgentMessage{}, err
	}
	return agentOutput, nil
}

// loadPendingResume reads a resume checkpoint persisted on a question message.
func loadPendingResume(msg agents.AgentMessage) *PendingResume {
	if msg.Params == nil {
		return nil
	}
	raw, ok := msg.Params[PendingResumeParamKey]
	if !ok {
		return nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var pr PendingResume
	if err := json.Unmarshal(b, &pr); err != nil {
		return nil
	}
	return &pr
}

// resumeOrchestration resumes a paused plan: it re-runs each paused branch from
// its own step (bounded so the join child does not run), seeded with the
// completed steps' outputs, then runs the join step with the merged results.
// Parallel siblings already finished on the original run, so branch-resumes are
// independent; they are run sequentially here (oa.Function is mutated per call)
// and their disjoint outputs merged. Re-pause is handled by the same path.
func (oa *OrchestratorAgent) resumeOrchestration(ctx context.Context, pr *PendingResume, agentMessage agents.AgentMessage, conversation *agents.Conversation, conversationId string, projectId string, tenantId string) (agents.AgentMessage, error) {
	logs.WithContext(ctx).Debug("OrchestratorAgent resumeOrchestration - Start")
	bb := NewBlackboard()
	ctx = WithBlackboard(ctx, bb)

	reqVars := unmarshalVars(pr.ReqVarsJSON)
	merged := unmarshalVars(pr.ResVarsJSON)
	var allTraces []models.StepTrace

	for _, branch := range pr.PausedBranches {
		_, fvm, err := oa.executeFuncGroup(ctx, pr.Plan, agentMessage, projectId, tenantId, branch.StartStep, branch.EndStep, reqVars, merged)
		if err != nil {
			return agents.AgentMessage{}, fmt.Errorf("branch resume %s failed: %w", branch.StartStep, err)
		}
		for k, v := range extractResVars(fvm) {
			merged[k] = v
		}
	}

	var executionResult map[string]interface{}
	if pr.JoinStep != "" {
		res, fvm, err := oa.executeFuncGroup(ctx, pr.Plan, agentMessage, projectId, tenantId, pr.JoinStep, "", reqVars, merged)
		if err != nil {
			return agents.AgentMessage{}, fmt.Errorf("join resume %s failed: %w", pr.JoinStep, err)
		}
		for k, v := range extractResVars(fvm) {
			merged[k] = v
		}
		executionResult = res
	} else {
		executionResult = resVarsToResult(merged)
	}

	if newPr, newMerged, paused := buildPendingResumeFromVars(pr.Plan, merged, agentMessage.MessageId); paused {
		return oa.emitClarification(ctx, newMerged, &newPr, allTraces, agentMessage, conversation, projectId, tenantId)
	}

	synthesisResult, synthesisTraces, err := oa.synthesize(ctx, agentMessage, executionResult, projectId, tenantId)
	allTraces = append(allTraces, synthesisTraces...)
	if err != nil {
		return agents.AgentMessage{}, err
	}

	agentOutput := agents.AgentMessage{
		Role: "assistant",
		Actions: []agents.AgentOutputAction{{
			ActionType: agents.ActionTypeAnswer,
			ActionName: oa.AgentName,
			Action:     synthesisResult,
		}},
		Traces:           allTraces,
		MessageId:        agentMessage.MessageId,
		MessageTimestamp: time.Now(),
	}
	conversation.Messages = append(conversation.Messages, agentOutput)
	conversation.NewMessages = append(conversation.NewMessages, agentOutput)
	if err := oa.SaveConversation(ctx, conversation, projectId, tenantId); err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to save conversation: %v", err))
		return agents.AgentMessage{}, err
	}
	return agentOutput, nil
}

func (oa *OrchestratorAgent) decompose(ctx context.Context, agentMessage agents.AgentMessage, projectId string, tenantId string) (map[string]interface{}, *agents.ClarificationRequest, []models.StepTrace, error) {
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
	if oa.EnableClarification {
		sp = sp + orchestratorClarificationGuidance
	}

	toolExecutor := func(ctx context.Context, toolName string, input map[string]interface{}) (map[string]interface{}, error) {
		return nil, fmt.Errorf("tool %s not expected during decomposition", toolName)
	}

	response, traces, err := oa.Model.RunToolLoop(ctx, chatRequest, toolsMap, sp, oa.MaxIterations, oa.ThinkingBudget, toolExecutor)
	if err != nil {
		return nil, nil, traces, err
	}

	if response.TerminalTool == models.TerminalToolAskUser {
		var action map[string]interface{}
		if err := json.Unmarshal([]byte(response.Content), &action); err != nil {
			return nil, nil, traces, fmt.Errorf("failed to parse clarification request: %w", err)
		}
		req, err := agents.ParseClarificationRequest(action)
		if err != nil {
			return nil, nil, traces, err
		}
		return nil, &req, traces, nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(response.Content), &result); err != nil {
		return nil, nil, traces, fmt.Errorf("failed to parse decomposition result: %w", err)
	}

	return result, nil, traces, nil
}

func (oa *OrchestratorAgent) executeFuncGroup(ctx context.Context, funcGroupMap map[string]interface{}, agentMessage agents.AgentMessage, projectId string, tenantId string, startStep string, endStep string, reqVars map[string]*functions.TemplateVars, resVars map[string]*functions.TemplateVars) (map[string]interface{}, map[string]functions.FuncTemplateVars, error) {
	logs.WithContext(ctx).Debug("OrchestratorAgent executeFuncGroup - Start")
	ctx, span := otel.Tracer("eru-ai").Start(ctx, "OrchestratorAgent.ExecuteFuncGroup")
	defer span.End()

	funcGroupJSON, err := json.Marshal(funcGroupMap)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal FuncGroup: %w", err)
	}

	var funcGroup functions.FuncGroup
	if err := json.Unmarshal(funcGroupJSON, &funcGroup); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal FuncGroup: %w", err)
	}

	oa.Function = funcGroup

	result, funcVarsMap, err := oa.ExecuteAgentFunctionResumable(ctx, agentMessage, projectId, tenantId, startStep, endStep, reqVars, resVars)
	if err != nil {
		return nil, nil, err
	}

	return result, funcVarsMap, nil
}

func (oa *OrchestratorAgent) replan(ctx context.Context, agentMessage agents.AgentMessage, previousPlan map[string]interface{}, previousErr error, projectId string, tenantId string) (map[string]interface{}, *agents.ClarificationRequest, []models.StepTrace, error) {
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
		return nil, nil, traces, err
	}

	if response.TerminalTool == models.TerminalToolAskUser {
		var action map[string]interface{}
		if err := json.Unmarshal([]byte(response.Content), &action); err != nil {
			return nil, nil, traces, fmt.Errorf("failed to parse clarification request: %w", err)
		}
		req, err := agents.ParseClarificationRequest(action)
		if err != nil {
			return nil, nil, traces, err
		}
		return nil, &req, traces, nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(response.Content), &result); err != nil {
		return nil, nil, traces, fmt.Errorf("failed to parse replan result: %w", err)
	}

	return result, nil, traces, nil
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

	var response models.Message
	var err error
	streamCb := agents.GetStreamCallback(ctx)
	if streamingModel, ok := oa.Model.(models.StreamingModelI); ok && streamCb != nil {
		response, err = streamingModel.QueryModelStreaming(ctx, chatRequest, func(chunk string) {
			streamCb(agents.StreamEvent{Event: agents.StreamEventTextDelta, Data: chunk})
		})
	} else {
		response, err = oa.Model.QueryModel(ctx, chatRequest)
	}
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
	if oa.EnableClarification {
		askTool := &utility.AskUserTool{}
		askTool.SetAttribute(ctx, "parameters", utility.AskUserToolSchema())
		askTool.SetAttribute(ctx, "description", "Ask the user clarifying questions when the task is ambiguous or missing information needed to plan the orchestration. Provide 2-4 concrete options per question and allow free text when options may not be exhaustive.")
		askTool.SetAttribute(ctx, "system_prompt", "")
		askTool.SetAttribute(ctx, "tool_name", utility.AskUserToolName)
		askTool.SetAttribute(ctx, "tool_type", "ASK_USER")
		askTool.SetToolAction(utility.AskUserToolName)
		toolsMap[utility.AskUserToolName] = askTool
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
	if len(oa.discoveredAgents) == 0 {
		return "No agents configured."
	}

	var sb strings.Builder
	for _, ad := range oa.discoveredAgents {
		sb.WriteString(fmt.Sprintf("Agent: %s\n", ad.AgentName))
		sb.WriteString(fmt.Sprintf("  Type: %s\n", ad.AgentType))
		sb.WriteString(fmt.Sprintf("  Tenant: %s\n", ad.TenantId))
		sb.WriteString(fmt.Sprintf("  Description: %s\n", ad.Description))
		sb.WriteString("\n")
	}
	return sb.String()
}
