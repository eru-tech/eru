package reasoning_agents

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"testing"

	catalog "github.com/eru-tech/eru/eru-ai/agents/eru_studio/catalog"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

func init() {
	logs.LogInit("test", "eru-studio-catalog-test")
}

// The Eru Studio prompt teaches the model a component library that lives in
// another repository. These tests hold the two together: everything the prompt
// names has to exist in the generated catalog, and everything the catalog offers
// has to be reachable from the prompt. When one of them fails, the fix is either
// to run agents/eru_studio/catalog/sync.sh or to correct the prompt - never to
// relax the test.

func TestSystemPromptCarriesTheGeneratedLibrary(t *testing.T) {
	prompt := studioSystemPrompt()

	if strings.Contains(prompt, componentLibraryPlaceholder) {
		t.Fatal("the component library placeholder was not substituted")
	}
	if !strings.Contains(prompt, "COMPONENT LIBRARY (generated from eru-studio") {
		t.Fatal("the generated component library is not in the system prompt")
	}
	// Every type the agent may emit has to be reachable from the prompt, or the
	// agent cannot know it exists.
	for _, componentType := range studioCatalog.ComponentTypes() {
		if !strings.Contains(prompt, componentType) {
			t.Errorf("the prompt never mentions %q", componentType)
		}
	}
	// And every retired name has to be called out, or the model will keep
	// reaching for the one it remembers.
	for _, alias := range studioCatalog.DeprecatedAliases() {
		if !strings.Contains(prompt, alias[0]+" -> use "+alias[1]) {
			t.Errorf("the prompt does not warn against the retired name %q", alias[0])
		}
	}
}

// promptTypeListHeadings are the section headings the prose used to declare the
// component library with. The library is generated now, so a heading like this
// coming back means someone has started a second source of truth - which is how
// the prompt ended up teaching call-api and calling list a container.
var promptTypeListHeadings = []string{
	"ALLOWED COMPONENT TYPES",
	"CONTAINER vs LEAF",
}

func TestProseDoesNotRedeclareTheLibrary(t *testing.T) {
	for _, heading := range promptTypeListHeadings {
		if strings.Contains(eruStudioSystemPrompt, heading) {
			t.Errorf("the prose prompt has a %q section again - which types exist, and which take children, "+
				"comes from the generated catalog block. Delete the section, or the two will drift apart.", heading)
		}
	}
}

// TestProseDoesNotContradictTheLibraryOnContainers is the check that would have
// caught the old prompt: it called list, tree, grid_list and divider containers
// while the renderer treats them as leaves.
func TestProseDoesNotContradictTheLibraryOnContainers(t *testing.T) {
	leaves := map[string]bool{}
	for _, componentType := range studioCatalog.ComponentTypes() {
		if !studioCatalog.IsContainer(componentType) {
			leaves[componentType] = true
		}
	}

	for _, line := range strings.Split(eruStudioSystemPrompt, "\n") {
		lower := strings.ToLower(line)
		// Only lines that make a claim about accepting children are interesting;
		// a line that says where a leaf's contents go instead is the point.
		claimsChildren := strings.Contains(lower, "may have children") ||
			strings.Contains(lower, "accept children") ||
			strings.Contains(lower, "are containers")
		if !claimsChildren {
			continue
		}
		for leaf := range leaves {
			if strings.Contains(line, leaf) && !strings.Contains(lower, "not") {
				t.Errorf("the prose calls %q a container, but the library declares it a leaf:\n  %s", leaf, strings.TrimSpace(line))
			}
		}
	}
}

// actionToken matches anything shaped like a runtime action name, which is how
// the prompt writes them. Catching them all is the point: an action the prompt
// teaches that the renderer does not implement is silently dead wiring.
var actionToken = regexp.MustCompile(`\b(?:no|call|fetch|save|set|clear|hide|unhide|show|start|stop|disable|enable|update|refresh|download|step|emit|toggle|open|close|navigate)-[a-z]+(?:-[a-z]+)*\b`)

// notActions are hyphenated words in the prompt's prose that look like actions
// but are not: English, CSS values, and the state-formula function names, which
// share the naming style without being actions.
var notActions = map[string]bool{
	"set-up": true, "call-out": true, "no-op": true, "open-ended": true,
	"close-up": true, "start-of": true, "update-to": true, "show-and": true,
	"stop-gap": true, "fetch-and": true, "save-as": true, "clear-cut": true,
	"no-repeat": true, "no-wrap": true, "no-gutter": true,
}

func isStateFormulaFn(name string) bool {
	for _, iface := range []string{"UpdateStateFormula", "StateFormula"} {
		for _, fn := range studioCatalog.InterfaceEnumStrings(iface, "fn") {
			if fn == name {
				return true
			}
		}
	}
	return false
}

func TestPromptActionsExistInCatalog(t *testing.T) {
	actions := map[string]bool{}
	for _, action := range studioCatalog.EventActions() {
		actions[action] = true
	}

	seen := map[string]bool{}
	for _, match := range actionToken.FindAllString(eruStudioSystemPrompt, -1) {
		if actions[match] || notActions[match] || seen[match] || isStateFormulaFn(match) {
			continue
		}
		seen[match] = true
	}
	if len(seen) == 0 {
		return
	}
	unknown := make([]string, 0, len(seen))
	for action := range seen {
		unknown = append(unknown, action)
	}
	sort.Strings(unknown)
	t.Errorf("the prompt teaches event actions the renderer does not implement: %s\n"+
		"ComponentEventSubscription.action allows only: %s\n"+
		"An action outside that union is wiring the renderer ignores. Fix the prompt, or add the action in eru-studio and re-run sync.sh.",
		strings.Join(unknown, ", "), studioCatalog.EventActionReference())
}

