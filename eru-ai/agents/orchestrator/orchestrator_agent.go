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

type AvailableTool struct {
	ToolName string   `json:"tool_name"`
	Actions  []string `json:"actions"`
}

type OrchestratorAgent struct {
	reasoning_agents.ReasoningAgent
	AllowedAgents      []string        `json:"available_agents"`
	AvailableTools     []AvailableTool `json:"available_tools"`
	ClientOutputAgents []string        `json:"client_output_agents"`
	DelegationStrategy string          `json:"delegation_strategy"`
	MaxReplans         int             `json:"max_replans"`
	SynthesisPrompt    string          `json:"synthesis_prompt"`
	discoveredAgents   []agents.DiscoveredAgent
	discoveredTools    []agents.DiscoveredTool
}

func (oa *OrchestratorAgent) AllowedAgentNames() []string {
	return oa.AllowedAgents
}

func (oa *OrchestratorAgent) SetDiscoveredAgents(discovered []agents.DiscoveredAgent) {
	oa.discoveredAgents = discovered
}

func (oa *OrchestratorAgent) AllowedToolActions() map[string][]string {
	allowed := make(map[string][]string)
	for _, t := range oa.AvailableTools {
		allowed[t.ToolName] = t.Actions
	}
	return allowed
}

func (oa *OrchestratorAgent) SetDiscoveredTools(discovered []agents.DiscoveredTool) {
	oa.discoveredTools = discovered
}

func (oa *OrchestratorAgent) GetSpec() agents.AgentI {
	return oa
}

func (oa *OrchestratorAgent) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &oa.ReasoningAgent); err != nil {
		return err
	}
	type orchestratorFields struct {
		AllowedAgents      []string        `json:"available_agents"`
		AvailableTools     []AvailableTool `json:"available_tools"`
		ClientOutputAgents []string        `json:"client_output_agents"`
		DelegationStrategy string          `json:"delegation_strategy"`
		MaxReplans         int             `json:"max_replans"`
		SynthesisPrompt    string          `json:"synthesis_prompt"`
	}
	var of orchestratorFields
	if err := json.Unmarshal(b, &of); err != nil {
		return err
	}
	oa.AllowedAgents = of.AllowedAgents
	oa.AvailableTools = of.AvailableTools
	oa.ClientOutputAgents = of.ClientOutputAgents
	oa.DelegationStrategy = of.DelegationStrategy
	oa.MaxReplans = of.MaxReplans
	oa.SynthesisPrompt = of.SynthesisPrompt
	if oa.MaxReplans <= 0 {
		oa.MaxReplans = 2
	}
	if oa.DelegationStrategy == "" {
		oa.DelegationStrategy = "adaptive"
	}
	return nil
}

func (oa *OrchestratorAgent) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("OrchestratorAgent MakeFromJson - Start")
	if err := json.Unmarshal(*rj, oa); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
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
	decompositionResult, decompositionQuestion, directAnswer, traces, err := oa.decompose(ctx, agentMessage, projectId, tenantId)
	if err != nil {
		return agents.AgentMessage{}, err
	}

	var allTraces []models.StepTrace
	allTraces = append(allTraces, traces...)

	if decompositionQuestion != nil {
		return oa.emitClarification(ctx, *decompositionQuestion, nil, allTraces, agentMessage, conversation, projectId, tenantId)
	}

	if directAnswer != "" {
		return oa.emitDirectAnswer(ctx, directAnswer, allTraces, agentMessage, conversation, projectId, tenantId)
	}

	decompositionResult, repairTraces, err := oa.repairPlan(ctx, agentMessage, decompositionResult, projectId, tenantId)
	allTraces = append(allTraces, repairTraces...)
	if err != nil {
		return agents.AgentMessage{}, err
	}

	assignStableConversationIds(decompositionResult, conversationId)

	oa.logPlan(ctx, decompositionResult, conversationId)
	if streamCb != nil {
		streamCb(agents.StreamEvent{Event: agents.StreamEventPlan, Data: oa.planEventData(ctx, decompositionResult)})
	}

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
			replanResult, replanRepairTraces, replanRepairErr := oa.repairPlan(ctx, agentMessage, replanResult, projectId, tenantId)
			allTraces = append(allTraces, replanRepairTraces...)
			if replanRepairErr != nil {
				logs.WithContext(ctx).Error(fmt.Sprint("re-planned plan failed template validation: ", replanRepairErr.Error()))
				break
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

	agentActions := []agents.AgentOutputAction{{
		ActionType: agents.ActionTypeAnswer,
		ActionName: oa.AgentName,
		Action:     synthesisResult,
	}}
	agentActions = append(agentActions, oa.collectClientOutputs(ctx, decompositionResult, funcVarsMap, executionResult)...)

	agentOutput := agents.AgentMessage{
		Role:             "assistant",
		Actions:          agentActions,
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

	agentOutput.Traces = clientTraces(ctx, agentOutput.Traces)
	return agentOutput, nil
}

// logPlan records the full FuncGroup server side. The plan is internal
// orchestration detail, so this log — not the response — is where it belongs.
func (oa *OrchestratorAgent) logPlan(ctx context.Context, plan map[string]interface{}, conversationId string) {
	planJSON, err := json.Marshal(plan)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprint("failed to marshal plan for logging : ", err.Error()))
		return
	}
	logs.WithContext(ctx).Info(fmt.Sprint("orchestrator plan - agent=", oa.AgentName, " conversation_id=", conversationId, " plan=", string(planJSON)))
}

