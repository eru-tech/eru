package reasoning_agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	models "github.com/eru-tech/eru/eru-ai/models"
	tools "github.com/eru-tech/eru/eru-ai/tools"
	utility "github.com/eru-tech/eru/eru-ai/tools/utility"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type ReasoningAgent struct {
	agents.Agent
	MaxIterations      int  `json:"max_iterations"`
	ThinkingBudget     int  `json:"thinking_budget"`
	EnableClarification bool `json:"enable_clarification"`
}

const clarificationGuidance = `

HUMAN-IN-THE-LOOP CLARIFICATION:
You may ask the user for clarification using the ask_user tool. Use it ONLY when the request is genuinely ambiguous or missing information you cannot infer or reasonably default — never to offload reasoning you can do yourself.
When you ask:
- Keep it to the fewest questions needed (ideally one).
- For each question provide 2-4 concrete, mutually exclusive options (value + human label).
- Set allow_free_text=true whenever the options may not be exhaustive, so the user can type their own answer.
- Set multi_select=true only when more than one option can legitimately be chosen.
Calling ask_user ends your turn; the user's answers will arrive as a follow-up message in the same conversation, after which you continue.`

func (ra *ReasoningAgent) GetSpec() agents.AgentI {
	return ra
}

func (ra *ReasoningAgent) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &ra.Agent); err != nil {
		return err
	}
	type reasoningFields struct {
		MaxIterations      int  `json:"max_iterations"`
		ThinkingBudget     int  `json:"thinking_budget"`
		EnableClarification bool `json:"enable_clarification"`
	}
	var rf reasoningFields
	if err := json.Unmarshal(b, &rf); err != nil {
		return err
	}
	ra.MaxIterations = rf.MaxIterations
	ra.ThinkingBudget = rf.ThinkingBudget
	ra.EnableClarification = rf.EnableClarification
	return nil
}

func (ra *ReasoningAgent) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("ReasoningAgent MakeFromJson - Start")
	err := json.Unmarshal(*rj, ra)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	if ra.MaxIterations <= 0 {
		ra.MaxIterations = 10
	}
	if ra.ThinkingBudget <= 0 {
		ra.ThinkingBudget = 10000
	}
	return nil
}

