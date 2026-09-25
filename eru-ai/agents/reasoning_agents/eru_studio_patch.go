package reasoning_agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	studio "github.com/eru-tech/eru/eru-ai/agents/eru_studio"
	catalog "github.com/eru-tech/eru/eru-ai/agents/eru_studio/catalog"
	utility "github.com/eru-tech/eru/eru-ai/tools/utility"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
)

// buildEruPageUpdateOutputSchema is the structured output the agent produces
// when the consumer opted into the patch protocol: either a full page, or the
// components that changed. The mode field is what tells the consumer - and this
// code - which of the two arrived.
func buildEruPageUpdateOutputSchema() eru_models.JSONSchema {
	pageSchema := buildEruPageOutputSchema()

	patchComponentSchema := patchComponentFrom(pageSchema)
	stateSchema := pageSchema.Properties["state"]

	pagePropsSchema := eru_models.JSONSchema{
		Type: "object",
		Description: "Changed EruPage root keys only (title, route, entity_name, display_mode, data_source, ...). " +
			"Never send components or state here - they have their own sections.",
		Properties:           patchablePageProps(pageSchema),
		AdditionalProperties: false,
	}

	statePatchSchema := eru_models.JSONSchema{
		Type:        "object",
		Description: "Changes to EruPage.state. Omit entirely when the page's state variables are unchanged.",
		Properties: map[string]eru_models.JSONSchema{
			"upsert": {
				Type:        "array",
				Items:       stateSchema.Items,
				Description: "State variables to add or update, matched by \"key\".",
			},
			"delete": {
				Type:        "array",
				Items:       &eru_models.JSONSchema{Type: "string"},
				Description: "Keys of state variables to remove.",
			},
		},
		AdditionalProperties: false,
	}

	patchSchema := eru_models.JSONSchema{
		Type: "object",
		Description: "The components that changed, and nothing else. Every component you send is applied by id " +
			"onto the page the client already holds.",
		Properties: map[string]eru_models.JSONSchema{
			"page_id":    {Type: "string", Description: "The id of the page being edited. Must match the page id you were given."},
			"page_props": pagePropsSchema,
			"upsert": {
				Type:  "array",
				Items: &patchComponentSchema,
				Description: "A FLAT list of components - never nested. A component whose id already exists is merged " +
					"into (send only the keys that changed); a component whose id is new is created. " +
					"Parenthood comes from the parent's \"children_ids\", or from \"parent_id\" plus \"index\" on the child.",
			},
			"delete": {
				Type:        "array",
				Items:       &eru_models.JSONSchema{Type: "string"},
				Description: "Ids of components to remove. Removing a container removes everything under it, so do not also list its children.",
			},
			"state": statePatchSchema,
		},
		AdditionalProperties: false,
	}

	return eru_models.JSONSchema{
		Type: "object",
		Description: "An Eru Studio page update. Answer with a patch whenever you were given an existing page: " +
			"send the components that changed, not the whole page. Send a full page only when there was no page to " +
			"start from, or when the new prompt replaces the entire layout.",
		Properties: map[string]eru_models.JSONSchema{
			"mode": {
				Type:        "string",
				Enum:        []any{studio.ModePatch, studio.ModeFull},
				Description: "\"patch\" when you are sending changed components, \"full\" when you are sending a whole page.",
			},
			"patch": patchSchema,
			"page":  pageSchema,
			"pages": buildNestedPagesSchema(pageSchema),
			"summary": {
				Type:        "string",
				Description: "One short sentence naming what changed, for the client's activity log. No markdown.",
			},
		},
		Required:             []string{"mode"},
		AdditionalProperties: false,
	}
}

