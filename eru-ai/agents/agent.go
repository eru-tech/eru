package agents

import (
	"context"
	"encoding/json"
	"errors"

	models "github.com/eru-tech/eru/eru-ai/models"
	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

type AgentMessage struct {
	Content   string                 `json:"content,omitempty"`
	Params    map[string]interface{} `json:"params,omitempty"`
	FileData  string                 `json:"file_data,omitempty"`
	FileName  string                 `json:"file_name,omitempty"`
	FileId    string                 `json:"file_id,omitempty"`
	ImageData string                 `json:"image_data,omitempty"`
}
type Agent struct {
	AgentType    string                   `json:"agent_type" eru:"required"`
	AgentName    string                   `json:"agent_name" eru:"required"`
	Description  string                   `json:"description"`
	SystemPrompt string                   `json:"system_prompt"`
	ToolNames    []string                 `json:"tools"`
	ModelName    string                   `json:"model"`
	Model        models.ModelI            `json:"-"`
	Tools        map[string]tools.Tooling `json:"-"`
}

type AgentI interface {
	GetSpec() AgentI
	Execute(ctx context.Context, agentMessage AgentMessage) (map[string]interface{}, error)
	MakeFromJson(ctx context.Context, rj *json.RawMessage) error
	GetAttribute(ctx context.Context, attributeName string) (attributeValue interface{}, err error)
	SetTools(tools map[string]tools.Tooling)
	SetModel(model models.ModelI)
}

func (agent *Agent) GetSpec() AgentI {
	return agent
}

func (agent *Agent) Execute(ctx context.Context, agentMessage AgentMessage) (map[string]interface{}, error) {
	err := errors.New("Execute Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return nil, err
}

func (agent *Agent) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &agent)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (agent *Agent) GetAttribute(ctx context.Context, attributeName string) (attributeValue interface{}, err error) {
	switch attributeName {
	case "agent_type":
		return agent.AgentType, nil
	case "agent_name":
		return agent.AgentName, nil
	case "system_prompt":
		return agent.SystemPrompt, nil
	case "description":
		return agent.Description, nil
	case "tools":
		return agent.ToolNames, nil
	case "model":
		return agent.ModelName, nil
	default:
		err := errors.New("attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
}

func (agent *Agent) SetTools(tools map[string]tools.Tooling) {
	agent.Tools = tools
}

func (agent *Agent) SetModel(model models.ModelI) {
	agent.Model = model
}
