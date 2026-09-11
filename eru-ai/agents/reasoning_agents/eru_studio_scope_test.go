package reasoning_agents

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	studio "github.com/eru-tech/eru/eru-ai/agents/eru_studio"
)

const scopeBasePageJSON = `{
  "id": "page_1",
  "name": "dash",
  "styles": {"classes": "", "responsive_classes": {"base": ""}, "responsive_styles": {"base": {}}, "custom": {}},
  "components": [
    {
      "id": "root",
      "type": "flex_container",
      "properties": {"base": {"flex_direction": "column"}},
      "styles": {"classes": "", "responsive_classes": {"base": ""}, "responsive_styles": {"base": {}}, "custom": {}},
      "children": [
        {
          "id": "tab_one",
          "type": "card",
          "properties": {"base": {"title": "Tab one"}},
          "styles": {"classes": "", "responsive_classes": {"base": ""}, "responsive_styles": {"base": {}}, "custom": {}},
          "children": [
            {"id": "one_btn", "type": "button",
             "properties": {"base": {"label": "Go", "color": "primary"}},
             "styles": {"classes": "", "responsive_classes": {"base": ""}, "responsive_styles": {"base": {}}, "custom": {}}}
          ]
        },
        {
          "id": "tab_two",
          "type": "card",
          "properties": {"base": {"title": "Tab two"}},
          "styles": {"classes": "", "responsive_classes": {"base": ""}, "responsive_styles": {"base": {}}, "custom": {}},
          "children": [
            {"id": "two_grid", "type": "grid",
             "properties": {"base": {"entity_name": "invoice"}},
             "styles": {"classes": "", "responsive_classes": {"base": ""}, "responsive_styles": {"base": {}}, "custom": {}}}
          ]
        }
      ]
    }
  ]
}`

func scopeBasePage(t *testing.T) map[string]interface{} {
	t.Helper()
	var page map[string]interface{}
	if err := json.Unmarshal([]byte(scopeBasePageJSON), &page); err != nil {
		t.Fatal(err)
	}
	return page
}

func TestScopedRequestPrunesWhatReachesTheModel(t *testing.T) {
	base := scopeBasePage(t)
	params := map[string]interface{}{
		"code":             scopeBasePageJSON,
		studio.ScopeParam:  "tab_one",
		"some_other_param": "left alone",
	}

	note, resolved, err := applyEruStudioScope(context.Background(), params, base, studio.ModePatch)
	if err != nil {
		t.Fatal(err)
	}
	if resolved == nil {
		t.Fatal("the scope was not resolved")
	}
	if !strings.Contains(note, "STRUCTURE ONLY") {
		t.Errorf("the model was not told what the skeleton means:\n%s", note)
	}

	// params["code"] is what the prompt builder reads, so this is what the model
	// actually sees.
	pruned, _ := params["code"].(string)
	if pruned == "" {
		t.Fatal("code was not rewritten to the pruned page")
	}
	if len(pruned) >= len(scopeBasePageJSON) {
		t.Errorf("the pruned page is not smaller: %d vs %d", len(pruned), len(scopeBasePageJSON))
	}
	if !strings.Contains(pruned, `"label":"Go"`) && !strings.Contains(pruned, `"label": "Go"`) {
		t.Error("the in-scope component lost its detail")
	}
	if strings.Contains(pruned, "invoice") {
		t.Error("an out-of-scope component's properties still reached the model")
	}
	if !strings.Contains(pruned, "two_grid") {
		t.Error("an out-of-scope component vanished instead of being reduced to structure")
	}
	if params["some_other_param"] != "left alone" {
		t.Error("scoping disturbed an unrelated param")
	}
}

func TestScopeAppliesInAutoModeToo(t *testing.T) {
	// A scoped edit with output_mode auto still prunes: the model will patch,
	// because it was given a page.
	base := scopeBasePage(t)
	params := map[string]interface{}{"code": scopeBasePageJSON, studio.ScopeParam: "tab_one"}
	note, resolved, err := applyEruStudioScope(context.Background(), params, base, studio.ModeAuto)
	if err != nil {
		t.Fatal(err)
	}
	if resolved == nil || note == "" {
		t.Fatal("auto mode did not apply the scope")
	}
	if params["code"] == scopeBasePageJSON {
		t.Error("the page was not pruned in auto mode")
	}
}

func TestScopeIsIgnoredWhenTheModelMustEmitAFullPage(t *testing.T) {
	// A full page is regenerated whole. Showing the model a pruned page would
	// lose every component it was not shown, so the scope has to be dropped.
	base := scopeBasePage(t)
	params := map[string]interface{}{"code": scopeBasePageJSON, studio.ScopeParam: "tab_one"}

	note, resolved, err := applyEruStudioScope(context.Background(), params, base, studio.ModeFull)
	if err != nil {
		t.Fatal(err)
	}
	if note != "" || resolved != nil {
		t.Error("the scope was applied in full mode")
	}
	if params["code"] != scopeBasePageJSON {
		t.Error("the page was pruned in full mode - components the model never saw would be lost")
	}
}