func TestOutputSchemaEnumsComeFromCatalog(t *testing.T) {
	schema := buildEruPageOutputSchema()

	components, ok := schema.Properties["components"]
	if !ok || components.Items == nil {
		t.Fatal("the page schema has no components array")
	}
	typeEnum := components.Items.Properties["type"].Enum
	if len(typeEnum) != len(studioCatalog.ComponentTypes()) {
		t.Errorf("the component type enum has %d entries, the catalog has %d", len(typeEnum), len(studioCatalog.ComponentTypes()))
	}

	events := components.Items.Properties["events"]
	if events.Items == nil {
		t.Fatal("the component schema has no events array")
	}
	// Every field the renderer accepts must be legal in the schema, or a feature
	// the library ships is unreachable for the agent.
	for _, field := range studioCatalog.InterfaceFields("ComponentEventSubscription") {
		if _, ok := events.Items.Properties[field.Name]; !ok {
			t.Errorf("event subscription field %q is missing from the output schema", field.Name)
		}
	}
}

func TestValidateOutputRejectsInventedComponents(t *testing.T) {
	agent := &EruStudioAgent{}
	page := map[string]interface{}{
		"id":     "p1",
		"name":   "test_page",
		"styles": map[string]interface{}{"classes": "", "responsive_classes": map[string]interface{}{}, "responsive_styles": map[string]interface{}{}, "custom": map[string]interface{}{}},
		"components": []interface{}{
			map[string]interface{}{
				"id":         "bar_1",
				"type":       "TopBar",
				"properties": map[string]interface{}{"base": map[string]interface{}{}},
				"styles":     map[string]interface{}{"classes": ""},
			},
		},
	}
	err := agent.ValidateOutput(context.Background(), page)
	if err == nil {
		t.Fatal("an invented component type passed validation")
	}
	if !strings.Contains(err.Error(), "TopBar") {
		t.Errorf("the validation error does not name the offending type: %v", err)
	}
}

func TestValidateOutputRejectsUnknownPropertyAndAction(t *testing.T) {
	agent := &EruStudioAgent{}
	page := map[string]interface{}{
		"id":     "p1",
		"name":   "test_page",
		"styles": map[string]interface{}{"classes": ""},
		"components": []interface{}{
			map[string]interface{}{
				"id":   "btn_1",
				"type": "button",
				"properties": map[string]interface{}{
					"base": map[string]interface{}{"variant": "mat-jumbo", "labl": "Save"},
				},
				"styles": map[string]interface{}{"classes": ""},
				"events": []interface{}{
					map[string]interface{}{"id": "e1", "event": "click", "action": "call-api", "apiName": "save"},
				},
			},
		},
	}
	err := agent.ValidateOutput(context.Background(), page)
	if err == nil {
		t.Fatal("a bad property value, an unknown property key and a dead action all passed validation")
	}
	for _, want := range []string{"mat-jumbo", "labl", "call-api"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the validation error does not mention %q:\n%v", want, err)
		}
	}
}

func TestValidateOutputAcceptsAMinimalValidPage(t *testing.T) {
	agent := &EruStudioAgent{}
	styles := map[string]interface{}{
		"classes":            "",
		"responsive_classes": map[string]interface{}{"base": ""},
		"responsive_styles":  map[string]interface{}{"base": map[string]interface{}{}},
		"custom":             map[string]interface{}{},
	}
	page := map[string]interface{}{
		"id":     "p1",
		"name":   "test_page",
		"styles": styles,
		"components": []interface{}{
			map[string]interface{}{
				"id":         "root_1",
				"type":       "flex_container",
				"properties": map[string]interface{}{"base": map[string]interface{}{"flex_direction": "column"}},
				"styles":     styles,
				"children": []interface{}{
					map[string]interface{}{
						"id":         "btn_1",
						"type":       "button",
						"properties": map[string]interface{}{"base": map[string]interface{}{"variant": "mat-raised-button", "label": "Save"}},
						"styles":     styles,
						"events": []interface{}{
							map[string]interface{}{"id": "e1", "event": "buttonpress", "action": "save-page-data"},
						},
					},
				},
			},
		},
	}
	if err := agent.ValidateOutput(context.Background(), page); err != nil {
		t.Fatalf("a valid page was rejected: %v", err)
	}
}

func TestCatalogIssueTextIsActionable(t *testing.T) {
	issues := studioCatalog.ValidatePage(map[string]interface{}{
		"id": "p1", "name": "n", "styles": map[string]interface{}{},
		"components": []interface{}{
			map[string]interface{}{
				"id": "c1", "type": "list",
				"properties": map[string]interface{}{"base": map[string]interface{}{}},
				"styles":     map[string]interface{}{},
				"children": []interface{}{
					map[string]interface{}{"id": "c2", "type": "text",
						"properties": map[string]interface{}{"base": map[string]interface{}{}},
						"styles":     map[string]interface{}{}},
				},
			},
		},
	})
	text := catalog.FormatIssues(issues, 10)
	if !strings.Contains(text, "cannot have children") {
		t.Errorf("children-on-a-leaf was not reported:\n%s", text)
	}
}
