package eval

import (
	"strings"
	"testing"

	agents "github.com/eru-tech/eru/eru-ai/agents"
)

func pageRun(components ...map[string]interface{}) Trajectory {
	list := make([]interface{}, 0, len(components))
	for _, c := range components {
		list = append(list, c)
	}
	return Trajectory{
		Actions: []agents.AgentOutputAction{{
			ActionType: agents.ActionTypeAnswer,
			Action: map[string]interface{}{
				"page": map[string]interface{}{"id": "p1", "components": list},
			},
		}},
	}
}

func responsive(values map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"base": values}
}

// The eight tiles that rendered a dash apiece.
func TestTheValuePathCopiedFromTheResultIsCaught(t *testing.T) {
	run := pageRun(map[string]interface{}{
		"id": "tile_iff_amt", "type": "tile",
		"properties": responsive(map[string]interface{}{
			"data_source":         "query",
			"query":               "db_tiles_rff",
			"primary_value_field": "iff_amt",
			"primary_value_path":  "0.iff_amt",
		}),
	})
	reason := PagesBindSoundly().Check(run)
	if reason == "" {
		t.Fatal("the copied row path must be reported")
	}
	if !strings.Contains(reason, "tile_iff_amt") {
		t.Errorf("the failing component should be named: %s", reason)
	}
}

// The charts and grids that walked into a key the response does not carry.
func TestTheInventedResultKeyIsCaught(t *testing.T) {
	run := pageRun(map[string]interface{}{
		"id": "grid_top5", "type": "grid",
		"properties": responsive(map[string]interface{}{
			"data_source":       "query",
			"query":             "db_avg_bal",
			"query_result_path": "result.0.Results",
		}),
	})
	if PagesBindSoundly().Check(run) == "" {
		t.Fatal("query_result_path starting at \"result\" must be reported")
	}
}

// What a sound page looks like: the envelope path where it belongs, and no path
// beside a plain column.
func TestASoundPagePasses(t *testing.T) {
	run := pageRun(
		map[string]interface{}{
			"id": "grid_top5", "type": "grid",
			"properties": responsive(map[string]interface{}{
				"data_source":       "query",
				"query":             "db_avg_bal",
				"query_result_path": "0.Results",
			}),
		},
		map[string]interface{}{
			"id": "tile_iff_amt", "type": "tile",
			"properties": responsive(map[string]interface{}{
				"data_source":         "query",
				"query":               "db_tiles_rff",
				"primary_value_field": "iff_amt",
			}),
		},
	)
	if reason := PagesBindSoundly().Check(run); reason != "" {
		t.Fatalf("a sound page must pass: %s", reason)
	}
}

// A path into a column that really does hold JSON is what the property is for.
func TestAPathIntoAJsonColumnPasses(t *testing.T) {
	run := pageRun(map[string]interface{}{
		"id": "tile_limit", "type": "tile",
		"properties": responsive(map[string]interface{}{
			"data_source":         "query",
			"query":               "db_cust_limit",
			"primary_value_field": "entity_data",
			"primary_value_path":  "limit.sanctioned",
		}),
	})
	if reason := PagesBindSoundly().Check(run); reason != "" {
		t.Fatalf("digging into a JSON column must be allowed: %s", reason)
	}
}

// Unbinding is how the rejected dashboard satisfied the probe rule.
func TestUnboundQueriesAreCaught(t *testing.T) {
	run := pageRun(map[string]interface{}{
		"id": "kpi_inv_fin_amt", "type": "tile",
		"properties": responsive(map[string]interface{}{
			"data_source":         "static",
			"primary_value_field": "amount",
		}),
	})
	reason := BindsEveryQuery("db_tiles_rff", "db_disb").Check(run)
	if reason == "" {
		t.Fatal("a page that binds none of the named queries must fail")
	}
	for _, name := range []string{"db_tiles_rff", "db_disb"} {
		if !strings.Contains(reason, name) {
			t.Errorf("%q should be named: %s", name, reason)
		}
	}
}

func TestBoundQueriesPass(t *testing.T) {
	run := pageRun(map[string]interface{}{
		"id": "kpi", "type": "tile",
		"properties": responsive(map[string]interface{}{
			"data_source": "query", "query": "db_tiles_rff",
		}),
	})
	if reason := BindsEveryQuery("db_tiles_rff").Check(run); reason != "" {
		t.Fatalf("a bound query must pass: %s", reason)
	}
}

