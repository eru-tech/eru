package eru_studio

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func mustJSON(t *testing.T, text string) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("bad test fixture: %v", err)
	}
	return out
}

const basePageJSON = `{
  "id": "page_1",
  "name": "invoice_page",
  "title": "Invoice",
  "styles": {"classes": "", "responsive_classes": {}, "responsive_styles": {}, "custom": {}},
  "state": [{"key": "selected_id", "initial": null}],
  "components": [
    {
      "id": "root",
      "type": "flex_container",
      "properties": {"base": {"flex_direction": "column", "gap": 8}},
      "styles": {"classes": "p-4"},
      "children": [
        {"id": "title_txt", "type": "text", "properties": {"base": {"label": "Invoice"}}, "styles": {"classes": ""}},
        {
          "id": "actions_row",
          "type": "flex_container",
          "properties": {"base": {"flex_direction": "row"}},
          "styles": {"classes": ""},
          "children": [
            {"id": "save_btn", "type": "button", "properties": {"base": {"label": "Save", "color": "primary"}}, "styles": {"classes": ""}},
            {"id": "cancel_btn", "type": "button", "properties": {"base": {"label": "Cancel"}}, "styles": {"classes": ""}}
          ]
        }
      ]
    }
  ]
}`

func componentById(page map[string]interface{}, id string) map[string]interface{} {
	components, _ := page["components"].([]interface{})
	return findIn(components, id)
}

func findIn(components []interface{}, id string) map[string]interface{} {
	for _, raw := range components {
		component, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if got, _ := component["id"].(string); got == id {
			return component
		}
		if children, ok := component["children"].([]interface{}); ok {
			if found := findIn(children, id); found != nil {
				return found
			}
		}
	}
	return nil
}

func childIds(component map[string]interface{}) []string {
	children, _ := component["children"].([]interface{})
	out := []string{}
	for _, raw := range children {
		if child, ok := raw.(map[string]interface{}); ok {
			if id, _ := child["id"].(string); id != "" {
				out = append(out, id)
			}
		}
	}
	return out
}

func TestFlattenNestRoundTripsAPage(t *testing.T) {
	base := mustJSON(t, basePageJSON)
	resolved := Flatten(base).Nest()

	// parent_id is written on the way out, so compare against the input with the
	// same field filled in.
	if got := childIds(componentById(resolved, "root")); !reflect.DeepEqual(got, []string{"title_txt", "actions_row"}) {
		t.Errorf("root children came back as %v", got)
	}
	if got := childIds(componentById(resolved, "actions_row")); !reflect.DeepEqual(got, []string{"save_btn", "cancel_btn"}) {
		t.Errorf("actions_row children came back as %v", got)
	}
	if componentById(resolved, "save_btn") == nil {
		t.Error("a leaf was lost in the round trip")
	}
	if resolved["title"] != "Invoice" {
		t.Errorf("page props were lost: %v", resolved["title"])
	}
}

func TestApplyPatchTouchesOnlyWhatItNames(t *testing.T) {
	base := mustJSON(t, basePageJSON)
	patch := &Patch{
		Upsert: []map[string]interface{}{
			{
				"id":         "save_btn",
				"type":       "button",
				"properties": map[string]interface{}{"base": map[string]interface{}{"color": "warn"}},
			},
		},
	}

	resolved, warnings, err := ApplyPatch(base, patch)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	saveButton := componentById(resolved, "save_btn")
	props, _ := saveButton["properties"].(map[string]interface{})
	baseProps, _ := props["base"].(map[string]interface{})
	if baseProps["color"] != "warn" {
		t.Errorf("the patched property did not land: %v", baseProps)
	}
	// The merge is per level: a property the patch did not mention survives.
	if baseProps["label"] != "Save" {
		t.Errorf("an unmentioned property was dropped: %v", baseProps)
	}
	if got, _ := saveButton["styles"].(map[string]interface{}); got == nil {
		t.Error("styles were dropped by a patch that did not mention them")
	}
	if componentById(resolved, "cancel_btn") == nil || componentById(resolved, "title_txt") == nil {
		t.Error("a sibling the patch never named was lost")
	}
}

func TestApplyPatchAddsAComponentUnderAParent(t *testing.T) {
	base := mustJSON(t, basePageJSON)
	patch := &Patch{
		Upsert: []map[string]interface{}{
			{
				"id":         "delete_btn",
				"type":       "button",
				"properties": map[string]interface{}{"base": map[string]interface{}{"label": "Delete"}},
				"styles":     map[string]interface{}{"classes": ""},
				"parent_id":  "actions_row",
				"index":      float64(1),
			},
		},
	}

	resolved, _, err := ApplyPatch(base, patch)
	if err != nil {
		t.Fatal(err)
	}
	if got := childIds(componentById(resolved, "actions_row")); !reflect.DeepEqual(got, []string{"save_btn", "delete_btn", "cancel_btn"}) {
		t.Errorf("the new component landed in the wrong place: %v", got)
	}
	if parent, _ := componentById(resolved, "delete_btn")["parent_id"].(string); parent != "actions_row" {
		t.Errorf("parent_id was not written: %q", parent)
	}
}