// buildNestedPagesSchema is how the model emits the other pages an edit needs: a
// row template, a side panel, a board card. They are separate pages because the
// renderer mounts them by id and the store saves them separately - so they
// cannot be expressed as part of the page that mounts them.
func buildNestedPagesSchema(pageSchema eru_models.JSONSchema) eru_models.JSONSchema {
	entry := eru_models.JSONSchema{
		Type: "object",
		Description: "One page mounted by a component on another page. Always complete - a nested page is small, " +
			"and the user saves it as a whole.",
		Properties: map[string]eru_models.JSONSchema{
			"page": pageSchema,
			"purpose": {
				Type: "string",
				Enum: []any{
					studio.PurposeLoopTemplate, studio.PurposeSidePanel, studio.PurposePopup,
					studio.PurposeInline, studio.PurposeBoardCard, studio.PurposeTileCard,
				},
				Description: "Why this page exists as a page of its own.",
			},
			"mounted_at": {
				Type:        "string",
				Description: "The id of the component that mounts it - the page_ref, grid or tile. It must be a component on the page you are editing.",
			},
			"mount_property": {
				Type:        "string",
				Description: "The property on that component holding this page's id: \"page\" for a page_ref, \"card_page_id\" for a grid or tile. You must ALSO set that property to this page's id.",
			},
			"is_new": {
				Type:        "boolean",
				Description: "true when you are creating this page, false when you are changing one that already exists.",
			},
		},
		Required:             []string{"page", "purpose", "mounted_at", "mount_property"},
		AdditionalProperties: false,
	}
	return eru_models.JSONSchema{
		Type:  "array",
		Items: &entry,
		Description: "The other pages this edit needs. Emit one entry per page you create or change besides the page " +
			"you were given. Leave the array out when the edit needs no nested page.",
	}
}

// patchComponentFrom derives the component shape a patch carries: the same
// EruComponent, plus the flat child references that let the renderer draw a
// component before its children have been generated.
func patchComponentFrom(pageSchema eru_models.JSONSchema) eru_models.JSONSchema {
	components, ok := pageSchema.Properties["components"]
	if !ok || components.Items == nil {
		return eru_models.JSONSchema{Type: "object", AdditionalProperties: true}
	}
	componentSchema := *components.Items

	properties := make(map[string]eru_models.JSONSchema, len(componentSchema.Properties)+1)
	for key, value := range componentSchema.Properties {
		properties[key] = value
	}
	properties["children_ids"] = eru_models.JSONSchema{
		Type:  "array",
		Items: &eru_models.JSONSchema{Type: "string"},
		Description: "Ids of this container's children, in order. This REPLACES the container's current child list. " +
			"Use it instead of \"children\" so each component can be sent - and rendered - on its own.",
	}
	if children, ok := properties["children"]; ok {
		children.Description = "Avoid in a patch: prefer children_ids plus one flat entry per child. " +
			"A nested subtree here is accepted but cannot be rendered progressively."
		properties["children"] = children
	}
	componentSchema.Properties = properties
	componentSchema.Description = "One component of the patch. Send the whole component when it is new; " +
		"when it already exists send only its id, its type and the keys that changed."
	// A patch entry is a partial component, so properties/styles are no longer
	// required - the base page already has them.
	componentSchema.Required = []string{"id", "type"}
	return componentSchema
}

// patchablePageProps is every EruPage root key except the two that have their
// own patch section.
func patchablePageProps(pageSchema eru_models.JSONSchema) map[string]eru_models.JSONSchema {
	out := make(map[string]eru_models.JSONSchema, len(pageSchema.Properties))
	for key, value := range pageSchema.Properties {
		if key == "components" || key == "state" {
			continue
		}
		out[key] = value
	}
	return out
}

// patchModeInstructions is added to the request, not to the system prompt: the
// mode is negotiated per call, so the rules for it belong with the call.
const patchModeInstructions = `--- OUTPUT MODE: PATCH ---
This client applies component-level updates, so answer with a patch rather than a whole page.

- Set "mode" to "patch" and fill "patch". Put in "patch.upsert" ONLY the components the prompt
  actually changes, plus any component you create. Leave every untouched component out entirely -
  the client still has it, and sending it again risks changing it by accident.
- "patch.upsert" is FLAT. Never nest a component inside another's "children". A container names its
  children with "children_ids" (which replaces its whole child list, so include the ones that stay),
  and a single new child can instead carry "parent_id" plus "index".
- For a component that already exists, send its "id", its "type", and only the keys that changed.
  Property bags merge per breakpoint, so {"properties":{"base":{"color":"warn"}}} changes the colour
  and leaves every other property alone.
- To remove something, list its id in "patch.delete". Deleting a container deletes its subtree, so do
  not list its children as well.
- Emit components parent-first where you can. The client renders each one as it arrives, so a parent
  that arrives before its children shows the layout sooner.
- Set "mode" to "full" and fill "page" instead ONLY when there is no existing page, or when the prompt
  genuinely replaces the whole layout. A full page is always correct; it is just slower and coarser.
- "patch.page_id" must be the page id you were given.

NESTED PAGES
- When the design needs another page - a row template, a side panel, a popup, a board card -
  put that page in "pages", complete, with its own id and name. Set "purpose", set "mounted_at"
  to the component that mounts it, and set "mount_property" to the property that holds its id.
- ALSO set that property on the mounting component itself, in the same answer: a page_ref whose
  "page" property is blank renders an empty panel no matter what "pages" contains.
- Emit a page in "pages" both when you create it and when you change an existing one. Each page
  comes back to the client separately, and the user accepts each one, so a page you did not touch
  must not appear there.
- Do not put the mounted page's components under the host as "children". The host mounts a page;
  it has no children of its own.
`

