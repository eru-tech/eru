package reasoning_agents

import (
	"context"
	"strings"
	"testing"

	studio "github.com/eru-tech/eru/eru-ai/agents/eru_studio"
	catalog "github.com/eru-tech/eru/eru-ai/agents/eru_studio/catalog"
)

func entityLedgerCtx() context.Context {
	ledger := studio.NewLedger()
	ledger.RecordEntities([]string{"fn", "ba"}, map[string]string{"scf_fn": "fn", "scf_ba": "ba"})
	return studio.WithLedger(context.Background(), ledger)
}

func pageBoundTo(entity string) map[string]interface{} {
	return map[string]interface{}{
		"id": "p1", "name": "p1", "styles": map[string]interface{}{},
		"components": []interface{}{
			map[string]interface{}{
				"id":         "grid1",
				"type":       "grid",
				"properties": map[string]interface{}{"base": map[string]interface{}{"entity_name": entity}},
				"styles":     map[string]interface{}{},
			},
		},
	}
}

func TestBindingToTheTableIsReportedWithTheEntityToUse(t *testing.T) {
	issues := entityBindingIssues(entityLedgerCtx(), pageBoundTo("scf_fn"), nil, nil)
	if len(issues) != 1 {
		t.Fatalf("expected the table binding to be caught, got %d", len(issues))
	}
	if issues[0].Code != catalog.CodeEntityIsATableName {
		t.Errorf("wrong code %q", issues[0].Code)
	}
	if !strings.Contains(issues[0].Message, "\"fn\"") {
		t.Errorf("the message must name the entity to use instead: %s", issues[0].Message)
	}
	if issues[0].ComponentId != "grid1" {
		t.Errorf("issue should name the component, got %q", issues[0].ComponentId)
	}
}

func TestBindingToTheEntityIsAccepted(t *testing.T) {
	if issues := entityBindingIssues(entityLedgerCtx(), pageBoundTo("fn"), nil, nil); len(issues) != 0 {
		t.Fatalf("binding to the entity is correct, got %v", issues)
	}
}

// Without metadata there is nothing to check against, and guessing which of two
// plausible names is the table would be worse than staying quiet.
func TestNoMetadataMeansNoOpinion(t *testing.T) {
	ctx := studio.WithLedger(context.Background(), studio.NewLedger())
	if issues := entityBindingIssues(ctx, pageBoundTo("scf_fn"), nil, nil); len(issues) != 0 {
		t.Fatalf("expected silence with no metadata, got %v", issues)
	}
}

// An entity the lookup never mentioned is not necessarily wrong - the answer is
// capped, and a page may legitimately name something outside the page's slice.
func TestAnUnknownNameIsNotFlagged(t *testing.T) {
	if issues := entityBindingIssues(entityLedgerCtx(), pageBoundTo("something_else"), nil, nil); len(issues) != 0 {
		t.Fatalf("an unrecognised name is not evidence of a table, got %v", issues)
	}
}

func TestNestedPagesAreCheckedToo(t *testing.T) {
	root := map[string]interface{}{"id": "p1", "name": "p1", "styles": map[string]interface{}{}, "components": []interface{}{}}
	nested := []map[string]interface{}{pageBoundTo("scf_ba")}
	issues := entityBindingIssues(entityLedgerCtx(), root, nil, nested)
	if len(issues) != 1 || !strings.Contains(issues[0].Message, "\"ba\"") {
		t.Fatalf("a nested page bound to a table should be caught, got %v", issues)
	}
}
