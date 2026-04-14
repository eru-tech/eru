package reasoning_agents

import (
	"context"
	"encoding/json"
	"fmt"
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
	MaxIterations  int `json:"max_iterations"`
	ThinkingBudget int `json:"thinking_budget"`
}

func (ra *ReasoningAgent) GetSpec() agents.AgentI {
	return ra
}

func (ra *ReasoningAgent) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &ra.Agent); err != nil {
		return err
	}
	type reasoningFields struct {
		MaxIterations  int `json:"max_iterations"`
		ThinkingBudget int `json:"thinking_budget"`
	}
	var rf reasoningFields
	if err := json.Unmarshal(b, &rf); err != nil {
		return err
	}
	ra.MaxIterations = rf.MaxIterations
	ra.ThinkingBudget = rf.ThinkingBudget
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

	agentResponse := make(map[string]interface{})
	responseMap := map[string]interface{}{}
	if jsonErr := json.Unmarshal([]byte(response.Content), &responseMap); jsonErr != nil {
		agentResponse["output"] = response.Content
	} else {
		agentResponse = responseMap
	}

	metrics := agents.BuildMetrics(traces, startTime, response.Usage)

	agentOutput := agents.AgentMessage{
		Role: "assistant",
		Actions: []agents.AgentOutputAction{{
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