// fullModeInstructions is the note for a client that can only take one page
// back. It has to be told, because the alternative is a page_ref pointing at a
// page that was never sent - an empty panel that looks like the agent did
// nothing.
var fullModeInstructions = `--- OUTPUT MODE: SINGLE PAGE ---
This answer can carry exactly ONE page, so it cannot carry a nested page.
- Design what the request needs on this page alone, using containers rather than mounts.
- Do NOT introduce a page_ref, or set a grid's or tile's card_page_id, unless it points at a page
  that ALREADY exists: a mount pointing at a page you did not send renders as an empty panel.
- If the request genuinely needs a page of its own - anything repeated per record, a real side
  panel, a popup, a board card - do not fake it and do not quietly downgrade it. Ask with the
  clarification tool, and make BUILDING IT PROPERLY the first option, with this exact value:

      {"value": "` + studio.OptionEnableMultiPage + `",
       "label": "Build it as multiple pages (re-runs this request with multi-page output)"}

  That option is not something you carry out in this answer: it tells the client to re-issue the
  request with multi-page output, which is the only way a nested page can reach it. It is the
  option that gives the user what they asked for, so it goes first and is the recommendation.
- Any single-page compromise you can offer goes AFTER it, described as the compromise it is.
  Never present a reduced version as if it were the whole thing, and never write an option whose
  only content is that you cannot do it.
`

// eruStudioModeInstructions is the per-request note describing the negotiated
// output mode.
func eruStudioModeInstructions(mode string) string {
	switch mode {
	case studio.ModePatch:
		return patchModeInstructions
	case studio.ModeAuto:
		return autoModeInstructions
	default:
		return fullModeInstructions
	}
}

// answerDropsPage catches an answer that deletes most of the user's page.
//
// Whatever the mode, what the client ends up rendering is one page, and an
// answer that silently leaves most of it out does not fail - it deletes. That is
// what happened on a request to ADD one tile: the answer came back as mode
// "full" carrying a single component, and applying it wiped a 24-component
// dashboard off the canvas. It happened again from the other direction: a patch
// whose children_ids named components the model had lost track of, resolving to
// a page with the grid, its title and its button gone. Every component in both
// was individually valid, so nothing else in this file had a reason to object.
//
// The comparison is always against the page the CLIENT holds, not against
// whatever intermediate page a repair is diffing from - the client's page is
// what the answer replaces, so it is the only thing worth protecting.
//
// A genuine "replace the whole layout" still rebuilds a page of comparable size,
// so the test is proportional rather than exact: keep most of what you were
// given, or say what you are deleting.
func answerDropsPage(clientPage map[string]interface{}, page map[string]interface{}, mode string) *catalog.Issue {
	baseIds := studio.ComponentIds(clientPage)
	if len(baseIds) < minComponentsToGuard {
		return nil
	}
	kept := map[string]bool{}
	for _, id := range studio.ComponentIds(page) {
		kept[id] = true
	}
	dropped := make([]string, 0, len(baseIds))
	for _, id := range baseIds {
		if !kept[id] {
			dropped = append(dropped, id)
		}
	}
	if len(dropped)*2 <= len(baseIds) {
		return nil
	}
	named := dropped
	if len(named) > maxNamedDroppedComponents {
		named = named[:maxNamedDroppedComponents]
	}
	advice := "A full answer must carry the COMPLETE page: everything you were given, plus your change. " +
		"If you only meant to change part of it, answer with mode \"patch\" instead - that is the normal case for an edit."
	if mode == studio.ModePatch {
		advice = "Applying this patch to the page you were given produces that result - usually because children_ids " +
			"left components out, or a parent was replaced without its children. List in children_ids every child the " +
			"parent keeps, and delete only what you meant to delete."
	}
	return &catalog.Issue{
		Path: "page",
		Code: catalog.CodeEnvelopeFullDropsPage,
		Message: fmt.Sprint(
			"this answer drops ", len(dropped), " of the ", len(baseIds),
			" components on the page you were given - including ", strings.Join(named, ", "), ". ", advice,
		),
	}
}

