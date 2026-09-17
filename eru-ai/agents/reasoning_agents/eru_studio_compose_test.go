package reasoning_agents

import (
	"context"
	"strings"
	"testing"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	studio "github.com/eru-tech/eru/eru-ai/agents/eru_studio"
)

func composePlan() studio.PagePlan {
	return studio.PagePlan{
		RootPageId: "financiers",
		Pages: []studio.PlanPage{
			{PageId: "financiers", Role: studio.PlanRoleRoot, Purpose: "Board of financiers", Entity: "fn"},
			{PageId: "financier_card", Role: studio.PlanRoleCard, Purpose: "One financier as a card",
				Entity: "fn", MountedAt: "financier_grid", MountProperty: "card_page_id"},
		},
	}
}

// Planning is for authoring from nothing. An edit already has its pages, and a
// plan there is a model call spent restating the request.
func TestPlanningOnlyAppliesToAuthoringFromNothing(t *testing.T) {
	envelope := studio.WithOutputMode(context.Background(), studio.ModePatch)
	bare := studio.WithOutputMode(context.Background(), studio.ModeFull)
	page := map[string]interface{}{"id": "p1"}

	if !planningApplies(envelope, nil, nil) {
		t.Error("authoring a new page should be planned")
	}
	if planningApplies(envelope, page, nil) {
		t.Error("an edit to an existing page should not be planned")
	}
	if planningApplies(envelope, nil, &studio.ResolvedScope{}) {
		t.Error("a scoped edit should not be planned")
	}
	if planningApplies(bare, nil, nil) {
		t.Error("bare-page mode has no \"pages\" to plan for")
	}
}

// A card page needs to be told it is a card. Handing it the whole design brief
// is how it ended up with a page header and a toolbar on something drawn forty
// times on one screen.
func TestNestedBriefIsScopedToTheOnePage(t *testing.T) {
	plan := composePlan()
	brief := nestedPageBrief(plan, plan.Nested()[0], map[string]interface{}{"entity_name": "fn"}, "make a page to display financiers in board view")

	for _, want := range []string{"financier_card", "card_page_id", "financier_grid", "This is a CARD", "Build only this page"} {
		if !strings.Contains(brief, want) {
			t.Errorf("the brief is missing %q", want)
		}
	}
	if !strings.Contains(brief, "do not build any of it") {
		t.Error("the brief must mark the rest of the design as context only")
	}
	if !strings.Contains(brief, "make a page to display financiers") {
		t.Error("the brief should still carry what the user asked for")
	}
}

func TestDetailBriefSaysItIsOneRecord(t *testing.T) {
	plan := studio.PagePlan{
		RootPageId: "root",
		Pages: []studio.PlanPage{
			{PageId: "root", Role: studio.PlanRoleRoot, Purpose: "List"},
			{PageId: "detail", Role: studio.PlanRoleDetail, Purpose: "Edit one", MountedAt: "panel", MountProperty: "page"},
		},
	}
	brief := nestedPageBrief(plan, plan.Nested()[0], nil, "a list with an edit panel")
	if !strings.Contains(brief, "DETAIL page") || !strings.Contains(brief, "ONE record") {
		t.Errorf("a detail page should be briefed as one record:\n%s", brief)
	}
}

// The pages built from the plan are the ones the root mounted by id. A copy the
// root volunteered anyway is a second version of the same card.
func TestPlannedPagesReplaceAnyCopyTheRootVolunteered(t *testing.T) {
	agent := &EruStudioAgent{}
	ctx := studio.WithOutputMode(context.Background(), studio.ModePatch)

	output := map[string]interface{}{
		"mode": studio.ModeFull,
		"page": map[string]interface{}{"id": "financiers", "name": "financiers", "styles": map[string]interface{}{}, "components": []interface{}{}},
		"pages": []interface{}{
			map[string]interface{}{"page_id": "financier_card", "page": map[string]interface{}{"id": "financier_card", "name": "volunteered"}},
			map[string]interface{}{"page_id": "something_else", "page": map[string]interface{}{"id": "something_else", "name": "keep me"}},
		},
	}
	agentOutput := agents.AgentMessage{Actions: []agents.AgentOutputAction{{Action: output}}}

	// With no model available nothing is built, so the answer is left alone
	// rather than losing the pages it already had.
	result := agent.attachPlannedPages(ctx, agentOutput, composePlan(), "prompt", "p", "t")
	action := result.Actions[0].Action
	pages, _ := action["pages"].([]interface{})
	if len(pages) != 2 {
		t.Fatalf("a failed build must not drop the pages the answer already had, got %d", len(pages))
	}
}

func TestAttachPlannedPagesLeavesAnEmptyAnswerAlone(t *testing.T) {
	agent := &EruStudioAgent{}
	ctx := studio.WithOutputMode(context.Background(), studio.ModePatch)
	empty := agents.AgentMessage{}
	if result := agent.attachPlannedPages(ctx, empty, composePlan(), "prompt", "p", "t"); len(result.Actions) != 0 {
		t.Error("an answer with no actions should come back untouched")
	}
}
