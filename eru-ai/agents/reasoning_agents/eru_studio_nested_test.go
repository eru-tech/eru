package reasoning_agents

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	studio "github.com/eru-tech/eru/eru-ai/agents/eru_studio"
)

const nestedBasePageJSON = `{
  "id": "invoice_page",
  "name": "invoice",
  "styles": {"classes": "", "responsive_classes": {"base": ""}, "responsive_styles": {"base": {}}, "custom": {}},
  "components": [
    {
      "id": "root",
      "type": "flex_container",
      "properties": {"base": {"flex_direction": "column"}},
      "styles": {"classes": "", "responsive_classes": {"base": ""}, "responsive_styles": {"base": {}}, "custom": {}},
      "children": [
        {"id": "title_txt", "type": "text",
         "properties": {"base": {"label": "Invoice"}},
         "styles": {"classes": "", "responsive_classes": {"base": ""}, "responsive_styles": {"base": {}}, "custom": {}}}
      ]
    }
  ]
}`

func nestedBasePage(t *testing.T) map[string]interface{} {
	t.Helper()
	var page map[string]interface{}
	if err := json.Unmarshal([]byte(nestedBasePageJSON), &page); err != nil {
		t.Fatal(err)
	}
	return page
}

func styleBlock() map[string]interface{} {
	return map[string]interface{}{
		"classes":            "",
		"responsive_classes": map[string]interface{}{"base": ""},
		"responsive_styles":  map[string]interface{}{"base": map[string]interface{}{}},
		"custom":             map[string]interface{}{},
	}
}

// loopTemplateOutput is what the model should produce for "show one card per
// line item": a page_ref that loops, plus the template page it mounts.
func loopTemplateOutput() map[string]interface{} {
	return map[string]interface{}{
		"mode":    studio.ModePatch,
		"summary": "Added a looped line-item list",
		"patch": map[string]interface{}{
			"page_id": "invoice_page",
			"upsert": []interface{}{
				map[string]interface{}{
					"id":   "items_ref",
					"type": "page_ref",
					"properties": map[string]interface{}{"base": map[string]interface{}{
						"page":        "line_item_tpl",
						"loop_source": "field",
						"loop_field":  "items",
					}},
					"styles":    styleBlock(),
					"parent_id": "root",
					"index":     float64(1),
				},
			},
		},
		"pages": []interface{}{
			map[string]interface{}{
				"purpose":        studio.PurposeLoopTemplate,
				"mounted_at":     "items_ref",
				"mount_property": "page",
				"is_new":         true,
				"page": map[string]interface{}{
					"id":     "line_item_tpl",
					"name":   "line_item_tpl",
					"styles": styleBlock(),
					"components": []interface{}{
						map[string]interface{}{
							"id":         "item_desc",
							"type":       "text",
							"properties": map[string]interface{}{"base": map[string]interface{}{"value_source": "field"}},
							"styles":     styleBlock(),
						},
					},
				},
			},
		},
	}
}

func TestNestedPagesComeBackAsSeparateEnvelopes(t *testing.T) {
	base := nestedBasePage(t)

	root, nested, err := resolveStudioOutput(context.Background(), loopTemplateOutput(), base)
	if err != nil {
		t.Fatal(err)
	}

	if root["role"] != studio.RoleRoot {
		t.Errorf("root role = %v", root["role"])
	}
	if root["persist"] != studio.PersistUserAction {
		t.Errorf("persist = %v - the client saves, never the agent", root["persist"])
	}
	if len(nested) != 1 {
		t.Fatalf("expected one nested envelope, got %d", len(nested))
	}

	// The root's manifest is an index, not a second copy.
	manifest, ok := root["pages"].([]interface{})
	if !ok || len(manifest) != 1 {
		t.Fatalf("the root does not advertise its nested pages: %v", root["pages"])
	}
	entry, _ := manifest[0].(map[string]interface{})
	if entry["page_id"] != "line_item_tpl" || entry["purpose"] != studio.PurposeLoopTemplate {
		t.Errorf("manifest entry = %v", entry)
	}
	if entry["mounted_at"] != "items_ref" {
		t.Errorf("the manifest does not say what mounts the page: %v", entry)
	}
	if _, carriesPage := entry["page"]; carriesPage {
		t.Error("the manifest repeats the page instead of indexing it")
	}

	// The nested envelope is a page in its own right, with its own revision.
	page := nested[0]
	if page["role"] != studio.RoleNested || page["mode"] != studio.ModeFull {
		t.Errorf("nested envelope = %v/%v", page["role"], page["mode"])
	}
	if page["page_id"] != "line_item_tpl" || page["parent_page_id"] != "invoice_page" {
		t.Errorf("nested identity = %v", page)
	}
	if page["mounted_at"] != "items_ref" || page["mount_property"] != "page" {
		t.Errorf("the nested envelope does not say where it mounts: %v", page)
	}
	if page["is_new"] != true {
		t.Error("a created page should be marked new so the client opens a new tab")
	}
	if page["revision"] == "" || page["revision"] == root["revision"] {
		t.Errorf("the nested page has no revision of its own: %v", page["revision"])
	}
	if page["persist"] != studio.PersistUserAction {
		t.Errorf("nested persist = %v", page["persist"])
	}
	if _, ok := page["page"].(map[string]interface{}); !ok {
		t.Error("the nested envelope carries no page")
	}
}

