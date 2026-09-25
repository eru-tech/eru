package reasoning_agents

import (
	"context"
	"strings"
	"testing"

	studio "github.com/eru-tech/eru/eru-ai/agents/eru_studio"
	utility "github.com/eru-tech/eru/eru-ai/tools/utility"
)

func chartBoundTo(queryName string) map[string]interface{} {
	return map[string]interface{}{
		"id": "dash", "name": "dash", "styles": map[string]interface{}{},
		"components": []interface{}{
			map[string]interface{}{"id": "c1", "type": "line_chart",
				"properties": map[string]interface{}{"base": map[string]interface{}{
					"query": queryName, "title": "Collections"}}},
		},
	}
}

func missingQueryCtx(setup func(*studio.Ledger)) context.Context {
	ledger := studio.NewLedger()
	ledger.Offer(utility.RunQueryToolName)
	setup(ledger)
	return studio.WithLedger(context.Background(), ledger)
}

// The defect a held-out fixture found on its first run.
//
// The agent was told twice that db_monthly_collections_summary does not exist
// and bound a chart to it anyway. Nothing objected, because the "not found" had
// been recorded as a run_query FAILURE, and a failed action is unenforceable -
// so the first query that did not exist switched off "do not bind a query you
// have not probed" for the whole turn.
func TestBindingToAQueryThatDoesNotExistIsRejected(t *testing.T) {
	ctx := missingQueryCtx(func(l *studio.Ledger) {
		l.RecordMissingQuery("db_monthly_collections_summary")
	})
	issues := unprobedQueryIssues(ctx, chartBoundTo("db_monthly_collections_summary"), nil)
	if len(issues) != 1 {
		t.Fatalf("expected the binding to be reported, got %+v", issues)
	}
	if issues[0].Code != "query_missing" {
		t.Errorf("code = %q", issues[0].Code)
	}
	// Telling the agent to probe it would send it round a loop it cannot leave.
	if strings.Contains(issues[0].Message, "without running") {
		t.Errorf("a missing query must not be reported as an unprobed one: %s", issues[0].Message)
	}
	for _, want := range []string{"does not exist", "list_queries", "drop the component"} {
		if !strings.Contains(issues[0].Message, want) {
			t.Errorf("the instruction must mention %q: %s", want, issues[0].Message)
		}
	}
}

// The heart of it: a missing query must be caught even though run_query is not
// enforceable, because that is exactly the state a missing query used to create.
func TestAMissingQueryIsCaughtEvenWhenTheToolIsUnenforceable(t *testing.T) {
	ctx := missingQueryCtx(func(l *studio.Ledger) {
		l.RecordMissingQuery("db_nope")
		l.RecordFailure(utility.RunQueryToolName, "connection reset")
	})
	if issues := unprobedQueryIssues(ctx, chartBoundTo("db_nope"), nil); len(issues) != 1 {
		t.Fatalf("a binding to a known-absent query must be reported regardless: %+v", issues)
	}
}

// And the fairness the Enforceable rule exists for is untouched: a genuinely
// broken tool still excuses an unprobed binding.
func TestABrokenToolStillExcusesAnUnprobedBinding(t *testing.T) {
	ctx := missingQueryCtx(func(l *studio.Ledger) {
		l.RecordFailure(utility.RunQueryToolName, "connection refused")
	})
	if issues := unprobedQueryIssues(ctx, chartBoundTo("db_real"), nil); len(issues) != 0 {
		t.Errorf("an agent must not be held to a tool that is down: %+v", issues)
	}
}

func TestAProbedQueryPasses(t *testing.T) {
	ctx := missingQueryCtx(func(l *studio.Ledger) { l.RecordQuery("db_real") })
	if issues := unprobedQueryIssues(ctx, chartBoundTo("db_real"), nil); len(issues) != 0 {
		t.Errorf("a probed binding is the correct state: %+v", issues)
	}
}

// Matching on error text is narrow on purpose: being wrong towards "the tool is
// broken" costs a missed check, being wrong the other way accuses the agent of
// binding to something that does exist.
func TestQueryMissingReadsTheErrorNarrowly(t *testing.T) {
	for _, message := range []string{
		`{"error":"Query db_x not found"}`,
		"query DB_X does not exist",
		"no such query: db_x",
	} {
		if !utility.QueryMissingForTest(message, "db_x") {
			t.Errorf("%q should read as a missing query", message)
		}
	}
	for _, message := range []string{
		"connection refused",
		"timeout while running db_x",
		"db_x returned an error from the database",
		"not found",
	} {
		if utility.QueryMissingForTest(message, "db_x") {
			t.Errorf("%q must NOT read as a missing query", message)
		}
	}
}

// End to end through the real validation entry point, in the mode the live run
// used, because the helper passing in isolation proved nothing about whether the
// page ever reaches it.
func TestTheMissingQueryReachesTheValidator(t *testing.T) {
	ledger := studio.NewLedger()
	ledger.Offer(utility.RunQueryToolName)
	ledger.RecordMissingQuery("db_monthly_collections_summary")

	ctx := studio.WithOutputMode(context.Background(), studio.ModeAuto)
	ctx = studio.WithLedger(ctx, ledger)
	ctx = studio.WithRepairState(ctx, studio.NewRepairState())

	out := map[string]interface{}{
		"mode": "full",
		"page": chartBoundTo("db_monthly_collections_summary"),
	}
	_, issues := eruStudioPageIssues(ctx, out)
	var found bool
	for _, issue := range issues {
		t.Logf("[%s] %.80s", issue.Code, issue.Message)
		if issue.Code == "query_missing" {
			found = true
		}
	}
	if !found {
		t.Errorf("the validator must report the missing-query binding; got %d issue(s)", len(issues))
	}
}
