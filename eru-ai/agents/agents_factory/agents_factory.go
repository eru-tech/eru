package agents_factory

import (
	agents "github.com/eru-tech/eru/eru-ai/agents"
	"github.com/eru-tech/eru/eru-ai/agents/eru_studio"
	"github.com/eru-tech/eru/eru-ai/agents/orchestrator"
	"github.com/eru-tech/eru/eru-ai/agents/reasoning_agents"
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
	case "ERU_FUNC":
		return new(reasoning_agents.EruFuncAgent)
	case "REASONING":
		return new(reasoning_agents.ReasoningAgent)
	case "ORCHESTRATOR":
		return new(orchestrator.OrchestratorAgent)
	default:
		return new(agents.Agent)
	}
}
