package reasoning_agents

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	catalog "github.com/eru-tech/eru/eru-ai/agents/processo_builder/catalog"
	tools "github.com/eru-tech/eru/eru-ai/tools"
	utility "github.com/eru-tech/eru/eru-ai/tools/utility"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
)

// ProcessoBuilderAgent shapes a data model: entities, and the fields on them.
//
// It is a reasoning agent that knows one domain. Everything it adds sits on the
// capability interfaces in agents/agent.go, so the loop itself stays ignorant of
// processo - the same arrangement the Eru Studio agent uses for pages.
//
// What it is NOT is a second copy of the rules. The per-datatype payload shapes
// come from the generated catalog; required-ness and the combinations that make
// a payload legal are enforced by the Go param structs on the processo tool at
// call time. This agent checks only what neither of those can: that a name is
// shaped the way the editor demands, that a reserved name is left alone, and
// that a key has some business on the datatype it was put on.
type ProcessoBuilderAgent struct {
	ReasoningAgent
	// internalTools are the tenant tools resolved for this agent's own lookups.
	internalTools map[string]tools.Tooling
}

func (pbAgent *ProcessoBuilderAgent) GetSpec() agents.AgentI {
	return pbAgent
}

// The scope of the request being served. ValidateOutput is handed only the
// answer, but confirming a write means reading the model back, and reading it
// back means knowing which workspace to read - so the scope travels on the
// context rather than on the agent, which is shared across requests.
type builderScopeKey struct{}

type builderScope struct {
	projectId string
	tenantId  string
}

func withBuilderScope(ctx context.Context, projectId, tenantId string) context.Context {
	return context.WithValue(ctx, builderScopeKey{}, builderScope{projectId: projectId, tenantId: tenantId})
}

func builderScopeFrom(ctx context.Context) (builderScope, bool) {
	scope, ok := ctx.Value(builderScopeKey{}).(builderScope)
	return scope, ok
}

func (pbAgent *ProcessoBuilderAgent) Execute(ctx context.Context, agentMessage agents.AgentMessage, conversationId string, projectId string, tenantId string) (agents.AgentMessage, error) {
	logs.WithContext(ctx).Debug("ProcessoBuilderAgent Execute - Start")
	return pbAgent.ReasoningAgent.Execute(withBuilderScope(ctx, projectId, tenantId), agentMessage, conversationId, projectId, tenantId)
}

func (pbAgent *ProcessoBuilderAgent) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("ProcessoBuilderAgent MakeFromJson - Start")
	if err := pbAgent.ReasoningAgent.MakeFromJson(ctx, rj); err != nil {
		return err
	}
	pbAgent.ReasoningAgent.Agent.Provider = pbAgent
	return nil
}

// ---------------------------------------------------------------- the contract

const processoBuilderSystemPrompt = `You are the Processo Data Model Builder. You shape the data model of ONE processo
workspace: its entities, and the fields on them.

YOUR JOB
Turn what the user describes - "track deals", "add a priority to tickets" - into entities and
fields that are correct the first time. You do the work by calling tools, not by describing it.

============================================================
BEFORE YOU WRITE ANYTHING
============================================================
1. Call get_processo_context ONCE. It returns org_id, process_id, process_name and
   org_process_id for the workspace this request is running against. Every write needs the
   first three. Never take these ids from the conversation, from something the user pasted,
   or from an error message, and never guess one - a wrong id writes to another customer's
   workspace.
2. Call get_entity_metadata to see what already exists. An entity you "create" that is
   already there, under a name you did not check, is the most common way to make a mess that
   cannot be undone: entities and fields cannot be renamed, only deleted and recreated.

============================================================
{{FIELD_CATALOG}}
============================================================
HOW A WRITE WORKS
============================================================
save_entity takes the COMPLETE entity list and REPLACES it. Read the current list first, add
yours to it, and send the whole thing. Sending only the new entity deletes every other one.

save_field writes ONE field. An empty field object ({}) with f_name set is how a field is
DELETED - never send an empty field object unless deletion is what was asked for.

A field's tab_name must be a tab that already exists on the entity. Check the entity's tabs
before you place a field; if the tab the field needs is not there, say so rather than
inventing one - creating tabs is not something you can do.

============================================================
WHAT TO ASK ABOUT
============================================================
Ask, do not guess, when the answer changes something that cannot be undone:
- a datatype that could reasonably be two things (is "amount" a number or a currency? is
  "stage" a status with a workflow or a plain dropdown?)
- whether a field holds personal or financial data, which decides is_pii / to_encrypt / is_pf
- deleting anything

Do not ask about things you can look up. The catalog tells you what keys a datatype takes;
get_entity_metadata tells you what exists.

============================================================
WHEN YOU ARE DONE
============================================================
Report what you actually did, entity by entity and field by field, using the names you wrote -
not the names the user used, if they differ. If something failed, say which one and why, and
do not describe it as done.`

