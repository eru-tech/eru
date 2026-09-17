package reasoning_agents

import (
	"context"
	"strings"
	"testing"

	studio "github.com/eru-tech/eru/eru-ai/agents/eru_studio"
	catalog "github.com/eru-tech/eru/eru-ai/agents/eru_studio/catalog"
	utility "github.com/eru-tech/eru/eru-ai/tools/utility"
)

func boundPage(id string, identifier string) map[string]interface{} {
	return map[string]interface{}{
		"id":     id,
		"name":   id,
		"styles": map[string]interface{}{},
		"components": []interface{}{
			map[string]interface{}{
				"id":   "field1",
				"type": "text",
				"properties": map[string]interface{}{
					"base": map[string]interface{}{"name": identifier, "identifier": identifier},
				},
				"styles": map[string]interface{}{},
			},
		},
	}
}

func ledgerCtx(offer bool, called bool, failed bool) context.Context {
	ledger := studio.NewLedger()
	if offer {
		ledger.Offer(utility.EntityMetadataToolName)
	}
	if called {
		ledger.Record(utility.EntityMetadataToolName)
	}
	if failed {
		ledger.RecordFailure(utility.EntityMetadataToolName, "boom")
	}
	return studio.WithLedger(context.Background(), ledger)
}

func TestPreflightFlagsBindingsTheAgentNeverLookedUp(t *testing.T) {
	issues := preflightIssues(ledgerCtx(true, false, false), boundPage("p1", "customer_name"), nil, nil)
	if len(issues) != 1 {
		t.Fatalf("expected one issue, got %d", len(issues))
	}
	if issues[0].Code != catalog.CodeBindingsWithoutMetadata {
		t.Errorf("wrong code %q", issues[0].Code)
	}
	if !strings.Contains(issues[0].Message, "customer_name") {
		t.Errorf("issue should name the binding it is complaining about: %s", issues[0].Message)
	}
}

func TestPreflightIsSatisfiedByCallingTheLookup(t *testing.T) {
	if issues := preflightIssues(ledgerCtx(true, true, false), boundPage("p1", "customer_name"), nil, nil); len(issues) != 0 {
		t.Fatalf("metadata was read, expected no issue, got: %v", issues)
	}
}

// Requiring a lookup the tenant does not have would fail every page on a
// deployment that never wired it up.
func TestPreflightIsSilentWhenTheLookupWasNeverOffered(t *testing.T) {
	if issues := preflightIssues(ledgerCtx(false, false, false), boundPage("p1", "customer_name"), nil, nil); len(issues) != 0 {
		t.Fatalf("lookup was not offered, expected no issue, got: %v", issues)
	}
}

// A broken lookup must not become a requirement the model can never satisfy -
// that turns one backend fault into an unanswerable request.
func TestPreflightIsSilentWhenTheLookupFailed(t *testing.T) {
	if issues := preflightIssues(ledgerCtx(true, false, true), boundPage("p1", "customer_name"), nil, nil); len(issues) != 0 {
		t.Fatalf("lookup failed, expected no issue, got: %v", issues)
	}
}

// An edit that does not introduce a binding is not answerable for bindings the
// page already had.
func TestPreflightIgnoresBindingsThePageAlreadyHad(t *testing.T) {
	base := boundPage("p1", "customer_name")
	resolved := boundPage("p1", "customer_name")
	if issues := preflightIssues(ledgerCtx(true, false, false), resolved, base, nil); len(issues) != 0 {
		t.Fatalf("binding was pre-existing, expected no issue, got: %v", issues)
	}

	changed := boundPage("p1", "invented_field")
	if issues := preflightIssues(ledgerCtx(true, false, false), changed, base, nil); len(issues) != 1 {
		t.Fatalf("a newly introduced binding should be flagged, got %d issues", len(issues))
	}
}

// A page with no binding claims is free to skip the lookup: the prompt tells the
// model to leave identifier off when it is using the user's own wording.
func TestPreflightAllowsAPageThatClaimsNoBindings(t *testing.T) {
	unbound := map[string]interface{}{
		"id":     "p1",
		"name":   "p1",
		"styles": map[string]interface{}{},
		"components": []interface{}{
			map[string]interface{}{
				"id":         "label1",
				"type":       "text",
				"properties": map[string]interface{}{"base": map[string]interface{}{"label": "Hello"}},
				"styles":     map[string]interface{}{},
			},
		},
	}
	if issues := preflightIssues(ledgerCtx(true, false, false), unbound, nil, nil); len(issues) != 0 {
		t.Fatalf("nothing claims a binding, expected no issue, got: %v", issues)
	}
}

func TestPreflightCoversNestedPages(t *testing.T) {
	root := map[string]interface{}{"id": "p1", "name": "p1", "styles": map[string]interface{}{}, "components": []interface{}{}}
	nested := []map[string]interface{}{boundPage("card1", "gst_number")}
	issues := preflightIssues(ledgerCtx(true, false, false), root, nil, nested)
	if len(issues) != 1 || !strings.Contains(issues[0].Message, "gst_number") {
		t.Fatalf("a nested page's bindings should be checked too, got: %v", issues)
	}
}

func TestBindingClaimsReadEntityNameOnThePage(t *testing.T) {
	page := map[string]interface{}{
		"id":          "p1",
		"name":        "p1",
		"entity_name": "fn",
		"styles":      map[string]interface{}{},
		"components":  []interface{}{},
	}
	claims := studio.BindingClaims(page)
	if len(claims) != 1 || claims[0].EntityName != "fn" {
		t.Fatalf("expected the page-level entity to count as a claim, got %v", claims)
	}
}
