package reasoning_agents

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	studio "github.com/eru-tech/eru/eru-ai/agents/eru_studio"
)

const existingPageJSON = `{
  "id": "page_1",
  "name": "invoice_page",
  "styles": {"classes": "", "responsive_classes": {"base": ""}, "responsive_styles": {"base": {}}, "custom": {}},
  "components": [
    {
      "id": "root",
      "type": "flex_container",
      "properties": {"base": {"flex_direction": "column"}},
      "styles": {"classes": "", "responsive_classes": {"base": ""}, "responsive_styles": {"base": {}}, "custom": {}},
      "children": [
        {
          "id": "save_btn",
          "type": "button",
          "properties": {"base": {"label": "Save", "color": "primary"}},
          "styles": {"classes": "", "responsive_classes": {"base": ""}, "responsive_styles": {"base": {}}, "custom": {}}
        }
      ]
    }
  ]
}`

func existingPage(t *testing.T) map[string]interface{} {
	t.Helper()
	var page map[string]interface{}
	if err := json.Unmarshal([]byte(existingPageJSON), &page); err != nil {
		t.Fatal(err)
	}
	return page
}

func TestNegotiateModeDefaultsToTheOldBehaviour(t *testing.T) {
	cases := []struct {
		requested string
		hasBase   bool
		want      string
	}{
		{"", false, studio.ModeFull},
		{"", true, studio.ModeFull},
		{"full", true, studio.ModeFull},
		{"patch", false, studio.ModePatch},
		{"patch", true, studio.ModePatch},
		// "auto" stays "auto" whether or not a page was sent: the model chooses
		// patch or full per turn, and either way it may return nested pages.
		{"auto", false, studio.ModeAuto},
		{"auto", true, studio.ModeAuto},
		{"AUTO", true, studio.ModeAuto},
		{"nonsense", true, studio.ModeFull},
	}
	for _, tc := range cases {
		if got := studio.NegotiateMode(tc.requested, tc.hasBase); got != tc.want {
			t.Errorf("NegotiateMode(%q, base=%t) = %q, want %q", tc.requested, tc.hasBase, got, tc.want)
		}
	}
}

// TestAuthoringAPageStillAllowsNestedPages is the regression for the bug this
// test file's neighbour created: a request to build a page from scratch with
// output_mode "auto" was collapsed to the bare-page protocol, so the agent could
// not return the row template a repeatable section needs - and correctly told the
// user the section could not be built.
func TestAuthoringAPageStillAllowsNestedPages(t *testing.T) {
	// A fresh build: output_mode auto, no page in code.
	mode := studio.NegotiateMode("auto", false)
	ctx := studio.WithOutputMode(context.Background(), mode)

	if !studio.EnvelopeEnabled(ctx) {
		t.Fatal("authoring with output_mode auto cannot return an envelope, so nested pages are impossible")
	}
	if studio.PatchExpected(ctx) {
		t.Error("authoring from scratch should not be forced to patch")
	}

	schema := (&EruStudioAgent{}).GetOutputSchema(ctx)
	if _, ok := schema.Properties["pages"]; !ok {
		t.Fatal("the schema cannot express a nested page")
	}
	if _, ok := schema.Properties["page"]; !ok {
		t.Error("the schema cannot express a full page")
	}

	instructions := eruStudioModeInstructions(mode)
	for _, want := range []string{"you choose the shape", "may return nested pages", "does not restrict"} {
		if !strings.Contains(instructions, want) {
			t.Errorf("the auto instructions do not say %q:\n%s", want, instructions)
		}
	}
	// The sentence that produced the clarification in the screenshot must not
	// reach a client that can read the envelope.
	if strings.Contains(instructions, "exactly ONE page") {
		t.Error("auto mode tells the model it can only send one page")
	}
}