func TestScopeWithoutAPageIsAnError(t *testing.T) {
	params := map[string]interface{}{studio.ScopeParam: "tab_one"}
	_, _, err := applyEruStudioScope(context.Background(), params, nil, studio.ModePatch)
	if err == nil {
		t.Fatal("scoping with no page was accepted")
	}
	if !strings.Contains(err.Error(), "nothing to scope") {
		t.Errorf("error = %v", err)
	}
}

func TestUnscopedRequestIsUntouched(t *testing.T) {
	base := scopeBasePage(t)
	params := map[string]interface{}{"code": scopeBasePageJSON}
	note, resolved, err := applyEruStudioScope(context.Background(), params, base, studio.ModePatch)
	if err != nil {
		t.Fatal(err)
	}
	if note != "" || resolved != nil {
		t.Error("an unscoped request produced a scope")
	}
	if params["code"] != scopeBasePageJSON {
		t.Error("an unscoped request had its page rewritten")
	}
}

func TestVerifyBaseRevisionCatchesTheWrongPage(t *testing.T) {
	base := scopeBasePage(t)
	actual := studio.Revision(base)

	if err := verifyBaseRevision(map[string]interface{}{}, base); err != nil {
		t.Errorf("a request without a claimed revision was rejected: %v", err)
	}
	if err := verifyBaseRevision(map[string]interface{}{studio.BaseRevisionParam: actual}, base); err != nil {
		t.Errorf("the correct revision was rejected: %v", err)
	}

	// This is the unsaved-changes case: the client sent a page that is not the
	// one it says it is holding.
	err := verifyBaseRevision(map[string]interface{}{studio.BaseRevisionParam: "r0000deadbeef"}, base)
	if err == nil {
		t.Fatal("a stale base revision was accepted")
	}
	for _, want := range []string{actual, "r0000deadbeef", "unsaved changes"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not mention %q: %v", want, err)
		}
	}

	if err := verifyBaseRevision(map[string]interface{}{studio.BaseRevisionParam: actual}, nil); err == nil {
		t.Error("a revision claim with no page was accepted")
	}
}

func TestValidateStudioUpdateEnforcesTheScope(t *testing.T) {
	base := scopeBasePage(t)
	resolved := (&studio.Scope{ComponentIds: []string{"one_btn"}}).Resolve(base)

	outOfScope := map[string]interface{}{
		"mode": studio.ModePatch,
		"patch": map[string]interface{}{
			"upsert": []interface{}{
				map[string]interface{}{
					"id":         "two_grid",
					"type":       "grid",
					"properties": map[string]interface{}{"base": map[string]interface{}{"entity_name": "payment"}},
				},
			},
		},
	}
	issues := validateStudioUpdate(outOfScope, base, resolved)
	if len(issues) == 0 {
		t.Fatal("a patch outside the scope was accepted")
	}
	if !strings.Contains(issues[0].Message, "two_grid") {
		t.Errorf("the issue does not name the out-of-scope component: %v", issues[0])
	}

	inScope := map[string]interface{}{
		"mode": studio.ModePatch,
		"patch": map[string]interface{}{
			"upsert": []interface{}{
				map[string]interface{}{
					"id":         "one_btn",
					"type":       "button",
					"properties": map[string]interface{}{"base": map[string]interface{}{"color": "warn"}},
				},
			},
		},
	}
	if issues := validateStudioUpdate(inScope, base, resolved); len(issues) > 0 {
		t.Errorf("an in-scope patch was rejected: %v", issues)
	}
}

func TestEnvelopeReportsTheScopeBackToTheClient(t *testing.T) {
	base := scopeBasePage(t)
	resolved := (&studio.Scope{ComponentIds: []string{"one_btn"}}).Resolve(base)
	studio.Prune(base, resolved)

	ctx := studio.WithScope(context.Background(), resolved)
	output := map[string]interface{}{
		"mode": studio.ModePatch,
		"patch": map[string]interface{}{
			"upsert": []interface{}{
				map[string]interface{}{"id": "one_btn", "type": "button",
					"properties": map[string]interface{}{"base": map[string]interface{}{"color": "warn"}}},
			},
		},
	}

	envelope, _, err := resolveStudioOutput(ctx, output, base)
	if err != nil {
		t.Fatal(err)
	}
	scope, ok := envelope["scope"].(map[string]interface{})
	if !ok {
		t.Fatalf("the envelope carries no scope: %v", envelope)
	}
	ids, _ := scope["component_ids"].([]interface{})
	if len(ids) != 1 || ids[0] != "one_btn" {
		t.Errorf("scope.component_ids = %v", ids)
	}
	if scope["omitted_from_prompt"] == nil {
		t.Error("the envelope does not say how much of the page was omitted from the prompt")
	}
}
