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
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
)

// planningApplies decides whether this request is worth planning.
//
// Planning earns its cost when a design is being authored from nothing, which is
// where the number of pages is genuinely open and where getting it wrong costs a
// whole generation. An edit is not that: the pages already exist, the user is
// pointing at one of them, and a plan would be a model call spent restating what
// the request already says.
func planningApplies(ctx context.Context, basePage map[string]interface{}, scope *studio.ResolvedScope) bool {
	return studio.EnvelopeEnabled(ctx) && len(basePage) == 0 && scope == nil
}

// planPages asks what the design is before it is drawn.
//
// A failure here is not a failure of the request: the page can still be written
// without a plan, exactly as it was before this step existed. So every error
// path logs and returns nil rather than propagating - a planning step that can
// break page generation would be worse than no planning step.
func (eruStudioAgent *EruStudioAgent) planPages(ctx context.Context, agentMessage agents.AgentMessage, projectId, tenantId string) *studio.PagePlan {
	schema, err := jsonSchemaFrom(studio.PlanSchema())
	if err != nil {
		logs.WithContext(ctx).Error("eru studio: the plan schema is malformed: " + err.Error())
		return nil
	}

	content := studio.PlanPrompt() + "\n\n--- USER PROMPT ---\n" + agentMessage.Content
	systemPrompt := planningSystemPrompt()

	var issues []catalog.Issue
	for attempt := 0; attempt < 2; attempt++ {
		started := emitStepStart(ctx, agents.StepPlanPages, attempt+1)
		payload, _, err := eruStudioAgent.GenerateStructured(ctx, systemPrompt, content, schema, projectId, tenantId, true)
		if err != nil {
			logs.WithContext(ctx).Error(fmt.Sprintf("eru studio: planning attempt %d failed: %v", attempt+1, err))
			agents.EmitStepFinished(ctx, agents.StepPlanPages, attempt+1, agents.OutcomeError, started, err.Error(), agents.CodeModelError)
			return nil
		}
		plan, err := studio.ParsePlan(payload)
		if err != nil {
			logs.WithContext(ctx).Error("eru studio: " + err.Error())
			agents.EmitStepFinished(ctx, agents.StepPlanPages, attempt+1, agents.OutcomeError, started, err.Error(), agents.CodeOutputValidation)
			return nil
		}
		// The page the client is editing is named by the request, not by the
		// model, so the plan is held to it rather than argued with.
		if identity := studio.PageIdentityFrom(ctx); identity.Id != "" {
			plan = studio.RebaseRoot(plan, identity.Id)
		}
		issues = studio.ValidatePlan(plan)
		if len(issues) == 0 {
			agents.EmitStepFinished(ctx, agents.StepPlanPages, attempt+1, agents.OutcomeSuccess, started, "", "")
			logs.WithContext(ctx).Info(fmt.Sprintf("eru studio: planned %d page(s): %s", len(plan.Pages), planSummary(plan)))
			return &plan
		}
		agents.EmitStepFinished(ctx, agents.StepPlanPages, attempt+1, agents.OutcomeRetry, started, catalog.FormatIssues(issues, 10), agents.CodeOutputValidation)
		content = fmt.Sprintf("%s\n\nYour previous plan was rejected:\n%s\n\nAnswer again with a corrected plan.",
			content, catalog.FormatIssues(issues, 10))
	}

	logs.WithContext(ctx).Error("eru studio: no usable plan after 2 attempts, building the page without one")
	return nil
}

// planningSystemPrompt is deliberately NOT the page-building prompt.
//
// The first version of this step reused it, on the reasoning that the planner
// should know everything the builder knows. That was wrong in a way worth
// recording: the builder's prompt is ninety-odd kilobytes of "emit an EruPage",
// and appending a paragraph asking for a plan instead does not outweigh it. The
// model answered with pages, and once with the raw output of a tool it had just
// called. Decomposition only pays if each step sees what that step needs - the
// planner needs the rules that decide how many pages there are, and nothing
// about styling, events or breakpoints.
func planningSystemPrompt() string {
	return `You are the planning step of an Eru Studio page designer.

An Eru Studio design is one or more pages. A page can MOUNT another page, and some layouts are only
possible that way:

` + catalog.RulesPrompt() + `

Your only job this turn is to decide which pages the design takes and how each one is mounted.

- Do NOT write components, properties, styles or events. There is a later step for that.
- Answer by calling structured_output with the plan schema, and nothing else.
- You may call the reference lookups first (entity metadata, the existing pages). Their results are
  input to your plan - never the answer itself.`
}