// A full page from scratch, with its row template alongside, is what the agent
// should now produce for "a form with repeatable rows".
func TestAuthoringWithATemplatePageResolves(t *testing.T) {
	ctx := studio.WithOutputMode(context.Background(), studio.ModeAuto)
	output := map[string]interface{}{
		"mode": studio.ModeFull,
		"page": map[string]interface{}{
			"id":     "business_card_capture",
			"name":   "business_card_capture",
			"styles": styleBlock(),
			"components": []interface{}{
				map[string]interface{}{
					"id":         "contacts_ref",
					"type":       "page_ref",
					"properties": map[string]interface{}{"base": map[string]interface{}{"page": "contact_row_tpl", "loop_source": "field", "loop_field": "contact_details"}},
					"styles":     styleBlock(),
				},
			},
		},
		"pages": []interface{}{
			map[string]interface{}{
				"purpose":        studio.PurposeLoopTemplate,
				"mounted_at":     "contacts_ref",
				"mount_property": "page",
				"is_new":         true,
				"page": map[string]interface{}{
					"id":     "contact_row_tpl",
					"name":   "contact_row_tpl",
					"styles": styleBlock(),
					"components": []interface{}{
						map[string]interface{}{"id": "contact_type_sel", "type": "select-eru",
							"properties": map[string]interface{}{"base": map[string]interface{}{"static_options": "Phone,Mobile,Email"}},
							"styles":     styleBlock()},
					},
				},
			},
		},
	}

	if issues := validateStudioUpdate(output, nil, nil); len(issues) > 0 {
		t.Fatalf("a page authored with its row template was rejected: %v", issues)
	}

	root, nested, err := resolveStudioOutput(ctx, output, nil)
	if err != nil {
		t.Fatal(err)
	}
	if root["mode"] != studio.ModeFull || root["role"] != studio.RoleRoot {
		t.Errorf("root envelope = %v/%v", root["mode"], root["role"])
	}
	if len(nested) != 1 {
		t.Fatalf("expected the template page as its own action, got %d", len(nested))
	}
	if nested[0]["page_id"] != "contact_row_tpl" || nested[0]["purpose"] != studio.PurposeLoopTemplate {
		t.Errorf("nested envelope = %v", nested[0])
	}
	if nested[0]["is_new"] != true {
		t.Error("a created template should be marked new so the client opens a new tab")
	}
}

func TestOutputSchemaFollowsTheNegotiatedMode(t *testing.T) {
	agent := &EruStudioAgent{}

	full := agent.GetOutputSchema(context.Background())
	if _, ok := full.Properties["components"]; !ok {
		t.Error("without patch mode the schema should still be a bare EruPage")
	}

	patchCtx := studio.WithOutputMode(context.Background(), studio.ModePatch)
	envelope := agent.GetOutputSchema(patchCtx)
	if _, ok := envelope.Properties["patch"]; !ok {
		t.Fatal("patch mode did not produce the envelope schema")
	}
	if _, ok := envelope.Properties["page"]; !ok {
		t.Error("the envelope schema must still allow a full page")
	}
	patch := envelope.Properties["patch"]
	upsert := patch.Properties["upsert"]
	if upsert.Items == nil {
		t.Fatal("the patch has no upsert item schema")
	}
	if _, ok := upsert.Items.Properties["children_ids"]; !ok {
		t.Error("a patch component cannot reference its children by id")
	}
	// A patch entry is partial, so only identity is required.
	if got := strings.Join(upsert.Items.Required, ","); got != "id,type" {
		t.Errorf("a patch component requires %q, want \"id,type\"", got)
	}
	if _, ok := patch.Properties["page_props"].Properties["components"]; ok {
		t.Error("page_props must not offer a wholesale components replacement")
	}
}

