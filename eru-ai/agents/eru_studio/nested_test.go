package eru_studio

import (
	"reflect"
	"strings"
	"testing"

	catalog "github.com/eru-tech/eru/eru-ai/agents/eru_studio/catalog"
)

// TestPageMountsMatchTheLibrary is the drift guard: the mount properties this
// package teaches the agent about have to exist on those component types, or the
// agent is wiring nested pages through a property the renderer ignores.
func TestPageMountsMatchTheLibrary(t *testing.T) {
	c := catalog.Get()
	for componentType, mounts := range pageMounts {
		if _, ok := c.Component(componentType); !ok {
			t.Errorf("%q is not a component type", componentType)
			continue
		}
		for _, mount := range mounts {
			if _, ok := c.Property(componentType, mount.Property); !ok {
				t.Errorf("%s has no property %q - the mount would be silently ignored by the renderer", componentType, mount.Property)
			}
		}
	}
	// And the other direction: the catalog's own idea of which types host a page
	// must match this one, since validation uses the catalog's.
	if got, want := PageHostTypes(), c.PageHostTypes(); !reflect.DeepEqual(got, want) {
		t.Errorf("page host types disagree: guidance has %v, validation has %v", got, want)
	}
}

func TestPurposeIsReadFromThePageRefsOwnProperties(t *testing.T) {
	cases := []struct {
		name  string
		props map[string]interface{}
		want  string
	}{
		{"a side panel", map[string]interface{}{"display_type": "side_panel"}, PurposeSidePanel},
		{"a popup", map[string]interface{}{"display_type": "popup"}, PurposePopup},
		{"inline", map[string]interface{}{"display_type": "inline"}, PurposeInline},
		{"a loop over data", map[string]interface{}{"loop_source": "data"}, PurposeLoopTemplate},
		{"a nested array", map[string]interface{}{"nesting_type": "nested_array"}, PurposeLoopTemplate},
		{"a loop wins over the display type", map[string]interface{}{"display_type": "popup", "loop_source": "field"}, PurposeLoopTemplate},
		{"nothing set", map[string]interface{}{}, PurposeInline},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := purposeFromProperties(tc.props); got != tc.want {
				t.Errorf("purpose = %q, want %q", got, tc.want)
			}
		})
	}
}

const hostPageJSON = `{
  "id": "invoice_page",
  "name": "invoice",
  "styles": {"classes": ""},
  "components": [
    {
      "id": "root",
      "type": "flex_container",
      "properties": {"base": {}},
      "styles": {"classes": ""},
      "children": [
        {"id": "title_txt", "type": "text", "properties": {"base": {"label": "Invoice"}}, "styles": {"classes": ""}},
        {
          "id": "items_ref",
          "type": "page_ref",
          "properties": {"base": {"page": "line_item_tpl", "loop_source": "field", "loop_field": "items"}},
          "styles": {"classes": ""},
          "children": [
            {"id": "item_desc", "type": "text", "properties": {"base": {"label": "Description"}}, "styles": {"classes": ""}},
            {"id": "item_amt", "type": "currency", "properties": {"base": {}}, "styles": {"classes": ""}}
          ]
        }
      ]
    }
  ]
}`

func TestFindMountsReadsEveryPageReference(t *testing.T) {
	page := mustJSON(t, hostPageJSON)
	mounts := FindMounts(page)
	if len(mounts) != 1 {
		t.Fatalf("expected one mount, got %d: %+v", len(mounts), mounts)
	}
	mount := mounts[0]
	if mount.ComponentId != "items_ref" || mount.PageId != "line_item_tpl" || mount.Property != "page" {
		t.Errorf("mount = %+v", mount)
	}
	if mount.Purpose != PurposeLoopTemplate {
		t.Errorf("purpose = %q - a page_ref with a loop source is a row template", mount.Purpose)
	}
	if len(mount.Inlined) != 2 {
		t.Errorf("the inlined nested page was not seen: %d components", len(mount.Inlined))
	}
}