func TestRootPageInlinesTheNestedPageForRendering(t *testing.T) {
	base := nestedBasePage(t)

	// Inlining is on by default: the client can render in one pass.
	root, _, err := resolveStudioOutput(context.Background(), loopTemplateOutput(), base)
	if err != nil {
		t.Fatal(err)
	}
	page, _ := root["page"].(map[string]interface{})
	host := findComponent(page, "items_ref")
	if host == nil {
		t.Fatal("the page_ref is not on the resolved page")
	}
	children, _ := host["children"].([]interface{})
	if len(children) != 1 {
		t.Errorf("the nested page was not inlined for rendering: %v", host)
	}

	// Off, the root carries the mount alone.
	off := studio.WithInlineNested(context.Background(), false)
	root, _, err = resolveStudioOutput(off, loopTemplateOutput(), base)
	if err != nil {
		t.Fatal(err)
	}
	page, _ = root["page"].(map[string]interface{})
	if _, hasChildren := findComponent(page, "items_ref")["children"]; hasChildren {
		t.Error("inlining was requested off but the nested page was inlined anyway")
	}
}

func findComponent(page map[string]interface{}, id string) map[string]interface{} {
	components, _ := page["components"].([]interface{})
	var search func(list []interface{}) map[string]interface{}
	search = func(list []interface{}) map[string]interface{} {
		for _, raw := range list {
			component, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			if got, _ := component["id"].(string); got == id {
				return component
			}
			if children, ok := component["children"].([]interface{}); ok {
				if found := search(children); found != nil {
					return found
				}
			}
		}
		return nil
	}
	return search(components)
}

func TestModelInlinedNestedPageIsSplitOutAnyway(t *testing.T) {
	// The model sees nested pages inlined under their host, so it will sometimes
	// answer the same way. That still has to produce a separate page, because
	// that is how it gets saved.
	base := nestedBasePage(t)
	output := map[string]interface{}{
		"mode": studio.ModePatch,
		"patch": map[string]interface{}{
			"upsert": []interface{}{
				map[string]interface{}{
					"id":         "panel_ref",
					"type":       "page_ref",
					"properties": map[string]interface{}{"base": map[string]interface{}{"page": "detail_tpl", "display_type": "side_panel"}},
					"styles":     styleBlock(),
					"parent_id":  "root",
					"children": []interface{}{
						map[string]interface{}{"id": "detail_txt", "type": "text",
							"properties": map[string]interface{}{"base": map[string]interface{}{"label": "Detail"}},
							"styles":     styleBlock()},
					},
				},
			},
		},
	}

	root, nested, err := resolveStudioOutput(context.Background(), output, base)
	if err != nil {
		t.Fatal(err)
	}
	if len(nested) != 1 {
		t.Fatalf("the inlined page was not split out: %d nested pages", len(nested))
	}
	if nested[0]["page_id"] != "detail_tpl" {
		t.Errorf("the split page took the wrong id: %v", nested[0]["page_id"])
	}
	warnings, _ := nested[0]["warnings"].([]interface{})
	if len(warnings) == 0 {
		t.Error("a page inferred from inlining should say so")
	}
	if root["pages"] == nil {
		t.Error("the root does not advertise the split-out page")
	}
}

