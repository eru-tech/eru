package reasoning_agents

import (
	"strings"
	"testing"
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

	issue := fullAnswerDropsPage(base, answer)
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

	if issue := fullAnswerDropsPage(base, answer); issue != nil {
		t.Fatalf("a complete answer was faulted: %s", issue.Message)
	}
}

func TestATrimOfHalfIsStillAllowed(t *testing.T) {
	// Removing components is a legitimate edit; only losing MOST of the page is
	// the mistake this guards.
	base := pageOf("a", "b", "c", "d")
	if issue := fullAnswerDropsPage(base, pageOf("a", "b")); issue != nil {
		t.Fatalf("dropping half was faulted: %s", issue.Message)
	}
}

func TestASmallPageIsNotGuarded(t *testing.T) {
	// A three-component page can plausibly be rebuilt from nothing.
	if issue := fullAnswerDropsPage(pageOf("a", "b", "c"), pageOf("z")); issue != nil {
		t.Fatalf("a small page was guarded: %s", issue.Message)
	}
}

func TestAuthoringFromNothingIsNotGuarded(t *testing.T) {
	if issue := fullAnswerDropsPage(map[string]interface{}{}, pageOf("a")); issue != nil {
		t.Fatalf("authoring with no base page was faulted: %s", issue.Message)
	}
}
