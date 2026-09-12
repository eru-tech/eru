package reasoning_agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	studio "github.com/eru-tech/eru/eru-ai/agents/eru_studio"
	catalog "github.com/eru-tech/eru/eru-ai/agents/eru_studio/catalog"
	models "github.com/eru-tech/eru/eru-ai/models"
	tools "github.com/eru-tech/eru/eru-ai/tools"
	utility "github.com/eru-tech/eru/eru-ai/tools/utility"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	"github.com/google/uuid"
)

type EruStudioAgent struct {
	ReasoningAgent
	// PageOrgId and PageProcessId scope the existing-page lookups. Both default
	// to the tenant id, which is how this deployment keys them; set them only
	// when a deployment separates the two.
	PageOrgId     string `json:"page_org_id,omitempty"`
	PageProcessId string `json:"page_process_id,omitempty"`
	// internalTools are the tenant tools resolved for this agent's own lookups.
	internalTools map[string]tools.Tooling
}

func (eruStudioAgent *EruStudioAgent) GetSpec() agents.AgentI {
	return eruStudioAgent
}

func (eruStudioAgent *EruStudioAgent) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("EruStudioAgent MakeFromJson - Start")
	err := eruStudioAgent.ReasoningAgent.MakeFromJson(ctx, rj)
	if err != nil {
		return err
	}
	eruStudioAgent.ReasoningAgent.Agent.Provider = eruStudioAgent

	// The embedded ReasoningAgent unmarshals its own fields, so this agent's own
	// config has to be read here or the overrides silently do nothing.
	var own struct {
		PageOrgId     string `json:"page_org_id"`
		PageProcessId string `json:"page_process_id"`
	}
	if uerr := json.Unmarshal(*rj, &own); uerr == nil {
		eruStudioAgent.PageOrgId = own.PageOrgId
		eruStudioAgent.PageProcessId = own.PageProcessId
	}
	return nil
}

func (eruStudioAgent *EruStudioAgent) Execute(ctx context.Context, agentMessage agents.AgentMessage, conversationId string, projectId string, tenantId string) (agents.AgentMessage, error) {
	logs.WithContext(ctx).Debug("EruStudioAgent Execute - Start")

	// The page the client already holds is both the starting point for the model
	// and, in patch mode, the base a patch is applied to.
	basePage := basePageFromParams(agentMessage.Params)

	// The client holds the only copy of the page that includes unsaved edits, so
	// the page it sends is the base of truth. When it also tells us which
	// revision that is, a mismatch means it sent something other than what the
	// user is looking at - a cached copy, the wrong page - and patching that
	// would resolve to a page that quietly undoes their work.
	if err := verifyBaseRevision(agentMessage.Params, basePage); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return agents.AgentMessage{}, err
	}

	requested, _ := agentMessage.Params[studio.OutputModeParam].(string)
	mode := studio.NegotiateMode(requested, len(basePage) > 0)
	ctx = studio.WithOutputMode(ctx, mode)
	ctx = studio.WithBasePage(ctx, basePage)
	// A streaming client gets each component as the model writes it. The scanner
	// belongs to this response, so it goes in the context rather than on the
	// agent, which is shared across requests.
	if agents.GetStreamCallback(ctx) != nil {
		ctx = studio.WithComponentScanner(ctx, studio.NewComponentScanner())
	}
	ctx = studio.WithInlineNested(ctx, studio.ParseInlineNested(agentMessage.Params[studio.InlineNestedParam]))
	ctx = studio.WithPageScope(ctx, eruStudioAgent.pageScopeFor(tenantId))

	// Who names the page is decided here rather than by the model, so a prompt
	// that asks for a particular id cannot end up in a standoff with the request.
	identity := resolvePageIdentity(agentMessage.Params, basePage, conversationId)
	ctx = studio.WithPageIdentity(ctx, identity)

	scopeNote, resolvedScope, err := applyEruStudioScope(ctx, agentMessage.Params, basePage, mode)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return agents.AgentMessage{}, err
	}
	if resolvedScope != nil {
		ctx = studio.WithScope(ctx, resolvedScope)
	}

	augment := buildEruStudioContextAugmentation(ctx, agentMessage.Params, identity)
	if scopeNote != "" {
		augment = scopeNote + "\n" + augment
	}
	if instructions := eruStudioModeInstructions(mode); instructions != "" {
		augment = instructions + "\n" + augment
	}
	if augment != "" {
		if strings.TrimSpace(agentMessage.Content) == "" {
			agentMessage.Content = augment
		} else {
			agentMessage.Content = fmt.Sprintf("%s\n\n--- USER PROMPT ---\n%s", augment, agentMessage.Content)
		}
	}
	delete(agentMessage.Params, "code")
	delete(agentMessage.Params, studio.OutputModeParam)
	delete(agentMessage.Params, studio.ScopeParam)
	delete(agentMessage.Params, studio.BaseRevisionParam)
	delete(agentMessage.Params, studio.InlineNestedParam)
	delete(agentMessage.Params, studio.PageIdParam)

	agentOutput, err := eruStudioAgent.ReasoningAgent.Execute(ctx, agentMessage, conversationId, projectId, tenantId)
	if err != nil {
		return agentOutput, err
	}
	if !studio.EnvelopeEnabled(ctx) {
		return eruStudioStampBarePage(ctx, agentOutput), nil
	}
	return eruStudioResolveEnvelope(ctx, agentOutput, basePage)
}

// verifyBaseRevision holds the client to the page it says it is holding.
func verifyBaseRevision(params map[string]interface{}, basePage map[string]interface{}) error {
	claimed, _ := params[studio.BaseRevisionParam].(string)
	claimed = strings.TrimSpace(claimed)
	if claimed == "" {
		return nil
	}
	if len(basePage) == 0 {
		return fmt.Errorf("%s %q was sent without a page in `code` - send the page the revision belongs to, or drop the revision",
			studio.BaseRevisionParam, claimed)
	}
	actual := studio.Revision(basePage)
	if claimed != actual {
		return fmt.Errorf("the page in `code` is revision %s, but %s says %s. "+
			"Send the page as it currently stands in the editor, including unsaved changes - patching a different page would resolve to one that undoes them",
			actual, studio.BaseRevisionParam, claimed)
	}
	return nil
}