// planEventData returns what the client receives on the plan event: the step
// graph only, or the whole FuncGroup when raw output was requested.
func (oa *OrchestratorAgent) planEventData(ctx context.Context, plan map[string]interface{}) interface{} {
	if agents.RawOutputEnabled(ctx) {
		return plan
	}
	summary, err := summarizePlan(plan)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprint("failed to summarize plan : ", err.Error()))
		return planSummary{}
	}
	return summary
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
	agentOutput.Traces = clientTraces(ctx, agentOutput.Traces)
	return agentOutput, nil
}

// emitDirectAnswer persists and returns a plain answer produced by the planner
// itself when the task needs no sub-agents or tools. The text has already been
// streamed to the client via text_delta events during decomposition, so this
// only records the final answer action and saves the conversation.
func (oa *OrchestratorAgent) emitDirectAnswer(ctx context.Context, answer string, traces []models.StepTrace, agentMessage agents.AgentMessage, conversation *agents.Conversation, projectId string, tenantId string) (agents.AgentMessage, error) {
	agentOutput := agents.AgentMessage{
		Role: "assistant",
		Actions: []agents.AgentOutputAction{{
			ActionType: agents.ActionTypeAnswer,
			ActionName: oa.AgentName,
			Action:     map[string]interface{}{"response": answer},
		}},
		Traces:           traces,
		MessageId:        agentMessage.MessageId,
		MessageTimestamp: time.Now(),
	}

	conversation.Messages = append(conversation.Messages, agentOutput)
	conversation.NewMessages = append(conversation.NewMessages, agentOutput)
	if err := oa.SaveConversation(ctx, conversation, projectId, tenantId); err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to save conversation: %v", err))
		return agents.AgentMessage{}, err
	}
	agentOutput.Traces = clientTraces(ctx, agentOutput.Traces)
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

	resumeActions := []agents.AgentOutputAction{{
		ActionType: agents.ActionTypeAnswer,
		ActionName: oa.AgentName,
		Action:     synthesisResult,
	}}
	resumeActions = append(resumeActions, oa.collectClientOutputs(ctx, pr.Plan, map[string]functions.FuncTemplateVars{pr.JoinStep: {ResVars: merged}}, executionResult)...)

	agentOutput := agents.AgentMessage{
		Role:             "assistant",
		Actions:          resumeActions,
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
	agentOutput.Traces = clientTraces(ctx, agentOutput.Traces)
	return agentOutput, nil
}

