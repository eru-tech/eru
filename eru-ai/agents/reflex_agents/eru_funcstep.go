package reflex_agents

import (
	"context"
	"encoding/json"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	eru_models "github.com/eru-tech/eru/eru-models"
)

type EruFuncStepAgent struct {
	ReflexAgent
}

func (eruFuncStepAgent *EruFuncStepAgent) GetSpec() agents.AgentI {
	return eruFuncStepAgent
}

func (eruFuncStepAgent *EruFuncStepAgent) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	err := eruFuncStepAgent.ReflexAgent.MakeFromJson(ctx, rj)
	if err != nil {
		return err
	}
	eruFuncStepAgent.ReflexAgent.Provider = eruFuncStepAgent
	return nil
}

func (eruFuncStepAgent *EruFuncStepAgent) GetSystemPrompt() string {
	const systemPrompt = `
	this is from eru_funcstep agent
	`
	return systemPrompt
}

func (eruFuncStepAgent *EruFuncStepAgent) GetOutputSchema(ctx context.Context) eru_models.JSONSchema {
	return eru_models.JSONSchema{}
}