func TestSplitInlinedPagesSeparatesTheMountedPage(t *testing.T) {
	page := mustJSON(t, hostPageJSON)
	root, nested := SplitInlinedPages(page)

	if len(nested) != 1 {
		t.Fatalf("expected one nested page, got %d", len(nested))
	}
	got := nested[0]
	if got.PageId != "line_item_tpl" {
		t.Errorf("page_id = %q - it should come from the host's page property", got.PageId)
	}
	if got.MountedAt != "items_ref" || got.ParentPageId != "invoice_page" {
		t.Errorf("mount info = %+v", got)
	}
	if got.Purpose != PurposeLoopTemplate {
		t.Errorf("purpose = %q", got.Purpose)
	}
	if got.Persist != PersistUserAction {
		t.Errorf("persist = %q - the client saves it, never the agent", got.Persist)
	}
	components, _ := got.Page["components"].([]interface{})
	if len(components) != 2 {
		t.Errorf("the nested page has %d components, want 2", len(components))
	}

	// The root keeps the mount but not the mounted components.
	host := componentById(root, "items_ref")
	if host == nil {
		t.Fatal("the host disappeared from the root page")
	}
	if _, hasChildren := host["children"]; hasChildren {
		t.Error("the mounted page's components are still under the host")
	}
	if baseProperties(host)["page"] != "line_item_tpl" {
		t.Error("the root lost the mount itself")
	}
	if componentById(root, "title_txt") == nil {
		t.Error("an ordinary component was lost in the split")
	}
}

func TestSplitInlinedPagesNamesAPageThatHadNoId(t *testing.T) {
	// A host with inlined components but no page id still has to produce a page,
	// or the components would be dropped on the floor.
	page := mustJSON(t, `{
	  "id": "p", "name": "p", "styles": {},
	  "components": [{"id": "panel_ref", "type": "page_ref", "properties": {"base": {}}, "styles": {},
	    "children": [{"id": "inner", "type": "text", "properties": {"base": {}}, "styles": {}}]}]
	}`)
	_, nested := SplitInlinedPages(page)
	if len(nested) != 1 {
		t.Fatalf("expected one nested page, got %d", len(nested))
	}
	if nested[0].PageId != "panel_ref_page" {
		t.Errorf("page_id = %q, want a name derived from the host", nested[0].PageId)
	}
}

func TestSplitInlinedPagesLeavesOrdinaryContainersAlone(t *testing.T) {
	page := mustJSON(t, basePageJSON)
	root, nested := SplitInlinedPages(page)
	if len(nested) != 0 {
		t.Errorf("a page with no mounts produced %d nested pages", len(nested))
	}
	if got := childIds(componentById(root, "actions_row")); !reflect.DeepEqual(got, []string{"save_btn", "cancel_btn"}) {
		t.Errorf("an ordinary container's children were disturbed: %v", got)
	}
}

func TestInlineNestedPagesPutsThemBackForRendering(t *testing.T) {
	page := mustJSON(t, hostPageJSON)
	root, nested := SplitInlinedPages(page)

	inlined := InlineNestedPages(root, nested)
	host := componentById(inlined, "items_ref")
	children, _ := host["children"].([]interface{})
	if len(children) != 2 {
		t.Fatalf("the nested page was not inlined back: %v", host)
	}
	// And the split page is untouched: inlining is a convenience, not a move.
	if _, hasChildren := componentById(root, "items_ref")["children"]; hasChildren {
		t.Error("inlining mutated the split root page")
	}
}

func TestInlineNestedPagesMatchesByPageIdWhenTheHostIdIsUnknown(t *testing.T) {
	root := mustJSON(t, `{
	  "id": "p", "name": "p", "styles": {},
	  "components": [{"id": "panel_ref", "type": "page_ref",
	    "properties": {"base": {"page": "detail_tpl", "display_type": "popup"}}, "styles": {}}]
	}`)
	nested := []*NestedPage{{
		PageId: "detail_tpl",
		Page: map[string]interface{}{"id": "detail_tpl", "components": []interface{}{
			map[string]interface{}{"id": "d1", "type": "text"},
		}},
	}}
	inlined := InlineNestedPages(root, nested)
	if children, _ := componentById(inlined, "panel_ref")["children"].([]interface{}); len(children) != 1 {
		t.Error("a nested page whose entry named no host was not matched by page id")
	}
}