// applyEruStudioScope narrows a scoped edit down to the part of the page it is
// about. It rewrites params["code"] to the pruned page, so only the components
// in play - and the containers around them - reach the model; the unpruned page
// stays the base for applying and validating the patch.
func applyEruStudioScope(ctx context.Context, params map[string]interface{}, basePage map[string]interface{}, mode string) (string, *studio.ResolvedScope, error) {
	if params == nil {
		return "", nil, nil
	}
	scope, err := studio.ParseScope(params[studio.ScopeParam])
	if err != nil {
		return "", nil, err
	}
	if scope.IsEmpty() {
		return "", nil, nil
	}
	if len(basePage) == 0 {
		// A scope with no page to scope is meaningless, not fatal: there is
		// nothing to prune and nothing to hold the answer to, so the request is
		// exactly the unscoped one. Failing here killed whole runs where a
		// planner passed a scope along to a page being authored from scratch -
		// the one case where there is certainly no existing page. Ignored and
		// logged, like the mode check below.
		logs.WithContext(ctx).Info(fmt.Sprintf("eru studio %s ignored: no page was sent in `code`, so there is nothing to scope", studio.ScopeParam))
		return "", nil, nil
	}
	if mode != studio.ModePatch && mode != studio.ModeAuto {
		// Pruning a page the model is about to re-emit in full would lose every
		// component it was not shown. Scoping needs the envelope, where the answer
		// is a patch against the page the client already holds.
		logs.WithContext(ctx).Info(fmt.Sprintf("eru studio %s ignored: it requires output_mode patch or auto, this request is %q", studio.ScopeParam, mode))
		return "", nil, nil
	}

	resolved := scope.Resolve(basePage)
	pruned := studio.Prune(basePage, resolved)
	encoded, err := json.Marshal(pruned)
	if err != nil {
		return "", nil, err
	}
	params["code"] = string(encoded)

	logs.WithContext(ctx).Info(fmt.Sprintf(
		"eru studio scoped edit: %d writable, %d shown in full, %d reduced to structure",
		len(resolved.Writable), len(resolved.Detailed), resolved.Omitted))

	return studio.ScopeInstructions(resolved, scope), resolved, nil
}

// eruStudioResolveEnvelope rewrites the answer action into the page-update
// envelope, and adds one action per nested page the edit produced.
//
// An edit can touch several pages: a repeated row needs a template page, a side
// panel or a popup is a page of its own, a board card is a page. The renderer
// mounts them by id and the store saves them one at a time, so each comes back
// as its own action - the client opens each in its own tab and the user accepts
// them separately. A question action is left alone: the agent is asking, not
// answering.
func eruStudioResolveEnvelope(ctx context.Context, agentOutput agents.AgentMessage, basePage map[string]interface{}) (agents.AgentMessage, error) {
	actions := make([]agents.AgentOutputAction, 0, len(agentOutput.Actions))
	for _, action := range agentOutput.Actions {
		if action.ActionType != agents.ActionTypeAnswer || action.Action == nil {
			actions = append(actions, action)
			continue
		}
		started := time.Now()
		agents.EmitStepStarted(ctx, agents.StepApplyPatch, 1)
		root, nested, err := resolveStudioOutput(ctx, action.Action, basePage)
		if err != nil {
			logs.WithContext(ctx).Error(fmt.Sprintf("eru studio patch could not be resolved: %v", err))
			agents.EmitStepFinished(ctx, agents.StepApplyPatch, 1, agents.OutcomeError, started, err.Error(), agents.CodePatchUnresolved)
			return agents.AgentMessage{}, err
		}
		mode, _ := root["mode"].(string)
		detail := mode
		if len(nested) > 0 {
			detail = fmt.Sprintf("%s + %d nested page(s)", mode, len(nested))
		}
		agents.EmitStepFinished(ctx, agents.StepApplyPatch, 1, agents.OutcomeSuccess, started, detail, "")

		action.Action = root
		actions = append(actions, action)
		for _, page := range nested {
			actions = append(actions, agents.AgentOutputAction{
				ActionType: agents.ActionTypeAnswer,
				ActionName: action.ActionName,
				Action:     page,
			})
		}
	}
	agentOutput.Actions = actions
	return agentOutput, nil
}

// resolvePageIdentity decides which id the answer's page must carry.
//
// The page in `code` is the page the user is editing, so its id is the answer's
// id - and it is the only source that is authoritative by construction. A
// client that is not sending the page can name it with the page_id param
// instead. Failing both, there is no page yet: the id falls back to the
// conversation, which is stable across the turns of one build, but the model is
// not told about it and anything it names itself wins.
//
// What it must never be is the conversation id the sub-agent runs under: that
// carries a "::<agent>" suffix, which is how a conversation is addressed and not
// how a page is. Handing it over as the page id also made the request contradict
// any prompt that asked for a particular id, and the model spent its reasoning
// deciding which of the two to obey - twice, both times choosing the
// conversation id over what the user had asked for.
func resolvePageIdentity(params map[string]interface{}, basePage map[string]interface{}, conversationId string) studio.PageIdentity {
	if id, ok := basePage["id"].(string); ok && strings.TrimSpace(id) != "" {
		return studio.PageIdentity{Id: strings.TrimSpace(id), Fixed: true}
	}
	if raw, ok := params[studio.PageIdParam]; ok {
		if id := strings.TrimSpace(stringifyParam(raw)); id != "" && id != "null" {
			return studio.PageIdentity{Id: id, Fixed: true}
		}
	}
	// The sub-agent's conversation id is "<conversation>::<agent>"; only the
	// conversation part identifies the build.
	fallback := strings.TrimSpace(conversationId)
	if cut := strings.Index(fallback, "::"); cut >= 0 {
		fallback = fallback[:cut]
	}
	if fallback == "" {
		fallback = uuid.New().String()
	}
	return studio.PageIdentity{Id: fallback}
}