func TestValidateNestedPagesRejectsAMountThatPointsNowhere(t *testing.T) {
	base := nestedBasePage(t)

	// The template page is sent, but the page_ref's "page" property was never set.
	output := loopTemplateOutput()
	patch, _ := output["patch"].(map[string]interface{})
	upsert, _ := patch["upsert"].([]interface{})
	component, _ := upsert[0].(map[string]interface{})
	component["properties"] = map[string]interface{}{"base": map[string]interface{}{"loop_source": "field", "loop_field": "items"}}

	issues := validateStudioUpdate(output, base, nil)
	if len(issues) == 0 {
		t.Fatal("a page_ref with no page was accepted")
	}
	joined := ""
	for _, issue := range issues {
		joined += issue.Message + " "
	}
	if !strings.Contains(joined, "empty panel") {
		t.Errorf("the issues do not name the failure: %v", issues)
	}
}

func TestValidateNestedPagesRejectsAPageNothingMounts(t *testing.T) {
	base := nestedBasePage(t)
	output := loopTemplateOutput()
	// Drop the page_ref, keep the template.
	output["patch"] = map[string]interface{}{
		"page_id": "invoice_page",
		"upsert": []interface{}{
			map[string]interface{}{"id": "title_txt", "type": "text",
				"properties": map[string]interface{}{"base": map[string]interface{}{"label": "Invoice!"}}},
		},
	}

	issues := validateStudioUpdate(output, base, nil)
	joined := ""
	for _, issue := range issues {
		joined += issue.Message + " "
	}
	if !strings.Contains(joined, "nothing on the page mounts") {
		t.Errorf("an unmounted page was accepted: %v", issues)
	}
}

func TestValidateNestedPagesChecksTheNestedPageItself(t *testing.T) {
	base := nestedBasePage(t)
	output := loopTemplateOutput()
	pages, _ := output["pages"].([]interface{})
	entry, _ := pages[0].(map[string]interface{})
	page, _ := entry["page"].(map[string]interface{})
	page["components"] = []interface{}{
		map[string]interface{}{"id": "bad", "type": "KPICard", "properties": map[string]interface{}{"base": map[string]interface{}{}}, "styles": styleBlock()},
	}

	issues := validateStudioUpdate(output, base, nil)
	joined := ""
	for _, issue := range issues {
		joined += issue.Path + " " + issue.Message + " "
	}
	if !strings.Contains(joined, "KPICard") {
		t.Errorf("an invented component on a nested page was accepted: %v", issues)
	}
	if !strings.Contains(joined, "line_item_tpl") {
		t.Errorf("the issue does not say which page it is on: %v", issues)
	}
}

func TestValidateNestedPageRequiresAMountPoint(t *testing.T) {
	base := nestedBasePage(t)
	output := loopTemplateOutput()
	pages, _ := output["pages"].([]interface{})
	entry, _ := pages[0].(map[string]interface{})
	delete(entry, "mounted_at")

	issues := validateStudioUpdate(output, base, nil)
	joined := ""
	for _, issue := range issues {
		joined += issue.Message + " "
	}
	if !strings.Contains(joined, "which component mounts it") {
		t.Errorf("a nested page with no mount point was accepted: %v", issues)
	}
}

func TestSinglePageModeRejectsAMountItCannotSend(t *testing.T) {
	// In full mode there is nowhere to put a nested page, so a mount pointing at
	// a page that does not exist renders an empty panel.
	agent := &EruStudioAgent{}
	page := map[string]interface{}{
		"id":     "p1",
		"name":   "p",
		"styles": styleBlock(),
		"components": []interface{}{
			map[string]interface{}{
				"id":         "panel_ref",
				"type":       "page_ref",
				"properties": map[string]interface{}{"base": map[string]interface{}{"page": "never_sent_tpl", "display_type": "popup"}},
				"styles":     styleBlock(),
			},
		},
	}
	err := agent.ValidateOutput(context.Background(), page)
	if err == nil {
		t.Fatal("a mount pointing at a page that was never sent was accepted")
	}
	if !strings.Contains(err.Error(), "never_sent_tpl") {
		t.Errorf("the error does not name the missing page: %v", err)
	}

	// A mount onto a page the base already references is fine: it exists.
	base := map[string]interface{}{
		"id": "p1",
		"components": []interface{}{
			map[string]interface{}{"id": "old_ref", "type": "page_ref",
				"properties": map[string]interface{}{"base": map[string]interface{}{"page": "never_sent_tpl"}}},
		},
	}
	ctx := studio.WithBasePage(context.Background(), base)
	if err := agent.ValidateOutput(ctx, page); err != nil {
		t.Errorf("a mount onto an existing page was rejected: %v", err)
	}
}

