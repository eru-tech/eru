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
		Role:    "assistant",
		Content: agentMessage.Content,
		Name:    reflex_agent.AgentName,
	}
	chatRequest := models.ChatRequest{
		Messages: []models.Message{
			msg,
		},
	}
	response, err := reflex_agent.Model.QueryModelWithTool(ctx, chatRequest, reflex_agent.Tools, reflex_agent.AgentName, reflex_agent.SystemPrompt)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	return response.Content, nil
}

func (reflex_agent *ReflexAgent) callTool(ctx context.Context, tool tools.Tooling, params map[string]interface{}) (map[string]interface{}, error) {
	logs.WithContext(ctx).Debug("callTool - Start")
	return tool.Execute(ctx, params)
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