// eruStudioStampBarePage applies the page's identity to a bare-page answer,
// where the action IS the page.
func eruStudioStampBarePage(ctx context.Context, agentOutput agents.AgentMessage) agents.AgentMessage {
	identity := studio.PageIdentityFrom(ctx)
	for _, action := range agentOutput.Actions {
		if action.ActionType != agents.ActionTypeAnswer || action.Action == nil {
			continue
		}
		if replaced, changed := identity.Stamp(action.Action); changed && replaced != "" {
			logs.WithContext(ctx).Info(fmt.Sprintf("eru studio kept the page id: answered with %q, restored %q", replaced, identity.Id))
		}
	}
	return agentOutput
}

// basePageFromParams decodes the existing page out of the request params. The
// param is a stringified EruPage for most callers and an object for a few, so
// both are accepted.
func basePageFromParams(params map[string]interface{}) map[string]interface{} {
	if params == nil {
		return nil
	}
	raw, ok := params["code"]
	if !ok {
		return nil
	}
	switch typed := raw.(type) {
	case map[string]interface{}:
		return typed
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" || trimmed == "{}" || trimmed == "null" {
			return nil
		}
		var page map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &page); err != nil {
			return nil
		}
		return page
	default:
		return nil
	}
}

func (eruStudioAgent *EruStudioAgent) GetOutputSchema(ctx context.Context) eru_models.JSONSchema {
	if studio.EnvelopeEnabled(ctx) {
		return buildEruPageUpdateOutputSchema()
	}
	return buildEruPageOutputSchema()
}

func (eruStudioAgent *EruStudioAgent) GetInputSchema(_ context.Context) eru_models.JSONSchema {
	return agents.AgentInputSchema(map[string]eru_models.JSONSchema{
		"code": agents.CodeParamSchema("EruPage JSON"),
		studio.PageIdParam: {
			Type: "string",
			Description: "The id of the page being edited, for a client that is not sending the page itself in `code`. " +
				"The answer comes back carrying this id. When `code` is sent, the id inside it wins and this is unnecessary. " +
				"Send it for a page that exists but is not being round-tripped: without it the agent has to name the page, " +
				"and what it names becomes that page's identity.",
		},
		studio.ScopeParam: {
			Type: "string",
			Description: "Which part of the page this prompt is about, when it is about a part: a component id, a comma-separated list of ids, " +
				"or a stringified {\"component_ids\": [...], \"include_descendants\": true, \"allow_page_props\": false}. " +
				"Scoping an edit does two things: components outside the scope reach the model as structure only (id/type/nesting), " +
				"which is most of the prompt on a real page, and a patch that changes anything outside the scope is rejected. " +
				"Requires output_mode patch or auto - a full page must be regenerated whole, so it cannot be scoped.",
		},
		studio.BaseRevisionParam: {
			Type: "string",
			Description: "The `revision` of the page being sent in `code`, from the last response's envelope. Optional but recommended: " +
				"if it does not match the page actually sent, the request fails instead of patching a page the user is not looking at.",
		},
		studio.InlineNestedParam: {
			Type: "string",
			Enum: []any{"true", "false"},
			Description: "Whether the root page comes back with each nested page's components inlined under the component that mounts it, " +
				"so the client can render everything in one pass. \"true\" (the default) or \"false\". " +
				"The nested pages are returned as their own actions either way, and those are what the user saves - " +
				"inlining is a rendering convenience, not the unit of persistence.",
		},
		studio.OutputModeParam: {
			Type: "string",
			Enum: []any{studio.ModeFull, studio.ModePatch, studio.ModeAuto},
			Description: "How the page should come back. \"full\" (the default) returns a bare EruPage, exactly as this agent has always answered. " +
				"\"patch\" returns a page-update envelope carrying only the components that changed, plus the full page they resolve to. " +
				"\"auto\" patches when an existing page was passed in `code` and sends a full page otherwise. " +
				"Only set this if the caller can read the envelope: see agents/eru_studio for its shape.",
		},
		"context": {
			Type:        "string",
			Description: "Stringified JSON of the data the page must render (rows fetched by an earlier step, sample data, entity hints). Drives component types, field names and component `data` properties. Pass the upstream value itself - never a paraphrase or a sample you typed.",
		},
		"entities": {
			Type:        "string",
			Description: "Stringified JSON array of the entities and their fields available to this page. Used to wire `name`, `entity_name` and form-field `identifier`.",
		},
		"apis": {
			Type:        "string",
			Description: "Stringified JSON array of the api names available to this page. Used to fill the `api_name` property of the components that fetch their own data (page_ref, select-eru, grid, the charts); the agent will not invent api names.",
		},
	}, nil)
}

// componentLibraryPlaceholder marks where the generated component library goes.
// The generated block is substituted in place rather than appended, so the
// prompt still reads in order - page shape, what components exist, how to choose
// between them - and so nothing in the prose has to restate the library.
const componentLibraryPlaceholder = "{{COMPONENT_LIBRARY}}"

// nestedPagesPlaceholder marks where the nesting guidance goes: which components
// mount another page (generated from the library) and when a design needs one.
const nestedPagesPlaceholder = "{{NESTED_PAGES}}"

func (eruStudioAgent *EruStudioAgent) GetSystemPrompt() string {
	prompt := studioSystemPrompt()
	if eruStudioAgent.entityMetadataDelegate(context.Background()) != nil {
		prompt += entityMetadataGuidance
	}
	if eruStudioAgent.internalTool("fetch_pages") != nil || eruStudioAgent.internalTool("fetch_page") != nil {
		prompt += pageLibraryGuidance
	}
	return prompt
}

// pageLibraryGuidance is added only when the lookups exist. Users refer to their
// other pages constantly - "like the invoice page", "the one I built yesterday",
// "same header as the dashboard" - and an agent that cannot read them either
// invents a layout or asks a question it could have answered itself.
const pageLibraryGuidance = `
============================================================
WHEN THE USER POINTS AT ANOTHER PAGE
============================================================
The pages already built in this workspace are readable, and the user will refer to them by name
rather than by id: "make it look like the invoice page", "same header as the dashboard", "copy the
filters from the one I built yesterday", "like page X but for payments".

When that happens:
1. Call list_pages to find it. Pass name_contains with the words the user used to narrow a long list.
2. Call get_page with the id from that list. Never invent a page id, and never pass the id of the
   page you are editing.
3. Read the reference for the SPECIFIC thing the user asked for - a layout, a header, an event
   wiring, a grid configuration - and build that into the page you are working on.

A reference page is not your answer:
- Never return it, and never copy it wholesale unless the user asked for a duplicate.
- Never reuse its component ids: ids are unique per page, so generate fresh ones.
- Do not copy its data bindings blindly - the entity behind this page may be different. Check the
  field names against the entity metadata before reusing them.
- If a very large page comes back as a structure only (ids and types), that is usually enough to
  imitate a layout. Ask for it again only when you need one component's properties, and say which.

If the user names a page that list_pages does not contain, say so and ask which page they meant
rather than guessing at a similar name.
`

