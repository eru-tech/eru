package reflex_agents

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	models "github.com/eru-tech/eru/eru-ai/models"
	utility "github.com/eru-tech/eru/eru-ai/tools/utility"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

type EruFuncStepAgent struct {
	agents.Agent
}

func (erufuncstep_agent *EruFuncStepAgent) GetSpec() agents.AgentI {
	return erufuncstep_agent
}

func (erufuncstep_agent *EruFuncStepAgent) Execute(ctx context.Context, agentMessage agents.AgentMessage, conversationId string, projectId string, tenantId string) (map[string]interface{}, error) {
	logs.WithContext(ctx).Debug("Agent Execute - Start")

	if erufuncstep_agent.Function.FuncGroupName != "" {
		response, err := erufuncstep_agent.ExecuteAgentFunction(ctx, agentMessage, projectId, tenantId)
		if err != nil {
			logs.WithContext(ctx).Error(fmt.Sprintf("Failed to execute agent function: %v", err))
			return nil, err
		}
		return response, nil
	}

	conversation, err := erufuncstep_agent.LoadConversationHistory(ctx, conversationId, projectId, tenantId)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to load conversation history: %v", err))
		return nil, err
	}
	agentMessage.Role = "user"
	agentMessage.MessageTimestamp = time.Now()
	conversation.Messages = append(conversation.Messages, agentMessage)
	conversation.NewMessages = append(conversation.NewMessages, agentMessage)

	msg := models.Message{
		Role:    agentMessage.Role,
		Content: agentMessage.Content,
		Name:    erufuncstep_agent.AgentName,
		Files:   agentMessage.Files,
	}

	// Build chat request with conversation history management
	var chatRequest models.ChatRequest
	if erufuncstep_agent.ConversationManager != nil {
		managedRequest, err := erufuncstep_agent.ConversationManager.BuildChatRequest(ctx, conversation, msg, erufuncstep_agent.AgentName)
		if err != nil {
			logs.WithContext(ctx).Error(fmt.Sprintf("Failed to build managed chat request: %v", err))
			// Fallback to simple request if conversation management fails
			chatRequest = models.ChatRequest{
				Messages: []models.Message{msg},
			}
		} else {
			chatRequest = *managedRequest
		}
	} else {
		// Fallback to simple request if no conversation manager is configured
		chatRequest = models.ChatRequest{
			Messages: []models.Message{msg},
		}
	}
	response, err := erufuncstep_agent.execute(ctx, chatRequest, erufuncstep_agent.AgentTools, 1, conversationId, projectId, tenantId)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to execute agent: %v", err))
		return nil, err
	}

	responseBytes, err := json.Marshal(response)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to marshal response: %v", err))
		return nil, err
	}
	agentResponseMessage := agents.AgentMessage{
		Role:             "assistant",
		Content:          string(responseBytes),
		MessageId:        agentMessage.MessageId, //same as the user message
		MessageTimestamp: time.Now(),
	}
	conversation.Messages = append(conversation.Messages, agentResponseMessage)
	conversation.NewMessages = append(conversation.NewMessages, agentResponseMessage)
	err = erufuncstep_agent.SaveConversation(ctx, conversation, projectId, tenantId)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to save conversation: %v", err))
		return nil, err
	}

	return response, nil
}

func (erufuncstep_agent *EruFuncStepAgent) execute(ctx context.Context, chatRequest models.ChatRequest, agentTools []agents.AgentTools, currentTry int, conversationId string, projectId string, tenantId string) (map[string]interface{}, error) {
	logs.WithContext(ctx).Debug("validate - Start")
	agentOutput := make(map[string]interface{})

	toolResults, err := erufuncstep_agent.ExecuteTools(ctx, chatRequest, agentTools, projectId, tenantId)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	logs.WithContext(ctx).Info(fmt.Sprintf("Tool results: %+v", toolResults))

	chatRequest.Messages = append(chatRequest.Messages, models.Message{
		Role:    "assistant",
		Content: erufuncstep_agent.SystemPrompt,
		Name:    erufuncstep_agent.AgentName,
	})
	chatRequest.Messages = append(chatRequest.Messages, models.Message{
		Role:    "assistant",
		Content: fmt.Sprintf("Tool results: %+v", toolResults),
		Name:    erufuncstep_agent.AgentName,
	})
	response := models.Message{}
	if erufuncstep_agent.OutputSchema.Type != "" {
		outputTool := utility.StructuredOutputTool{}
		outputTool.SetAttribute(ctx, "output_schema", erufuncstep_agent.OutputSchema)
		outputTool.SetAttribute(ctx, "parameters", erufuncstep_agent.OutputSchema)
		outputTool.SetAttribute(ctx, "description", "Output the result")
		outputTool.SetAttribute(ctx, "tool_name", "structured_output")
		outputTool.SetAttribute(ctx, "tool_type", "STRUCTURED_OUTPUT")
		outputTool.SetToolAction("structured_output")
		agentResponse, err := erufuncstep_agent.ExecuteTools(ctx, chatRequest, []agents.AgentTools{{Tool: &outputTool, ToolOutputType: "json"}}, projectId, tenantId)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return nil, err
		}
		agentOutput["output"] = agentResponse

	} else {
		response, err = erufuncstep_agent.Model.QueryModel(ctx, chatRequest)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return nil, err
		}
		agentOutput["output"] = response.Content
	}
	logs.WithContext(ctx).Info(fmt.Sprintf("Response: %+v", response.Content))
	/* if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		logs.WithContext(ctx).Info(fmt.Sprintf("%+v", response.Content))
		if currentTry < erufuncstep_agent.RetryCount {
			errMsgString := fmt.Sprintf("Error in the json string. Please try again. \n Error: %s \n Erroneous JSON Code generated in previous try: %s", err.Error(), response.Content["raw"])
			msg := models.Message{
				Role:      "user",
				Content:   errMsgString,
				Name:      erufuncstep_agent.AgentName,
				Files:     []models.FileMessage{},
			}
			chatRequest.Messages = append(chatRequest.Messages, msg)
			return erufuncstep_agent.execute(ctx, chatRequest, erufuncstep_agent.Tools, erufuncstep_agent.AgentName, erufuncstep_agent.SystemPrompt, currentTry+1)
		}
		return nil, err
	} */

	agentOutput["retry_count"] = currentTry
	return agentOutput, err
}

func (erufuncstep_agent *EruFuncStepAgent) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &erufuncstep_agent)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}
