package eru_studio

import (
	"strings"
	"testing"

	catalog "github.com/eru-tech/eru/eru-ai/agents/eru_studio/catalog"
)

func boardPlan() PagePlan {
	return PagePlan{
		RootPageId: "financiers",
		Summary:    "A board of financiers with a designed card",
		Pages: []PlanPage{
			{PageId: "financiers", Role: PlanRoleRoot, Purpose: "Browse financiers as a board", Entity: "fn",
				Skeleton: "header, then a grid in board view"},
			{PageId: "financier_card", Role: PlanRoleCard, Purpose: "One financier as a card", Entity: "fn",
				MountedAt: "financier_grid", MountProperty: "card_page_id"},
		},
	}
}

func planCodes(issues []catalog.Issue) []string {
	out := make([]string, 0, len(issues))
	for _, issue := range issues {
		out = append(out, string(issue.Code))
	}
	return out
}

func TestAGoodPlanValidates(t *testing.T) {
	if issues := ValidatePlan(boardPlan()); len(issues) != 0 {
		t.Fatalf("a complete plan was rejected: %v", planCodes(issues))
	}
	if !boardPlan().IsMultiPage() {
		t.Error("a board plus its card is more than one page")
	}
	if nested := boardPlan().Nested(); len(nested) != 1 || nested[0].PageId != "financier_card" {
		t.Errorf("Nested() should be everything but the root, got %v", nested)
	}
}

func TestASinglePagePlanIsNotMultiPage(t *testing.T) {
	plan := PagePlan{RootPageId: "p1", Pages: []PlanPage{{PageId: "p1", Role: PlanRoleRoot, Purpose: "A form"}}}
	if issues := ValidatePlan(plan); len(issues) != 0 {
		t.Fatalf("a one-page plan is normal, got %v", planCodes(issues))
	}
	if plan.IsMultiPage() {
		t.Error("one page is not multi-page")
	}
}

// The failure the plan step exists to catch, caught before anything is drawn.
func TestAPlannedPageThatNothingMountsIsRejected(t *testing.T) {
	plan := boardPlan()
	plan.Pages[1].MountedAt = ""
	issues := ValidatePlan(plan)
	if len(issues) == 0 {
		t.Fatal("a page nothing mounts was accepted")
	}
	found := false
	for _, issue := range issues {
		if issue.Code == catalog.CodePlanPageUnmounted {
			found = true
			if issue.ComponentId != "financier_card" {
				t.Errorf("the issue should name the page, got %q", issue.ComponentId)
			}
		}
	}
	if !found {
		t.Errorf("wrong issue: %v", planCodes(issues))
	}
}

func TestAPlanMustSayHowAPageIsMounted(t *testing.T) {
	plan := boardPlan()
	plan.Pages[1].MountProperty = ""
	if issues := ValidatePlan(plan); len(issues) == 0 {
		t.Fatal("a mount with no property was accepted")
	}
}

func TestAPlanWithNoRootIsRejected(t *testing.T) {
	plan := boardPlan()
	plan.RootPageId = "somewhere_else"
	codes := planCodes(ValidatePlan(plan))
	found := false
	for _, code := range codes {
		if code == string(catalog.CodePlanNoRoot) {
			found = true
		}
	}
	if !found {
		t.Errorf("a root_page_id naming no page was accepted: %v", codes)
	}
}

func TestAPlanWithDuplicateIdsIsRejected(t *testing.T) {
	plan := boardPlan()
	plan.Pages[1].PageId = "financiers"
	codes := planCodes(ValidatePlan(plan))
	found := false
	for _, code := range codes {
		if code == string(catalog.CodePlanDuplicateId) {
			found = true
		}
	}
	if !found {
		t.Errorf("two pages with one id were accepted: %v", codes)
	}
}

// The client names the page it is editing. A plan that invented another id would
// have every mount pointing at a page that is not the one on screen.
func TestRebaseRootHoldsThePlanToTheRequestsPageId(t *testing.T) {
	plan := RebaseRoot(boardPlan(), "real_page_id")
	if plan.RootPageId != "real_page_id" {
		t.Fatalf("root not rebased: %s", plan.RootPageId)
	}
	root, ok := plan.Root()
	if !ok || root.Role != PlanRoleRoot {
		t.Error("the root entry should have been renamed with it")
	}
	if len(plan.Nested()) != 1 {
		t.Error("rebasing must not disturb the other pages")
	}
}

// The root generation has to be told the other pages exist AND that it is not
// the one writing them, or the design ends up with two copies of the card.
func TestPlanCommitmentNamesTheMountsAndForbidsAuthoringThem(t *testing.T) {
	text := PlanCommitment(boardPlan())
	for _, want := range []string{"financier_card", "financier_grid", "card_page_id", "BEING BUILT SEPARATELY", "do NOT author them"} {
		if !strings.Contains(text, want) {
			t.Errorf("the commitment is missing %q:\n%s", want, text)
		}
	}
}

func TestPlanCommitmentForOnePageSaysSo(t *testing.T) {
	plan := PagePlan{RootPageId: "p1", Pages: []PlanPage{{PageId: "p1", Role: PlanRoleRoot, Purpose: "A form"}}}
	text := PlanCommitment(plan)
	if !strings.Contains(text, "single page") {
		t.Errorf("a one-page plan should say so:\n%s", text)
	}
	if strings.Contains(text, "BEING BUILT SEPARATELY") {
		t.Error("there is nothing being built separately")
	}
}

func TestPlanPromptTeachesTheCasesThatGoWrong(t *testing.T) {
	prompt := PlanPrompt()
	for _, want := range []string{"BOARD view", "card_page_id", "page_ref", "Most requests are one page"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the planning prompt is missing %q", want)
		}
	}
}