// entityMetadataGuidance is added only when the agent can actually run the
// lookup. Telling the model about a tool it does not have is worse than not
// mentioning it: it will try, fail, and spend a turn finding out.
const entityMetadataGuidance = `
============================================================
FIELD NAMES AND LABELS COME FROM THE ENTITY METADATA
============================================================
A form field binds through its "name" (and "identifier"), and that name must be a real field of a
real entity - not a plausible-looking guess. Its label should be that field's display name, which is
the wording the user already reads elsewhere in the product.

Call get_entity_metadata before you name form fields or write their labels. It returns each entity
with its fields: the real field name, its display name and its data type. Then:
- component properties.base.name  = the field name from the metadata
- component properties.base.label = that field's display name
- pick the component type to match the data type (a date field gets "date", a money field gets
  "currency", a long text field gets "textarea", a foreign key gets "select-eru" bound to the entity)
- set entityName / entity_name on the page and the components to the entity you actually found

If the metadata has no entity for what the user asked about, do not invent field names to fill the
gap. Build the page with the user's own wording as labels, leave "identifier" off so nothing claims
to bind, and say in your summary which fields have no entity behind them.
`

// studioSystemPrompt is the prose prompt with the generated component library
// substituted in. It panics if the placeholder is missing, because a prompt with
// no component library would leave the model to invent component types - a test
// covers it, so this can only fire on an edit that removed the marker.
func studioSystemPrompt() string {
	for _, placeholder := range []string{componentLibraryPlaceholder, nestedPagesPlaceholder} {
		if !strings.Contains(eruStudioSystemPrompt, placeholder) {
			panic("eru studio prompt is missing " + placeholder + " - the generated block has nowhere to go")
		}
	}
	prompt := strings.Replace(eruStudioSystemPrompt, componentLibraryPlaceholder, studioCatalog.Contract(), 1)
	return strings.Replace(prompt, nestedPagesPlaceholder, studio.NestingGuidance(), 1)
}

// InternalToolRequests names the lookups this agent needs for itself. They are
// resolved from the tenant's configured tools, so nobody has to attach them: an
// owner should not have to know that naming a form field requires the entity
// metadata, or that "make it like the invoice page" requires reading that page.
func (eruStudioAgent *EruStudioAgent) InternalToolRequests() []agents.InternalToolRequest {
	return []agents.InternalToolRequest{
		{Action: "execute_query", Why: "entity and field metadata, so form fields bind to real fields with their real labels"},
		{Action: "fetch_pages", Why: "the list of existing pages, so a page the user refers to by name can be found"},
		{Action: "fetch_page", Why: "an existing page's JSON, so a layout the user points at can be imitated"},
	}
}

// SetInternalTools receives whatever the tenant actually has. A missing action
// simply removes that capability - the agent says so in its answer rather than
// guessing.
func (eruStudioAgent *EruStudioAgent) SetInternalTools(resolved map[string]tools.Tooling) {
	eruStudioAgent.internalTools = resolved
}

func (eruStudioAgent *EruStudioAgent) internalTool(action string) tools.Tooling {
	if eruStudioAgent.internalTools == nil {
		return nil
	}
	return eruStudioAgent.internalTools[action]
}

// ExtraTools gives the agent the reference lookups it works from: the component
// library, the tenant's entity metadata, and the pages that already exist.
func (eruStudioAgent *EruStudioAgent) ExtraTools(ctx context.Context) map[string]tools.Tooling {
	extra := map[string]tools.Tooling{}

	specTool := &utility.ComponentSpecTool{}
	_ = specTool.SetAttribute(ctx, "parameters", utility.ComponentSpecToolSchema())
	_ = specTool.SetAttribute(ctx, "description", utility.ComponentSpecToolDescription())
	_ = specTool.SetAttribute(ctx, "system_prompt", "")
	_ = specTool.SetAttribute(ctx, "tool_name", utility.ComponentSpecToolName)
	_ = specTool.SetAttribute(ctx, "tool_type", "COMPONENT_SPEC")
	specTool.SetToolAction(utility.ComponentSpecToolName)
	extra[utility.ComponentSpecToolName] = specTool

	// A form field's `name` has to be a real entity field and its label has to be
	// that field's display name. The model cannot know either, and a guess
	// produces a page that looks right and binds to nothing.
	if delegate := eruStudioAgent.entityMetadataDelegate(ctx); delegate != nil {
		metadataTool := &utility.EntityMetadataTool{Delegate: delegate}
		_ = metadataTool.SetAttribute(ctx, "parameters", utility.EntityMetadataToolSchema())
		_ = metadataTool.SetAttribute(ctx, "description", utility.EntityMetadataToolDescription())
		_ = metadataTool.SetAttribute(ctx, "system_prompt", "")
		_ = metadataTool.SetAttribute(ctx, "tool_name", utility.EntityMetadataToolName)
		_ = metadataTool.SetAttribute(ctx, "tool_type", "ENTITY_METADATA")
		metadataTool.SetToolAction(utility.EntityMetadataToolName)
		extra[utility.EntityMetadataToolName] = metadataTool
	} else {
		logs.WithContext(ctx).Info("eru studio: no tool offers execute_query, so " + utility.EntityMetadataToolName + " is not offered")
	}

	// "Make it look like the invoice page" needs the list of pages to find that
	// page, and its JSON to read how it was built.
	listDelegate := eruStudioAgent.internalTool("fetch_pages")
	getDelegate := eruStudioAgent.internalTool("fetch_page")
	if listDelegate != nil || getDelegate != nil {
		scope := studio.PageScopeFrom(ctx)
		library := &utility.PageLibraryTool{
			ListDelegate: listDelegate,
			GetDelegate:  getDelegate,
			OrgId:        scope.OrgId,
			ProcessId:    scope.ProcessId,
		}
		if listDelegate != nil {
			listTool := *library
			_ = listTool.SetAttribute(ctx, "parameters", utility.ListPagesToolSchema())
			_ = listTool.SetAttribute(ctx, "description", utility.ListPagesToolDescription())
			_ = listTool.SetAttribute(ctx, "system_prompt", "")
			_ = listTool.SetAttribute(ctx, "tool_name", utility.ListPagesToolName)
			_ = listTool.SetAttribute(ctx, "tool_type", "PAGE_LIBRARY")
			listTool.SetToolAction(utility.ListPagesToolName)
			extra[utility.ListPagesToolName] = &listTool
		}
		if getDelegate != nil {
			getTool := *library
			_ = getTool.SetAttribute(ctx, "parameters", utility.GetPageToolSchema())
			_ = getTool.SetAttribute(ctx, "description", utility.GetPageToolDescription())
			_ = getTool.SetAttribute(ctx, "system_prompt", "")
			_ = getTool.SetAttribute(ctx, "tool_name", utility.GetPageToolName)
			_ = getTool.SetAttribute(ctx, "tool_type", "PAGE_LIBRARY")
			getTool.SetToolAction(utility.GetPageToolName)
			extra[utility.GetPageToolName] = &getTool
		}
	} else {
		logs.WithContext(ctx).Info("eru studio: no tool offers fetch_pages/fetch_page, so existing pages cannot be referenced")
	}

	return extra
}