func (pbAgent *ProcessoBuilderAgent) GetSystemPrompt() string {
	prompt := processoBuilderSystemPrompt
	placeholder := "{{FIELD_CATALOG}}"
	if !strings.Contains(prompt, placeholder) {
		// A silently unsubstituted placeholder is the defect the Eru Func agent
		// still carries; fail the build's tests rather than ship the literal.
		panic("processo builder prompt lost its " + placeholder + " placeholder")
	}
	return strings.Replace(prompt, placeholder, catalog.Get().Contract(), 1)
}

// GetOutputSchema is a report of what was done, not the payloads themselves -
// those went out through the tools. It is structured so the claim can be checked
// against the catalog rather than taken on trust.
func (pbAgent *ProcessoBuilderAgent) GetOutputSchema(ctx context.Context) eru_models.JSONSchema {
	actionEnum := []interface{}{"created", "updated", "deleted", "unchanged", "failed"}
	return eru_models.JSONSchema{
		Type: "object",
		Properties: map[string]eru_models.JSONSchema{
			"summary": {
				Type:        "string",
				Description: "What you did, in prose, for someone who did not watch it happen. Name the entities and fields you wrote.",
			},
			"entities": {
				Type:        "array",
				Description: "Every entity you touched.",
				Items: &eru_models.JSONSchema{
					Type: "object",
					Properties: map[string]eru_models.JSONSchema{
						"name":   {Type: "string", Description: "the entity name as written"},
						"action": {Type: "string", Enum: actionEnum},
						"note":   {Type: "string", Description: "why, when the action is failed or unchanged"},
					},
					Required: []string{"name", "action"},
				},
			},
			"fields": {
				Type:        "array",
				Description: "Every field you touched.",
				Items: &eru_models.JSONSchema{
					Type: "object",
					Properties: map[string]eru_models.JSONSchema{
						"entity_name": {Type: "string"},
						"name":        {Type: "string", Description: "the field name as written"},
						"label":       {Type: "string"},
						"datatype":    {Type: "string"},
						"action":      {Type: "string", Enum: actionEnum},
						"note":        {Type: "string", Description: "why, when the action is failed or unchanged"},
					},
					Required: []string{"entity_name", "name", "datatype", "action"},
				},
			},
		},
		Required: []string{"summary"},
	}
}

func (pbAgent *ProcessoBuilderAgent) GetInputSchema(ctx context.Context) eru_models.JSONSchema {
	return agents.AgentInputSchema(map[string]eru_models.JSONSchema{
		"entity_name": {
			Type:        "string",
			Description: "The entity this request is about, when the caller already knows it. Leave it out and the agent works it out from the request.",
		},
	}, nil)
}

// PlanningNote is what an orchestrator has to know to use this agent well. A
// planner that does not know it works entity-at-a-time plans one step per field
// and then has to thread ids between them.
func (pbAgent *ProcessoBuilderAgent) PlanningNote() string {
	return "ONE step per entity, not one per field. Give it the entity and everything it should " +
		"carry in a single request - it resolves the workspace ids itself, reads the existing model " +
		"itself, and writes the entity and all its fields in that one call. Never add a step to look " +
		"up org_id, process_id or entity metadata before it: it does both for itself."
}

// ------------------------------------------------------------------ its tools

// InternalToolRequests names the ACTIONS this agent needs, not tool names,
// because whatever the tenant called its eru-ql tool, the action is the same.
func (pbAgent *ProcessoBuilderAgent) InternalToolRequests() []agents.InternalToolRequest {
	return []agents.InternalToolRequest{
		{Action: "execute_query", Why: "resolves the workspace ids and reads the entity and field metadata, so the model never guesses an id or a field name"},
	}
}

func (pbAgent *ProcessoBuilderAgent) SetInternalTools(resolved map[string]tools.Tooling) {
	pbAgent.internalTools = resolved
}

func (pbAgent *ProcessoBuilderAgent) internalTool(action string) tools.Tooling {
	if pbAgent.internalTools == nil {
		return nil
	}
	return pbAgent.internalTools[action]
}

// eruqlDelegate prefers a tool the owner attached over the one resolved
// internally: an owner who wired a specific eru-ql tool meant that one.
func (pbAgent *ProcessoBuilderAgent) eruqlDelegate(ctx context.Context) tools.Tooling {
	for _, agentTool := range pbAgent.AgentTools {
		if utility.IsEruqlTool(ctx, agentTool.Tool) {
			return agentTool.Tool
		}
	}
	return pbAgent.internalTool("execute_query")
}

