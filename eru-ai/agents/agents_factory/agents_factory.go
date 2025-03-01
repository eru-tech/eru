package agents_factory

import (
	agents "github.com/eru-tech/eru/eru-ai/agents"
	"github.com/eru-tech/eru/eru-ai/agents/reflex_agents"
)

func GetAgent(agentType string) agents.AgentI {
	switch agentType {
	case "REFLEX":
		return new(reflex_agents.ReflexAgent)
	default:
		return new(agents.Agent)
	}
}
