package agents_factory

import (
	agents "github.com/eru-tech/eru/eru-ai/agents"
	"github.com/eru-tech/eru/eru-ai/agents/eru_studio"
	"github.com/eru-tech/eru/eru-ai/agents/reflex_agents"
)

func GetAgent(agentType string) agents.AgentI {
	switch agentType {
	case "REFLEX":
		return new(reflex_agents.ReflexAgent)
	case "GO_TEMPLATE":
		return new(reflex_agents.GoTemplateAgent)
	case "ERU_WIDGET":
		return new(eru_studio.EruWidgetAgent)
	case "ERU_FUNCSTEP":
		return new(reflex_agents.EruFuncStepAgent)
	default:
		return new(agents.Agent)
	}
}
