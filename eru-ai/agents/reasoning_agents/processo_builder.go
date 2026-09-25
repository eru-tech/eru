package reasoning_agents

import (
	"context"
	"encoding/json"
	"errors"
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
2b. A field that ALREADY MATCHES what was asked for needs nothing doing - but it has to be
   THAT field. Match on the exact name you were given, never on a similar one: asked for
   test_ask_a and finding test_scan_a, the answer is that test_ask_a does not exist and must
   be created, NOT that the request is already satisfied. Reporting a near-miss as a match
   tells the user their field is there when nothing was written, which is the one failure
   they cannot see. Compare what you
   were asked for against what get_entity_metadata returned, attribute by attribute: if the
   field is already there with that datatype and that storage, report it "unchanged" with a
   one-line note and move on. Do NOT offer to delete and recreate it. Deletion is
   irreversible and there is nothing to gain by it - the request is already satisfied, and
   asking the user to authorise destroying data to reach the state they are already in is
   the worst answer available. Only a field whose datatype or storage genuinely DIFFERS from
   the request is a delete-and-recreate question, and then you ask before touching it.
3. Then decide, deliberately, between EXTENDING what is there and creating something new.
   A workspace that has been in use has entities that already hold most of what is being
   asked for, under names the user did not think to mention - the same thing is called a
   contact in one company and a lead, a party or a card in another. A new entity beside an
   existing one that means the same thing is not a clean addition: the records split across
   two places, and neither can be merged afterwards.
   So before creating an entity, look for the closest existing one by what it HOLDS, not by
   what it is called, and then:
   - if one already covers the request, add the missing fields to it rather than creating a
     new entity, and say in your summary which one you extended and why;
   - if the closest match is genuinely a different thing, create the new entity and say in
     your summary what you compared it against and why it is not the same;
   - if it is a real toss-up and the answer changes what gets built, ask the user, naming the
     candidate and what it already holds.
   Creating a new entity without having named the alternative you rejected is not acceptable.

============================================================
{{FIELD_CATALOG}}
============================================================
HOW A WRITE WORKS
============================================================
save_entity writes ONE entity - entity_data is a single-object array holding just the entity
you are creating or editing. Never send the existing model: the backend merges what you give
it against what is already stored, so an entity you leave out keeps everything it has. Sending
the whole list is rejected, and it would rewrite every entity's display order besides.
An entity carries exactly these: name, display_name, description, hide_entity, is_people and
is_conv. Creating and editing take the same payload - the backend works out which it is and
where the entity sits - so there is no flag to set and none to get the wrong way round.

save_field writes ONE field. An empty field object ({}) with f_name set is how a field is
DELETED - never send an empty field object unless deletion is what was asked for.

READING THE SAVE RESPONSE. A save runs through a group of conditional steps, and a step whose
condition is false comes back as "<step> Ignored". That is a branch that did not apply - it is
ordinary control flow, NOT a rejection, and it tells you nothing about whether another step
wrote. So a response containing "Save Entity Field Ignored" does not mean the field was not
saved; more often than not it was. Never report a field as failed on the strength of an
"Ignored" message. If you genuinely need to know whether a write landed, read the model back
with get_entity_metadata and look. Reporting a successful write as a failure is its own bug:
it tells the user their data model is unchanged when it has in fact changed.

============================================================
WHAT TO ASK ABOUT
============================================================
Ask, do not guess, when the answer changes something that cannot be undone:
- a datatype that could reasonably be two things (is "amount" a number or a currency? is
  "stage" a status with a workflow or a plain dropdown?)
- whether a field holds personal or financial data, which decides is_pii / to_encrypt / is_pf
- deleting anything

**A required value you were not given and cannot look up is an ASK, never a guess.** If a
save is rejected for a missing key, that rejection is the signal to ask the user for it -
not to invent something and send it again. A plausible-looking invention is the worst
outcome available: the save is ACCEPTED and the field IS written, pointing at something that
does not exist. Nothing complains now; it breaks later, at the moment a user tries to use the
field, and by then nobody connects the two. A name like "default" is a guess, not a default.
These are the ones with no default worth guessing:
- attachment: storage_name names a storage that must already exist
- dropdown with option_type ENTITY_DATA or API: the entity, field or api it reads from
- status: the actual open and close statuses this business uses
- object: the entity to embed

Do not ask about things you can look up. The catalog tells you what keys a datatype takes;
get_entity_metadata tells you what exists. Ask once, with the options you can see, and
carry on when you have the answer.

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

// confirmWrites re-reads the data model and checks the answer against it in
// both directions: fields reported as written must be there, and fields
// reported as failed must not.
//
// It exists because the save response does not say what happened. The save runs
// through a function group whose steps are individually conditional, and a step
// whose condition is false is reported with an informational "<step> Ignored"
// message. That is ordinary control flow - a branch that did not apply - and it
// says nothing about whether some other branch wrote. So the envelope is
// ambiguous both ways: "created" can mean "submitted and not contradicted", and
// "Ignored" reads like a refusal when the field in fact landed.
//
// Reading the model back is the only thing that settles it, which is why a
// reported failure is checked just as hard as a reported success. Of the two,
// the false failure is the worse one to let through: the caller is told the
// data model is unchanged while it has in fact changed.
func (pbAgent *ProcessoBuilderAgent) confirmWrites(ctx context.Context, output map[string]interface{}) error {
	claimed := map[string][]string{}    // entity -> field names the answer says it wrote
	disclaimed := map[string][]string{} // entity -> field names the answer says it did not write
	// asserted are fields reported "unchanged", which is not a neutral
	// non-statement: it claims the field is ALREADY in the data model and
	// already correct, so nothing needed doing.
	//
	// That claim went unchecked until a run answered a request for test_ask_a by
	// reporting test_scan_a - a different, similarly named field - as unchanged,
	// and told the user their request was already satisfied. Nothing was written
	// and nothing was wrong as far as any check could see. A false "already
	// there" is worse than a false "created": the user has no reason to look.
	asserted := map[string][]string{}
	for _, raw := range sliceOf(output["fields"]) {
		field, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if action, _ := field["action"].(string); action == "failed" {
			name, _ := field["name"].(string)
			entity, _ := field["entity_name"].(string)
			if name != "" && entity != "" {
				disclaimed[entity] = append(disclaimed[entity], name)
			}
			continue
		}
		if action, _ := field["action"].(string); action == "unchanged" {
			name, _ := field["name"].(string)
			entity, _ := field["entity_name"].(string)
			if name != "" && entity != "" {
				asserted[entity] = append(asserted[entity], name)
			}
			continue
		}
		if skipUnwritten(field) {
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
	if len(claimed) == 0 && len(disclaimed) == 0 && len(asserted) == 0 {
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

	entityNames := make([]interface{}, 0, len(claimed)+len(disclaimed)+len(asserted))
	seenEntity := map[string]bool{}
	for _, set := range []map[string][]string{claimed, disclaimed, asserted} {
		for entity := range set {
			if !seenEntity[entity] {
				seenEntity[entity] = true
				entityNames = append(entityNames, entity)
			}
		}
	}
	result, _, err := metadata.Execute(ctx, scope.projectId, scope.tenantId, utility.EntityMetadataToolName,
		map[string]interface{}{"entity_names": entityNames, utility.FreshReadParam: true})
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

	// What the read-back actually contains, beside what is about to be compared
	// against it. Three confident diagnoses of this check have been wrong, all
	// for want of this one line: "the field is missing" cannot be told from "the
	// name differs by a character" or "the read is stale" without seeing both
	// sides.
	for entity, fields := range present {
		names := make([]string, 0, len(fields))
		for name := range fields {
			names = append(names, name)
		}
		sort.Strings(names)
		logs.WithContext(ctx).Info(fmt.Sprintf("confirmWrites read back %q with %d field(s): %s", entity, len(names), strings.Join(names, ", ")))
	}
	logs.WithContext(ctx).Info(fmt.Sprintf("confirmWrites is checking claimed=%v asserted=%v disclaimed=%v", claimed, asserted, disclaimed))

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

	// A field reported as failed that is present in the read-back is a false
	// failure: the write landed and the caller is about to be told it did not.
	var landed []string
	for entity, names := range disclaimed {
		fields, seen := present[entity]
		if !seen {
			continue
		}
		for _, name := range names {
			if fields[name] {
				landed = append(landed, entity+"."+name)
			}
		}
	}

	// A field reported "unchanged" that is NOT in the read-back was never there:
	// the answer told the user the request was already satisfied by something
	// that does not exist.
	absent := comparePresence(asserted, present)

	if len(missing) == 0 && len(landed) == 0 && len(absent) == 0 {
		return nil
	}

	var problems []string
	if len(missing) > 0 {
		sort.Strings(missing)
		problems = append(problems, fmt.Sprintf(
			"you reported these as written but they are not in the data model when it is read back: %s.\n"+
				"Check the arguments you sent (entity_name and the datatype's own keys), try the save "+
				"again, and if it still does not appear, report it as failed with what the tool returned - do not "+
				"report it as created",
			strings.Join(missing, ", ")))
	}
	if len(landed) > 0 {
		sort.Strings(landed)
		problems = append(problems, fmt.Sprintf(
			"you reported these as failed but they ARE in the data model when it is read back: %s.\n"+
				"The save succeeded. A \"<step> Ignored\" message in the response is not a refusal - it reports a "+
				"conditional step whose condition was false and which was therefore skipped, which is ordinary "+
				"control flow and says nothing about whether another step wrote. Report these with the action they "+
				"actually had (created, or updated) rather than as failed",
			strings.Join(landed, ", ")))
	}
	if len(absent) > 0 {
		sort.Strings(absent)
		problems = append(problems, fmt.Sprintf(
			"you reported these as unchanged, which says they are already in the data model, but they are NOT there "+
				"when it is read back: %s.\n"+
				"Reporting a field as unchanged claims it already exists and already matches the request. Check the "+
				"name you were asked for against the names that came back: a similarly named field is a DIFFERENT "+
				"field, and answering about it tells the user their request is done when nothing was written. Create "+
				"the field you were actually asked for",
			strings.Join(absent, ", ")))
	}
	return errors.New(strings.Join(problems, "\n\n"))
}

// comparePresence is which of these claimed-present fields are not in the
// read-back. Separated from confirmWrites so the comparison can be tested
// without a live workspace.
func comparePresence(asserted map[string][]string, present map[string]map[string]bool) []string {
	var absent []string
	for entity, names := range asserted {
		fields, seen := present[entity]
		for _, name := range names {
			if !seen || !fields[name] {
				absent = append(absent, entity+"."+name)
			}
		}
	}
	return absent
}

// unchangedButAbsent reads the "unchanged" claims out of an answer and reports
// the ones the workspace does not actually hold.
func unchangedButAbsent(output map[string]interface{}, present map[string]map[string]bool) []string {
	asserted := map[string][]string{}
	for _, raw := range sliceOf(output["fields"]) {
		field, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if action, _ := field["action"].(string); action != "unchanged" {
			continue
		}
		name, _ := field["name"].(string)
		entity, _ := field["entity_name"].(string)
		if name != "" && entity != "" {
			asserted[entity] = append(asserted[entity], name)
		}
	}
	return comparePresence(asserted, present)
}

// A field reported as failed or unchanged was not written, so holding it to the
// catalog would turn an honest report of a failure into a second failure.
// confirmWrites does not use this to dismiss failures - it checks those against
// the read-back too - but the catalog check has nothing to say about a payload
// that was never accepted.
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
