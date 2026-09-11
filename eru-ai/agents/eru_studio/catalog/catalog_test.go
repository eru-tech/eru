package catalog

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCatalogLoads(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("the embedded catalog does not load: %v", err)
	}
	if c.CatalogVersion == "" {
		t.Error("catalog_version is empty")
	}
	if len(c.Source.Fingerprint) < 8 {
		t.Errorf("source fingerprint looks wrong: %q", c.Source.Fingerprint)
	}
	if len(c.ComponentTypes()) < 40 {
		t.Errorf("only %d component types - the export probably failed", len(c.ComponentTypes()))
	}
	if len(c.CommonProperties) == 0 {
		t.Error("no common properties were exported")
	}
	if len(c.CommonEvents) == 0 {
		t.Error("no common events were exported")
	}
}

// TestEveryComponentExtracted guards the export itself: a component whose schema
// could not be read would silently give the agent an empty property list, and
// the agent would then invent properties for it.
func TestEveryComponentExtracted(t *testing.T) {
	c := Get()
	for name, component := range c.Components {
		if component.Extraction == "failed" {
			t.Errorf("%s: schema extraction failed - re-run the exporter and look at its warnings", name)
		}
		if component.Deprecated {
			if component.AliasOf == "" {
				t.Errorf("%s is deprecated but names no replacement", name)
			}
			continue
		}
		if len(c.EffectiveProperties(name)) == 0 {
			t.Errorf("%s has no properties at all, not even the common ones", name)
		}
	}
}

func TestRequiredInterfacesExported(t *testing.T) {
	c := Get()
	for _, name := range []string{"EruPage", "EruComponent", "ComponentEventSubscription", "ValidationRule", "PageStateVariable"} {
		if len(c.InterfaceFields(name)) == 0 {
			t.Errorf("interface %s was not exported - validation of that node cannot work", name)
		}
	}
	if len(c.EventActions()) < 20 {
		t.Errorf("only %d event actions were exported", len(c.EventActions()))
	}
	if got := c.Breakpoints(); len(got) != 6 {
		t.Errorf("expected 6 breakpoints, got %v", got)
	}
}

func TestEffectivePropertiesRespectExclusionsAndOverrides(t *testing.T) {
	c := Get()
	for name, component := range c.Components {
		props := map[string]bool{}
		for _, property := range c.EffectiveProperties(name) {
			if props[property.Key] {
				t.Errorf("%s: property %q appears twice in the effective set", name, property.Key)
			}
			props[property.Key] = true
		}
		for _, excludedKey := range component.ExcludedCommonProperties {
			// An excluded common property may be re-added by the component itself.
			ownKey := false
			for _, own := range append(component.Properties, component.CommonPropertyOverrides...) {
				if own.Key == excludedKey {
					ownKey = true
				}
			}
			if props[excludedKey] && !ownKey {
				t.Errorf("%s excludes %q but it is still in the effective set", name, excludedKey)
			}
		}
	}
}

func TestEnumValuesOnlyConstrainStaticLists(t *testing.T) {
	c := Get()
	variant, ok := c.Property("button", "variant")
	if !ok {
		t.Fatal("button has no variant property")
	}
	if got := variant.EnumValues(); len(got) != 7 {
		t.Errorf("button.variant should offer 7 fixed values, got %v", got)
	}

	// A dropdown whose options come from an entity or an api cannot be checked
	// against a literal list.
	entityName, ok := c.Property("select-eru", "entity_name")
	if ok && len(entityName.EnumValues()) > 0 {
		t.Errorf("select-eru.entity_name must not be treated as a fixed choice: %v", entityName.EnumValues())
	}
}

func validStyles() map[string]interface{} {
	return map[string]interface{}{
		"classes":            "",
		"responsive_classes": map[string]interface{}{"base": ""},
		"responsive_styles":  map[string]interface{}{"base": map[string]interface{}{}},
		"custom":             map[string]interface{}{},
	}
}

func page(components ...interface{}) map[string]interface{} {
	return map[string]interface{}{
		"id":         "p1",
		"name":       "p",
		"styles":     validStyles(),
		"components": components,
	}
}

func component(id, componentType string, extra map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{
		"id":         id,
		"type":       componentType,
		"properties": map[string]interface{}{"base": map[string]interface{}{}},
		"styles":     validStyles(),
	}
	for key, value := range extra {
		out[key] = value
	}
	return out
}

func issueText(issues []Issue) string { return FormatIssues(issues, 50) }

func TestValidatePageAcceptsAGoodPage(t *testing.T) {
	good := page(component("root", "flex_container", map[string]interface{}{
		"children": []interface{}{
			component("t1", "text", map[string]interface{}{
				"properties": map[string]interface{}{"base": map[string]interface{}{"value_source": "label", "label": "Hello"}},
			}),
		},
	}))
	if issues := Get().ValidatePage(good); len(issues) > 0 {
		t.Fatalf("a valid page was rejected:\n%s", issueText(issues))
	}
}