// minComponentsToGuard keeps the check off pages small enough that a rewrite is
// plausibly the whole job.
const minComponentsToGuard = 4

// maxNamedDroppedComponents is how many ids go in the message: enough for the
// model to recognise what it lost, not the whole page again.
const maxNamedDroppedComponents = 8

// autoModeInstructions is the note for a client that reads the envelope and left
// the choice to the agent. It has to say both halves out loud, because the two
// are easy to conflate: whether this answer is a patch or a whole page, and
// whether it may carry more than one page. The second is always yes here.
const autoModeInstructions = `--- OUTPUT MODE: AUTO ---
This client reads the page-update envelope, so you choose the shape of this answer.

- You were given an existing page: set "mode" to "patch" and send only what changed, following the
  patch rules. That is the normal case for an edit.
- You were NOT given a page, or the prompt replaces the whole layout: set "mode" to "full" and put
  the complete EruPage in "page". "full" REPLACES what the user has: every component you leave out
  is deleted from their page. So a full answer repeats the entire page you were given - all of it,
  not just the part you touched - plus your change. If repeating it is not what you meant, the
  answer is "patch".
- EITHER WAY you may return nested pages in "pages". Building a page from scratch does not restrict
  you to one page: a repeatable row still needs its template page, a side panel is still its own
  page, a board card is still its own page. Create them, give each a fresh id, mount each from the
  page you are building, and list them in "pages" with their purpose and mount point.
- So never tell the user that a repeatable section, a side panel or a board card cannot be built
  here. It can. If the design needs a nested page, emit it.
`

// parseNestedPages reads the "pages" the model emitted alongside the root page.
func parseNestedPages(output map[string]interface{}, rootPageId string) ([]*studio.NestedPage, error) {
	raw, ok := output["pages"]
	if !ok || raw == nil {
		return nil, nil
	}
	list, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("\"pages\" must be an array of nested pages, got %T", raw)
	}

	pages := make([]*studio.NestedPage, 0, len(list))
	for i, item := range list {
		entry, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("pages[%d] must be an object", i)
		}
		page, ok := entry["page"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("pages[%d] has no \"page\" object", i)
		}
		pageId, _ := page["id"].(string)
		if strings.TrimSpace(pageId) == "" {
			return nil, fmt.Errorf("pages[%d].page has no \"id\" - a nested page is mounted by id, so it needs one", i)
		}
		name, _ := page["name"].(string)
		purpose, _ := entry["purpose"].(string)
		mountedAt, _ := entry["mounted_at"].(string)
		mountProperty, _ := entry["mount_property"].(string)
		isNew, _ := entry["is_new"].(bool)

		if _, set := page["parent_page_id"]; !set && rootPageId != "" {
			page["parent_page_id"] = rootPageId
		}

		pages = append(pages, &studio.NestedPage{
			PageId:        pageId,
			Name:          name,
			Purpose:       purpose,
			ParentPageId:  rootPageId,
			MountedAt:     mountedAt,
			MountProperty: mountProperty,
			IsNew:         isNew,
			Persist:       studio.PersistUserAction,
			Revision:      studio.Revision(page),
			Page:          page,
		})
	}
	return pages, nil
}