func (ra *ReasoningAgent) Execute(ctx context.Context, agentMessage agents.AgentMessage, conversationId string, projectId string, tenantId string) (agents.AgentMessage, error) {
	logs.WithContext(ctx).Debug("ReasoningAgent Execute - Start")
	ctx, span := otel.Tracer("eru-ai").Start(ctx, "ReasoningAgent.Execute",
		oteltrace.WithAttributes(attribute.String("agent_name", ra.AgentName), attribute.String("conversation_id", conversationId)),
	)
	defer span.End()
	startTime := time.Now()

	if answers, ok := agentMessage.ClarificationAnswers(); ok {
		var req agents.ClarificationRequest
		if priorConv, lerr := ra.LoadConversationHistory(ctx, conversationId, projectId, tenantId); lerr == nil && priorConv != nil {
			if _, qa, found := agents.PendingQuestion(priorConv); found {
				req, _ = agents.ParseClarificationRequest(qa.Action)
			}
		}
		answerText := agents.FormatAnswersForModel(req, answers)
		if strings.TrimSpace(agentMessage.Content) != "" {
			agentMessage.Content = agentMessage.Content + "\n\n" + answerText
		} else {
			agentMessage.Content = answerText
		}
	}

	chatRequest, conversation, err := ra.LoadConversations(ctx, conversationId, agentMessage, projectId, tenantId)
	if err != nil {
		return agents.AgentMessage{}, err
	}

	if ra.Function.FuncGroupName != "" {
		return ra.executeWithFunction(ctx, agentMessage, conversation, projectId, tenantId)
	}

	toolsMap := make(map[string]tools.Tooling)
	for _, at := range ra.AgentTools {
		if at.Tool != nil {
			key := at.ToolKey
			if key == "" {
				key = at.ToolName
			}
			toolsMap[key] = at.Tool
		}
	}

	outputSchema := ra.getOutputSchema(ctx)
	if outputSchema.Type != "" {
		outputTool := &utility.StructuredOutputTool{}
		outputTool.SetAttribute(ctx, "output_schema", outputSchema)
		outputTool.SetAttribute(ctx, "parameters", outputSchema)
		outputTool.SetAttribute(ctx, "description", "Output the final result as structured JSON. Call this tool when you have your final answer ready.")
		outputTool.SetAttribute(ctx, "tool_name", "structured_output")
		outputTool.SetAttribute(ctx, "tool_type", "STRUCTURED_OUTPUT")
		outputTool.SetToolAction("structured_output")
		toolsMap["structured_output"] = outputTool
	}

	if ra.EnableClarification {
		askTool := &utility.AskUserTool{}
		askTool.SetAttribute(ctx, "parameters", utility.AskUserToolSchema())
		askTool.SetAttribute(ctx, "description", "Ask the user clarifying questions when the request is ambiguous or missing information needed to proceed. Provide 2-4 concrete, mutually exclusive options per question and allow free text when the options may not be exhaustive.")
		askTool.SetAttribute(ctx, "system_prompt", "")
		askTool.SetAttribute(ctx, "tool_name", utility.AskUserToolName)
		askTool.SetAttribute(ctx, "tool_type", "ASK_USER")
		askTool.SetToolAction(utility.AskUserToolName)
		toolsMap[utility.AskUserToolName] = askTool
	}

	toolExecutor := func(ctx context.Context, toolName string, input map[string]interface{}) (map[string]interface{}, error) {
		for _, at := range ra.AgentTools {
			if at.Tool == nil {
				continue
			}
			tnI, _ := at.Tool.GetAttribute(ctx, "tool_name")
			if tn, ok := tnI.(string); ok && tn == toolName {
				result, _, execErr := at.Tool.Execute(ctx, projectId, tenantId, at.ActionName, input)
				return result, execErr
			}
		}
		return nil, fmt.Errorf("tool %s not found", toolName)
	}

	sp := ra.SystemPrompt
	if ra.GetProvider() != nil {
		sp = ra.GetProvider().GetSystemPrompt() + "\n" + sp
	}
	if ra.EnableClarification {
		sp = sp + clarificationGuidance
	}

	var response models.Message
	var traces []models.StepTrace

	streamCb := agents.GetStreamCallback(ctx)
	if streamCb != nil {
		if streamingModel, ok := ra.Model.(models.StreamingModelI); ok {
			modelCb := func(me models.ModelStreamEvent) {
				streamCb(agents.StreamEvent{
					Event:     string(me.Type),
					Data:      me,
					Iteration: me.Iteration,
				})
			}
			response, traces, err = streamingModel.RunToolLoopStreaming(ctx, chatRequest, toolsMap, sp, ra.MaxIterations, ra.ThinkingBudget, toolExecutor, modelCb)
		} else {
			response, traces, err = ra.Model.RunToolLoop(ctx, chatRequest, toolsMap, sp, ra.MaxIterations, ra.ThinkingBudget, toolExecutor)
		}
	} else {
		response, traces, err = ra.Model.RunToolLoop(ctx, chatRequest, toolsMap, sp, ra.MaxIterations, ra.ThinkingBudget, toolExecutor)
	}
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return agents.AgentMessage{}, err
	}

	agentResponse := parseAgentResponse(response.Content)

	metrics := agents.BuildMetrics(traces, startTime, response.Usage)

	actionType := agents.ActionTypeAnswer
	if response.TerminalTool == models.TerminalToolAskUser {
		actionType = agents.ActionTypeQuestion
	}

	agentOutput := agents.AgentMessage{
		Role: "assistant",
		Actions: []agents.AgentOutputAction{{
			ActionType: actionType,
			ActionName: ra.AgentName,
			Action:     agentResponse,
		}},
		Traces:           traces,
		Metrics:          metrics,
		MessageId:        agentMessage.MessageId,
		MessageTimestamp: time.Now(),
	}

	conversation.Messages = append(conversation.Messages, agentOutput)
	conversation.NewMessages = append(conversation.NewMessages, agentOutput)
	err = ra.SaveConversation(ctx, conversation, projectId, tenantId)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to save conversation: %v", err))
		return agents.AgentMessage{}, err
	}

	return agentOutput, nil
}