func TestValidatePageCatchesStructuralMistakes(t *testing.T) {
	cases := []struct {
		name string
		page map[string]interface{}
		want string
	}{
		{
			name: "invented root key",
			page: func() map[string]interface{} {
				p := page()
				p["theme"] = "dark"
				return p
			}(),
			want: "unknown EruPage key",
		},
		{
			name: "invented component type",
			page: page(component("x", "KPICard", nil)),
			want: "unknown component type",
		},
		{
			name: "children on a leaf",
			page: page(component("x", "text", map[string]interface{}{
				"children": []interface{}{component("inner", "text", nil)},
			})),
			want: "cannot have children",
		},
		{
			name: "duplicate ids",
			page: page(component("dup", "text", nil), component("dup", "text", nil)),
			want: "duplicate component id",
		},
		{
			name: "properties not under a breakpoint",
			page: page(component("x", "text", map[string]interface{}{
				"properties": map[string]interface{}{"label": "hi"},
			})),
			want: "must nest values under a breakpoint",
		},
		{
			name: "components stringified",
			page: map[string]interface{}{"id": "p", "name": "p", "styles": validStyles(), "components": "[]"},
			want: "must be a JSON array",
		},
		{
			name: "unknown property",
			page: page(component("x", "button", map[string]interface{}{
				"properties": map[string]interface{}{"base": map[string]interface{}{"buttonLabel": "Go"}},
			})),
			want: "has no property",
		},
		{
			name: "property value outside the option list",
			page: page(component("x", "button", map[string]interface{}{
				"properties": map[string]interface{}{"base": map[string]interface{}{"color": "purple"}},
			})),
			want: "is not allowed",
		},
		{
			name: "unknown action",
			page: page(component("x", "button", map[string]interface{}{
				"events": []interface{}{map[string]interface{}{"id": "e", "event": "click", "action": "call-api"}},
			})),
			want: "unknown action",
		},
		{
			name: "action missing its companion field",
			page: page(component("x", "button", map[string]interface{}{
				"events": []interface{}{map[string]interface{}{"id": "e", "event": "click", "action": "call-function"}},
			})),
			want: "requires \"function_name\"",
		},
		{
			name: "action with no target",
			page: page(component("x", "button", map[string]interface{}{
				"events": []interface{}{map[string]interface{}{"id": "e", "event": "click", "action": "refresh-grid"}},
			})),
			want: "needs \"fieldNames\"",
		},
		{
			name: "event the component cannot emit",
			page: page(component("x", "text", map[string]interface{}{
				"events": []interface{}{map[string]interface{}{"id": "e", "event": "row_select", "action": "no-action"}},
			})),
			want: "does not emit",
		},
		{
			name: "unknown validation type",
			page: page(component("x", "textbox", map[string]interface{}{
				"validation_rules": []interface{}{map[string]interface{}{"type": "notEmpty", "message": "required"}},
			})),
			want: "unknown validation type",
		},
		{
			name: "unknown breakpoint alongside a real one",
			page: page(component("x", "text", map[string]interface{}{
				"properties": map[string]interface{}{
					"base":    map[string]interface{}{},
					"desktop": map[string]interface{}{},
				},
			})),
			want: "unknown breakpoint",
		},
		{
			name: "properties with no breakpoint at all",
			page: page(component("x", "text", map[string]interface{}{
				"properties": map[string]interface{}{"desktop": map[string]interface{}{}},
			})),
			want: "must nest values under a breakpoint",
		},
	}

	c := Get()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			issues := c.ValidatePage(tc.page)
			text := issueText(issues)
			if !strings.Contains(text, tc.want) {
				t.Errorf("expected an issue containing %q, got:\n%s", tc.want, text)
			}
		})
	}
}

// A page host renders the page it mounts in place, so the client sends that
// page's components under it. They are another page's components, not children
// of a leaf, and rejecting them would reject the real client payload.
func TestValidatePageAllowsAnInlinedNestedPageUnderItsHost(t *testing.T) {
	c := Get()
	host := page(component("panel_ref", "page_ref", map[string]interface{}{
		"properties": map[string]interface{}{"base": map[string]interface{}{"page": "detail_page", "display_type": "side_panel"}},
		"children": []interface{}{
			component("detail_txt", "text", map[string]interface{}{
				"properties": map[string]interface{}{"base": map[string]interface{}{"label": "Detail"}},
			}),
		},
	}))
	if issues := c.ValidatePage(host); len(issues) > 0 {
		t.Errorf("an inlined nested page was rejected:\n%s", issueText(issues))
	}

	// An empty children array is what the client actually sends on a page_ref it
	// has not expanded. Reporting it would cost a retry for nothing.
	empty := page(component("panel_ref", "page_ref", map[string]interface{}{"children": []interface{}{}}))
	if issues := c.ValidatePage(empty); len(issues) > 0 {
		t.Errorf("an empty children array was reported:\n%s", issueText(issues))
	}

	// But the components it mounts are still that page's, addressed by id - a
	// page host has no child list of its own to reorder.
	withIds := page(component("panel_ref", "page_ref", map[string]interface{}{
		"children_ids": []interface{}{"detail_txt"},
	}))
	issues := c.ValidatePage(withIds)
	if len(issues) == 0 {
		t.Error("children_ids on a page host was accepted")
	}
}