// resolveStudioOutput turns the model's structured output into the responses the
// consumer receives: one envelope for the page that was asked for, and one for
// each nested page the edit needed. In patch mode the root patch is applied to
// the page the client sent, so the root envelope carries both the patch and the
// page it produces.
func resolveStudioOutput(ctx context.Context, output map[string]interface{}, basePage map[string]interface{}) (map[string]interface{}, []map[string]interface{}, error) {
	// base_revision names the page the CLIENT holds. Normally that is the page
	// the answer is a diff against; during a repair the answer is a diff against
	// an attempt the client never saw, so the client's own page is what it has to
	// identify.
	baseRevision := studio.Revision(basePage)
	if state := studio.RepairStateFrom(ctx); state.Active() {
		baseRevision = studio.Revision(studio.BasePageFrom(ctx))
	}
	envelope := studio.Envelope{
		ProtocolVersion: studio.ProtocolVersion,
		Role:            studio.RoleRoot,
		Persist:         studio.PersistUserAction,
		BaseRevision:    baseRevision,
		Scope:           studio.ReportScope(studio.ScopeFrom(ctx)),
	}

	mode, _ := output["mode"].(string)
	switch mode {
	case studio.ModePatch:
		patch, err := studio.ParsePatch(output)
		if err != nil {
			return nil, nil, fmt.Errorf("mode was \"patch\" but the patch could not be read: %w", err)
		}
		if patch.IsEmpty() && output["pages"] == nil {
			return nil, nil, fmt.Errorf("mode was \"patch\" but the patch changes nothing - either patch something or answer with mode \"full\"")
		}
		resolved, warnings, err := studio.ApplyPatch(basePage, patch)
		if err != nil {
			return nil, nil, err
		}
		envelope.Mode = studio.ModePatch
		envelope.Patch = patch
		envelope.Page = resolved
		envelope.Warnings = warnings
		if len(warnings) > 0 {
			logs.WithContext(ctx).Info(fmt.Sprintf("eru studio patch applied with %d warning(s): %s", len(warnings), strings.Join(warnings, "; ")))
		}
	case studio.ModeFull:
		page, ok := output["page"].(map[string]interface{})
		if !ok {
			return nil, nil, fmt.Errorf("mode was \"full\" but \"page\" is missing or is not an object")
		}
		envelope.Mode = studio.ModeFull
		envelope.Page = page
	default:
		return nil, nil, fmt.Errorf("\"mode\" must be %q or %q, got %q", studio.ModePatch, studio.ModeFull, mode)
	}

	// The root page's identity comes from the request, not from the answer: a
	// patch that renamed the page would resolve to a page the client cannot match
	// to the one the user is editing.
	if replaced, changed := studio.PageIdentityFrom(ctx).Stamp(envelope.Page); changed && replaced != "" {
		logs.WithContext(ctx).Info(fmt.Sprintf("eru studio kept the page id: answered with %q, restored %q",
			replaced, envelope.Page["id"]))
	}
	if pageId, ok := envelope.Page["id"].(string); ok {
		envelope.PageId = pageId
	}

	// The model may have inlined a nested page under the component that mounts
	// it, which is how the client renders one. Split those out so every page is
	// its own unit of persistence, whichever way the model chose to express it.
	rootOnly, inlined := studio.SplitInlinedPages(envelope.Page)
	declared, err := parseNestedPages(output, envelope.PageId)
	if err != nil {
		return nil, nil, err
	}
	// A repair answers only what was wrong, so the pages the rejected attempt got
	// right are not re-sent. They are still part of the answer. Anything the
	// repair did re-send wins, because MergeNestedPages keeps the first of an id.
	if state := studio.RepairStateFrom(ctx); state.Active() {
		declared = append(declared, state.CarriedPages()...)
	}
	nested := studio.MergeNestedPages(declared, inlined, envelope.PageId)
	if len(nested) > 0 {
		// The root page keeps the mount, not the mounted components.
		envelope.Page = rootOnly
	}

	if state := studio.RepairStateFrom(ctx); state.Active() && envelope.Mode == studio.ModePatch {
		// The patch was computed against the attempt the client never saw, so
		// sending it would have the client apply a diff to the wrong page.
		logs.WithContext(ctx).Info("eru studio: a repair patch is returned as a full page, since the client never held the page it patched")
		envelope.Mode = studio.ModeFull
		envelope.Patch = nil
	}

	envelope.Revision = studio.Revision(envelope.Page)
	envelope.Pages = studio.Manifest(nested)
	// Anything the request itself needed answering for - a revision claim that no
	// longer described the page - travels with the answer rather than replacing
	// it.
	envelope.Warnings = append(studio.NoticesFrom(ctx).All(), envelope.Warnings...)

	if studio.InlineNestedEnabled(ctx) && len(nested) > 0 {
		// A rendering convenience: the client can draw the whole thing without
		// resolving mounts. The nested pages still travel separately, and those
		// are what get saved.
		envelope.Page = studio.InlineNestedPages(envelope.Page, nested)
	}

	root, err := encodeEnvelope(envelope, output)
	if err != nil {
		return nil, nil, err
	}

	nestedEnvelopes := make([]map[string]interface{}, 0, len(nested))
	for _, page := range nested {
		encoded, err := encodeEnvelope(studio.Envelope{
			ProtocolVersion: studio.ProtocolVersion,
			Role:            studio.RoleNested,
			Mode:            studio.ModeFull,
			Persist:         studio.PersistUserAction,
			PageId:          page.PageId,
			Revision:        page.Revision,
			Page:            page.Page,
			ParentPageId:    page.ParentPageId,
			MountedAt:       page.MountedAt,
			MountProperty:   page.MountProperty,
			Purpose:         page.Purpose,
			IsNew:           page.IsNew,
			Warnings:        page.Warnings,
		}, nil)
		if err != nil {
			return nil, nil, err
		}
		nestedEnvelopes = append(nestedEnvelopes, encoded)
	}

	return root, nestedEnvelopes, nil
}

