package eval

import (
	"strings"
	"testing"
)

// The message that blocked two fixtures for a day by naming the wrong problem.
//
// An unread workspace produced "this scenario acts on test, which the workspace
// does not have" - for an entity that existed. The precondition was reading the
// zero Outcome as an empty one.
func TestAnUnreadWorkspaceIsNotAnEmptyOne(t *testing.T) {
	fixture := Fixture{Name: "x", RequiresPresent: []string{"test"}}

	unread := fixture.CheckPreconditions(Outcome{})
	if unread == "" {
		t.Fatal("an unreadable workspace must still block - nothing can be judged against it")
	}
	if strings.Contains(unread, "does not have") {
		t.Errorf("it must not claim the entity is absent:\n%s", unread)
	}
	if !strings.Contains(unread, "could not be read") {
		t.Errorf("it must say what actually went wrong:\n%s", unread)
	}

	// A workspace CAN legitimately be empty, and that is a different state
	// again - which is why Read is an explicit flag rather than "are there any
	// entities".
	if empty := fixture.CheckPreconditions(Outcome{Read: true, Entities: map[string]EntityState{}}); !strings.Contains(empty, "does not have") {
		t.Errorf("a read-but-empty workspace genuinely lacks the entity:\n%s", empty)
	}

	// Read, and genuinely missing: the original message is right here.
	missing := fixture.CheckPreconditions(Outcome{Read: true, Entities: map[string]EntityState{
		"something_else": {Name: "something_else"},
	}})
	if !strings.Contains(missing, "does not have") {
		t.Errorf("a genuinely absent entity must still say so:\n%s", missing)
	}

	// Read, and present: nothing to block.
	if ok := fixture.CheckPreconditions(Outcome{Read: true, Entities: map[string]EntityState{
		"test": {Name: "test"},
	}}); ok != "" {
		t.Errorf("a present entity must not block: %s", ok)
	}
}

// A fixture that asks for nothing must not be blocked by an unread workspace -
// it had no precondition to check in the first place.
func TestAFixtureWithNoPreconditionIsNeverBlockedByAnUnreadWorkspace(t *testing.T) {
	if reason := (Fixture{Name: "x"}).CheckPreconditions(Outcome{}); reason != "" {
		t.Errorf("nothing was required, so nothing is missing: %s", reason)
	}
}
