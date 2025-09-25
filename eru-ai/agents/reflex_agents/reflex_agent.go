package reflex_agents

import (
	"context"
	"encoding/json"
	"fmt"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	models "github.com/eru-tech/eru/eru-ai/models"
	tools "github.com/eru-tech/eru/eru-ai/tools"
	utility "github.com/eru-tech/eru/eru-ai/tools/utility"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

type ReflexAgent struct {
	agents.Agent
}

func (reflex_agent *ReflexAgent) GetSpec() agents.AgentI {
	return reflex_agent
}

func (reflex_agent *ReflexAgent) Execute(ctx context.Context, agentMessage agents.AgentMessage, projectId string, tenantId string) (map[string]interface{}, error) {
	logs.WithContext(ctx).Debug("Agent Execute - Start")

	conversation, err := reflex_agent.LoadConversationHistory(ctx, agentMessage.ConversationId, projectId, tenantId)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to load conversation history: %v", err))
		return nil, err
	}
	err = reflex_agent.AddMessageToConversation(ctx, conversation, "user", agentMessage, nil)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to add user message to conversation: %v", err))
		return nil, err
	}

	msg := models.Message{
		Role:    "user",
		Content: agentMessage.Content,
		Name:    reflex_agent.AgentName,
		Files:   agentMessage.Files,
	}
	chatRequest := models.ChatRequest{
		Messages: []models.Message{
			msg,
		},
	}
	response, err := reflex_agent.execute(ctx, chatRequest, reflex_agent.AgentTools, 1, projectId, tenantId)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to execute agent: %v", err))
		return nil, err
	}
	err = reflex_agent.AddMessageToConversation(ctx, conversation, "assistant", agents.AgentMessage{}, response)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to add assistant response to conversation: %v", err))
		return nil, err
	}
	err = reflex_agent.SaveConversation(ctx, conversation, projectId)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to save conversation: %v", err))
		return nil, err
	}

	return response, nil
}

func (reflex_agent *ReflexAgent) execute(ctx context.Context, chatRequest models.ChatRequest, agentTools []agents.AgentTools, currentTry int, projectId string, tenantId string) (map[string]interface{}, error) {
	logs.WithContext(ctx).Debug("validate - Start")
	agentOutput := make(map[string]interface{})

	toolResults, err := reflex_agent.ExecuteTools(ctx, chatRequest, agentTools, projectId, tenantId)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	logs.WithContext(ctx).Info(fmt.Sprintf("Tool results: %+v", toolResults))

	chatRequest.Messages = append(chatRequest.Messages, models.Message{
		Role:    "assistant",
		Content: reflex_agent.SystemPrompt,
		Name:    reflex_agent.AgentName,
	})
	chatRequest.Messages = append(chatRequest.Messages, models.Message{
		Role:    "assistant",
		Content: fmt.Sprintf("Tool results: %+v", toolResults),
		Name:    reflex_agent.AgentName,
	})
	response := models.Message{}
	if reflex_agent.OutputSchema.Type != "" {
		outputTool := utility.StructuredOutputTool{}
		outputTool.SetAttribute(ctx, "output_schema", reflex_agent.OutputSchema)
		outputTool.SetAttribute(ctx, "parameters", reflex_agent.OutputSchema)
		outputTool.SetAttribute(ctx, "description", "Output the result")
		outputTool.SetAttribute(ctx, "tool_name", "structured_output")
		outputTool.SetAttribute(ctx, "tool_type", "STRUCTURED_OUTPUT")
		outputTool.SetToolAction("structured_output")
		agentResponse, err := reflex_agent.ExecuteTools(ctx, chatRequest, []agents.AgentTools{{Tool: &outputTool, ToolOutputType: "json"}}, projectId, tenantId)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return nil, err
		}
		agentOutput["output"] = agentResponse

	} else {
		response, err = reflex_agent.Model.QueryModel(ctx, chatRequest)
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
		if currentTry < reflex_agent.RetryCount {
			errMsgString := fmt.Sprintf("Error in the json string. Please try again. \n Error: %s \n Erroneous JSON Code generated in previous try: %s", err.Error(), response.Content["raw"])
			msg := models.Message{
				Role:      "user",
				Content:   errMsgString,
				Name:      reflex_agent.AgentName,
				Files:     []models.FileMessage{},
			}
			chatRequest.Messages = append(chatRequest.Messages, msg)
			return reflex_agent.execute(ctx, chatRequest, reflex_agent.Tools, reflex_agent.AgentName, reflex_agent.SystemPrompt, currentTry+1)
		}
		return nil, err
	} */

	agentOutput["retry_count"] = currentTry
	return agentOutput, err
}

/* func (reflex_agent *ReflexAgent) validate(ctx context.Context, jsonString string) error {
	logs.WithContext(ctx).Debug("validate - Start")
	jsonI := map[string]interface{}{}
	err := json.Unmarshal([]byte(jsonString), &jsonI)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	logs.WithContext(ctx).Info(fmt.Sprintf("jsonI: %+v", jsonI))
	return nil
} */

func (reflex_agent *ReflexAgent) callTool(ctx context.Context, projectId string, tenantId string, tool tools.Tooling, params map[string]interface{}) (map[string]interface{}, bool, error) {
	logs.WithContext(ctx).Debug("callTool - Start")
	return tool.Execute(ctx, projectId, tenantId, "", params)
}

func (reflex_agent *ReflexAgent) callModel(ctx context.Context, model models.ModelI, params map[string]interface{}) (map[string]interface{}, error) {
	logs.WithContext(ctx).Debug("callModel - Start")
	return nil, nil
}

func (reflex_agent *ReflexAgent) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &reflex_agent)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}