// A component reached through a save_page argument is judged the same as one in
// the answer, so the assertion does not depend on where the page surfaced.
func TestComponentsAreFoundInToolArguments(t *testing.T) {
	run := Trajectory{ToolCalls: []ToolCall{{
		Name: "save_page",
		Input: map[string]interface{}{
			"page_def": map[string]interface{}{
				"id": "p1",
				"components": []interface{}{map[string]interface{}{
					"id": "grid", "type": "grid",
					"properties": responsive(map[string]interface{}{
						"data_source":       "query",
						"query":             "db_avg_bal",
						"query_result_path": "result.0.Results",
					}),
				}},
			},
		},
	}}}
	if PagesBindSoundly().Check(run) == "" {
		t.Fatal("a page passed to save_page must be judged too")
	}
}

func TestAGridWithoutColumnLabelsIsCaught(t *testing.T) {
	run := pageRun(map[string]interface{}{
		"id": "grid_top5", "type": "grid",
		"properties": responsive(map[string]interface{}{
			"data_source": "query", "query": "db_avg_bal",
		}),
	})
	if GridsLabelTheirColumns().Check(run) == "" {
		t.Fatal("a query grid with no column labels must be reported")
	}
}

func TestALabelledGridPasses(t *testing.T) {
	run := pageRun(map[string]interface{}{
		"id": "grid_top5", "type": "grid",
		"properties": responsive(map[string]interface{}{
			"data_source": "query", "query": "db_avg_bal",
			"column_overrides": map[string]interface{}{
				"an":      map[string]interface{}{"label": "Anchor"},
				"avg_bal": map[string]interface{}{"label": "Average Balance"},
			},
		}),
	})
	if reason := GridsLabelTheirColumns().Check(run); reason != "" {
		t.Fatalf("a labelled grid must pass: %s", reason)
	}
}

// An entity-backed grid takes its headings from the data model, so it is not
// this rule's business.
func TestAnEntityGridIsNotRequiredToOverride(t *testing.T) {
	run := pageRun(map[string]interface{}{
		"id": "grid_inv", "type": "grid",
		"properties": responsive(map[string]interface{}{
			"data_source": "entity", "entity_name": "invoices",
		}),
	})
	if reason := GridsLabelTheirColumns().Check(run); reason != "" {
		t.Fatalf("an entity grid must pass: %s", reason)
	}
}

// The point of reading the catalog's table rather than re-implementing it: the
// eval inherits rules nobody wrote here.
//
// A board grid with no card_page_id is CodeMountBoardCardUnset - a constraint
// this package never had a copy of. If it is reported, the assertion really is
// driven by the catalog's declarations and not by a private list that happens to
// agree with them today.
func TestTheEvalInheritsCatalogRulesItNeverImplemented(t *testing.T) {
	run := pageRun(map[string]interface{}{
		"id": "board_grid", "type": "grid",
		"properties": responsive(map[string]interface{}{
			"data_source": "entity", "entity_name": "invoices", "view_mode": "board",
		}),
	})
	reason := PagesBindSoundly().Check(run)
	if reason == "" {
		t.Fatal("a board grid with no card_page_id must be reported, from the catalog table")
	}
	if !strings.Contains(reason, "card_page_id") {
		t.Errorf("the catalog's own wording must come through: %s", reason)
	}
	if !strings.Contains(reason, "board_grid") {
		t.Errorf("the failing component must be named: %s", reason)
	}
}

// And a board grid that HAS its card page passes, so the inherited rule is armed
// by the same condition the validator uses rather than firing on every grid.
func TestAnArmedCatalogRuleStillRespectsItsCondition(t *testing.T) {
	run := pageRun(map[string]interface{}{
		"id": "board_grid", "type": "grid",
		"properties": responsive(map[string]interface{}{
			"data_source": "entity", "view_mode": "board", "card_page_id": "card_page",
		}),
	})
	if reason := PagesBindSoundly().Check(run); reason != "" {
		t.Errorf("a board grid with a card page must pass: %s", reason)
	}
	table := pageRun(map[string]interface{}{
		"id": "table_grid", "type": "grid",
		"properties": responsive(map[string]interface{}{
			"data_source": "entity", "view_mode": "table",
		}),
	})
	if reason := PagesBindSoundly().Check(table); reason != "" {
		t.Errorf("a table grid is not a board and must pass: %s", reason)
	}
}