// entityMetadataDelegate is the tool that runs the metadata query: the internal
// one resolved from the tenant, or an eru-ql tool the owner attached explicitly,
// which still wins so an owner can point the agent at a different one.
func (eruStudioAgent *EruStudioAgent) entityMetadataDelegate(ctx context.Context) tools.Tooling {
	for _, attached := range eruStudioAgent.AgentTools {
		if utility.IsEruqlTool(ctx, attached.Tool) {
			return attached.Tool
		}
	}
	return eruStudioAgent.internalTool("execute_query")
}

// pageScopeFor resolves the org and process the existing pages belong to.
//
// The processo page actions take org_id and process_id explicitly, and neither
// is on the agent's execution context - only the project and the tenant are.
// Both default to the tenant id, because that is how the entity metadata query
// is keyed (org_process_id = the tenant id) and it is the only mapping this code
// can see. A deployment that separates them sets page_org_id / page_process_id
// on the agent; this is the one place it is decided.
func (eruStudioAgent *EruStudioAgent) pageScopeFor(tenantId string) studio.PageScope {
	scope := studio.PageScope{OrgId: eruStudioAgent.PageOrgId, ProcessId: eruStudioAgent.PageProcessId}
	if scope.OrgId == "" {
		scope.OrgId = tenantId
	}
	if scope.ProcessId == "" {
		scope.ProcessId = tenantId
	}
	return scope
}

// EnrichStream turns the model's half-written answer into components a client
// can render now. A structured-output agent's answer is a tool argument, so
// without this the page arrives in one lump after the whole thing is generated -
// the longest wait in the product, for output that was ready in pieces.
func (eruStudioAgent *EruStudioAgent) EnrichStream(ctx context.Context, event models.ModelStreamEvent) []agents.StreamEvent {
	if event.Type != models.StreamToolInputDelta || event.ToolName != models.TerminalToolStructuredOutput {
		return nil
	}
	scanner := studio.ComponentScannerFrom(ctx)
	if scanner == nil {
		return nil
	}
	scanned := scanner.Write(event.Content)
	if len(scanned) == 0 {
		return nil
	}
	out := make([]agents.StreamEvent, 0, len(scanned))
	for _, component := range scanned {
		out = append(out, agents.StreamEvent{
			Event:     agents.StreamEventPageComponent,
			Data:      component,
			Iteration: event.Iteration,
		})
	}
	return out
}

// ValidateOutput checks the emitted page against the component library: that
// every type exists, that every property key and enum value is real for that
// type, that leaves carry no children and that each event action has the field
// it cannot run without. The JSON schema cannot express any of this - property
// bags are per-type - so this is the check that actually holds the model to the
// library.
func (eruStudioAgent *EruStudioAgent) ValidateOutput(ctx context.Context, output map[string]interface{}) error {
	page, issues := eruStudioPageIssues(ctx, output)
	if page == nil {
		return nil
	}
	if len(issues) == 0 {
		return nil
	}
	logs.WithContext(ctx).Info(fmt.Sprintf("eru studio page validation found %d issue(s)", len(issues)))
	return fmt.Errorf("the page does not match the eru-studio component library:\n%s", catalog.FormatIssues(issues, maxReportedPageIssues))
}

// maxReportedPageIssues caps what goes back to the model. A page that is wrong
// everywhere produces hundreds of issues and the model only needs to see the
// shape of the mistake to fix all of them.
const maxReportedPageIssues = 25

func eruStudioPageIssues(ctx context.Context, output map[string]interface{}) (map[string]interface{}, []catalog.Issue) {
	if output == nil {
		return nil, nil
	}
	if studio.EnvelopeEnabled(ctx) {
		return output, validateStudioUpdate(output, studio.BasePageFrom(ctx), studio.ScopeFrom(ctx))
	}
	// A bare page can carry no nested page, so every mount on it must point at a
	// page that already exists. One that does not renders an empty panel.
	issues := studioCatalog.ValidatePage(output)
	basePage := studio.BasePageFrom(ctx)
	known := map[string]bool{}
	for _, mount := range studio.FindMounts(basePage) {
		if mount.PageId != "" {
			known[mount.PageId] = true
		}
	}
	return output, append(issues, studio.ValidateMounts(output, nil, known)...)
}