func TestValidatePageAllowsRuntimeExpressions(t *testing.T) {
	// A value the logic evaluator resolves cannot be matched against an option
	// list, so it must not be reported.
	withExpression := page(component("x", "button", map[string]interface{}{
		"properties": map[string]interface{}{"base": map[string]interface{}{"color": "@state.button_color"}},
	}))
	if issues := Get().ValidatePage(withExpression); len(issues) > 0 {
		t.Errorf("an expression-valued property was reported:\n%s", issueText(issues))
	}
}

func TestValidatePageAllowsStyleKeysTheRendererAccepts(t *testing.T) {
	// StyleProperties carries an index signature, so any CSS-ish key is legal.
	styles := validStyles()
	styles["responsive_styles"] = map[string]interface{}{
		"base": map[string]interface{}{"letter_spacing": "0.05em", "box_shadow": "0 1px 2px rgba(0,0,0,.1)"},
	}
	withStyles := page(component("x", "text", map[string]interface{}{"styles": styles}))
	if issues := Get().ValidatePage(withStyles); len(issues) > 0 {
		t.Errorf("free-form style keys were reported:\n%s", issueText(issues))
	}
}

func TestValidateComponentsAllowsChildIdReferences(t *testing.T) {
	flat := []interface{}{
		component("root", "flex_container", map[string]interface{}{"children_ids": []interface{}{"t1"}}),
		component("t1", "text", nil),
	}
	if issues := Get().ValidateComponents(flat, "upsert"); len(issues) > 0 {
		t.Errorf("a flat patch component list was rejected:\n%s", issueText(issues))
	}
	// children_ids is only legal in a patch, never on a full page.
	onPage := page(component("root", "flex_container", map[string]interface{}{"children_ids": []interface{}{"t1"}}))
	if issues := Get().ValidatePage(onPage); len(issues) == 0 {
		t.Error("children_ids was accepted on a full page")
	}
}

func TestSuggestTypeHelpsWithNearMisses(t *testing.T) {
	c := Get()
	for _, tc := range []struct{ from, want string }{
		{"select", "select-eru"},
		{"checkbox", "checkbox-eru"},
		{"flexcontainer", "flex_container"},
		{"linechart", "line_chart"},
	} {
		if got := c.SuggestType(tc.from); got != tc.want {
			t.Errorf("SuggestType(%q) = %q, want %q", tc.from, got, tc.want)
		}
	}
}

func TestSpecMentionsAllowedValuesAndDefaults(t *testing.T) {
	spec := Get().Spec([]string{"button"})
	for _, want := range []string{"variant", "mat-raised-button", "default \"mat-button\"", "children: NOT allowed"} {
		if !strings.Contains(spec, want) {
			t.Errorf("the button spec does not mention %q:\n%s", want, spec)
		}
	}
}

func TestSpecNamesUnknownTypesClearly(t *testing.T) {
	spec := Get().Spec([]string{"Dropdown"})
	if !strings.Contains(spec, "NOT A COMPONENT TYPE") {
		t.Errorf("an unknown type was not reported as such: %s", spec)
	}
}

func TestContractStaysSmallEnoughForEveryPrompt(t *testing.T) {
	contract := Get().Contract()
	// The whole point of the type index is that it is affordable on every call.
	// The full spec of all types is roughly 20x this; if the index approaches it,
	// something has been inlined that belongs behind get_component_spec.
	if len(contract) > 8000 {
		t.Errorf("the always-on component contract has grown to %d bytes", len(contract))
	}
	for _, componentType := range Get().ComponentTypes() {
		if !strings.Contains(contract, componentType) {
			t.Errorf("the contract does not list %q, so the agent cannot know it exists", componentType)
		}
	}
}

func TestCatalogJSONIsWellFormedAfterRoundTrip(t *testing.T) {
	raw, err := catalogFS.ReadFile(CatalogFile)
	if err != nil {
		t.Fatal(err)
	}
	var generic map[string]interface{}
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("the catalog is not valid JSON: %v", err)
	}
	if warnings, ok := generic["extraction_warnings"].([]interface{}); ok && len(warnings) > 0 {
		t.Errorf("the exporter reported %d warning(s) - the catalog may be incomplete: %v", len(warnings), warnings)
	}
}