func TestApplyPatchDeletesASubtree(t *testing.T) {
	base := mustJSON(t, basePageJSON)
	resolved, warnings, err := ApplyPatch(base, &Patch{Delete: []string{"actions_row"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	for _, id := range []string{"actions_row", "save_btn", "cancel_btn"} {
		if componentById(resolved, id) != nil {
			t.Errorf("%s survived the delete of its subtree", id)
		}
	}
	if got := childIds(componentById(resolved, "root")); !reflect.DeepEqual(got, []string{"title_txt"}) {
		t.Errorf("root children after the delete: %v", got)
	}
}

func TestApplyPatchWarnsAboutADeleteThatFindsNothing(t *testing.T) {
	base := mustJSON(t, basePageJSON)
	_, warnings, err := ApplyPatch(base, &Patch{Delete: []string{"nope"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected one warning, got %v", warnings)
	}
}

func TestApplyPatchReordersWithChildrenIds(t *testing.T) {
	base := mustJSON(t, basePageJSON)
	patch := &Patch{
		Upsert: []map[string]interface{}{
			{"id": "actions_row", "type": "flex_container", "children_ids": []interface{}{"cancel_btn", "save_btn"}},
		},
	}
	resolved, warnings, err := ApplyPatch(base, patch)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if got := childIds(componentById(resolved, "actions_row")); !reflect.DeepEqual(got, []string{"cancel_btn", "save_btn"}) {
		t.Errorf("children_ids did not reorder: %v", got)
	}
}

func TestApplyPatchKeepsAChildDroppedFromChildrenIdsAddressable(t *testing.T) {
	base := mustJSON(t, basePageJSON)
	patch := &Patch{
		Upsert: []map[string]interface{}{
			{"id": "actions_row", "type": "flex_container", "children_ids": []interface{}{"save_btn"}},
		},
	}
	resolved, _, err := ApplyPatch(base, patch)
	if err != nil {
		t.Fatal(err)
	}
	if componentById(resolved, "cancel_btn") == nil {
		t.Error("a child dropped from children_ids without being deleted disappeared entirely")
	}
	if got := childIds(componentById(resolved, "actions_row")); !reflect.DeepEqual(got, []string{"save_btn"}) {
		t.Errorf("actions_row children: %v", got)
	}
}

func TestApplyPatchBuildsAPageFromNothing(t *testing.T) {
	patch := &Patch{
		PageId:    "new_page",
		PageProps: map[string]interface{}{"name": "fresh_page", "title": "Fresh"},
		Upsert: []map[string]interface{}{
			{"id": "root", "type": "flex_container", "children_ids": []interface{}{"hello_txt"}},
			{"id": "hello_txt", "type": "text", "properties": map[string]interface{}{"base": map[string]interface{}{"label": "Hello"}}},
		},
	}
	resolved, warnings, err := ApplyPatch(nil, patch)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if resolved["id"] != "new_page" || resolved["name"] != "fresh_page" {
		t.Errorf("page props did not land: %v", resolved)
	}
	components, _ := resolved["components"].([]interface{})
	if len(components) != 1 {
		t.Fatalf("expected a single root, got %d: %v", len(components), components)
	}
	if got := childIds(componentById(resolved, "root")); !reflect.DeepEqual(got, []string{"hello_txt"}) {
		t.Errorf("the flat patch did not nest: %v", got)
	}
}

func TestApplyPatchAcceptsANestedSubtree(t *testing.T) {
	// A patch may still send a whole subtree; it is shorthand, not a second
	// protocol.
	base := mustJSON(t, basePageJSON)
	patch := &Patch{
		Upsert: []map[string]interface{}{
			{
				"id":        "footer_row",
				"type":      "flex_container",
				"parent_id": "root",
				"children": []interface{}{
					map[string]interface{}{"id": "footer_txt", "type": "text", "properties": map[string]interface{}{"base": map[string]interface{}{"label": "Total"}}},
				},
			},
		},
	}
	resolved, warnings, err := ApplyPatch(base, patch)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if got := childIds(componentById(resolved, "footer_row")); !reflect.DeepEqual(got, []string{"footer_txt"}) {
		t.Errorf("the nested subtree was not kept: %v", got)
	}
	if got := childIds(componentById(resolved, "root")); len(got) != 3 || got[2] != "footer_row" {
		t.Errorf("root children: %v", got)
	}
}

func TestApplyPatchWarnsAboutAnUnknownChildReference(t *testing.T) {
	base := mustJSON(t, basePageJSON)
	patch := &Patch{
		Upsert: []map[string]interface{}{
			{"id": "root", "type": "flex_container", "children_ids": []interface{}{"title_txt", "ghost_component"}},
		},
	}
	_, warnings, err := ApplyPatch(base, patch)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, warning := range warnings {
		if strings.Contains(warning, "ghost_component") {
			found = true
		}
	}
	if !found {
		t.Errorf("an unresolvable child reference was not reported: %v", warnings)
	}
}

func TestApplyPatchUpdatesPageState(t *testing.T) {
	base := mustJSON(t, basePageJSON)
	patch := &Patch{
		State: &StatePatch{
			Upsert: []map[string]interface{}{
				{"key": "selected_id", "initial": ""},
				{"key": "row_count", "initial": float64(0)},
			},
			Delete: []string{"missing_key"},
		},
	}
	resolved, warnings, err := ApplyPatch(base, patch)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 {
		t.Errorf("expected one warning for the missing state key, got %v", warnings)
	}
	state, _ := resolved["state"].([]interface{})
	if len(state) != 2 {
		t.Fatalf("state should hold two variables, got %v", state)
	}
	first, _ := state[0].(map[string]interface{})
	if first["key"] != "selected_id" || first["initial"] != "" {
		t.Errorf("the existing state variable was not updated in place: %v", first)
	}
}

func TestApplyPatchRefusesToPatchComponentsThroughPageProps(t *testing.T) {
	base := mustJSON(t, basePageJSON)
	patch := &Patch{PageProps: map[string]interface{}{"components": []interface{}{}, "title": "New"}}
	resolved, warnings, err := ApplyPatch(base, patch)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) == 0 {
		t.Error("wholesale replacement of components through page_props was not reported")
	}
	if componentById(resolved, "root") == nil {
		t.Error("page_props.components wiped the page")
	}
	if resolved["title"] != "New" {
		t.Error("the legitimate page prop in the same patch was not applied")
	}
}

func TestRevisionIsStableAndContentAddressed(t *testing.T) {
	first := mustJSON(t, basePageJSON)
	second := mustJSON(t, basePageJSON)
	if Revision(first) != Revision(second) {
		t.Error("the same page hashed to two revisions")
	}
	if Revision(first) == "" {
		t.Error("an empty revision for a real page")
	}

	// Timestamps must not change the identity of a page.
	componentById(second, "save_btn")["updated_at"] = "2026-09-08T10:00:00Z"
	if Revision(first) != Revision(second) {
		t.Error("a timestamp changed the page revision")
	}

	changed := mustJSON(t, basePageJSON)
	props, _ := componentById(changed, "save_btn")["properties"].(map[string]interface{})
	baseProps, _ := props["base"].(map[string]interface{})
	baseProps["color"] = "warn"
	if Revision(first) == Revision(changed) {
		t.Error("a real change did not change the revision")
	}
}

func TestPatchIsEmptyAndTouchedIds(t *testing.T) {
	if !(&Patch{}).IsEmpty() {
		t.Error("an empty patch did not report itself as empty")
	}
	var nilPatch *Patch
	if !nilPatch.IsEmpty() {
		t.Error("a nil patch did not report itself as empty")
	}
	patch := &Patch{Upsert: []map[string]interface{}{{"id": "a"}, {"id": "b"}, {"no_id": true}}}
	if patch.IsEmpty() {
		t.Error("a patch with upserts reported itself as empty")
	}
	if got := patch.TouchedIds(); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("TouchedIds = %v", got)
	}
}

func TestParsePatchReadsTheModelOutput(t *testing.T) {
	output := mustJSON(t, `{
	  "mode": "patch",
	  "patch": {
	    "page_id": "p1",
	    "upsert": [{"id": "a", "type": "text"}],
	    "delete": ["b"],
	    "state": {"upsert": [{"key": "k", "initial": 1}]}
	  }
	}`)
	patch, err := ParsePatch(output)
	if err != nil {
		t.Fatal(err)
	}
	if patch.PageId != "p1" || len(patch.Upsert) != 1 || len(patch.Delete) != 1 || patch.State == nil {
		t.Errorf("the patch did not parse: %+v", patch)
	}
	if _, err := ParsePatch(map[string]interface{}{}); err == nil {
		t.Error("output with no patch section parsed as a patch")
	}
}