func planSummary(plan studio.PagePlan) string {
	parts := make([]string, 0, len(plan.Pages))
	for _, page := range plan.Pages {
		parts = append(parts, fmt.Sprintf("%s(%s)", page.PageId, page.Role))
	}
	return strings.Join(parts, ", ")
}

// composeNestedPages builds each page the plan called for, one call each.
//
// This is the decomposition: the root page is written by the main generation,
// which streams and carries the conversation, and every page it mounts is
// written here on its own. One page per call means each call sees only what that
// page needs - a card page is told about a card, not about page state, events
// and responsive breakpoints - and a mistake in one cannot corrupt another.
//
// A page that cannot be built is left out with a warning rather than failing the
// answer: a board with a default card is a worse page, not a broken one.
func (eruStudioAgent *EruStudioAgent) composeNestedPages(
	ctx context.Context,
	plan studio.PagePlan,
	rootPage map[string]interface{},
	userPrompt string,
	projectId string,
	tenantId string,
) ([]map[string]interface{}, []string) {
	pages := []map[string]interface{}{}
	warnings := []string{}

	schema := buildEruPageOutputSchema()
	systemPrompt := eruStudioAgent.GetSystemPrompt()

	for _, planned := range plan.Nested() {
		content := nestedPageBrief(plan, planned, rootPage, userPrompt)

		// A page built on its own gets the same treatment as the root: checked,
		// and given a chance to fix what was wrong. Without this the split traded
		// the root's retry loop away - a nested page came back with an invented
		// property and was shipped, because the only thing looking at it was a
		// log line.
		var payload map[string]interface{}
		var failed error
		for attempt := 0; attempt < nestedPageAttempts; attempt++ {
			started := emitStepStart(ctx, agents.StepBuildPage, attempt+1)
			built, _, err := eruStudioAgent.GenerateStructured(ctx, systemPrompt, content, schema, projectId, tenantId, true)
			if err != nil {
				logs.WithContext(ctx).Error(fmt.Sprintf("eru studio: could not build planned page %q: %v", planned.PageId, err))
				agents.EmitStepFinished(ctx, agents.StepBuildPage, attempt+1, agents.OutcomeError, started, err.Error(), agents.CodeModelError)
				failed = err
				break
			}
			stampPlannedIdentity(built, planned)

			// The same checks the root page gets, not a subset.
			//
			// This ran only the catalog and the entity check, so every query rule
			// - probe before you bind, bound but never probed, and a binding to a
			// query the tool said does not exist - was silently skipped for any
			// page the planner built. A nested page could bind to a query that
			// is not there and nothing would object, which is precisely the
			// failure the query rules exist for. A page the user sees is a page
			// held to the whole standard, however it came to be built.
			issues := pageIssuesIn(ctx, catalog.Get(), built)
			issues = append(issues, entityBindingIssues(ctx, built, nil, nil)...)
			issues = append(issues, unprobedQueryIssues(ctx, built, nil)...)
			issues = append(issues, preflightIssues(ctx, built, nil, nil)...)
			payload = built
			if len(issues) == 0 {
				agents.EmitStepFinished(ctx, agents.StepBuildPage, attempt+1, agents.OutcomeSuccess, started, "", "")
				break
			}
			if attempt == nestedPageAttempts-1 {
				// Out of attempts. A page with a flaw is better than a mount
				// pointing at nothing, so it still goes back - with a warning, so
				// the flaw is the user's to see rather than ours to hide.
				agents.EmitStepFinished(ctx, agents.StepBuildPage, attempt+1, agents.OutcomeError, started, catalog.FormatIssues(issues, 10), agents.CodeOutputValidation)
				logs.WithContext(ctx).Info(fmt.Sprintf("eru studio: planned page %q still has %d issue(s) after %d attempt(s); returning it anyway",
					planned.PageId, len(issues), nestedPageAttempts))
				warnings = append(warnings, fmt.Sprintf("the %s page %q has %d unresolved issue(s): %s",
					planned.Role, planned.PageId, len(issues), catalog.FormatIssues(issues, 3)))
				break
			}
			agents.EmitStepFinished(ctx, agents.StepBuildPage, attempt+1, agents.OutcomeRetry, started, catalog.FormatIssues(issues, 10), agents.CodeOutputValidation)
			content = fmt.Sprintf("%s\n\n--- YOUR PREVIOUS ANSWER WAS REJECTED ---\n%s\n\nBuild this page again, fixing exactly these and changing nothing else.",
				content, catalog.FormatIssues(issues, maxReportedPageIssues))
		}
		if payload == nil {
			warnings = append(warnings, fmt.Sprintf("the %s page %q could not be built, so %s falls back to its default",
				planned.Role, planned.PageId, planned.MountProperty))
			_ = failed
			continue
		}

		pages = append(pages, map[string]interface{}{
			"page_id":        planned.PageId,
			"mounted_at":     planned.MountedAt,
			"mount_property": planned.MountProperty,
			"purpose":        planned.Purpose,
			"is_new":         true,
			"page":           payload,
		})
	}
	return pages, warnings
}