// ExtraTools are the lookups this agent cannot work without. An owner should not
// have to know it reads entity metadata or resolves its own ids.
func (pbAgent *ProcessoBuilderAgent) ExtraTools(ctx context.Context) map[string]tools.Tooling {
	extra := map[string]tools.Tooling{
		utility.FieldSpecToolName: pbAgent.fieldSpecTool(ctx),
	}
	delegate := pbAgent.eruqlDelegate(ctx)
	if delegate == nil {
		// Without eru-ql it can still answer questions about the model from the
		// catalog; it just cannot resolve ids or read what exists. Degrade rather
		// than fail - the prompt already tells it to say so.
		logs.WithContext(ctx).Info("ProcessoBuilderAgent has no eru-ql delegate: context and metadata lookups are unavailable")
		return extra
	}
	extra[utility.ProcessoContextToolName] = pbAgent.contextTool(ctx, delegate)
	extra[utility.EntityMetadataToolName] = pbAgent.entityMetadataTool(ctx, delegate)
	return extra
}

func (pbAgent *ProcessoBuilderAgent) fieldSpecTool(ctx context.Context) tools.Tooling {
	tool := &utility.FieldSpecTool{}
	_ = tool.SetAttribute(ctx, "tool_name", utility.FieldSpecToolName)
	_ = tool.SetAttribute(ctx, "tool_type", "FieldSpec")
	_ = tool.SetAttribute(ctx, "description", utility.FieldSpecToolDescription())
	_ = tool.SetAttribute(ctx, "parameters", utility.FieldSpecToolSchema())
	tool.SetToolAction(utility.FieldSpecToolName)
	return tool
}

func (pbAgent *ProcessoBuilderAgent) contextTool(ctx context.Context, delegate tools.Tooling) tools.Tooling {
	tool := &utility.ProcessoContextTool{Delegate: delegate}
	_ = tool.SetAttribute(ctx, "tool_name", utility.ProcessoContextToolName)
	_ = tool.SetAttribute(ctx, "tool_type", "ProcessoContext")
	_ = tool.SetAttribute(ctx, "description", utility.ProcessoContextToolDescription())
	_ = tool.SetAttribute(ctx, "parameters", utility.ProcessoContextToolSchema())
	tool.SetToolAction(utility.ProcessoContextToolName)
	return tool
}

func (pbAgent *ProcessoBuilderAgent) entityMetadataTool(ctx context.Context, delegate tools.Tooling) tools.Tooling {
	tool := &utility.EntityMetadataTool{Delegate: delegate}
	_ = tool.SetAttribute(ctx, "tool_name", utility.EntityMetadataToolName)
	_ = tool.SetAttribute(ctx, "tool_type", "EntityMetadata")
	_ = tool.SetAttribute(ctx, "description", utility.EntityMetadataToolDescription())
	_ = tool.SetAttribute(ctx, "parameters", utility.EntityMetadataToolSchema())
	tool.SetToolAction(utility.EntityMetadataToolName)
	return tool
}

// ------------------------------------------------------------------ validation

// maxReportedBuilderIssues caps one rejection. Past this the model is better
// served by fixing what it can see than by a wall of text.
const maxReportedBuilderIssues = 20