func encodeEnvelope(envelope studio.Envelope, output map[string]interface{}) (map[string]interface{}, error) {
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(encoded, &out); err != nil {
		return nil, err
	}
	if output != nil {
		if summary, ok := output["summary"].(string); ok && strings.TrimSpace(summary) != "" {
			out["summary"] = summary
		}
	}
	return out, nil
}

// validateStudioUpdate checks a patch-mode output. The patch's components are
// partial by design, so they are checked as a flat list where child references
// are legal, and the resolved page is checked as a whole once the patch has been
// applied - which is what catches a patch that is individually valid but leaves
// the page in a state the renderer cannot draw.
// introducedPageIssues holds a patch to what it changed rather than to what it
// inherited.
//
// Checking the whole resolved page is what catches a patch that is individually
// fine but leaves the page broken, and that is worth keeping. What it also did
// was re-report every violation the saved page already carried - a property
// renamed years ago, a misspelled page key - none of which the patch authored or
// was asked to touch. The model cannot tell the difference, so it spent a whole
// extra generate-and-validate cycle rewriting components the user never
// mentioned, on a request that only added a few controls.
//
// An issue present in the base page and still present after the patch is the
// page's own, not this edit's. Subtracting those leaves exactly the issues the
// patch introduced - which is the thing the check was for.
// pageIssues is everything wrong with a finished page: the component library
// checks, and the layout checks that ask whether it will render as anything
// anyone would have drawn on purpose.
func pageIssues(c *catalog.Catalog, page map[string]interface{}) []catalog.Issue {
	return append(c.ValidatePage(page), studio.LayoutIssues(page)...)
}

// pageIssuesIn is pageIssues plus the checks that need to know what the user
// asked for, which only the request context carries.
func pageIssuesIn(ctx context.Context, c *catalog.Catalog, page map[string]interface{}) []catalog.Issue {
	return append(pageIssues(c, page), studio.ChromeStyleIssues(page, studio.StyleIntentFrom(ctx))...)
}

func introducedPageIssues(ctx context.Context, c *catalog.Catalog, basePage, resolved map[string]interface{}) []catalog.Issue {
	resolvedIssues := pageIssuesIn(ctx, c, resolved)
	if len(basePage) == 0 || len(resolvedIssues) == 0 {
		return resolvedIssues
	}
	preExisting := make(map[string]bool)
	for _, issue := range pageIssuesIn(ctx, c, basePage) {
		preExisting[issue.Fingerprint()] = true
	}
	out := make([]catalog.Issue, 0, len(resolvedIssues))
	for _, issue := range resolvedIssues {
		if preExisting[issue.Fingerprint()] {
			continue
		}
		out = append(out, issue)
	}
	return out
}

