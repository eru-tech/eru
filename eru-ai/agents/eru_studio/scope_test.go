package eru_studio

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// A page with a shape worth scoping: two sibling sections, each with its own
// subtree, under a common root.
const scopedPageJSON = `{
  "id": "page_1",
  "name": "dash",
  "title": "Dashboard",
  "styles": {"classes": ""},
  "components": [
    {
      "id": "root",
      "type": "flex_container",
      "properties": {"base": {"flex_direction": "column"}},
      "styles": {"classes": "p-4"},
      "children": [
        {
          "id": "tab_one",
          "type": "card",
          "properties": {"base": {"title": "Tab one"}},
          "styles": {"classes": "shadow"},
          "children": [
            {"id": "one_txt", "type": "text", "properties": {"base": {"label": "One"}}, "styles": {"classes": ""}},
            {"id": "one_btn", "type": "button", "properties": {"base": {"label": "Go", "color": "primary"}}, "styles": {"classes": ""},
             "events": [{"id": "e1", "event": "buttonpress", "action": "save-page-data"}]}
          ]
        },
        {
          "id": "tab_two",
          "type": "card",
          "properties": {"base": {"title": "Tab two"}},
          "styles": {"classes": "shadow"},
          "children": [
            {"id": "two_grid", "type": "grid", "properties": {"base": {"entity_name": "invoice"}}, "styles": {"classes": ""}}
          ]
        }
      ]
    }
  ]
}`

func scopedPage(t *testing.T) map[string]interface{} {
	t.Helper()
	var page map[string]interface{}
	if err := json.Unmarshal([]byte(scopedPageJSON), &page); err != nil {
		t.Fatal(err)
	}
	return page
}

func TestParseScopeAcceptsWhateverAClientSends(t *testing.T) {
	cases := []struct {
		name string
		raw  interface{}
		want []string
	}{
		{"a bare id", "tab_one", []string{"tab_one"}},
		{"a comma-separated list", "tab_one, one_btn", []string{"tab_one", "one_btn"}},
		{"a JSON array string", `["tab_one","one_btn"]`, []string{"tab_one", "one_btn"}},
		{"a decoded array", []interface{}{"tab_one"}, []string{"tab_one"}},
		{"an object string", `{"component_ids":["tab_one"]}`, []string{"tab_one"}},
		{"an object with a string list", map[string]interface{}{"component_ids": "tab_one,one_btn"}, []string{"tab_one", "one_btn"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scope, err := ParseScope(tc.raw)
			if err != nil {
				t.Fatal(err)
			}
			if scope.IsEmpty() {
				t.Fatal("the scope came back empty")
			}
			if !reflect.DeepEqual(scope.ComponentIds, tc.want) {
				t.Errorf("component_ids = %v, want %v", scope.ComponentIds, tc.want)
			}
		})
	}

	for _, empty := range []interface{}{nil, "", "  ", "{}", "null", []interface{}{}, map[string]interface{}{}} {
		scope, err := ParseScope(empty)
		if err != nil {
			t.Errorf("ParseScope(%v) errored: %v", empty, err)
		}
		if !scope.IsEmpty() {
			t.Errorf("ParseScope(%v) produced %+v", empty, scope)
		}
	}

	if _, err := ParseScope(`{"component_ids":`); err == nil {
		t.Error("malformed JSON was accepted")
	}
	if _, err := ParseScope(42); err == nil {
		t.Error("a number was accepted as a scope")
	}
}

func TestResolveScopeTakesTheSubtreeAndTheAncestors(t *testing.T) {
	page := scopedPage(t)
	resolved := (&Scope{ComponentIds: []string{"tab_one"}}).Resolve(page)

	if got := sortedSet(resolved.Writable); !reflect.DeepEqual(got, []string{"one_btn", "one_txt", "tab_one"}) {
		t.Errorf("writable = %v - scoping a container should bring its contents", got)
	}
	// Ancestors are shown for context but must not be writable: the model needs
	// the root's direction to lay out inside it, not permission to restyle it.
	if resolved.Writable["root"] {
		t.Error("the ancestor is writable")
	}
	if !resolved.Detailed["root"] {
		t.Error("the ancestor is not shown in full, so the model cannot see what it is laying out inside")
	}
	if resolved.Detailed["two_grid"] {
		t.Error("an unrelated subtree is shown in full")
	}
}

