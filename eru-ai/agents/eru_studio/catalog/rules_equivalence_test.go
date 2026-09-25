package catalog

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Moving a check from a hand-written method into the rule table must not change
// what it says.
//
// This is the test that makes the refactor safe. The old methods and the new
// rules are both run over the same corpus and their issues compared exactly -
// code, path and message. A message difference matters as much as a missing
// issue: the message is the only thing the model gets to act on, so a reworded
// one is a behaviour change dressed as a tidy-up.
//
// Delete this file only when the legacy methods below are gone AND a release has
// gone out on the table.

// legacyQueryResultPath is validator.queryResultPath as it stood before the rule
// table absorbed it. Kept here, verbatim, purely as the oracle.
func legacyQueryResultPath(v *validator, path, componentType string, properties map[string]interface{}) {
	raw, ok := properties["query_result_path"]
	if !ok {
		return
	}
	text, ok := raw.(string)
	if !ok || strings.TrimSpace(text) == "" || isExpression(text) {
		return
	}
	head := strings.TrimSpace(strings.Split(strings.TrimSpace(text), ".")[0])
	if head == "" {
		return
	}
	if _, err := strconv.Atoi(head); err == nil {
		return
	}
	for _, known := range []string{"Results", "results", "rows", "data"} {
		if head == known {
			return
		}
	}
	v.add(path, CodeQueryResultPathWrong,
		"%s.query_result_path = %q starts with %q, which is not in the response - a saved query answers with an unnamed array, so there is no such key to step into. "+
			"Leave query_result_path out entirely: a bare array and an eru-ql `[{\"Results\": [...]}]` envelope are both unwrapped automatically. "+
			"Set it only to reach deeper than that, and then start it with the array index, as in \"0.Results\"",
		componentType, text, head)
}

// legacyFieldPaths is validator.fieldPaths as it stood before the rule table
// absorbed it.
func legacyFieldPaths(v *validator, path, componentType string, bag map[string]interface{}) {
	source, _ := bag["data_source"].(string)
	if source != "query" {
		return
	}
	keys := make([]string, 0, len(bag))
	for key := range bag {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		raw := bag[key]
		if !strings.HasSuffix(key, "_path") || key == "query_result_path" || key == "state_result_path" {
			continue
		}
		text, ok := raw.(string)
		if !ok || strings.TrimSpace(text) == "" || isExpression(text) {
			continue
		}
		field, _ := bag[strings.TrimSuffix(key, "_path")+"_field"].(string)
		if strings.TrimSpace(field) == "" {
			continue
		}
		segments := strings.Split(strings.TrimSpace(text), ".")
		_, notAnIndex := strconv.Atoi(strings.TrimSpace(segments[0]))
		startsAtARow := notAnIndex == nil
		repeatsTheField := strings.TrimSpace(segments[len(segments)-1]) == strings.TrimSpace(field)
		if !startsAtARow && !repeatsTheField {
			continue
		}
		v.add(path, CodeFieldPathIsRowPath,
			"%s.%s = %q is a path into the query RESULT, but it is read as a path into the value of %q - which with data_source \"query\" is already that column's value, "+
				"so it walks into nothing and the component renders empty. Leave %s out; set it only when that column itself holds JSON, and then start it at a key inside that JSON",
			componentType, key, text, field, key)
	}
}