func validateStudioUpdate(ctx context.Context, output map[string]interface{}, basePage map[string]interface{}, scope *studio.ResolvedScope) []catalog.Issue {
	c := catalog.Get()
	issues := validateNestedPages(ctx, output, basePage)
	issues = append(issues, unreadReferenceIssues(ctx)...)

	mode, _ := output["mode"].(string)
	switch mode {
	case studio.ModeFull:
		page, ok := output["page"].(map[string]interface{})
		if !ok {
			return append(issues, catalog.Issue{Code: catalog.CodeEnvelopeFullMissingPage, Message: "mode is \"full\" but \"page\" is missing or is not an object"})
		}
		// An edit that answers with the WHOLE page is still an edit, so it is
		// judged the same way a patch is: on what it changed.
		//
		// Without this, asking for the full page back turns every pre-existing
		// wart in the user's own page into a blocking error the moment the agent
		// faithfully echoes it. A real page carries things the catalog does not
		// know - "tab_label", which the tabs component writes onto its own
		// panels; "_originPageId" on an event; a grid's events under
		// properties.base - and the agent is then told to fix eighteen problems
		// in a region the user expressly told it not to touch. It obliges, the
		// tabs lose the property that maps them to their panels, and the page
		// the user gets back is worse than the one they had.
		if gutted := answerDropsPage(studio.BasePageFrom(ctx), page, studio.ModeFull); gutted != nil {
			issues = append(issues, *gutted)
		}
		// Judged against the page the USER has, never against a rejected attempt.
		//
		// basePage is the repair base while a repair is running, which is the
		// answer that was just thrown out. Forgiving what was already wrong
		// there forgives precisely what the repair was called to fix: the
		// rejected page keeps its faults, the corrected components arrive
		// beside them under fresh ids, and the page ships with both.
		forgiven := basePage
		if studio.RepairStateFrom(ctx).Active() {
			forgiven = studio.BasePageFrom(ctx)
		}
		if len(forgiven) > 0 {
			return append(issues, introducedPageIssues(ctx, c, forgiven, page)...)
		}
		return append(issues, pageIssuesIn(ctx, c, page)...)
	case studio.ModePatch:
		patch, err := studio.ParsePatch(output)
		if err != nil {
			return append(issues, catalog.Issue{Path: "patch", Code: catalog.CodePatchUnreadable, Message: err.Error()})
		}
		if patch.IsEmpty() && output["pages"] == nil {
			return append(issues, catalog.Issue{Path: "patch", Code: catalog.CodePatchEmpty, Message: "the patch changes nothing - either patch something or answer with mode \"full\""})
		}
		upsert := make([]interface{}, 0, len(patch.Upsert))
		for _, component := range patch.Upsert {
			upsert = append(upsert, component)
		}
		issues = append(issues, c.ValidateComponents(upsert, "patch.upsert")...)
		// A scoped edit is a promise to the user that the rest of their page is
		// not in play. Enforcing it here is what makes it a promise rather than
		// a hope.
		for _, violation := range studio.ScopeViolations(patch, scope) {
			issues = append(issues, catalog.Issue{Path: "patch", Code: catalog.CodePatchScopeViolation, Message: violation})
		}

		// A child named in children_ids that nothing defines is not a typo the
		// applier can absorb: the component is dropped from the tree, so the
		// model has just deleted something it believed it was keeping.
		for _, missing := range studio.MissingChildIds(basePage, patch) {
			issues = append(issues, catalog.Issue{
				Path: "patch.upsert",
				Code: catalog.CodePatchUnknownChild,
				Message: fmt.Sprint(missing, " - so it is dropped from the page. ",
					"children_ids may only name components this patch defines or the page already has; ",
					"if you meant to keep it, include it in the patch, and if you meant to remove it, say so in \"delete\"."),
			})
		}

		resolved, _, err := studio.ApplyPatch(basePage, patch)
		if err != nil {
			return append(issues, catalog.Issue{Path: "patch", Code: catalog.CodePatchNotApplicable, Message: err.Error()})
		}
		// What the client applies is this resolved page, so the same guard that
		// protects a full answer protects a patch.
		if gutted := answerDropsPage(studio.BasePageFrom(ctx), resolved, studio.ModePatch); gutted != nil {
			issues = append(issues, *gutted)
		}
		return append(issues, introducedPageIssues(ctx, c, basePage, resolved)...)
	default:
		return append(issues, catalog.Issue{Path: "mode", Code: catalog.CodeEnvelopeUnknownMode, Message: fmt.Sprintf("must be %q or %q, got %q", studio.ModePatch, studio.ModeFull, mode)})
	}
}