func TestResolveScopeCanExcludeDescendants(t *testing.T) {
	page := scopedPage(t)
	no := false
	resolved := (&Scope{ComponentIds: []string{"tab_one"}, IncludeDescendants: &no}).Resolve(page)
	if got := sortedSet(resolved.Writable); !reflect.DeepEqual(got, []string{"tab_one"}) {
		t.Errorf("writable = %v, want just the container", got)
	}
}

func TestResolveScopeReportsIdsThePageDoesNotHaveButStillAllowsThem(t *testing.T) {
	page := scopedPage(t)
	resolved := (&Scope{ComponentIds: []string{"one_btn", "not_yet_btn"}}).Resolve(page)
	if !reflect.DeepEqual(resolved.Missing, []string{"not_yet_btn"}) {
		t.Errorf("missing = %v", resolved.Missing)
	}
	// A client may scope a component the edit is about to create.
	if !resolved.Writable["not_yet_btn"] {
		t.Error("an id that is not on the page yet is not writable, so the edit cannot create it")
	}
}

func TestPruneKeepsScopeInFullAndReducesTheRest(t *testing.T) {
	page := scopedPage(t)
	resolved := (&Scope{ComponentIds: []string{"tab_one"}}).Resolve(page)
	pruned := Prune(page, resolved)

	full, _ := json.Marshal(page)
	small, _ := json.Marshal(pruned)
	if len(small) >= len(full) {
		t.Errorf("pruning did not shrink the page: %d -> %d bytes", len(full), len(small))
	}

	// In scope: everything.
	button := componentById(pruned, "one_btn")
	if button["properties"] == nil || button["styles"] == nil || button["events"] == nil {
		t.Errorf("an in-scope component lost detail: %v", button)
	}

	// Out of scope: structure only, but still addressable and still nested.
	grid := componentById(pruned, "two_grid")
	if grid == nil {
		t.Fatal("an out-of-scope component disappeared instead of being reduced")
	}
	if grid["properties"] != nil || grid["styles"] != nil {
		t.Errorf("an out-of-scope component kept its detail: %v", grid)
	}
	if grid["id"] != "two_grid" || grid["type"] != "grid" {
		t.Errorf("an out-of-scope component lost its identity: %v", grid)
	}
	if got := childIds(componentById(pruned, "tab_two")); !reflect.DeepEqual(got, []string{"two_grid"}) {
		t.Errorf("the structure outside the scope was not preserved: %v", got)
	}
	if resolved.Omitted != 2 {
		t.Errorf("omitted = %d, want 2 (tab_two and two_grid)", resolved.Omitted)
	}

	// Page-level keys survive: the model still knows which page it is editing.
	if pruned["id"] != "page_1" || pruned["title"] != "Dashboard" {
		t.Errorf("page keys were lost: %v", pruned)
	}

	// And the original is untouched - it is still the base for applying the patch.
	if componentById(page, "two_grid")["properties"] == nil {
		t.Error("pruning mutated the page the patch will be applied to")
	}
}

func TestPruneIsANoOpWithoutAScope(t *testing.T) {
	page := scopedPage(t)
	if got := Prune(page, nil); !reflect.DeepEqual(got, page) {
		t.Error("pruning without a scope changed the page")
	}
	empty := (&Scope{}).Resolve(page)
	if got := Prune(page, empty); !reflect.DeepEqual(got, page) {
		t.Error("pruning with an empty scope changed the page")
	}
}

func TestScopeInstructionsTellTheModelWhatTheSkeletonMeans(t *testing.T) {
	page := scopedPage(t)
	scope := &Scope{ComponentIds: []string{"tab_one"}}
	resolved := scope.Resolve(page)
	Prune(page, resolved)

	instructions := ScopeInstructions(resolved, scope)
	for _, want := range []string{"STRUCTURE ONLY", "one_btn", "Never re-emit", "children_ids"} {
		if !strings.Contains(instructions, want) {
			t.Errorf("the scope note does not mention %q:\n%s", want, instructions)
		}
	}
	if !strings.Contains(instructions, "Do not change page-level keys") {
		t.Error("a scoped edit should not be licensed to rename the page")
	}

	allowed := &Scope{ComponentIds: []string{"tab_one"}, AllowPageProps: true}
	if strings.Contains(ScopeInstructions(allowed.Resolve(page), allowed), "Do not change page-level keys") {
		t.Error("allow_page_props was ignored")
	}
	if got := ScopeInstructions(nil, nil); got != "" {
		t.Errorf("an unscoped edit produced a scope note: %q", got)
	}
}