func TestResolveStudioOutputBuildsAPatchEnvelope(t *testing.T) {
	base := existingPage(t)
	output := map[string]interface{}{
		"mode":    studio.ModePatch,
		"summary": "Made Save destructive",
		"patch": map[string]interface{}{
			"page_id": "page_1",
			"upsert": []interface{}{
				map[string]interface{}{
					"id":         "save_btn",
					"type":       "button",
					"properties": map[string]interface{}{"base": map[string]interface{}{"color": "warn"}},
				},
			},
		},
	}

	resolved, _, err := resolveStudioOutput(context.Background(), output, base)
	if err != nil {
		t.Fatal(err)
	}
	if resolved["mode"] != studio.ModePatch {
		t.Errorf("mode = %v", resolved["mode"])
	}
	if resolved["protocol_version"] != studio.ProtocolVersion {
		t.Errorf("protocol_version = %v", resolved["protocol_version"])
	}
	if resolved["base_revision"] != studio.Revision(base) {
		t.Error("base_revision does not identify the page the patch was applied to")
	}
	if resolved["revision"] == resolved["base_revision"] {
		t.Error("the resolved page has the same revision as its base")
	}
	if resolved["page_id"] != "page_1" {
		t.Errorf("page_id = %v", resolved["page_id"])
	}
	if resolved["summary"] != "Made Save destructive" {
		t.Errorf("the summary was dropped: %v", resolved["summary"])
	}

	// The envelope carries the resolved page, so a consumer that cannot apply
	// patches has something to render.
	page, ok := resolved["page"].(map[string]interface{})
	if !ok {
		t.Fatal("the envelope has no resolved page")
	}
	encoded, _ := json.Marshal(page)
	if !strings.Contains(string(encoded), `"color":"warn"`) {
		t.Error("the patch did not reach the resolved page")
	}
	if !strings.Contains(string(encoded), `"label":"Save"`) {
		t.Error("an unpatched property was lost from the resolved page")
	}
	if _, ok := resolved["patch"]; !ok {
		t.Error("the envelope does not carry the patch itself")
	}
}