// nestedPageBrief is everything one page needs and nothing else.
func nestedPageBrief(plan studio.PagePlan, planned studio.PlanPage, rootPage map[string]interface{}, userPrompt string) string {
	var b strings.Builder

	b.WriteString("You are building ONE page of a design that is already underway. Build only this page.\n\n")
	fmt.Fprintf(&b, "PAGE ID: %s (use exactly this id)\n", planned.PageId)
	fmt.Fprintf(&b, "ROLE: %s\n", planned.Role)
	fmt.Fprintf(&b, "PURPOSE: %s\n", planned.Purpose)
	if planned.Entity != "" {
		fmt.Fprintf(&b, "ENTITY: %s\n", planned.Entity)
	}
	if planned.Skeleton != "" {
		fmt.Fprintf(&b, "LAYOUT: %s\n", planned.Skeleton)
	}
	fmt.Fprintf(&b, "MOUNTED BY: component %q on the root page, through %q\n", planned.MountedAt, planned.MountProperty)

	switch planned.Role {
	case studio.PlanRoleCard:
		b.WriteString(`
This is a CARD. It is drawn once per record inside a board, so it is small and it repeats:
- Show the few fields someone scans a board for, not the whole record.
- No page header, no toolbar, no grid - those belong to the page that mounts this one.
- Build it to look right at roughly 280-360px wide and to stay readable when it is one of forty on screen.
`)
	case studio.PlanRoleDetail:
		b.WriteString(`
This is a DETAIL page mounted in a panel:
- It shows or edits ONE record; the list that chooses the record belongs to the page that mounts this one.
- No page-level chrome that would duplicate what is already around the panel.
`)
	}

	if entity := rootEntity(rootPage); entity != "" && planned.Entity == "" {
		fmt.Fprintf(&b, "\nThe page that mounts this one reads %q.\n", entity)
	}

	b.WriteString("\nThe rest of the design (for context only - do not build any of it):\n")
	for _, page := range plan.Pages {
		if page.PageId == planned.PageId {
			continue
		}
		fmt.Fprintf(&b, "- %s (%s): %s\n", page.PageId, page.Role, page.Purpose)
	}

	b.WriteString("\n--- WHAT THE USER ASKED FOR (the whole design; you are building one page of it) ---\n")
	b.WriteString(userPrompt)
	b.WriteString("\n\nAnswer with the EruPage for this page only.")
	return b.String()
}

func rootEntity(page map[string]interface{}) string {
	entity, _ := page["entity_name"].(string)
	return entity
}

// jsonSchemaFrom converts a plain schema map into the model's schema type, so a
// schema can be written as JSON where that reads better than a struct literal.
func jsonSchemaFrom(raw map[string]interface{}) (eru_models.JSONSchema, error) {
	encoded, err := json.Marshal(raw)
	if err != nil {
		return eru_models.JSONSchema{}, err
	}
	var schema eru_models.JSONSchema
	if err := json.Unmarshal(encoded, &schema); err != nil {
		return eru_models.JSONSchema{}, err
	}
	return schema, nil
}

