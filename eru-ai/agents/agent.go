package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	models "github.com/eru-tech/eru/eru-ai/models"
	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
)

type AgentMessage struct {
	Content string                 `json:"content,omitempty"`
	Params  map[string]interface{} `json:"params,omitempty"`
	Files   []models.FileMessage   `json:"files,omitempty"`
}

type AgentTools struct {
	ToolName       string        `json:"tool_name"`
	ActionName     string        `json:"action_name"`
	ActionPrompt   string        `json:"action_prompt"`
	DependentTools []AgentTools  `json:"dependent_tools"`
	ToolKey        string        `json:"tool_key"`
	ToolOutputType string        `json:"tool_output_type"`
	Tool           tools.Tooling `json:"-"`
}
type Agent struct {
	AgentType    string                `json:"agent_type" eru:"required"`
	AgentName    string                `json:"agent_name" eru:"required"`
	Description  string                `json:"description"`
	SystemPrompt string                `json:"system_prompt"`
	AgentTools   []AgentTools          `json:"agent_tools"`
	ModelName    string                `json:"model"`
	Model        models.ModelI         `json:"-"`
	OutputSchema eru_models.JSONSchema `json:"output_schema"`
	//Tools        map[string]tools.Tooling `json:"-"`
	RetryCount int `json:"retry_count"`
}

type AgentI interface {
	GetSpec() AgentI
	Execute(ctx context.Context, agentMessage AgentMessage, projectId string, tenantId string) (map[string]interface{}, error)
	MakeFromJson(ctx context.Context, rj *json.RawMessage) error
	GetAttribute(ctx context.Context, attributeName string) (attributeValue interface{}, err error)
	//SetTools(tools map[string]tools.Tooling)
	ExecuteTools(ctx context.Context, chatRequest models.ChatRequest, agentTools []AgentTools, projectId string, tenantId string) (toolResults map[string]interface{}, err error)
	SetModel(model models.ModelI)
}

func (agent *Agent) GetSpec() AgentI {
	return agent
}

func (agent *Agent) Execute(ctx context.Context, agentMessage AgentMessage, projectId string, tenantId string) (map[string]interface{}, error) {
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
	case "agent_tools":
		return agent.AgentTools, nil
	case "model":
		return agent.ModelName, nil
	case "output_schema":
		return agent.OutputSchema, nil
	default:
		err := errors.New("attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
}

/* func (agent *Agent) SetTools(tools map[string]tools.Tooling) {
	agent.Tools = tools
} */

func (agent *Agent) SetModel(model models.ModelI) {
	agent.Model = model
}
func (agent *Agent) ExecuteTools(ctx context.Context, chatRequest models.ChatRequest, agentTools []AgentTools, projectId string, tenantId string) (toolResults map[string]interface{}, err error) {
	toolResults = make(map[string]interface{})
	for i, agentTool := range agentTools {
		tools := make(map[string]tools.Tooling)
		toolKey := agentTool.ToolKey
		if toolKey == "" {
			toolKey = fmt.Sprintf("%s_%s_%d", agent.AgentName, agentTool.ToolName, i)
		}
		tools[toolKey] = agentTool.Tool
		toolOutputType := agentTool.ToolOutputType
		if toolOutputType == "" {
			toolOutputType = "string"
		}
		response, err := agent.Model.QueryModelWithTool(ctx, chatRequest, tools, agent.AgentName, agentTool.ActionPrompt)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return nil, err
		}
		if response.Content != nil {
			toolParams := make(map[string]interface{})
			toolParams["params"] = response.Content
			toolResult, _, err := agentTool.Tool.Execute(ctx, projectId, tenantId, agentTool.ActionName, toolParams)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return nil, err
			}
			if toolOutputType == "json" {
				toolResults[toolKey] = toolResult
			} else {
				toolResultBytes, err := json.Marshal(toolResult)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return nil, err
				}
				toolResults[toolKey] = string(toolResultBytes)
			}
		}
		if len(agentTool.DependentTools) > 0 {
			dependentToolResults, err := agent.ExecuteTools(ctx, chatRequest, agentTool.DependentTools, projectId, tenantId)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return nil, err
			}
			for key, value := range dependentToolResults {
				toolResults[key] = value
			}
		}
	}
	return toolResults, nil
}