// corpus is the set of property bags both implementations are compared over.
// Every shape that has actually appeared in a run belongs here, including the
// ones that must NOT raise.
func corpus() []struct {
	name       string
	component  string
	properties map[string]interface{}
} {
	return []struct {
		name       string
		component  string
		properties map[string]interface{}
	}{
		{"the invented result key", "grid", map[string]interface{}{
			"data_source": "query", "query": "db_avg_bal", "query_result_path": "result.0.Results",
		}},
		{"the sound envelope path", "grid", map[string]interface{}{
			"data_source": "query", "query": "db_avg_bal", "query_result_path": "0.Results",
		}},
		{"a bare Results head", "grid", map[string]interface{}{
			"data_source": "query", "query_result_path": "Results",
		}},
		{"rows head", "grid", map[string]interface{}{"data_source": "query", "query_result_path": "rows"}},
		{"data head", "grid", map[string]interface{}{"data_source": "query", "query_result_path": "data.0"}},
		{"no result path at all", "grid", map[string]interface{}{"data_source": "query"}},
		{"an empty result path", "grid", map[string]interface{}{"data_source": "query", "query_result_path": "  "}},
		{"a runtime expression", "grid", map[string]interface{}{
			"data_source": "query", "query_result_path": "@state.path",
		}},
		{"a template expression", "grid", map[string]interface{}{
			"data_source": "query", "query_result_path": "{{ .path }}",
		}},
		{"a non-string result path", "grid", map[string]interface{}{
			"data_source": "query", "query_result_path": 7,
		}},
		{"a result path on a chart with no data_source", "line_chart", map[string]interface{}{
			"query": "db_disb", "query_result_path": "result.0.Results",
		}},

		{"the row path in a value path", "tile", map[string]interface{}{
			"data_source": "query", "query": "db_tiles_rff",
			"primary_value_field": "iff_amt", "primary_value_path": "0.iff_amt",
		}},
		{"two value paths wrong at once", "tile", map[string]interface{}{
			"data_source": "query", "query": "db_tiles_rff",
			"primary_value_field": "iff_amt", "primary_value_path": "0.iff_amt",
			"secondary_value_field": "iff_cnt", "secondary_value_path": "0.iff_cnt",
		}},
		{"a path repeating its field name", "tile", map[string]interface{}{
			"data_source": "query", "primary_value_field": "amt", "primary_value_path": "row.amt",
		}},
		{"a legitimate path into a JSON column", "tile", map[string]interface{}{
			"data_source": "query", "primary_value_field": "entity_data", "primary_value_path": "limit.sanctioned",
		}},
		{"a path with no matching field", "tile", map[string]interface{}{
			"data_source": "query", "primary_value_path": "0.amt",
		}},
		{"a field with no path", "tile", map[string]interface{}{
			"data_source": "query", "primary_value_field": "iff_amt",
		}},
		{"the same shape on page_data, which is not this rule's business", "tile", map[string]interface{}{
			"data_source": "page_data", "primary_value_field": "amt", "primary_value_path": "0.amt",
		}},
		{"title_path too, not just the value paths", "tile", map[string]interface{}{
			"data_source": "query", "title_field": "an", "title_path": "0.an",
		}},
		{"state_result_path is excluded", "tile", map[string]interface{}{
			"data_source": "query", "state_result_path": "0.rows", "state_result_field": "x",
		}},
		{"both rules firing on one component", "tile", map[string]interface{}{
			"data_source": "query", "query_result_path": "result.0.Results",
			"primary_value_field": "amt", "primary_value_path": "0.amt",
		}},
	}
}

func issuesOf(run func(*validator)) []string {
	v := &validator{c: Get()}
	run(v)
	out := make([]string, 0, len(v.issues))
	for _, issue := range v.issues {
		out = append(out, fmt.Sprintf("%s|%s|%s", issue.Code, issue.Path, issue.Message))
	}
	sort.Strings(out)
	return out
}

func TestTheRuleTableSaysExactlyWhatTheHandWrittenChecksSaid(t *testing.T) {
	for _, entry := range corpus() {
		t.Run(entry.name, func(t *testing.T) {
			legacy := issuesOf(func(v *validator) {
				legacyQueryResultPath(v, "p", entry.component, entry.properties)
				legacyFieldPaths(v, "p", entry.component, entry.properties)
			})
			table := issuesOf(func(v *validator) {
				v.checkRules(ScopeEveryBreakpoint, "p", entry.component, entry.properties)
			})

			if len(legacy) != len(table) {
				t.Fatalf("issue count differs: legacy %d, table %d\nlegacy: %v\ntable:  %v",
					len(legacy), len(table), legacy, table)
			}
			for i := range legacy {
				if legacy[i] != table[i] {
					t.Errorf("issue %d differs.\nlegacy: %s\ntable:  %s", i, legacy[i], table[i])
				}
			}
		})
	}
}

// The two rules that predate kinds must be untouched by the change.
func TestTheOriginalRequiresRulesStillFire(t *testing.T) {
	board := issuesOf(func(v *validator) {
		v.checkRules(ScopeBase, "p", "grid", map[string]interface{}{"view_mode": "board"})
	})
	if len(board) != 1 || !strings.Contains(board[0], string(CodeMountBoardCardUnset)) {
		t.Errorf("a board grid with no card_page_id must still be reported: %v", board)
	}

	set := issuesOf(func(v *validator) {
		v.checkRules(ScopeBase, "p", "grid", map[string]interface{}{"view_mode": "board", "card_page_id": "p2"})
	})
	if len(set) != 0 {
		t.Errorf("a board grid WITH a card page must pass: %v", set)
	}

	table := issuesOf(func(v *validator) {
		v.checkRules(ScopeBase, "p", "grid", map[string]interface{}{"view_mode": "table"})
	})
	if len(table) != 0 {
		t.Errorf("the rule is armed by board only: %v", table)
	}
}

// A rule for every component must not leak into components it was not meant for
// via the AnyComponent wildcard.
func TestAnyComponentStillRespectsTheArmingCondition(t *testing.T) {
	out := issuesOf(func(v *validator) {
		v.checkRules(ScopeEveryBreakpoint, "p", "text", map[string]interface{}{
			"data_source": "static", "primary_value_field": "amt", "primary_value_path": "0.amt",
		})
	})
	if len(out) != 0 {
		t.Errorf("data_source static does not arm the row-path rule: %v", out)
	}
}

// The prompt is generated from the same table, so a new rule reaches the model
// without anyone remembering to write it down.
func TestTheNewRulesReachTheSystemPrompt(t *testing.T) {
	prompt := RulesPrompt()
	for _, want := range []string{"query_result_path", "*_path property", "every component"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt must state %q:\n%s", want, prompt)
		}
	}
}
