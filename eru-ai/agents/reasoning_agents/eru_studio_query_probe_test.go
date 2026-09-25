package reasoning_agents

import (
	"context"
	"strings"
	"testing"

	studio "github.com/eru-tech/eru/eru-ai/agents/eru_studio"
	utility "github.com/eru-tech/eru/eru-ai/tools/utility"
)

func chartPage(query string, source string) map[string]interface{} {
	return map[string]interface{}{
		"id": "dash", "title": "Dash",
		"components": []interface{}{
			map[string]interface{}{
				"id": "c1", "type": "line_chart",
				"properties": map[string]interface{}{"base": map[string]interface{}{
					"query": query, "value_source": source,
				}},
			},
		},
	}
}

func queryLedgerCtx(offer bool, probed ...string) context.Context {
	l := studio.NewLedger()
	if offer {
		l.Offer(utility.RunQueryToolName)
	}
	for _, q := range probed {
		l.RecordQuery(q)
	}
	return studio.WithLedger(context.Background(), l)
}

// The dashboard bound eight queries, never called run_query, and said it had.
func TestBindingAQueryNeverRunIsRejected(t *testing.T) {
	issues := unprobedQueryIssues(queryLedgerCtx(true), chartPage("db_disb", "query"), nil)
	if len(issues) != 1 {
		t.Fatalf("expected one issue, got %+v", issues)
	}
	if !strings.Contains(issues[0].Message, "db_disb") {
		t.Errorf("the issue should name the query: %q", issues[0].Message)
	}
	if !strings.Contains(issues[0].Message, utility.RunQueryToolName) {
		t.Errorf("the issue should say what to call: %q", issues[0].Message)
	}
}

func TestBindingAQueryThatWasRunIsAccepted(t *testing.T) {
	if issues := unprobedQueryIssues(queryLedgerCtx(true, "db_disb"), chartPage("db_disb", "query"), nil); len(issues) != 0 {
		t.Errorf("a probed query must pass, got %+v", issues)
	}
}

// Requiring a tool that was never offered would be a demand the model cannot
// satisfy, so the check stays silent then - the same rule the metadata check follows.
func TestUnprobedQueryCheckIsSilentWhenRunQueryWasNotOffered(t *testing.T) {
	if issues := unprobedQueryIssues(queryLedgerCtx(false), chartPage("db_disb", "query"), nil); len(issues) != 0 {
		t.Errorf("expected silence when run_query was unavailable, got %+v", issues)
	}
}

// A query named on a component that reads from page data or state is not a
// binding, so it is not held to the probe.
func TestQueryNamedButNotReadFromIsNotABinding(t *testing.T) {
	if issues := unprobedQueryIssues(queryLedgerCtx(true), chartPage("db_disb", "state"), nil); len(issues) != 0 {
		t.Errorf("a non-query value_source must not be held to the probe, got %+v", issues)
	}
}

// Tiles say data_source rather than value_source, and were the components that
// silently showed nothing.
func TestTileDataSourceQueryIsAlsoHeldToTheProbe(t *testing.T) {
	page := map[string]interface{}{
		"id": "dash", "title": "Dash",
		"components": []interface{}{
			map[string]interface{}{
				"id": "t1", "type": "tile",
				"properties": map[string]interface{}{"base": map[string]interface{}{
					"query": "db_tiles_os", "data_source": "query",
				}},
			},
		},
	}
	issues := unprobedQueryIssues(queryLedgerCtx(true), page, nil)
	if len(issues) != 1 || !strings.Contains(issues[0].Message, "db_tiles_os") {
		t.Errorf("a tile bound to an unprobed query should be caught, got %+v", issues)
	}
}
