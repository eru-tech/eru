package reasoning_agents

import (
	"strings"
	"testing"

	studio "github.com/eru-tech/eru/eru-ai/agents/eru_studio"
)

func pageOf(ids ...string) map[string]interface{} {
	components := make([]interface{}, 0, len(ids))
	for _, id := range ids {
		components = append(components, map[string]interface{}{"id": id, "type": "text"})
	}
	return map[string]interface{}{"id": "p1", "components": components}
}

func TestAFullAnswerThatGutsThePageIsRejected(t *testing.T) {
	base := pageOf("a", "b", "c", "d", "e", "f")
	answer := pageOf("a")

	issue := answerDropsPage(base, answer, studio.ModeFull)
	if issue == nil {
		t.Fatal("dropping 5 of 6 components was accepted")
	}
	if !strings.Contains(issue.Message, "patch") {
		t.Fatalf("the message does not point at the fix: %s", issue.Message)
	}
	for _, id := range []string{"b", "c"} {
		if !strings.Contains(issue.Message, id) {
			t.Errorf("dropped component %q is not named: %s", id, issue.Message)
		}
	}
}

func TestAFullAnswerCarryingThePageIsAccepted(t *testing.T) {
	base := pageOf("a", "b", "c", "d", "e", "f")
	// The whole page back, plus the new component the user asked for.
	answer := pageOf("a", "b", "c", "d", "e", "f", "new_tile")

	if issue := answerDropsPage(base, answer, studio.ModeFull); issue != nil {
		t.Fatalf("a complete answer was faulted: %s", issue.Message)
	}
}

func TestATrimOfHalfIsStillAllowed(t *testing.T) {
	// Removing components is a legitimate edit; only losing MOST of the page is
	// the mistake this guards.
	base := pageOf("a", "b", "c", "d")
	if issue := answerDropsPage(base, pageOf("a", "b"), studio.ModeFull); issue != nil {
		t.Fatalf("dropping half was faulted: %s", issue.Message)
	}
}

func TestASmallPageIsNotGuarded(t *testing.T) {
	// A three-component page can plausibly be rebuilt from nothing.
	if issue := answerDropsPage(pageOf("a", "b", "c"), pageOf("z"), studio.ModeFull); issue != nil {
		t.Fatalf("a small page was guarded: %s", issue.Message)
	}
}

func TestAuthoringFromNothingIsNotGuarded(t *testing.T) {
	if issue := answerDropsPage(map[string]interface{}{}, pageOf("a"), studio.ModeFull); issue != nil {
		t.Fatalf("authoring with no base page was faulted: %s", issue.Message)
	}
}

func TestAPatchThatResolvesToAGuttedPageIsRejected(t *testing.T) {
	// The shape that actually got through: every component in the patch was
	// valid, and applying it still left the page with a fraction of what the
	// user had.
	base := pageOf("a", "b", "c", "d", "e", "f")
	issue := answerDropsPage(base, pageOf("a", "b"), studio.ModePatch)
	if issue == nil {
		t.Fatal("a patch resolving to a gutted page was accepted")
	}
	if !strings.Contains(issue.Message, "children_ids") {
		t.Fatalf("the message does not point at the usual cause: %s", issue.Message)
	}
}

func TestChildrenIdsNamingSomethingNobodyDefinesIsReported(t *testing.T) {
	base := pageOf("card", "kept_child")
	patch := &studio.Patch{Upsert: []map[string]interface{}{
		{"id": "card", "children_ids": []interface{}{"kept_child", "new_child", "ghost_child"}},
		{"id": "new_child", "type": "text"},
	}}
	missing := studio.MissingChildIds(base, patch)
	if len(missing) != 1 {
		t.Fatalf("expected only the undefined child to be reported, got %v", missing)
	}
	if !strings.Contains(missing[0], "ghost_child") {
		t.Fatalf("the wrong child was reported: %s", missing[0])
	}
}

func TestAnInlineSubtreeCountsAsDefiningItsChildren(t *testing.T) {
	patch := &studio.Patch{Upsert: []map[string]interface{}{
		{"id": "card", "children_ids": []interface{}{"inline_child"}, "children": []interface{}{
			map[string]interface{}{"id": "inline_child", "type": "text"},
		}},
	}}
	if missing := studio.MissingChildIds(map[string]interface{}{}, patch); len(missing) != 0 {
		t.Fatalf("a child defined inline was reported missing: %v", missing)
	}
}