func TestMergeNestedPagesPrefersWhatTheModelDeclared(t *testing.T) {
	declared := []*NestedPage{{
		PageId:    "tpl",
		Purpose:   PurposeLoopTemplate,
		MountedAt: "items_ref",
		Page:      map[string]interface{}{"id": "tpl", "components": []interface{}{}},
	}}
	inferred := []*NestedPage{
		{PageId: "tpl", Purpose: PurposeInline, Page: map[string]interface{}{"id": "tpl"}},
		{PageId: "other", Purpose: PurposePopup, Page: map[string]interface{}{"id": "other"}},
	}

	merged := MergeNestedPages(declared, inferred, "root_page")
	if len(merged) != 2 {
		t.Fatalf("expected two pages, got %d", len(merged))
	}
	if merged[0].PageId != "tpl" || merged[0].Purpose != PurposeLoopTemplate {
		t.Errorf("the declared page did not win: %+v", merged[0])
	}
	if merged[0].ParentPageId != "root_page" || merged[0].Persist != PersistUserAction || merged[0].Revision == "" {
		t.Errorf("the declared page was not completed: %+v", merged[0])
	}
	if merged[1].PageId != "other" || len(merged[1].Warnings) == 0 {
		t.Errorf("an inferred page should be reported as such: %+v", merged[1])
	}
}

func TestValidateMountsCatchesTheEmptyPanel(t *testing.T) {
	// A page_ref with no page is the failure that looks like the agent did
	// nothing: the panel opens and it is blank.
	blank := mustJSON(t, `{
	  "id": "p", "name": "p", "styles": {},
	  "components": [{"id": "panel_ref", "type": "page_ref", "properties": {"base": {"display_type": "side_panel"}}, "styles": {}}]
	}`)
	issues := ValidateMounts(blank, nil, nil)
	if len(issues) != 1 || !strings.Contains(issues[0].Message, "empty panel") {
		t.Errorf("a page_ref with no page was not reported: %v", issues)
	}
}

func TestValidateMountsCatchesAMountWithNoPage(t *testing.T) {
	page := mustJSON(t, `{
	  "id": "p", "name": "p", "styles": {},
	  "components": [{"id": "panel_ref", "type": "page_ref", "properties": {"base": {"page": "ghost_tpl"}}, "styles": {}}]
	}`)

	issues := ValidateMounts(page, nil, nil)
	if len(issues) == 0 || !strings.Contains(issues[0].Message, "ghost_tpl") {
		t.Errorf("mounting a page nobody sent was not reported: %v", issues)
	}

	// Unless it is a page that already exists.
	if issues := ValidateMounts(page, nil, map[string]bool{"ghost_tpl": true}); len(issues) > 0 {
		t.Errorf("mounting an existing page was reported: %v", issues)
	}

	// Or one the same answer sent.
	sent := []*NestedPage{{PageId: "ghost_tpl", Page: map[string]interface{}{"id": "ghost_tpl"}}}
	if issues := ValidateMounts(page, sent, nil); len(issues) > 0 {
		t.Errorf("mounting a page sent alongside it was reported: %v", issues)
	}
}

func TestValidateMountsCatchesAnUnmountedPage(t *testing.T) {
	page := mustJSON(t, basePageJSON)
	orphan := []*NestedPage{{PageId: "orphan_tpl", Page: map[string]interface{}{"id": "orphan_tpl"}}}
	issues := ValidateMounts(page, orphan, nil)
	if len(issues) == 0 || !strings.Contains(issues[0].Message, "nothing on the page mounts") {
		t.Errorf("a page nothing mounts was not reported: %v", issues)
	}
}

func TestNestingGuidanceCoversEveryMountAndTheRequiredCases(t *testing.T) {
	guidance := NestingGuidance()
	for _, componentType := range PageHostTypes() {
		for _, mount := range pageMounts[componentType] {
			if !strings.Contains(guidance, componentType+".properties.base."+mount.Property) {
				t.Errorf("the guidance does not mention %s.%s", componentType, mount.Property)
			}
		}
	}
	// The cases the agent gets wrong today, each named explicitly.
	for _, want := range []string{"repeats once per record", "side_panel", "popup", "card_page_id", "NEVER hard-code"} {
		if !strings.Contains(guidance, want) {
			t.Errorf("the guidance does not cover %q", want)
		}
	}
	// And the cases where nesting would be wrong.
	for _, want := range []string{"just a section", "tabs container", "navigate-to-page"} {
		if !strings.Contains(guidance, want) {
			t.Errorf("the guidance does not warn against %q", want)
		}
	}
}