func (ra *ReasoningAgent) getOutputSchema(ctx context.Context) eru_models.JSONSchema {
	outputSchema := ra.OutputSchema
	if ra.GetProvider() != nil {
		providerSchema := ra.GetProvider().GetOutputSchema(ctx)
		if providerSchema.Type != "" {
			outputSchema = providerSchema
		}
	}
	return outputSchema
}

func (ra *ReasoningAgent) executeWithFunction(ctx context.Context, agentMessage agents.AgentMessage, conversation *agents.Conversation, projectId string, tenantId string) (agents.AgentMessage, error) {
	response, err := ra.ExecuteAgentFunction(ctx, agentMessage, projectId, tenantId)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to execute agent function: %v", err))
		return agents.AgentMessage{}, err
	}

	responseBytes, err := json.Marshal(response)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to marshal agent function response: %v", err))
		return agents.AgentMessage{}, err
	}

	var agentMsg agents.AgentMessage
	err = json.Unmarshal(responseBytes, &agentMsg)
	if err != nil {
		agentMsg = agents.AgentMessage{
			Role: "assistant",
			Actions: []agents.AgentOutputAction{{
				Action: response,
			}},
			MessageId:        agentMessage.MessageId,
			MessageTimestamp: time.Now(),
		}
	}

	conversation.Messages = append(conversation.Messages, agentMsg)
	conversation.NewMessages = append(conversation.NewMessages, agentMsg)
	err = ra.SaveConversation(ctx, conversation, projectId, tenantId)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to save conversation: %v", err))
		return agents.AgentMessage{}, err
	}
	return agentMsg, nil
}

// parseAgentResponse turns the model's final Message.Content into a structured
// map suitable for AgentOutputAction.Action. It handles three cases:
//  1. Content is already valid JSON object → return parsed map.
//  2. Content is a JSON object wrapped in markdown fences (```json … ``` or
//     ``` … ```) → strip fences, parse, return parsed map. This is what
//     happens when the model returns its answer as plain text instead of
//     calling structured_output.
//  3. Anything else → return {"output": <raw content>}.
func parseAgentResponse(content string) map[string]interface{} {
	trimmed := strings.TrimSpace(content)

	if parsed, ok := tryUnmarshalObject(trimmed); ok {
		return parsed
	}

	if stripped, ok := stripMarkdownFences(trimmed); ok {
		if parsed, ok := tryUnmarshalObject(stripped); ok {
			return parsed
		}
	}

	if start, end, ok := findOuterJSONObject(trimmed); ok {
		if parsed, parsedOk := tryUnmarshalObject(trimmed[start : end+1]); parsedOk {
			return parsed
		}
	}

	return map[string]interface{}{"output": content}
}

func tryUnmarshalObject(s string) (map[string]interface{}, bool) {
	if s == "" {
		return nil, false
	}
	out := map[string]interface{}{}
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, false
	}
	return out, true
}

// stripMarkdownFences removes a single ```lang … ``` wrapper if present.
// Returns (stripped, true) when fences were found and removed.
func stripMarkdownFences(s string) (string, bool) {
	if !strings.HasPrefix(s, "```") {
		return s, false
	}
	rest := strings.TrimPrefix(s, "```")
	if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
		rest = rest[nl+1:]
	}
	rest = strings.TrimSpace(rest)
	if idx := strings.LastIndex(rest, "```"); idx >= 0 {
		rest = strings.TrimSpace(rest[:idx])
	}
	return rest, true
}

// findOuterJSONObject locates the first balanced { … } object in the string,
// ignoring braces that appear inside JSON strings. This recovers JSON when
// the model precedes/follows it with prose.
func findOuterJSONObject(s string) (int, int, bool) {
	start := -1
	depth := 0
	inString := false
	escaped := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			depth--
			if depth == 0 && start >= 0 {
				return start, i, true
			}
		}
	}
	return 0, 0, false
}
