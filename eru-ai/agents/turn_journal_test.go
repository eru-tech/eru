package agents

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	models "github.com/eru-tech/eru/eru-ai/models"
)

const pageV0 = `{"components":[{"id":"tb-root","children":[
  {"id":"tb-title","properties":{"base":{"label":"Eru Control Testbed"}},"styles":{"base":{"background_color":"#ffffff"}}},
  {"id":"tb-tag","properties":{"base":{"label":"Tag"}}}]}]}`

const pageV1 = `{"components":[{"id":"tb-root","children":[
  {"id":"tb-title","properties":{"base":{"label":"xxx"}},"styles":{"base":{"background_color":"#ffffff"}}},
  {"id":"tb-tag","properties":{"base":{"label":"Tag"}}}]}]}`

func TestDiffNamesTheLeafThatChangedAndWhatItHeld(t *testing.T) {
	entries, ok := DiffArtifacts(pageV0, pageV1)
	if !ok {
		t.Fatal("no diff reported between two different pages")
	}
	if len(entries) != 1 {
		t.Fatalf("reported %d changes, want 1: %v", len(entries), entries)
	}
	got := entries[0].String()
	for _, want := range []string{"tb-title", "label", "Eru Control Testbed", "xxx"} {
		if !strings.Contains(got, want) {
			t.Fatalf("change line %q does not mention %q", got, want)
		}
	}
}

func TestDiffMatchesComponentsByIdNotPosition(t *testing.T) {
	// A component inserted at the top must not report every component below it
	// as having changed - that would drown the one real edit.
	before := `{"components":[{"id":"a","v":1},{"id":"b","v":2}]}`
	after := `{"components":[{"id":"c","v":9},{"id":"a","v":1},{"id":"b","v":2}]}`

	entries, ok := DiffArtifacts(before, after)
	if !ok {
		t.Fatal("no diff reported")
	}
	for _, e := range entries {
		if strings.Contains(e.Path, "[a]") || strings.Contains(e.Path, "[b]") {
			t.Fatalf("untouched component reported as changed: %s", e.String())
		}
	}
}

func TestIdenticalArtifactsHaveNoJournal(t *testing.T) {
	if _, ok := DiffArtifacts(pageV0, pageV0); ok {
		t.Fatal("a turn that changed nothing produced a change list")
	}
}

func TestJournalGivesAFollowUpItsAntecedent(t *testing.T) {
	ctx := context.Background()
	store := storeWithTtl(t, "1h")
	recordSessionTurn(ctx, store, "c-antecedent", SessionTurn{Role: "user", Code: pageV0})
	recordSessionTurn(ctx, store, "c-antecedent", SessionTurn{Role: "assistant", Code: `{"response":"done"}`})
	recordSessionTurn(ctx, store, "c-antecedent", SessionTurn{Role: "user", Code: pageV1})
	state := sessionStateOf(ctx, store, "c-antecedent")

	note := JournalNote(state)
	if note == "" {
		t.Fatal("no journal for a conversation that has already changed the page")
	}
	// "revert the label back to what it was" is answerable only if the old value
	// is in the note.
	if !strings.Contains(note, "Eru Control Testbed") {
		t.Fatalf("the journal does not say what the label used to be:\n%s", note)
	}
	if !strings.Contains(note, "xxx") {
		t.Fatalf("the journal does not say what the label became:\n%s", note)
	}
	// The assistant's answer envelope is a different shape from the page and
	// must not be diffed against it.
	if strings.Contains(note, "response") {
		t.Fatalf("the answer envelope leaked into the change list:\n%s", note)
	}
}

func TestNoJournalOnTheFirstTurn(t *testing.T) {
	ctx := context.Background()
	store := storeWithTtl(t, "1h")
	recordSessionTurn(ctx, store, "c-first", SessionTurn{Role: "user", Code: pageV0})
	if note := JournalNote(sessionStateOf(ctx, store, "c-first")); note != "" {
		t.Fatalf("a first turn has nothing to refer back to, but got:\n%s", note)
	}
}

func TestTheRealEditIsNeverCrowdedOutByNormalisation(t *testing.T) {
	// The canvas fills in defaults between turns. Those must not be able to push
	// the one edit the user is about to refer back to off the end of the list.
	before := map[string]interface{}{"components": []interface{}{}}
	after := map[string]interface{}{"components": []interface{}{}}

	comps := make([]interface{}, 0, 60)
	compsAfter := make([]interface{}, 0, 60)
	for i := 0; i < 60; i++ {
		id := "c" + string(rune('A'+i%26)) + string(rune('a'+i/26))
		comps = append(comps, map[string]interface{}{"id": id})
		compsAfter = append(compsAfter, map[string]interface{}{"id": id, "events": []interface{}{}, "name": ""})
	}
	comps = append(comps, map[string]interface{}{"id": "zz-title", "label": "Eru Control Testbed"})
	compsAfter = append(compsAfter, map[string]interface{}{"id": "zz-title", "label": "xxx"})
	before["components"] = comps
	after["components"] = compsAfter

	b, _ := json.Marshal(before)
	a, _ := json.Marshal(after)

	entries, ok := DiffArtifacts(string(b), string(a))
	if !ok {
		t.Fatal("no diff reported")
	}
	found := false
	for _, e := range entries {
		if strings.Contains(e.Path, "zz-title") && strings.Contains(e.String(), "Eru Control Testbed") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the one real edit was lost among %d entries; first is %q", len(entries), entries[0].String())
	}
	if entries[0].From == "" || entries[0].To == "" {
		t.Fatalf("an appearance outranked a real value change: %q", entries[0].String())
	}
}

func TestBookkeepingLeavesAreNotChanges(t *testing.T) {
	before := `{"id":"a","updated_at":"2026-01-01","label":"x"}`
	after := `{"id":"a","updated_at":"2026-09-15","label":"x"}`
	if _, ok := DiffArtifacts(before, after); ok {
		t.Fatal("a refreshed updated_at was reported as a change the user could revert")
	}
}

func TestThePlannerGetsTheJournalToo(t *testing.T) {
	// The planner builds its own request straight from BuildChatRequest rather
	// than from what the conversation loader assembled. If the journal is
	// attached anywhere else, the half of the system that decides whether a
	// follow-up is answerable never sees it.
	ctx := context.Background()
	store := storeWithTtl(t, "1h")
	recordSessionTurn(ctx, store, "c-plan", SessionTurn{Role: "user", Code: pageV0})
	recordSessionTurn(ctx, store, "c-plan", SessionTurn{Role: "user", Code: pageV1})

	cm := ConversationManager{Config: DefaultConversationConfig(""), ChatMemory: store}
	conversation := &Conversation{
		ConversationId: "c-plan",
		MemoryKey:      "c-plan",
		Messages:       []AgentMessage{{Role: "user", Content: "change the label to xxx"}},
	}

	req, err := cm.BuildChatRequest(ctx, conversation, models.Message{Role: "user", Content: "revert it"}, "processo_scf")
	if err != nil {
		t.Fatalf("BuildChatRequest: %v", err)
	}

	var journal string
	for _, m := range req.Messages {
		if m.Name == "conversation_journal" {
			journal = m.Content
		}
	}
	if journal == "" {
		t.Fatal("the planning request carries no journal, so \"revert it\" has no antecedent")
	}
	if !strings.Contains(journal, "Eru Control Testbed") {
		t.Fatalf("journal does not carry the old value:\n%s", journal)
	}
	if req.Messages[len(req.Messages)-1].Content != "revert it" {
		t.Fatal("the journal displaced the instruction from the end of the request")
	}
}