func TestNestedPagesSchemaTellsTheModelWhatItNeeds(t *testing.T) {
	envelope := (&EruStudioAgent{}).GetOutputSchema(studio.WithOutputMode(context.Background(), studio.ModePatch))
	pages, ok := envelope.Properties["pages"]
	if !ok || pages.Items == nil {
		t.Fatal("the envelope schema cannot express a nested page")
	}
	for _, required := range []string{"page", "purpose", "mounted_at", "mount_property"} {
		if _, ok := pages.Items.Properties[required]; !ok {
			t.Errorf("a nested page entry cannot carry %q", required)
		}
	}
	if strings.Join(pages.Items.Required, ",") != "page,purpose,mounted_at,mount_property" {
		t.Errorf("a nested page entry requires %v - all four are needed to mount it", pages.Items.Required)
	}
	purposes := pages.Items.Properties["purpose"].Enum
	if len(purposes) < 6 {
		t.Errorf("the purpose enum has %d values, want every nesting reason", len(purposes))
	}
}

func TestSystemPromptTeachesNesting(t *testing.T) {
	prompt := studioSystemPrompt()
	if strings.Contains(prompt, nestedPagesPlaceholder) {
		t.Fatal("the nesting guidance placeholder was not substituted")
	}
	for _, want := range []string{
		"MORE THAN ONE PAGE",
		"page_ref.properties.base.page",
		"grid.properties.base.card_page_id",
		"NEVER hard-code",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt does not teach %q", want)
		}
	}
}

func TestPatchInstructionsCoverNestedPages(t *testing.T) {
	instructions := eruStudioModeInstructions(studio.ModePatch)
	for _, want := range []string{"NESTED PAGES", "mounted_at", "mount_property", "empty panel"} {
		if !strings.Contains(instructions, want) {
			t.Errorf("the patch instructions do not cover %q", want)
		}
	}
}

func TestNestedPagesTurnIntoOneActionEach(t *testing.T) {
	base := nestedBasePage(t)
	message := agents.AgentMessage{
		Role: "assistant",
		Actions: []agents.AgentOutputAction{{
			ActionType: agents.ActionTypeAnswer,
			ActionName: "eru_studio",
			Action:     loopTemplateOutput(),
		}},
	}

	out, err := eruStudioResolveEnvelope(context.Background(), message, base)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Actions) != 2 {
		t.Fatalf("expected an action per page, got %d", len(out.Actions))
	}
	if out.Actions[0].Action["role"] != studio.RoleRoot {
		t.Errorf("the first action is not the root page: %v", out.Actions[0].Action["role"])
	}
	if out.Actions[1].Action["role"] != studio.RoleNested {
		t.Errorf("the second action is not a nested page: %v", out.Actions[1].Action["role"])
	}
	if out.Actions[1].ActionName != "eru_studio" {
		t.Errorf("the nested action lost its agent name: %q", out.Actions[1].ActionName)
	}
	for _, action := range out.Actions {
		if action.ActionType != agents.ActionTypeAnswer {
			t.Errorf("action type = %q", action.ActionType)
		}
	}
}

func TestQuestionActionIsLeftAlone(t *testing.T) {
	message := agents.AgentMessage{
		Actions: []agents.AgentOutputAction{{
			ActionType: agents.ActionTypeQuestion,
			ActionName: "eru_studio",
			Action:     map[string]interface{}{"questions": []interface{}{}},
		}},
	}
	out, err := eruStudioResolveEnvelope(context.Background(), message, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Actions) != 1 || out.Actions[0].ActionType != agents.ActionTypeQuestion {
		t.Errorf("a clarification was rewritten as a page: %+v", out.Actions)
	}
}