// validateNestedPages checks the other pages an edit produced: that each is a
// legal page, that something mounts it, and that the mount actually points at
// it. A page_ref whose page property was never set renders an empty panel, which
// looks to the user like the agent did nothing.
func validateNestedPages(ctx context.Context, output map[string]interface{}, basePage map[string]interface{}) []catalog.Issue {
	c := catalog.Get()

	rootPageId := ""
	if page, ok := output["page"].(map[string]interface{}); ok {
		rootPageId, _ = page["id"].(string)
	} else if basePage != nil {
		rootPageId, _ = basePage["id"].(string)
	}

	nested, err := parseNestedPages(output, rootPageId)
	if err != nil {
		return []catalog.Issue{{Path: "pages", Code: catalog.CodeNestedPagesUnreadable, Message: err.Error()}}
	}
	if len(nested) == 0 {
		return nil
	}

	issues := []catalog.Issue{}
	for _, page := range nested {
		for _, issue := range pageIssuesIn(ctx, c, page.Page) {
			issue.Path = fmt.Sprintf("pages (id %q).%s", page.PageId, issue.Path)
			issue.ComponentId = page.PageId + "/" + issue.ComponentId
			issues = append(issues, issue)
		}
		if page.MountedAt == "" {
			issues = append(issues, catalog.Issue{
				Path:        fmt.Sprintf("pages (id %q)", page.PageId),
				Code:        catalog.CodeNestedPageNoMountedAt,
				ComponentId: page.PageId,
				Message:     "a nested page must say which component mounts it - set \"mounted_at\" to that component's id",
			})
		}
	}

	// Mount consistency is checked against the page the edit resolves to, since
	// a patch may be what creates the page_ref that mounts a new page.
	resolved := resolvedRootPage(output, basePage)
	known := map[string]bool{}
	if basePage != nil {
		for _, mount := range studio.FindMounts(basePage) {
			if mount.PageId != "" {
				known[mount.PageId] = true
			}
		}
	}
	return append(issues, studio.ValidateMounts(resolved, nested, known)...)
}

// resolvedRootPage is the page the edit ends up with, so mounts can be checked
// against what the client will actually render.
func resolvedRootPage(output map[string]interface{}, basePage map[string]interface{}) map[string]interface{} {
	// An EMPTY "page" is not an answer, it is a leftover key.
	//
	// Taking it at face value throws away the base and everything in it, and the
	// checks downstream then describe a page nobody wrote: every query reads as
	// unbound, every mount as missing. The model is told it deleted the whole
	// dashboard, which is both untrue and unfixable, and the attempts it spends
	// trying to rebuild are spent against a page that was never gone. If the key
	// carries nothing, fall through and let the patch decide.
	if page, ok := output["page"].(map[string]interface{}); ok && len(page) > 0 {
		return page
	}
	patch, err := studio.ParsePatch(output)
	if err != nil {
		return basePage
	}
	resolved, _, err := studio.ApplyPatch(basePage, patch)
	if err != nil {
		return basePage
	}
	return resolved
}

// unreadReferenceIssues faults an answer that imitates a page it never opened.
//
// The user points at another page - "make the grid look like the one on
// invoice_360_detail" - and the agent can read it: it lists the pages, sees the
// name, and then writes the grid from memory anyway, reporting that it matched a
// page it never fetched. Nothing about the result looks wrong, which is what
// makes it worth catching: the page is plausible, internally valid, and not what
// was asked for.
func unreadReferenceIssues(ctx context.Context) []catalog.Issue {
	ledger := studio.LedgerFrom(ctx)
	if !ledger.Enforceable(utility.GetPageToolName) {
		return nil
	}
	currentPageId := studio.PageIdentityFrom(ctx).Id
	var issues []catalog.Issue
	for _, name := range ledger.UnreadReferencedPages(currentPageId) {
		issues = append(issues, catalog.Issue{
			Path: "page",
			Code: catalog.CodeReferencePageNotRead,
			Message: fmt.Sprint(
				"this request points at the page \"", name, "\", and the page list you called shows it exists, ",
				"but you never read it with ", utility.GetPageToolName, ". Imitating a page from memory produces something ",
				"plausible that does not match it. Call ", utility.GetPageToolName, " with that page's id, read how the part ",
				"the user named is actually built, and base your answer on that.",
			),
		})
	}
	return issues
}