func (oa *OrchestratorAgent) decompose(ctx context.Context, agentMessage agents.AgentMessage, projectId string, tenantId string) (map[string]interface{}, *agents.ClarificationRequest, string, []models.StepTrace, error) {
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

	var response models.Message
	var traces []models.StepTrace
	var err error
	streamCb := agents.GetStreamCallback(ctx)
	if streamingModel, ok := oa.Model.(models.StreamingModelI); ok && streamCb != nil {
		modelCb := func(me models.ModelStreamEvent) {
			streamCb(agents.StreamEvent{Event: string(me.Type), Data: me, Iteration: me.Iteration})
		}
		response, traces, err = streamingModel.RunToolLoopStreaming(ctx, chatRequest, toolsMap, sp, oa.MaxIterations, oa.ThinkingBudget, toolExecutor, modelCb)
	} else {
		response, traces, err = oa.Model.RunToolLoop(ctx, chatRequest, toolsMap, sp, oa.MaxIterations, oa.ThinkingBudget, toolExecutor)
	}
	if err != nil {
		return nil, nil, "", traces, err
	}

	if response.TerminalTool == models.TerminalToolAskUser {
		var action map[string]interface{}
		if err := json.Unmarshal([]byte(response.Content), &action); err != nil {
			return nil, nil, "", traces, fmt.Errorf("failed to parse clarification request: %w", err)
		}
		req, err := agents.ParseClarificationRequest(action)
		if err != nil {
			return nil, nil, "", traces, err
		}
		return nil, &req, "", traces, nil
	}

	if response.TerminalTool != models.TerminalToolStructuredOutput {
		return nil, nil, response.Content, traces, nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(response.Content), &result); err != nil {
		return nil, nil, "", traces, fmt.Errorf("failed to parse decomposition result: %w", err)
	}

	return result, nil, "", traces, nil
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

// repairPlan parses every go template in the plan and, when any is invalid, feeds
// the parse errors back to the model for correction. Templates are validated
// before execution so a malformed one cannot half-run the plan and leave side
// effects behind.
func (oa *OrchestratorAgent) repairPlan(ctx context.Context, agentMessage agents.AgentMessage, plan map[string]interface{}, projectId string, tenantId string) (map[string]interface{}, []models.StepTrace, error) {
	logs.WithContext(ctx).Debug("OrchestratorAgent repairPlan - Start")
	maxAttempts := oa.RetryCount
	if maxAttempts < 2 {
		maxAttempts = 2
	}
	var traces []models.StepTrace
	for attempt := 0; ; attempt++ {
		issues := validatePlan(ctx, plan, oa.discoveredAgents, oa.discoveredTools)
		if len(issues) == 0 {
			return plan, traces, nil
		}
		issueText := formatPlanIssues(issues)
		logs.WithContext(ctx).Error(fmt.Sprint("invalid plan (attempt ", attempt+1, "/", maxAttempts+1, "):\n", issueText))
		if attempt >= maxAttempts {
			return nil, traces, fmt.Errorf("plan is still invalid after %d repair attempt(s):\n%s", maxAttempts, issueText)
		}
		if streamCb := agents.GetStreamCallback(ctx); streamCb != nil {
			streamCb(agents.StreamEvent{Event: agents.StreamEventStatus, Data: "repairing_plan"})
		}
		repairedPlan, repairTraces, err := oa.repairPlanOnce(ctx, agentMessage, plan, issueText, projectId, tenantId)
		traces = append(traces, repairTraces...)
		if err != nil {
			return nil, traces, err
		}
		plan = repairedPlan
	}
}

func (oa *OrchestratorAgent) repairPlanOnce(ctx context.Context, agentMessage agents.AgentMessage, plan map[string]interface{}, issueText string, projectId string, tenantId string) (map[string]interface{}, []models.StepTrace, error) {
	planJSON, err := json.Marshal(plan)
	if err != nil {
		return nil, nil, err
	}

	repairContent := fmt.Sprintf(
		"The FuncGroup you produced is invalid. Fix ONLY the problems listed below and return the corrected FuncGroup via structured_output.\n\nProblems found:\n%s\nPrevious plan:\n%s\n\nOriginal request: %s\n\nRules:\n- Change only what is needed to clear the problems listed above; keep the rest of the plan (steps, order, agents, tools, logic) exactly as it is.\n- Every '{{' must have a matching '}}' and every '(' a matching ')'; do not leave stray braces or parentheses at the end of an action.\n- Any template that builds an object must still be wrapped in / piped through stringify.\n- Reference a previous step only by its exact func_steps key, and only a step that actually runs before this one.\n- Use only the agents and tool actions listed in the AVAILABLE AGENTS / AVAILABLE TOOLS sections.",
		issueText,
		string(planJSON),
		agentMessage.Content,
	)

	chatRequest := models.ChatRequest{
		Messages: []models.Message{{
			Role:    "user",
			Content: repairContent,
		}},
	}

	toolsMap := oa.buildDecompositionTools(ctx)
	delete(toolsMap, utility.AskUserToolName)

	sp := oa.GetSystemPrompt()
	if oa.GetProvider() != nil {
		providerPrompt := oa.GetProvider().GetSystemPrompt()
		if providerPrompt != "" {
			sp = providerPrompt
		}
	}

	toolExecutor := func(ctx context.Context, toolName string, input map[string]interface{}) (map[string]interface{}, error) {
		return nil, fmt.Errorf("tool %s not expected during plan repair", toolName)
	}

	response, traces, err := oa.Model.RunToolLoop(ctx, chatRequest, toolsMap, sp, oa.MaxIterations, oa.ThinkingBudget, toolExecutor)
	if err != nil {
		return nil, traces, err
	}
	if response.TerminalTool != models.TerminalToolStructuredOutput {
		return nil, traces, fmt.Errorf("plan repair did not return a FuncGroup")
	}

	var repairedPlan map[string]interface{}
	if err := json.Unmarshal([]byte(response.Content), &repairedPlan); err != nil {
		return nil, traces, fmt.Errorf("failed to parse repaired plan: %w", err)
	}
	return repairedPlan, traces, nil
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
		"Original request: %s\n\nSub-agent results:\n%s%s\n\n%s\n\n%s\n\n%s",
		agentMessage.Content,
		string(resultJSON),
		bbSection,
		synthesisPrompt,
		"IMPORTANT: Respond with human-readable prose only. Do NOT embed raw JSON, code fences, or structured/widget payloads in your response — those are returned to the client separately as distinct actions. Summarize the result in words.",
		"GROUNDING (ABSOLUTE): every figure, name, date and total you state MUST appear in the sub-agent results above. Never invent, estimate, extrapolate or illustrate values, and never restate numbers that a sub-agent may itself have fabricated in place of missing data. If the results are empty, null, an error, or contain no rows for what the user asked, say plainly that no data was returned and what failed — do not produce a summary, table or percentages. Do not compute shares or totals unless the underlying values are present.",
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
	toolDescriptions := oa.buildToolDescriptions()

	systemPrompt := `You are an expert orchestration engineer for the Eru platform.
Your job is to decompose complex user tasks into a FuncGroup that coordinates sub-agents and tools.
Output ONLY the FuncGroup JSON via the structured_output tool. No markdown, no explanations.

============================================================
AVAILABLE AGENTS
============================================================

` + agentDescriptions + `

============================================================
AVAILABLE TOOLS
============================================================

` + toolDescriptions + `

============================================================
SELECTION — USE ONLY WHAT THE TASK NEEDS
============================================================

The lists above are what you MAY use, not what you MUST use. Do NOT use every
available agent/tool. Choose only the minimal set of agents and tools that best
accomplish the user's request — often that is a single agent or a short chain.
Omit anything not required.

============================================================
RULE #1 — STEP KEY = AGENT NAME / TOOL+ACTION NAME (most common mistake)
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
STEP TYPE — AGENT or TOOL
============================================================

A step is EITHER an agent step OR a tool step. Use ONLY agents listed in
AVAILABLE AGENTS and tools listed in AVAILABLE TOOLS.

Agent step:
  "agent_name": "<name from available agents>"
  "tenant_id":  "<tenant_id from available agents>"

Tool step:
  "tool_name":   "<tool from available tools>"
  "tool_action": "<one of that tool's allowed actions>"
  "tenant_id":   "<tenant_id from available tools>"

Do NOT use query_name, function_name, or api steps.

============================================================
RULE #2 — AGENT INPUT FORMAT (transform_request is MANDATORY)
============================================================

Every agent receives its request body decoded into this exact JSON shape:
  {"content": "<string>"}      ← "content" is the agent's input message (REQUIRED)
You may also include "params" (object) and "files" (array) ONLY if needed.
ANY other/unknown top-level key is REJECTED by the agent (unknown field error),
and a bare string or number is REJECTED (it must be a JSON object).

Therefore EVERY step MUST set "transform_request" to a Go template that renders
a JSON object of the form {"content":"..."}. Build it with the dict function and
ALWAYS pipe the result through stringify so it renders as a JSON string (a bare
dict renders as Go's map[...] and is NOT valid JSON):

AGENT OUTPUT SHAPE — how to read a previous step's result:
Every agent RESPONDS with this envelope:
  {"actions":[{"action_name":"<that_agent_name>","action":{ ...output fields... }}]}
The useful values live in actions[0].action.<field>, where <field> is one of the
"Output fields" listed for that agent in the AVAILABLE AGENTS section above. So to
read a prior step's output, use:
  (index .ResVars.<prev_step>.Body.actions 0).action.<field>
NEVER use .ResVars.<prev_step>.Body.content for an agent step — the text is NOT there.

  First step (from the user's message — .Vars.Body IS already {"content":...}):
    "transform_request": "{{stringify (dict \"content\" .Vars.Body.content)}}"

  Chained step (feed prior agent's output field as the next input). Example: a
  generate_sql agent whose Output fields are "sql":
    "transform_request": "{{stringify (dict \"content\" (index .ResVars.generate_sql.Body.actions 0).action.sql)}}"

RULE: any template that builds an object/dict (in transform_request OR
transform_response) MUST end with " | stringify" (or wrap in stringify) so the
final output is a JSON string. A bare dict renders as Go's map[...] and is invalid.

WRONG (these all break the agent):
  "{{dict \"content\" .Vars.Body.content}}"                  → renders map[...], invalid JSON (not stringified)
  "{{.Vars.Body.content}}"                                     → bare unquoted string, not an object
  "{{stringify (dict \"content\" .ResVars.generate_sql.Body.content)}}" → wrong path; agent output is in actions[0].action.<field>, not .content
  passing the whole .ResVars.prev.Body                         → carries unknown AgentMessage fields → rejected

To combine multiple agents' outputs into one input, concatenate the fields into a
single content string, e.g.:
  "{{stringify (dict \"content\" (printf \"sql: %s\\nrows: %s\" (index .ResVars.generate_sql.Body.actions 0).action.sql (index .ResVars.execute_sql.Body.actions 0).action.result))}}"

============================================================
RULE #2b — PASS FETCHED DATA IN params.context, NOT PROSE
============================================================

When a step's job is to RENDER or ANALYSE data produced by an earlier step (UI /
page / widget / chart agents such as eru_studio, report or summary agents), the
rows MUST be passed as "params" -> "context", with "content" carrying only the
instruction. These agents read params.context as their data source:

  "transform_request": "{{stringify (dict \"content\" .Vars.Body.content \"params\" (dict \"context\" (stringify .ResVars.<data_step>.Body)))}}"

Do NOT bury the rows inside the content string for these agents, and NEVER
paraphrase, sample, round or retype the data into the template — always pass the
upstream value itself, so the downstream agent renders real values instead of
inventing plausible ones.

If the plan produces data and then displays it, the displaying step MUST reference
the data step through .ResVars. A display step whose transform_request contains no
.ResVars reference to the data step is WRONG — it will render fabricated data.

============================================================
RULE #3 — TOOL STEP INPUT / OUTPUT (different from agents)
============================================================

A tool step's request body MUST be {"params": { ...action input fields... }} —
the action's "Input schema" fields go INSIDE a root "params" object (NOT the
{"content":...} agent envelope, and NOT at the root). Any other root key is
rejected. Build params with dict, wrap in another dict under "params", stringify.
Example — an execute_sql action whose Input schema needs {"query","project_id",
"vars"}, fed from a prior agent's sql output:
  "transform_request": "{{stringify (dict \"params\" (dict \"query\" (index .ResVars.generate_sql.Body.actions 0).action.sql \"project_id\" \"processo\" \"vars\" (dict)))}}"

Reading a tool's OUTPUT to chain forward:
  - If the tool shows an "Output schema": read .ResVars.<tool_step>.Body.<field> per that schema.
  - If the tool's Output is "dynamic": do NOT try to pick fields — pass the whole
    result as content to a downstream AGENT or to synthesis:
      "transform_request": "{{stringify (dict \"content\" (stringify .ResVars.<tool_step>.Body))}}"

NOTE: agent output lives in actions[0].action.<field>; tool output lives directly
in .ResVars.<tool_step>.Body (no actions envelope). Don't mix them up.

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
    "transform_request": "{{stringify (dict \"content\" .Vars.Body.content)}}",
    "func_steps": {
      "summarizer": {
        "agent_name": "summarizer",
        "tenant_id": "t1",
        "transform_request": "{{stringify (dict \"content\" (index .ResVars.extractor.Body.actions 0).action.<extractor_output_field>)}}"
      }
    }
  }
}

Example — parallel: two independent agents, then merge:
{
  "sentiment_analyzer": {
    "agent_name": "sentiment_analyzer",
    "tenant_id": "t1",
    "transform_request": "{{stringify (dict \"content\" .Vars.Body.content)}}"
  },
  "topic_classifier": {
    "agent_name": "topic_classifier",
    "tenant_id": "t1",
    "transform_request": "{{stringify (dict \"content\" .Vars.Body.content)}}"
  }
}

Example — parallel then sequential merge:
{
  "sentiment_analyzer": {
    "agent_name": "sentiment_analyzer",
    "tenant_id": "t1",
    "transform_request": "{{stringify (dict \"content\" .Vars.Body.content)}}"
  },
  "topic_classifier": {
    "agent_name": "topic_classifier",
    "tenant_id": "t1",
    "transform_request": "{{stringify (dict \"content\" .Vars.Body.content)}}",
    "func_steps": {
      "report_generator": {
        "agent_name": "report_generator",
        "tenant_id": "t1",
        "wait_for": "sentiment_analyzer",
        "transform_request": "{{stringify (dict \"content\" (printf \"sentiment: %s\\ntopics: %s\" (index .ResVars.sentiment_analyzer.Body.actions 0).action.<field> (index .ResVars.topic_classifier.Body.actions 0).action.<field>))}}"
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
.ResVars.<step_key>.Body — response FROM a completed step (an agent envelope: {"actions":[{"action_name":...,"action":{...}}]})
.ReqVars.<step_key>.Body — request sent TO a step
(index .ResVars.<step_key>.Body.actions 0).action.<field> — a prior agent's output value

Syntax:
  {{.Vars.Body.content}}                                   — the user's input string
  {{(index .ResVars.<step>.Body.actions 0).action.<field>}} — a prior agent's output value
  {{stringify (dict "content" .Vars.Body.content)}}        — wrap into the agent input object (JSON string)
  {{printf "%s / %s" .X .Y}}                                — combine strings before wrapping
  {{index .Vars.Body "field-with-dash"}}

stringify (= JSON-encode) is MANDATORY whenever the template builds a dict/object,
in BOTH transform_request and transform_response. Output must be a JSON string.

Conditions:
  {{if eq .Vars.Body.status "active"}}true{{else}}false{{end}}

============================================================
OPTIONAL STEP FIELDS
============================================================

Transforms (any object output MUST end with " | stringify"):
  "transform_request":  "<Go template → JSON string, e.g. {{dict ... | stringify}}>"
  "transform_response": "<Go template → JSON string, e.g. {{dict ... | stringify}}>"

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

[ ] Every func_step key exactly matches agent_name, or tool_name_action for tool steps (Rule #1)
[ ] Agent steps: transform_request renders {"content":"..."} (Rule #2)
[ ] Tool steps: transform_request renders {"params": {<Input schema fields>}} (Rule #3)
[ ] EVERY dict/object in transform_request AND transform_response ends with " | stringify"
[ ] EVERY template parses: each "{{" has a matching "}}", every "(" a matching ")", and no stray brace or parenthesis is left at the end of an action
[ ] No step passes a bare string or the raw .Vars.Body / whole AgentMessage
[ ] func_category_name and func_group_name are set (snake_case)
[ ] Each step uses ONLY (agent_name) OR (tool_name+tool_action) + tenant_id (no query/function/api)
[ ] Sequential steps are NESTED, parallel steps are SIBLINGS
[ ] transform_request passes data correctly between steps
[ ] Only agents/tools from the AVAILABLE lists are used
[ ] wait_for only references sibling step keys, not nested ones
[ ] EVERY .ResVars/.ReqVars reference names an EXACT func_steps key of an earlier step (not an agent name, tool name or invented short form)
[ ] A step that renders or analyses earlier data receives it via params.context and references that step through .ResVars (Rule #2b)

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
		if fields := outputFieldNames(ad.OutputSchema); len(fields) > 0 {
			sb.WriteString(fmt.Sprintf("  Output fields (in actions[0].action): %s\n", strings.Join(fields, ", ")))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func outputFieldNames(schema eru_models.JSONSchema) []string {
	var fields []string
	for k := range schema.Properties {
		fields = append(fields, k)
	}
	return fields
}

// collectClientOutputs returns the structured outputs to forward to the client
// as separate actions. If ClientOutputAgents is set, it forwards each named
// step's output; otherwise it falls back to the terminal step's output.
func (oa *OrchestratorAgent) collectClientOutputs(ctx context.Context, plan map[string]interface{}, funcVarsMap map[string]functions.FuncTemplateVars, executionResult map[string]interface{}) []agents.AgentOutputAction {
	var out []agents.AgentOutputAction
	if len(oa.ClientOutputAgents) > 0 {
		seen := make(map[string]bool)
		for _, name := range oa.ClientOutputAgents {
			if seen[name] {
				continue
			}
			body, found := stepOutputBody(funcVarsMap, name)
			if !found {
				logs.WithContext(ctx).Info(fmt.Sprint("collectClientOutputs - no output found for step ", name))
				continue
			}
			out = append(out, agents.AgentOutputAction{
				ActionType: agents.ActionTypeData,
				ActionName: name,
				Action:     extractStructuredOutput(body),
			})
			seen[name] = true
		}
		return out
	}
	if len(executionResult) > 0 {
		actionName := terminalStepName(ctx, plan)
		if actionName == "" {
			actionName = oa.AgentName
		}
		out = append(out, agents.AgentOutputAction{
			ActionType: agents.ActionTypeData,
			ActionName: actionName,
			Action:     extractStructuredOutput(executionResult),
		})
	}
	return out
}

// stepOutputBody finds a completed step's response body in the per-step vars.
func stepOutputBody(funcVarsMap map[string]functions.FuncTemplateVars, stepName string) (interface{}, bool) {
	for _, fv := range funcVarsMap {
		if fv.ResVars == nil {
			continue
		}
		if tv, ok := fv.ResVars[stepName]; ok && tv != nil && tv.Body != nil {
			return tv.Body, true
		}
	}
	return nil, false
}

// extractStructuredOutput unwraps an agent envelope ({"actions":[{"action":{...}}]})
// to the inner action object; for tool results (no actions envelope) it returns the
// body as-is.
func extractStructuredOutput(body interface{}) map[string]interface{} {
	m, ok := body.(map[string]interface{})
	if !ok {
		return map[string]interface{}{"output": body}
	}
	if actionsI, ok := m["actions"]; ok {
		if actions, ok := actionsI.([]interface{}); ok && len(actions) > 0 {
			if a0, ok := actions[0].(map[string]interface{}); ok {
				if act, ok := a0["action"].(map[string]interface{}); ok {
					return act
				}
			}
		}
	}
	return m
}

func (oa *OrchestratorAgent) buildToolDescriptions() string {
	if len(oa.discoveredTools) == 0 {
		return "No tools configured."
	}

	var sb strings.Builder
	for _, dt := range oa.discoveredTools {
		sb.WriteString(fmt.Sprintf("Tool: %s  Action: %s\n", dt.ToolName, dt.ActionName))
		sb.WriteString(fmt.Sprintf("  Tenant: %s\n", dt.TenantId))
		sb.WriteString(fmt.Sprintf("  Description: %s\n", dt.Description))
		if inb, err := json.Marshal(dt.InputSchema); err == nil {
			sb.WriteString(fmt.Sprintf("  Input schema (transform_request must produce this): %s\n", string(inb)))
		}
		if dt.OutputSchema.Type != "" {
			if outb, err := json.Marshal(dt.OutputSchema); err == nil {
				sb.WriteString(fmt.Sprintf("  Output schema (result at .ResVars.<step>.Body): %s\n", string(outb)))
			}
		} else {
			sb.WriteString("  Output: dynamic — to chain, pass {{stringify .ResVars.<step>.Body}} to a downstream agent or synthesis\n")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