func buildEruStudioContextAugmentation(_ context.Context, params map[string]any, identity studio.PageIdentity) string {
	var b strings.Builder

	if identity.Fixed {
		fmt.Fprintf(&b, "This page's id is %q. Return it verbatim as EruPage.id - it is the identity of the page the user is editing, "+
			"not a name, and changing it makes the client save a new page instead of updating this one.\n\n", identity.Id)
	}

	if codeRaw, ok := params["code"]; ok {
		codeStr := stringifyParam(codeRaw)
		if strings.TrimSpace(codeStr) != "" && strings.TrimSpace(codeStr) != "{}" && strings.TrimSpace(codeStr) != "null" {
			b.WriteString("--- EXISTING ERU PAGE JSON ---\n")
			b.WriteString("This is the page produced in a previous iteration. Build on top of it: preserve component ids, properties, styles, events, and validation_rules unless the user prompt clearly requires changing them. Add, remove, or restructure components only as needed to satisfy the new prompt.\n\n")
			b.WriteString(codeStr)
			b.WriteString("\n--- END EXISTING ERU PAGE JSON ---\n\n")
		} else {
			b.WriteString("No existing EruPage was provided. Build the page from scratch.\n\n")
		}
	}

	if ctxRaw, ok := params["context"]; ok {
		ctxStr := stringifyParam(ctxRaw)
		if strings.TrimSpace(ctxStr) != "" && strings.TrimSpace(ctxStr) != "{}" {
			b.WriteString("--- DATA CONTEXT (sample data and entity hints) ---\n")
			b.WriteString("Use this to choose appropriate component types, field names, and to populate component `data` properties (stringified JSON) where applicable. Handle nil/empty data gracefully via component defaults.\n\n")
			b.WriteString(ctxStr)
			b.WriteString("\n--- END DATA CONTEXT ---\n\n")
		}
	}

	if entitiesRaw, ok := params["entities"]; ok {
		entStr := stringifyParam(entitiesRaw)
		if strings.TrimSpace(entStr) != "" && strings.TrimSpace(entStr) != "[]" {
			b.WriteString("--- AVAILABLE ENTITIES ---\n")
			b.WriteString("Use these entities and their fields when wiring `name`, `entity_name`, and form-field `identifier`.\n\n")
			b.WriteString(entStr)
			b.WriteString("\n--- END AVAILABLE ENTITIES ---\n\n")
		}
	}

	if apisRaw, ok := params["apis"]; ok {
		apisStr := stringifyParam(apisRaw)
		if strings.TrimSpace(apisStr) != "" && strings.TrimSpace(apisStr) != "[]" {
			b.WriteString("--- AVAILABLE APIs ---\n")
			b.WriteString("Use these api names for the `api_name` property of components that fetch their own data (page_ref, select-eru, grid, charts). Do not invent api names.\n\n")
			b.WriteString(apisStr)
			b.WriteString("\n--- END AVAILABLE APIs ---\n\n")
		}
	}

	return b.String()
}

func stringifyParam(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case nil:
		return ""
	default:
		bytes, err := json.Marshal(val)
		if err != nil {
			return ""
		}
		return string(bytes)
	}
}

// The value sets below are read from the generated eru-studio component catalog
// rather than typed out here, so a component, action or enum added in the
// Angular library reaches this agent by running
// agents/eru_studio/catalog/sync.sh - never by someone remembering to edit a Go
// slice. See agents/eru_studio/catalog.
var (
	allowedComponentTypes  = enumOf(studioCatalog.ComponentTypes())
	allowedEventActions    = enumOf(studioCatalog.EventActions())
	allowedStateScopes     = enumOf(studioCatalog.InterfaceEnumStrings("ComponentEventSubscription", "state_scope"))
	allowedRecordSources   = enumOf(studioCatalog.InterfaceEnumStrings("ComponentEventSubscription", "record_source"))
	allowedPageDataSources = enumOf(studioCatalog.InterfaceEnumStrings("EruPage", "data_source"))
	allowedValidationTypes = enumOf(studioCatalog.InterfaceEnumStrings("ValidationRule", "type"))
	allowedDisplayModes    = enumOf(studioCatalog.InterfaceEnumStrings("EruPage", "display_mode"))
	allowedStateFormulaFns = enumOf(studioCatalog.InterfaceEnumStrings("UpdateStateFormula", "fn"))
	allowedBreakpoints     = studioCatalog.Breakpoints()
	// "none" is not in the renderer's nesting_type union but existing pages carry
	// it as the explicit "not nested" marker, so it stays accepted.
	allowedNestingTypes = enumOf(append([]string{"none"}, studioCatalog.InterfaceEnumStrings("EruComponent", "nesting_type")...))
)

var studioCatalog = catalog.Get()

func enumOf(values []string) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

// mergeCatalogFields fills a hand-written schema out with every field the
// renderer interface declares. The hand-written entry always wins: it carries
// the guidance the model needs, while the catalog only guarantees the field
// exists and what values it takes.
func mergeCatalogFields(interfaceName string, handwritten map[string]eru_models.JSONSchema) map[string]eru_models.JSONSchema {
	merged := make(map[string]eru_models.JSONSchema, len(handwritten))
	for key, schema := range handwritten {
		merged[key] = schema
	}
	for _, field := range studioCatalog.InterfaceFields(interfaceName) {
		if _, ok := merged[field.Name]; ok {
			continue
		}
		merged[field.Name] = schemaForInterfaceField(field)
	}
	return merged
}

// schemaForInterfaceField turns a TypeScript field declaration into the JSON
// schema fragment the model is held to.
func schemaForInterfaceField(field catalog.InterfaceField) eru_models.JSONSchema {
	schema := eru_models.JSONSchema{Description: field.Description}
	if len(field.Values) > 0 {
		schema.Type = "string"
		schema.Enum = field.Values
		return schema
	}
	switch declared := strings.TrimSpace(field.Type); {
	case declared == "string":
		schema.Type = "string"
	case declared == "boolean":
		schema.Type = "boolean"
	case declared == "number":
		schema.Type = "number"
	case declared == "string[]":
		schema.Type = "array"
		schema.Items = &eru_models.JSONSchema{Type: "string"}
	case strings.HasSuffix(declared, "[]"):
		schema.Type = "array"
		schema.Items = &eru_models.JSONSchema{Type: "object", AdditionalProperties: true}
	case declared == "any":
		// A deliberately untyped value - anything the action needs.
	default:
		schema.Type = "object"
		schema.AdditionalProperties = true
	}
	return schema
}