func TestScopeViolationsRejectEditsOutsideTheScope(t *testing.T) {
	page := scopedPage(t)
	resolved := (&Scope{ComponentIds: []string{"one_btn"}}).Resolve(page)

	cases := []struct {
		name  string
		patch *Patch
		want  string
	}{
		{
			name: "restyling something out of scope",
			patch: &Patch{Upsert: []map[string]interface{}{
				{"id": "two_grid", "type": "grid", "styles": map[string]interface{}{"classes": "hidden"}},
			}},
			want: "two_grid",
		},
		{
			name:  "deleting something out of scope",
			patch: &Patch{Delete: []string{"tab_two"}},
			want:  "outside this edit's scope",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			violations := ScopeViolations(tc.patch, resolved)
			if len(violations) == 0 {
				t.Fatal("an out-of-scope change was allowed")
			}
			if !strings.Contains(strings.Join(violations, " "), tc.want) {
				t.Errorf("violation text does not mention %q: %v", tc.want, violations)
			}
		})
	}
}

func TestScopeViolationsAllowReparentingThroughAnOutOfScopeContainer(t *testing.T) {
	// "Move this button into that panel" needs to touch the panel's child list.
	// That is the edit, not a licence to restyle the panel.
	page := scopedPage(t)
	resolved := (&Scope{ComponentIds: []string{"one_btn"}}).Resolve(page)

	move := &Patch{Upsert: []map[string]interface{}{
		{"id": "one_btn", "type": "button", "parent_id": "tab_two", "index": float64(0)},
		{"id": "tab_two", "type": "card", "children_ids": []interface{}{"one_btn", "two_grid"}},
	}}
	if violations := ScopeViolations(move, resolved); len(violations) > 0 {
		t.Errorf("a reparenting move was rejected: %v", violations)
	}

	// The same container, with anything else changed, is not allowed.
	overreach := &Patch{Upsert: []map[string]interface{}{
		{"id": "tab_two", "type": "card", "children_ids": []interface{}{"one_btn"},
			"properties": map[string]interface{}{"base": map[string]interface{}{"title": "Renamed"}}},
	}}
	violations := ScopeViolations(overreach, resolved)
	if len(violations) != 1 || !strings.Contains(violations[0], "properties") {
		t.Errorf("restyling on the way past was not caught: %v", violations)
	}
}

func TestScopeViolationsAreSilentWithoutAScope(t *testing.T) {
	patch := &Patch{Upsert: []map[string]interface{}{{"id": "anything", "type": "text"}}, Delete: []string{"whatever"}}
	if got := ScopeViolations(patch, nil); got != nil {
		t.Errorf("an unscoped edit was policed: %v", got)
	}
	if got := ScopeViolations(nil, &ResolvedScope{Writable: map[string]bool{"a": true}}); got != nil {
		t.Errorf("a nil patch produced violations: %v", got)
	}
}

func TestReportScopeIsWhatTheClientSeesBack(t *testing.T) {
	page := scopedPage(t)
	resolved := (&Scope{ComponentIds: []string{"tab_one"}}).Resolve(page)
	Prune(page, resolved)

	report := ReportScope(resolved)
	if report == nil {
		t.Fatal("a scoped edit reported no scope")
	}
	if !reflect.DeepEqual(report.ComponentIds, []string{"one_btn", "one_txt", "tab_one"}) {
		t.Errorf("component_ids = %v", report.ComponentIds)
	}
	if report.OmittedFromPrompt != 2 {
		t.Errorf("omitted_from_prompt = %d", report.OmittedFromPrompt)
	}
	if ReportScope(nil) != nil {
		t.Error("an unscoped edit reported a scope")
	}
}
