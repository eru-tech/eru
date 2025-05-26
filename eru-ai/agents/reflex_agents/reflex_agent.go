package reflex_agents

import (
	"context"
	"encoding/json"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	models "github.com/eru-tech/eru/eru-ai/models"
	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

type ReflexAgent struct {
	agents.Agent
}

func (reflex_agent *ReflexAgent) GetSpec() agents.AgentI {
	return reflex_agent
}

func (reflex_agent *ReflexAgent) Execute(ctx context.Context, agentMessage agents.AgentMessage) (map[string]interface{}, error) {
	logs.WithContext(ctx).Debug("Agent Execute - Start")

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
	return reflex_agent.execute(ctx, chatRequest, reflex_agent.Tools, reflex_agent.AgentName, reflex_agent.SystemPrompt, 1)
}
func (reflex_agent *ReflexAgent) execute(ctx context.Context, chatRequest models.ChatRequest, tools map[string]tools.Tooling, agentName string, systemPrompt string, currentTry int) (map[string]interface{}, error) {
	logs.WithContext(ctx).Debug("validate - Start")
	agentOutput := make(map[string]interface{})
	response, err := reflex_agent.Model.QueryModelWithTool(ctx, chatRequest, reflex_agent.Tools, reflex_agent.AgentName, reflex_agent.SystemPrompt)
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

	agentOutput["output"] = response.Content
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

func (reflex_agent *ReflexAgent) callTool(ctx context.Context, projectId string, tenantId string, tool tools.Tooling, params map[string]interface{}) (map[string]interface{}, error) {
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