// ValidateOutput holds the report to the catalog. It is not a second copy of the
// tool's rules: it catches what nothing else does - a field name the editor
// would refuse but the backend accepts, a reserved system name, a key put on the
// wrong datatype - and it catches a report that claims something the catalog
// says is impossible, which means the write did not do what the answer says.
func (pbAgent *ProcessoBuilderAgent) ValidateOutput(ctx context.Context, output map[string]interface{}) error {
	logs.WithContext(ctx).Debug("ProcessoBuilderAgent ValidateOutput - Start")
	c := catalog.Get()
	var issues []catalog.Issue

	for i, raw := range sliceOf(output["entities"]) {
		entity, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if skipUnwritten(entity) {
			continue
		}
		issues = append(issues, c.ValidateEntity(fmt.Sprintf("entities[%d]", i), entity)...)
	}

	for i, raw := range sliceOf(output["fields"]) {
		field, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if skipUnwritten(field) {
			continue
		}
		issues = append(issues, c.ValidateFieldIdentity(fmt.Sprintf("fields[%d]", i), field)...)
	}

	if len(issues) == 0 {
		if err := pbAgent.confirmWrites(ctx, output); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("the data model you reported does not match what processo accepts:\n%s",
		catalog.FormatIssues(issues, maxReportedBuilderIssues))
}

// confirmWrites re-reads the data model and checks that the fields the answer
// claims to have written are actually there.
//
// It exists because a processo write can fail without saying so. The save runs
// through a function group whose steps are individually conditional, and a step
// that does not apply answers with an informational "Ignored" message and an
// empty result envelope - which is indistinguishable, at the tool boundary, from
// a step that applied and wrote nothing. The model has no error to report and
// truthfully reports what it was told, so "created" can mean "submitted and not
// contradicted" rather than "present".
//
// Reading the model back is the only thing that tells the two apart.
func (pbAgent *ProcessoBuilderAgent) confirmWrites(ctx context.Context, output map[string]interface{}) error {
	claimed := map[string][]string{} // entity -> field names the answer says it wrote
	for _, raw := range sliceOf(output["fields"]) {
		field, ok := raw.(map[string]interface{})
		if !ok || skipUnwritten(field) {
			continue
		}
		if action, _ := field["action"].(string); action == "deleted" {
			// A deletion is confirmed by absence, which the same read proves, but
			// the check below is written for presence; leave deletions alone
			// rather than assert the opposite badly.
			continue
		}
		name, _ := field["name"].(string)
		entity, _ := field["entity_name"].(string)
		if name == "" || entity == "" {
			continue
		}
		claimed[entity] = append(claimed[entity], name)
	}
	if len(claimed) == 0 {
		return nil
	}

	// The metadata tool takes the tenant from the arguments, not from the model,
	// so without the request's scope the read-back would query the wrong
	// workspace - or none - and report every field missing.
	scope, ok := builderScopeFrom(ctx)
	if !ok {
		logs.WithContext(ctx).Info("ProcessoBuilderAgent cannot confirm its writes: the request scope is not on the context")
		return nil
	}

	delegate := pbAgent.eruqlDelegate(ctx)
	if delegate == nil {
		// Nothing to read the model back with. Saying so in the log beats
		// failing an answer that may well be correct.
		logs.WithContext(ctx).Info("ProcessoBuilderAgent cannot confirm its writes: no eru-ql delegate")
		return nil
	}
	metadata := pbAgent.entityMetadataTool(ctx, delegate)

	entityNames := make([]interface{}, 0, len(claimed))
	for entity := range claimed {
		entityNames = append(entityNames, entity)
	}
	result, _, err := metadata.Execute(ctx, scope.projectId, scope.tenantId, utility.EntityMetadataToolName,
		map[string]interface{}{"entity_names": entityNames})
	if err != nil {
		// A lookup that cannot run must not turn a good answer into a failure.
		logs.WithContext(ctx).Error(fmt.Sprintf("ProcessoBuilderAgent could not confirm its writes: %v", err))
		return nil
	}

	// The tool is called in-process, so its result holds its own Go types rather
	// than the generic maps a JSON hop would leave. Round-tripping it is what
	// makes the shape predictable - asserting on []interface{} silently matches
	// nothing and would report every field missing.
	var readBack struct {
		Entities []struct {
			Name   string `json:"name"`
			Fields []struct {
				Name string `json:"name"`
			} `json:"fields"`
		} `json:"entities"`
	}
	encoded, encodeErr := json.Marshal(result)
	if encodeErr != nil || json.Unmarshal(encoded, &readBack) != nil {
		logs.WithContext(ctx).Error("ProcessoBuilderAgent could not read back the data model: unexpected metadata shape")
		return nil
	}
	if len(readBack.Entities) == 0 {
		// An empty read is far more likely to be a lookup that did not resolve
		// than proof that every entity vanished.
		logs.WithContext(ctx).Info("ProcessoBuilderAgent read back an empty data model: not confirming")
		return nil
	}

	present := map[string]map[string]bool{}
	for _, entity := range readBack.Entities {
		if entity.Name == "" {
			continue
		}
		fields := map[string]bool{}
		for _, f := range entity.Fields {
			if f.Name != "" {
				fields[f.Name] = true
			}
		}
		present[entity.Name] = fields
	}

	var missing []string
	for entity, names := range claimed {
		fields, seen := present[entity]
		if !seen {
			// The entity itself is not in the read-back. That is a stronger
			// failure than a missing field and is worth naming as such.
			missing = append(missing, fmt.Sprintf("%s (whole entity not found when reading the model back)", entity))
			continue
		}
		for _, name := range names {
			if !fields[name] {
				missing = append(missing, entity+"."+name)
			}
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return fmt.Errorf(
		"you reported these as written but they are not in the data model when it is read back: %s.\n"+
			"A processo save can answer without an error and still write nothing - an \"Ignored\" message with an "+
			"empty result means the step did not apply, not that it succeeded. Check the arguments you sent "+
			"(entity_name, tab_name and the datatype's own keys), try the save again, and if it still does not "+
			"appear, report it as failed with what the tool returned - do not report it as created",
		strings.Join(missing, ", "))
}

// A field reported as failed or unchanged was not written, so holding it to the
// catalog would turn an honest report of a failure into a second failure.
func skipUnwritten(entry map[string]interface{}) bool {
	action, _ := entry["action"].(string)
	return action == "failed" || action == "unchanged"
}

func sliceOf(value interface{}) []interface{} {
	if list, ok := value.([]interface{}); ok {
		return list
	}
	return nil
}