// emitStepStart reports a step and hands back the moment it began, which is what
// the finish event needs to report a duration.
func emitStepStart(ctx context.Context, step string, attempt int) time.Time {
	agents.EmitStepStarted(ctx, step, attempt)
	return time.Now()
}

// attachPlannedPages builds the pages the plan called for and folds them into
// the answer, so everything downstream - mount checking, the manifest, the
// per-page envelopes - sees one answer that happens to have been written in
// several pieces.
func (eruStudioAgent *EruStudioAgent) attachPlannedPages(
	ctx context.Context,
	agentOutput agents.AgentMessage,
	plan studio.PagePlan,
	userPrompt string,
	projectId string,
	tenantId string,
) agents.AgentMessage {
	if len(agentOutput.Actions) == 0 {
		return agentOutput
	}
	action := agentOutput.Actions[0].Action
	if action == nil {
		return agentOutput
	}

	rootPage := resolvedRootPage(action, studio.BasePageFrom(ctx))
	built, warnings := eruStudioAgent.composeNestedPages(ctx, plan, rootPage, userPrompt, projectId, tenantId)
	if len(built) == 0 && len(warnings) == 0 {
		return agentOutput
	}

	// The root generation was told not to author these pages. If it did anyway,
	// the ones built from the plan are the ones that were mounted by id, so they
	// are kept and the volunteered copies are dropped.
	existing, _ := action["pages"].([]interface{})
	byId := map[string]bool{}
	for _, page := range built {
		byId[page["page_id"].(string)] = true
	}
	merged := make([]interface{}, 0, len(built)+len(existing))
	for _, page := range built {
		merged = append(merged, page)
	}
	for _, raw := range existing {
		page, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if id, _ := page["page_id"].(string); id != "" && byId[id] {
			logs.WithContext(ctx).Info(fmt.Sprintf("eru studio: dropping the root generation's copy of planned page %q", id))
			continue
		}
		merged = append(merged, raw)
	}
	action["pages"] = merged

	if len(warnings) > 0 {
		existingWarnings, _ := action["warnings"].([]interface{})
		for _, warning := range warnings {
			existingWarnings = append(existingWarnings, warning)
		}
		action["warnings"] = existingWarnings
	}

	agentOutput.Actions[0].Action = action
	logs.WithContext(ctx).Info(fmt.Sprintf("eru studio: attached %d planned page(s) to the answer", len(built)))
	return agentOutput
}

// nestedPageAttempts bounds a page's own retries. Two: one to build it, one to
// fix it. A third would mostly buy a different mistake, and every attempt is a
// model call the user is waiting on.
const nestedPageAttempts = 2

// stampPlannedIdentity holds a built page to what the plan said it was. The root
// page already mounts it by id, so a page that renamed itself is a mount
// pointing at nothing.
func stampPlannedIdentity(page map[string]interface{}, planned studio.PlanPage) {
	page["id"] = planned.PageId
	if name, _ := page["name"].(string); strings.TrimSpace(name) == "" {
		page["name"] = planned.PageId
	}
	if planned.Entity != "" {
		if entity, _ := page["entity_name"].(string); strings.TrimSpace(entity) == "" {
			page["entity_name"] = planned.Entity
		}
	}
}

// PlanningNote tells an orchestrator how this agent is used.
//
// A design is often more than one page - a board and the card it draws, a list
// and the panel beside it - and this agent produces all of them in a single
// response. A planner that assumes one page per step chains two calls together
// and has to write a template joining them, which is where the exchange-rate
// plan failed three repair rounds in a row on unbalanced parentheses. There was
// never a second step to make.
func (eruStudioAgent *EruStudioAgent) PlanningNote() string {
	return "ONE step, always. This agent produces the whole design in a single call - the page being " +
		"edited and every page it mounts (a board's card, a side panel's form) come back together as " +
		"separate actions. Never plan a second step for another page, and never chain two of these. " +
		"WHEN TO INCLUDE IT: whenever the request implies a person will enter, review or browse the " +
		"data - not only when a page is asked for by name. A requirement phrased as data (\"we need to " +
		"capture X\", \"track Y against Z\") is not met by storage alone: until there is a page, nobody " +
		"can put a record in or read one out. Plan the data model step first and this one after it, " +
		"naming the entities that step creates."
}