func TestResolveStudioOutputPassesAFullPageThrough(t *testing.T) {
	base := existingPage(t)
	output := map[string]interface{}{"mode": studio.ModeFull, "page": base}

	resolved, _, err := resolveStudioOutput(context.Background(), output, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resolved["mode"] != studio.ModeFull {
		t.Errorf("mode = %v", resolved["mode"])
	}
	if got, present := resolved["base_revision"]; present {
		t.Errorf("a full page authored from nothing should carry no base revision, got %v", got)
	}
	if resolved["revision"] != studio.Revision(base) {
		t.Error("the revision does not identify the page that was sent")
	}
}

func TestResolveStudioOutputRejectsAnUnusableAnswer(t *testing.T) {
	cases := []struct {
		name   string
		output map[string]interface{}
		want   string
	}{
		{"no mode", map[string]interface{}{"page": map[string]interface{}{}}, "must be"},
		{"patch mode with no patch", map[string]interface{}{"mode": "patch"}, "could not be read"},
		{"patch that changes nothing", map[string]interface{}{"mode": "patch", "patch": map[string]interface{}{"page_id": "p"}}, "changes nothing"},
		{"full mode with no page", map[string]interface{}{"mode": "full"}, "\"page\" is missing"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := resolveStudioOutput(context.Background(), tc.output, nil)
			if err == nil {
				t.Fatal("an unusable answer was accepted")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestValidateStudioUpdateChecksTheResolvedPageNotJustThePatch(t *testing.T) {
	base := existingPage(t)

	// Individually the patch entry is fine; it is only wrong once applied,
	// because the button it moves under is a leaf.
	output := map[string]interface{}{
		"mode": studio.ModePatch,
		"patch": map[string]interface{}{
			"upsert": []interface{}{
				map[string]interface{}{
					"id":         "extra_txt",
					"type":       "text",
					"parent_id":  "save_btn",
					"properties": map[string]interface{}{"base": map[string]interface{}{"label": "hi"}},
					"styles":     map[string]interface{}{"classes": ""},
				},
			},
		},
	}
	issues := validateStudioUpdate(output, base, nil)
	found := false
	for _, issue := range issues {
		if strings.Contains(issue.Message, "cannot have children") {
			found = true
		}
	}
	if !found {
		t.Errorf("a patch that gives a leaf a child was accepted: %v", issues)
	}
}

func TestValidateStudioUpdateAcceptsAGoodPatch(t *testing.T) {
	base := existingPage(t)
	output := map[string]interface{}{
		"mode": studio.ModePatch,
		"patch": map[string]interface{}{
			"upsert": []interface{}{
				map[string]interface{}{
					"id":         "save_btn",
					"type":       "button",
					"properties": map[string]interface{}{"base": map[string]interface{}{"color": "warn"}},
				},
			},
		},
	}
	if issues := validateStudioUpdate(output, base, nil); len(issues) > 0 {
		t.Errorf("a valid patch was rejected: %v", issues)
	}
}

func TestValidateStudioUpdateCatchesAnInventedPropertyInAPatch(t *testing.T) {
	base := existingPage(t)
	output := map[string]interface{}{
		"mode": studio.ModePatch,
		"patch": map[string]interface{}{
			"upsert": []interface{}{
				map[string]interface{}{
					"id":         "save_btn",
					"type":       "button",
					"properties": map[string]interface{}{"base": map[string]interface{}{"buttonColour": "red"}},
				},
			},
		},
	}
	issues := validateStudioUpdate(output, base, nil)
	if len(issues) == 0 {
		t.Fatal("an invented property in a patch was accepted")
	}
	if !strings.Contains(issues[0].Message, "buttonColour") {
		t.Errorf("the issue does not name the property: %v", issues[0])
	}
}

func TestBasePageFromParamsAcceptsBothShapes(t *testing.T) {
	fromString := basePageFromParams(map[string]interface{}{"code": existingPageJSON})
	if fromString["id"] != "page_1" {
		t.Errorf("a stringified page was not decoded: %v", fromString)
	}
	fromObject := basePageFromParams(map[string]interface{}{"code": map[string]interface{}{"id": "page_2"}})
	if fromObject["id"] != "page_2" {
		t.Errorf("an object page was not accepted: %v", fromObject)
	}
	for _, empty := range []interface{}{"", "{}", "null", "not json", nil} {
		if got := basePageFromParams(map[string]interface{}{"code": empty}); got != nil {
			t.Errorf("%v was decoded as a page: %v", empty, got)
		}
	}
	if got := basePageFromParams(nil); got != nil {
		t.Errorf("nil params produced %v", got)
	}
}

func TestPatchModeInstructionsReachTheModel(t *testing.T) {
	if instructions := eruStudioModeInstructions(studio.ModePatch); !strings.Contains(instructions, "FLAT") {
		t.Error("the patch instructions do not tell the model the upsert list is flat")
	}
	// Single-page mode has to say so: a mount pointing at a page it cannot send
	// renders an empty panel, which reads as the agent having done nothing.
	full := eruStudioModeInstructions(studio.ModeFull)
	if !strings.Contains(full, "ONE page") {
		t.Errorf("full mode does not tell the model it can only send one page:\n%s", full)
	}
	if !strings.Contains(full, "page_ref") {
		t.Error("full mode does not warn against mounting a page it cannot send")
	}
}

// TestEveryParamTheAgentReadsIsDeclared is the guard for a whole class of silent
// failure: the orchestrator validates a plan against the agent's declared params
// and drops anything it does not recognise, so a param the agent reads but never
// declares can never be set through an orchestrated step. It simply takes its
// default forever, with no error anywhere.
func TestEveryParamTheAgentReadsIsDeclared(t *testing.T) {
	agent := &EruStudioAgent{}
	schema := agent.GetInputSchema(context.Background())

	params, ok := schema.Properties[agents.AgentInputParamsKey]
	if !ok {
		t.Fatal("the agent declares no params at all")
	}
	for _, param := range []string{
		"code",
		studio.OutputModeParam,
		studio.ScopeParam,
		studio.BaseRevisionParam,
		studio.InlineNestedParam,
		"context",
		"entities",
		"apis",
	} {
		declared, ok := params.Properties[param]
		if !ok {
			t.Errorf("params.%s is read by the agent but not declared - an orchestrated step can never set it", param)
			continue
		}
		if declared.Description == "" {
			t.Errorf("params.%s is declared with no description, so a planner cannot know what it does", param)
		}
	}

	// ParamKeys is what the orchestrator's allowlist is built from.
	discovered := agents.DiscoveredAgent{InputSchema: schema}
	keys := discovered.ParamKeys()
	for _, param := range []string{studio.InlineNestedParam, studio.ScopeParam, studio.BaseRevisionParam} {
		found := false
		for _, key := range keys {
			if key == param {
				found = true
			}
		}
		if !found {
			t.Errorf("%q is missing from ParamKeys %v - the orchestrator will reject a plan that sets it", param, keys)
		}
	}
}

// A saved page can carry violations from long before this edit: a property that
// was renamed, a misspelled page key. Re-reporting them makes the model rewrite
// components the user never mentioned, which is a whole extra generate cycle on
// a request that only added a control.
func TestValidateStudioUpdateIgnoresIssuesThePatchInherited(t *testing.T) {
	base := existingPage(t)
	// The page already has a legacy property the catalog no longer knows.
	root := base["components"].([]interface{})[0].(map[string]interface{})
	saveBtn := root["children"].([]interface{})[0].(map[string]interface{})
	saveBtn["properties"].(map[string]interface{})["base"].(map[string]interface{})["pieData"] = "[]"

	if issues := studioCatalog.ValidatePage(base); len(issues) == 0 {
		t.Fatal("fixture is meant to start with a pre-existing violation")
	}

	output := map[string]interface{}{
		"mode": studio.ModePatch,
		"patch": map[string]interface{}{
			"page_id": "page_1",
			"upsert": []interface{}{
				map[string]interface{}{
					"id":         "cancel_btn",
					"type":       "button",
					"parent_id":  "root",
					"properties": map[string]interface{}{"base": map[string]interface{}{"label": "Cancel"}},
					"styles":     map[string]interface{}{"classes": ""},
				},
			},
		},
	}

	for _, issue := range validateStudioUpdate(output, base, nil) {
		if strings.Contains(issue.Message, "pieData") {
			t.Errorf("the patch was failed for a violation it inherited: %s", issue)
		}
	}
}

// The subtraction must not become a way to smuggle a bad component in: an issue
// the patch actually introduces still fails it.
func TestValidateStudioUpdateStillCatchesWhatThePatchIntroduces(t *testing.T) {
	base := existingPage(t)
	root := base["components"].([]interface{})[0].(map[string]interface{})
	saveBtn := root["children"].([]interface{})[0].(map[string]interface{})
	saveBtn["properties"].(map[string]interface{})["base"].(map[string]interface{})["pieData"] = "[]"

	output := map[string]interface{}{
		"mode": studio.ModePatch,
		"patch": map[string]interface{}{
			"page_id": "page_1",
			"upsert": []interface{}{
				map[string]interface{}{
					"id":         "cancel_btn",
					"type":       "button",
					"parent_id":  "root",
					"properties": map[string]interface{}{"base": map[string]interface{}{"not_a_real_button_property": true}},
					"styles":     map[string]interface{}{"classes": ""},
				},
			},
		},
	}

	found := false
	for _, issue := range validateStudioUpdate(output, base, nil) {
		if strings.Contains(issue.Message, "not_a_real_button_property") {
			found = true
		}
	}
	if !found {
		t.Error("a property the patch introduced was not reported")
	}
}
