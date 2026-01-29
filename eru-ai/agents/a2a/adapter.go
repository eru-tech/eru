package a2a

import (
	"context"

	"github.com/eru-tech/eru/eru-ai/agents"
)

type AgentCard struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tools       []string `json:"tools"`
}

type Adapter struct {
	Registry *agents.AgentRegistry
}

func NewAdapter(reg *agents.AgentRegistry) *Adapter {
	return &Adapter{Registry: reg}
}

func (a *Adapter) Discover(ctx context.Context) ([]AgentCard, error) {
	names := a.Registry.List(ctx)
	cards := make([]AgentCard, 0, len(names))
	for _, n := range names {
		cards = append(cards, AgentCard{Name: n})
	}
	return cards, nil
}

func (a *Adapter) SubmitTask(ctx context.Context, agentName string, msg agents.AgentMessage, conversationId string, projectId string, tenantId string) (agents.AgentMessage, error) {
	ag := a.Registry.Get(ctx, agentName)
	if ag == nil {
		return agents.AgentMessage{}, nil
	}
	return ag.Execute(ctx, msg, conversationId, projectId, tenantId)
}