func buildEruPageOutputSchema() eru_models.JSONSchema {
	breakpointObject := eru_models.JSONSchema{
		Type:                 "object",
		AdditionalProperties: true,
		Description:          "Free-form key/value pairs from the component property catalog. Keys vary per component type.",
	}

	responsiveProps := eru_models.JSONSchema{
		Type:        "object",
		Description: "Responsive component properties. Put defaults under `base`. Add sm/md/lg/xl/2xl ONLY when the user requests responsive behaviour.",
		Properties: map[string]eru_models.JSONSchema{
			"base": breakpointObject,
			"sm":   breakpointObject,
			"md":   breakpointObject,
			"lg":   breakpointObject,
			"xl":   breakpointObject,
			"2xl":  breakpointObject,
		},
		AdditionalProperties: false,
	}

	responsiveStyles := eru_models.JSONSchema{
		Type:        "object",
		Description: "Responsive style values. Put defaults under `base`.",
		Properties: map[string]eru_models.JSONSchema{
			"base": breakpointObject,
			"sm":   breakpointObject,
			"md":   breakpointObject,
			"lg":   breakpointObject,
			"xl":   breakpointObject,
			"2xl":  breakpointObject,
		},
		AdditionalProperties: false,
	}

	responsiveClasses := eru_models.JSONSchema{
		Type:        "object",
		Description: "Tailwind utility classes per breakpoint.",
		Properties: map[string]eru_models.JSONSchema{
			"base": {Type: "string"},
			"sm":   {Type: "string"},
			"md":   {Type: "string"},
			"lg":   {Type: "string"},
			"xl":   {Type: "string"},
			"2xl":  {Type: "string"},
		},
		AdditionalProperties: false,
	}

	stylesSchema := eru_models.JSONSchema{
		Type: "object",
		Properties: map[string]eru_models.JSONSchema{
			"classes":            {Type: "string", Description: "Always-applied Tailwind classes (use empty string when none)."},
			"responsive_classes": responsiveClasses,
			"responsive_styles":  responsiveStyles,
			"custom":             {Type: "object", AdditionalProperties: true, Description: "Free-form CSS map (camelCase keys)."},
		},
		Required:             []string{"classes", "responsive_classes", "responsive_styles", "custom"},
		AdditionalProperties: false,
	}

	navParamSchema := eru_models.JSONSchema{
		Type:        "object",
		Description: "One query param carried to the target page by `navigate-to-page`. `param` should match a state variable declared on the target page.",
		Properties: map[string]eru_models.JSONSchema{
			"param":            {Type: "string"},
			"value":            {},
			"value_expression": {Type: "string", Description: "Evaluated by the logic evaluator, e.g. \"@state.selected_company\"."},
			"encode":           {Type: "string", Enum: []any{"json"}, Description: "Set only for a small structured value; larger payloads belong in app state."},
		},
		Required:             []string{"param"},
		AdditionalProperties: false,
	}

	eventSchema := eru_models.JSONSchema{
		Type:        "object",
		Description: "ComponentEventSubscription wiring an emitted event to a runtime action.",
		Properties: map[string]eru_models.JSONSchema{
			"id":                     {Type: "string"},
			"event":                  {Type: "string", Description: "DOM/component/page event (click, valueChange, focus, blur, mouseenter, buttonpress, on_load, on_api_success, on_api_error, row_select, on_upload, on_complete, timeout, timer_start, ...)."},
			"action":                 {Type: "string", Enum: allowedEventActions},
			"apiName":                {Type: "string", Description: "Legacy field: the renderer no longer reads it and there is no call-api action. Fetch through a component's own `api_name` property instead."},
			"function_name":          {Type: "string", Description: "REQUIRED when action is `call-function`."},
			"query_name":             {Type: "string", Description: "REQUIRED when action is `call-query`."},
			"api_payload_fields":     {Type: "array", Items: &eru_models.JSONSchema{Type: "string"}, Description: "State vars / page-data fields sent as the payload of call-function / call-query."},
			"fieldNames":             {Type: "array", Items: &eru_models.JSONSchema{Type: "string"}, Description: "Target field/component ids. Use [page_ref_id] for *-side-panel and refresh-page-ref, [timer_id] for *-timer actions, [grid_id] for refresh-grid, [component_id] for hide/show/enable/disable-component and start/stop-loading."},
			"page_id":                {Type: "string", Description: "Target page id for `navigate-to-page`."},
			"nav_params":             {Type: "array", Items: &navParamSchema, Description: "Extra query params written alongside the page id by `navigate-to-page`."},
			"payload":                {Type: "object", AdditionalProperties: true},
			"state_key":              {Type: "string", Description: "State variable for `update-state`; signal name for `emit-to-parent`; URL query param name (default \"view\") for `navigate-to-page`."},
			"state_scope":            {Type: "string", Enum: allowedStateScopes, Description: "Scope for `update-state`. \"page\" (default) writes this page's state; \"app\" writes the window-wide app state readable anywhere as @app.<key>."},
			"state_formula":          {Type: "object", AdditionalProperties: true, Description: "UpdateStateFormula { fn: set|increment|decrement|toggle|set-from-field|set-from-payload|reset|expr, value?, by?, values?, field?, expr?, payload_path? }"},
			"property_key":           {Type: "string", Description: "REQUIRED when action is `update-property` — the property overridden on the target component."},
			"value":                  {},
			"value_expression":       {Type: "string", Description: "Optional expression for `set-field` / `update-property`; takes precedence over the static `value`."},
			"on_success":             {Type: "array", Items: &eru_models.JSONSchema{Type: "object", AdditionalProperties: true}},
			"on_error":               {Type: "array", Items: &eru_models.JSONSchema{Type: "object", AdditionalProperties: true}},
			"error_field":            {Type: "string"},
			"error_state_key":        {Type: "string"},
			"validate_before_action": {Type: "boolean"},
			"validate_field_names":   {Type: "array", Items: &eru_models.JSONSchema{Type: "string"}},
		},
		Required:             []string{"id", "event", "action"},
		AdditionalProperties: false,
	}
	// Fields the renderer accepts but nobody has written a description for yet
	// still have to be legal, or the model cannot use a feature the library
	// already ships (a grid row action, a drill column, a set-page-data source).
	eventSchema.Properties = mergeCatalogFields("ComponentEventSubscription", eventSchema.Properties)

	validationRuleSchema := eru_models.JSONSchema{
		Type: "object",
		Properties: map[string]eru_models.JSONSchema{
			"type":    {Type: "string", Enum: allowedValidationTypes},
			"value":   {},
			"message": {Type: "string"},
		},
		Required:             []string{"type", "message"},
		AdditionalProperties: false,
	}

	// Recursive children: described as objects with the same shape; loose
	// because Go's JSON encoder cannot represent a self-referential schema
	// without infinite recursion. The root EruComponent schema below is
	// strict; the prompt enforces the same shape on children.
	componentChildSchema := eru_models.JSONSchema{
		Type: "object",
		Description: "A nested EruComponent. MUST have the SAME shape as a top-level component:\n" +
			"  { id, type (one of the allowed types), properties.{base[, sm, md, lg, xl, 2xl]}, " +
			"styles.{classes, responsive_classes, responsive_styles, custom}, events?, validation_rules?, children? }.\n" +
			"Do NOT invent extra top-level keys on a child (no `style`, `left`, `right`, `header`, `sections`, etc.).",
		AdditionalProperties: true,
	}

	componentSchema := eru_models.JSONSchema{
		Type:        "object",
		Description: "An EruComponent.",
		Properties: map[string]eru_models.JSONSchema{
			"id":               {Type: "string", Description: "Unique component id (slug + short suffix)."},
			"type":             {Type: "string", Enum: allowedComponentTypes, Description: "Component type. MUST be one of the listed values; never invent a new type."},
			"isNested":         {Type: "boolean"},
			"nesting_type":     {Type: "string", Enum: allowedNestingTypes},
			"pageId":           {Type: "string"},
			"index":            {Type: "integer"},
			"entityName":       {Type: "string"},
			"properties":       responsiveProps,
			"styles":           stylesSchema,
			"events":           {Type: "array", Items: &eventSchema},
			"validation_rules": {Type: "array", Items: &validationRuleSchema},
			"children":         {Type: "array", Items: &componentChildSchema, Description: "Only allowed for container types (flex_container, grid_container, card, expansion_panel, stepper, sidebar_stepper, sidenav, toolbar, tabs). page_ref and widget MUST NOT use children — they embed another page/widget by id."},
			"parent_id":        {Type: "string"},
			"created_at":       {Type: "string"},
			"updated_at":       {Type: "string"},
		},
		Required:             []string{"id", "type", "properties", "styles"},
		AdditionalProperties: false,
	}

	pageStateVariableSchema := eru_models.JSONSchema{
		Type: "object",
		Properties: map[string]eru_models.JSONSchema{
			"key":     {Type: "string"},
			"initial": {},
			"formula": {Type: "object", AdditionalProperties: true, Description: "{ fn: count|sum|avg|min|max|expr, source?: pageDataArray, field?, filter?, value? }"},
		},
		Required:             []string{"key"},
		AdditionalProperties: false,
	}

	pageStylesSchema := eru_models.JSONSchema{
		Type: "object",
		Properties: map[string]eru_models.JSONSchema{
			"classes":            {Type: "string"},
			"responsive_classes": responsiveClasses,
			"responsive_styles":  responsiveStyles,
			"custom":             {Type: "object", AdditionalProperties: true},
		},
		Required:             []string{"classes", "responsive_classes", "responsive_styles", "custom"},
		AdditionalProperties: false,
	}

	masterDetailSchema := eru_models.JSONSchema{
		Type: "object",
		Properties: map[string]eru_models.JSONSchema{
			"master_field":      {Type: "string"},
			"detail_field":      {Type: "string"},
			"relationship_type": {Type: "string", Enum: []any{"one-to-many", "many-to-one", "one-to-one"}},
			"auto_open":         {Type: "boolean"},
			"display_mode":      {Type: "string", Enum: []any{"popup", "side_panel"}},
		},
		AdditionalProperties: false,
	}

	pageSchema := eru_models.JSONSchema{
		Type: "object",
		Description: "EruPage root object. Mirrors the EruPage TypeScript interface in eru-studio/src/lib/models/eru-project.model.ts.\n" +
			"Allowed top-level keys: id, name, title, entity_name, route, components, styles, state, display_mode, parent_page_id, master_detail_config, events, data_source, state_scope, state_field, state_result_path.\n" +
			"DO NOT add invented keys such as `theme`, `layout`, `slug`, `version`, `description`, `topbar`, `sidebar`, `colorScheme`.",
		Properties: map[string]eru_models.JSONSchema{
			"id":                   {Type: "string", Description: "Stable page id. Reuse the value provided in the user message verbatim."},
			"name":                 {Type: "string", Description: "snake_case page name."},
			"title":                {Type: "string"},
			"entity_name":          {Type: "string"},
			"route":                {Type: "string"},
			"components":           {Type: "array", Items: &componentSchema, Description: "Top-level components (typically a single root container holding everything else)."},
			"styles":               pageStylesSchema,
			"state":                {Type: "array", Items: &pageStateVariableSchema},
			"display_mode":         {Type: "string", Enum: allowedDisplayModes},
			"parent_page_id":       {Type: "string"},
			"master_detail_config": masterDetailSchema,
			"events":               {Type: "array", Items: &eventSchema, Description: "Page-level event subscriptions (currently `on_load`, fired by a parent page_ref once the nested page's data has arrived)."},
			"data_source":          {Type: "string", Enum: allowedPageDataSources, Description: "Seed this page's data from a state variable when it is opened on its own (navigate-to-page, deep link, viewer). Ignored while the page is mounted inside a page_ref. Default \"none\"."},
			"state_scope":          {Type: "string", Enum: allowedStateScopes, Description: "Which store `state_field` is read from. Default \"page\"."},
			"state_field":          {Type: "string", Description: "State variable holding the record for this page. Only meaningful when data_source is \"state\"."},
			"state_result_path":    {Type: "string", Description: "Optional dotted path into the state value before it is used, e.g. \"program_data.charges\"."},
		},
		Required:             []string{"id", "name", "components", "styles"},
		AdditionalProperties: false,
	}

	return pageSchema
}
