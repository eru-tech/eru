package reasoning_agents

import (
	"context"
	"strings"
	"testing"

	studio "github.com/eru-tech/eru/eru-ai/agents/eru_studio"
	catalog "github.com/eru-tech/eru/eru-ai/agents/eru_studio/catalog"
)

func probedLedger(ctx context.Context, names ...string) context.Context {
	ledger := studio.NewLedger()
	for _, name := range names {
		ledger.RecordQuery(name)
	}
	return studio.WithLedger(ctx, ledger)
}

func tilePage(source, query string) map[string]interface{} {
	properties := map[string]interface{}{"data_source": source}
	if query != "" {
		properties["query"] = query
	}
	return map[string]interface{}{
		"id": "p1", "name": "dash", "styles": map[string]interface{}{},
		"components": []interface{}{
			map[string]interface{}{
				"id": "kpi", "type": "tile", "styles": map[string]interface{}{},
				"properties": map[string]interface{}{"base": properties},
			},
		},
	}
}

// The rejected dashboard satisfied "probe before you bind" by unbinding every
// tile it had just probed for.
func TestUnbindingIsNotAWayToPassTheProbeRule(t *testing.T) {
	ctx := probedLedger(context.Background(), "db_tiles_rff", "db_disb")
	issues := unboundProbedQueryIssues(ctx, tilePage("static", ""), nil)
	if len(issues) != 1 {
		t.Fatalf("expected the dropped bindings to be reported, got %d issue(s)", len(issues))
	}
	if issues[0].Code != catalog.CodeQueryProbedNotBound {
		t.Fatalf("unexpected code %q", issues[0].Code)
	}
	for _, name := range []string{"db_tiles_rff", "db_disb"} {
		if !strings.Contains(issues[0].Message, name) {
			t.Errorf("%q should be named in the message", name)
		}
	}
}

func TestAProbedQueryThatIsBoundPasses(t *testing.T) {
	ctx := probedLedger(context.Background(), "db_tiles_rff")
	if issues := unboundProbedQueryIssues(ctx, tilePage("query", "db_tiles_rff"), nil); len(issues) != 0 {
		t.Fatalf("a bound query must not be reported: %v", issues)
	}
}

// Probing nothing is the ordinary case for a page with no query on it.
func TestNoProbesNoIssue(t *testing.T) {
	ctx := studio.WithLedger(context.Background(), studio.NewLedger())
	if issues := unboundProbedQueryIssues(ctx, tilePage("page_data", ""), nil); len(issues) != 0 {
		t.Fatalf("expected no issues, got %v", issues)
	}
}

// A chart binds a query by naming it and nothing else - it has no data_source
// property to declare. Requiring one hid every chart from both query rules.
func TestAChartBindingCounts(t *testing.T) {
	page := map[string]interface{}{
		"id": "p1", "name": "dash", "styles": map[string]interface{}{},
		"components": []interface{}{
			map[string]interface{}{
				"id": "trend", "type": "line_chart", "styles": map[string]interface{}{},
				"properties": map[string]interface{}{
					"base": map[string]interface{}{"query": "db_disb", "query_result_path": "0.Results"},
				},
			},
		},
	}
	if names := boundQueryNames(page); len(names) != 1 || names[0] != "db_disb" {
		t.Fatalf("a chart's query must count as bound, got %v", names)
	}
	ctx := probedLedger(context.Background(), "db_disb")
	if issues := unboundProbedQueryIssues(ctx, page, nil); len(issues) != 0 {
		t.Fatalf("a chart bound to its probed query must not be reported: %v", issues)
	}
}

// A component pointed at page data is still not a query binding, even when a
// query name is left on it.
func TestAPageDataComponentIsNotAQueryBinding(t *testing.T) {
	page := map[string]interface{}{
		"id": "p1", "name": "dash", "styles": map[string]interface{}{},
		"components": []interface{}{
			map[string]interface{}{
				"id": "kpi", "type": "tile", "styles": map[string]interface{}{},
				"properties": map[string]interface{}{
					"base": map[string]interface{}{"data_source": "page_data", "query": "db_disb"},
				},
			},
		},
	}
	if names := boundQueryNames(page); len(names) != 0 {
		t.Fatalf("page_data must not count as a query binding, got %v", names)
	}
}
