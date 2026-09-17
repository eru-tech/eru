package orchestrator

import (
	"context"
	"fmt"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	utility "github.com/eru-tech/eru/eru-ai/tools/utility"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

// The planner used to decide everything from the words in the prompt. It could
// not look anything up, so when a plan needed a fact about the tenant - which
// entities exist, what a field is called - it either guessed or planned a step
// to go and find out with whatever blunt instrument it had. That is how a
// hand-written SELECT against an invented table ended up standing in for a
// purpose-built metadata lookup.
//
// These are the lookups a planner may make before it commits. They are strictly
// read-only and there are deliberately few of them: research that costs a model
// call per plan has to earn its place, and a planner given every tool starts
// doing the work instead of planning it.

// researchTools are the read-only lookups offered during planning. It is empty
// when nothing backs them, and the planner is then told nothing about them -
// the same graceful degradation the page agent uses, because a model told about
// a tool it does not have will try it, fail, and spend an iteration finding out.
func (oa *OrchestratorAgent) researchTools(ctx context.Context) map[string]tools.Tooling {
	research := map[string]tools.Tooling{}

	if delegate := oa.eruqlDelegate(ctx); delegate != nil {
		metadataTool := &utility.EntityMetadataTool{Delegate: delegate}
		_ = metadataTool.SetAttribute(ctx, "parameters", utility.EntityMetadataToolSchema())
		_ = metadataTool.SetAttribute(ctx, "description", utility.EntityMetadataToolDescription())
		_ = metadataTool.SetAttribute(ctx, "system_prompt", "")
		_ = metadataTool.SetAttribute(ctx, "tool_name", utility.EntityMetadataToolName)
		_ = metadataTool.SetAttribute(ctx, "tool_type", "ENTITY_METADATA")
		metadataTool.SetToolAction(utility.EntityMetadataToolName)
		research[utility.EntityMetadataToolName] = metadataTool
	}

	return research
}

// eruqlDelegate finds an attached eru-ql tool to run a lookup through.
func (oa *OrchestratorAgent) eruqlDelegate(ctx context.Context) tools.Tooling {
	for _, attached := range oa.AgentTools {
		if utility.IsEruqlTool(ctx, attached.Tool) {
			return attached.Tool
		}
	}
	return nil
}

// researchGuidance is added to the planning prompt only when a lookup is
// actually available.
func researchGuidance(available map[string]tools.Tooling) string {
	if len(available) == 0 {
		return ""
	}
	if _, ok := available[utility.EntityMetadataToolName]; !ok {
		return ""
	}
	return `
============================================================
CHECK BEFORE YOU COMMIT
============================================================
You may look things up BEFORE you produce the plan. Call ` + utility.EntityMetadataToolName + ` to see the
tenant's entities and their real fields.

Use it when the plan depends on a fact about the data that you would otherwise be guessing:
- the user named an entity and you are about to pass that name on to an agent
- a step would have to name fields, and you do not know what they are called
- you are about to decide the request needs SQL at all

Two things this is NOT for:
- Do not do the work here. You are choosing steps; the agents do the task.
- Do not look up what an agent looks up for itself - each agent's entry above says what it
  resolves on its own, and a step that duplicates it wastes a call and invites a worse answer.

Look up at most what you need, then call structured_output with the plan.
`
}

// executeResearchTool runs one of the planner's lookups.
func executeResearchTool(ctx context.Context, research map[string]tools.Tooling, toolName string, projectId string, tenantId string, input map[string]interface{}) (map[string]interface{}, error, bool) {
	tool, ok := research[toolName]
	if !ok || tool == nil {
		return nil, nil, false
	}
	logs.WithContext(ctx).Info(fmt.Sprint("planner research lookup: ", toolName))
	result, _, err := tool.Execute(ctx, projectId, tenantId, toolName, input)
	return result, err, true
}
