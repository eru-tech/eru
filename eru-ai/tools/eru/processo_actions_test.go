package eru

import (
	"context"
	"strings"
	"testing"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
)

func init() { logs.LogInit("test", "processo-phase0a-test") }

func TestProcessoExposesThePhase0aActions(t *testing.T) {
	names := processoActionNames()
	for _, want := range []string{ProcessoRemoveEntity, ProcessoGetEntityFieldData} {
		if !names[want] {
			t.Errorf("processo does not expose %s", want)
		}
		// The tool's own actions are never tenant scoped.
		if names[want+"_default"] || names[want+"_project"] {
			t.Errorf("%s was tenant scoped, which changes its name", want)
		}
	}
}

// remove_entity destroys data, so its description has to say so where the model
// reads it rather than only in a prompt somewhere else.
func TestRemoveEntityWarnsThatItCannotBeUndone(t *testing.T) {
	for _, a := range processoToolActions {
		if a.ActionName != ProcessoRemoveEntity {
			continue
		}
		combined := strings.ToLower(a.Description + " " + a.SystemPrompt)
		if !strings.Contains(combined, "cannot be undone") {
			t.Errorf("remove_entity should warn it cannot be undone, got %q", combined)
		}
		return
	}
	t.Fatal("remove_entity action not found")
}

func TestGetEntityFieldDataValidatesItsInputs(t *testing.T) {
	tool := &ProcessoTool{}
	if _, _, _, err := tool.GetEntityFieldData(context.Background(), "processo", "t", map[string]interface{}{
		"org_id": "o", "process_id": "p", "process_name": "n", "entity_name": "deals",
	}); err == nil {
		t.Error("field_name is mandatory")
	}
}

// ------------------------------------------------------------ page access

func TestProcessoExposesThePageAccessActions(t *testing.T) {
	names := processoActionNames()
	for _, want := range []string{
		ProcessoSavePageVisibility, ProcessoSavePageVisibilityTemplate,
		ProcessoDeletePageVisibilityTemplate, ProcessoFetchPageVisibilityTemplates,
		ProcessoGetUserAttributeEntities,
	} {
		if !names[want] {
			t.Errorf("processo does not expose %s", want)
		}
	}
}

// The union semantics are the thing that gets page access wrong — a reader
// assumes users AND roles AND attributes all have to match. It has to be in the
// text the model reads.
func TestPageVisibilityExplainsTheUnion(t *testing.T) {
	for _, a := range processoToolActions {
		if a.ActionName != ProcessoSavePageVisibility {
			continue
		}
		if !strings.Contains(strings.ToUpper(a.SystemPrompt), "UNION") {
			t.Errorf("save_page_visibility should say the audience is a union, got %q", a.SystemPrompt)
		}
		return
	}
	t.Fatal("save_page_visibility not found")
}

// A private rule naming nobody is valid to the backend and locks everyone out.
func TestPageVisibilityRefusesARuleThatAdmitsNobody(t *testing.T) {
	tool := &ProcessoTool{}
	_, _, _, err := tool.SavePageVisibility(context.Background(), "processo", "t", map[string]interface{}{
		"org_id": "o", "process_id": "p", "process_name": "n",
		"page_id": "page-1", "visibility_type": "private",
	})
	if err == nil {
		t.Fatal("a private rule with no audience should be rejected")
	}
	if !strings.Contains(err.Error(), "admits nobody") {
		t.Errorf("the error should explain why: %v", err)
	}
}

func TestPageVisibilityAcceptsAnAudienceOfJustATemplate(t *testing.T) {
	tool := &ProcessoTool{}
	// Reaches the HTTP call and fails there, which is past the validation we
	// are asserting on — a validation rejection would not mention a base url.
	_, _, _, err := tool.SavePageVisibility(context.Background(), "processo", "t", map[string]interface{}{
		"org_id": "o", "process_id": "p", "process_name": "n",
		"page_id": "page-1", "visibility_type": "private", "template_id": "tpl-1",
	})
	if err != nil && strings.Contains(err.Error(), "admits nobody") {
		t.Error("a template alone is a valid audience")
	}
}

func TestPageVisibilityRequiresAPage(t *testing.T) {
	tool := &ProcessoTool{}
	if _, _, _, err := tool.SavePageVisibility(context.Background(), "processo", "t", map[string]interface{}{
		"org_id": "o", "process_id": "p", "process_name": "n", "visibility_type": "public",
	}); err == nil {
		t.Error("page_id is mandatory")
	}
}

// Creating a template must not send a blank template_id: the upsert key has to
// be absent for the insert branch.
func TestTemplateSaveOmitsABlankId(t *testing.T) {
	for _, a := range processoToolActions {
		if a.ActionName != ProcessoSavePageVisibilityTemplate || a.GetParameters == nil {
			continue
		}
		desc := a.GetParameters().Properties["template_id"].Description
		if !strings.Contains(strings.ToUpper(desc), "OMIT") {
			t.Errorf("template_id should tell the model to omit it when creating, got %q", desc)
		}
		return
	}
	t.Fatal("save_page_visibility_template not found")
}

func TestTemplateSaveValidatesNameAndAudience(t *testing.T) {
	tool := &ProcessoTool{}
	if _, _, _, err := tool.SavePageVisibilityTemplate(context.Background(), "processo", "t", map[string]interface{}{
		"org_id": "o", "process_id": "p", "org_process_id": "op", "map_roles": []interface{}{"manager"},
	}); err == nil {
		t.Error("template_name is mandatory")
	}
	if _, _, _, err := tool.SavePageVisibilityTemplate(context.Background(), "processo", "t", map[string]interface{}{
		"org_id": "o", "process_id": "p", "org_process_id": "op", "template_name": "Managers",
	}); err == nil {
		t.Error("a template with no audience should be rejected")
	}
}

func TestDeleteTemplateRequiresAnId(t *testing.T) {
	tool := &ProcessoTool{}
	if _, _, _, err := tool.DeletePageVisibilityTemplate(context.Background(), "processo", "t", map[string]interface{}{
		"org_id": "o", "process_id": "p", "org_process_id": "op",
	}); err == nil {
		t.Error("template_id is mandatory")
	}
}

// -------------------------------------------------------- approval matrix

func approvalRule(frank int, filtered bool, roles ...string) map[string]interface{} {
	r := map[string]interface{}{"frank": frank, "no_of_approver": 1}
	if len(roles) > 0 {
		r["roles"] = roles
	}
	if filtered {
		r["filter"] = map[string]interface{}{"1___amount": map[string]interface{}{"$gt": []interface{}{"100"}}}
	}
	return r
}

func approvalMatrix(levels map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"org_id": "o", "process_id": "p", "entity_name": "deals",
		"matrix_json": map[string]interface{}{
			"matrix_name": "Deal approval", "entity_name": "deals",
			"approval_levels": levels,
		},
	}
}

func saveMatrix(t *testing.T, levels map[string]interface{}) error {
	t.Helper()
	tool := &ProcessoTool{}
	_, _, _, err := tool.SaveApprovalMatrix(context.Background(), "processo", "t", approvalMatrix(levels))
	return err
}

func TestProcessoExposesTheApprovalMatrixActions(t *testing.T) {
	names := processoActionNames()
	for _, want := range []string{ProcessoGetApprovalMatrix, ProcessoSaveApprovalMatrix} {
		if !names[want] {
			t.Errorf("processo does not expose %s", want)
		}
	}
}

// A rule after the catch-all can never be reached, because the catch-all matches
// everything. The screen prevents it; the backend does not.
func TestApprovalMatrixRejectsRulesAfterTheCatchAll(t *testing.T) {
	err := saveMatrix(t, map[string]interface{}{
		"1": map[string]interface{}{"filters": []interface{}{
			approvalRule(1, false, "manager"),
			approvalRule(2, true, "director"),
		}},
	})
	if err == nil {
		t.Fatal("a rule after the catch-all should be rejected")
	}
	if !strings.Contains(err.Error(), "never be reached") {
		t.Errorf("the error should explain why: %v", err)
	}
}

func TestApprovalMatrixAcceptsACatchAllLast(t *testing.T) {
	err := saveMatrix(t, map[string]interface{}{
		"1": map[string]interface{}{"filters": []interface{}{
			approvalRule(1, true, "director"),
			approvalRule(2, false, "manager"),
		}},
	})
	// Validation passes, so it reaches the HTTP call and fails there instead.
	if err != nil && strings.Contains(err.Error(), "never be reached") {
		t.Errorf("a catch-all in last position is valid: %v", err)
	}
}

// Levels are sequential, so a gap means the later level is never reached.
func TestApprovalMatrixRejectsGappedLevels(t *testing.T) {
	err := saveMatrix(t, map[string]interface{}{
		"1": map[string]interface{}{"filters": []interface{}{approvalRule(1, false, "manager")}},
		"3": map[string]interface{}{"filters": []interface{}{approvalRule(1, false, "director")}},
	})
	if err == nil || !strings.Contains(err.Error(), "no gaps") {
		t.Errorf("gapped levels should be rejected, got %v", err)
	}
}

func TestApprovalMatrixEnforcesTheLevelCap(t *testing.T) {
	levels := map[string]interface{}{}
	for _, k := range []string{"1", "2", "3", "4"} {
		levels[k] = map[string]interface{}{"filters": []interface{}{approvalRule(1, false, "manager")}}
	}
	if err := saveMatrix(t, levels); err == nil || !strings.Contains(err.Error(), "at most 3") {
		t.Errorf("a fourth level should be rejected, got %v", err)
	}
}

func TestApprovalMatrixRejectsARuleThatApprovesThroughNobody(t *testing.T) {
	err := saveMatrix(t, map[string]interface{}{
		"1": map[string]interface{}{"filters": []interface{}{
			map[string]interface{}{"frank": 1, "no_of_approver": 1},
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "approves through nobody") {
		t.Errorf("a rule with no approver should be rejected, got %v", err)
	}
}

func TestApprovalMatrixRejectsDuplicateFrank(t *testing.T) {
	err := saveMatrix(t, map[string]interface{}{
		"1": map[string]interface{}{"filters": []interface{}{
			approvalRule(1, true, "manager"),
			approvalRule(1, true, "director"),
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "frank") {
		t.Errorf("duplicate frank should be rejected, got %v", err)
	}
}

// Saving against a different entity than the matrix names would route the wrong
// records.
func TestApprovalMatrixRefusesAnEntityMismatch(t *testing.T) {
	tool := &ProcessoTool{}
	body := approvalMatrix(map[string]interface{}{
		"1": map[string]interface{}{"filters": []interface{}{approvalRule(1, true, "manager")}},
	})
	body["matrix_json"].(map[string]interface{})["entity_name"] = "leads"
	_, _, _, err := tool.SaveApprovalMatrix(context.Background(), "processo", "t", body)
	if err == nil || !strings.Contains(err.Error(), "wrong entity") {
		t.Errorf("an entity mismatch should be rejected, got %v", err)
	}
}

func TestGetApprovalMatrixRequiresAnEntity(t *testing.T) {
	tool := &ProcessoTool{}
	if _, _, _, err := tool.GetApprovalMatrix(context.Background(), "processo", "t", map[string]interface{}{
		"org_id": "o", "process_id": "p",
	}); err == nil {
		t.Error("entity_name is mandatory")
	}
}

// ------------------------------------------- per-datatype field validation

func fieldPayload(datatype string, extra map[string]interface{}) map[string]interface{} {
	f := map[string]interface{}{
		"name": "x_y", "label": "X", "datatype": datatype, "show_grid": "yes",
	}
	for k, v := range extra {
		f[k] = v
	}
	return map[string]interface{}{
		"org_id": "o", "process_id": "p", "process_name": "n", "entity_name": "deals", "field": f,
	}
}

func saveField(t *testing.T, datatype string, extra map[string]interface{}) error {
	t.Helper()
	tool := &ProcessoTool{}
	_, _, _, err := tool.SaveField(context.Background(), "processo", "t", fieldPayload(datatype, extra))
	return err
}

// Validation runs before the HTTP call, so a payload that passes fails later on
// the base url instead — anything mentioning the rule means validation caught it.
func rejectedFor(err error, phrase string) bool {
	return err != nil && strings.Contains(err.Error(), phrase)
}

func TestProgressRequiresAnAscendingRange(t *testing.T) {
	if err := saveField(t, "progress", map[string]interface{}{"start_value": 10, "end_value": 5}); !rejectedFor(err, "must be less than") {
		t.Errorf("start >= end should be rejected, got %v", err)
	}
	if err := saveField(t, "progress", nil); !rejectedFor(err, "mandatory numbers") {
		t.Errorf("missing bounds should be rejected, got %v", err)
	}
	if err := saveField(t, "progress", map[string]interface{}{"start_value": 0, "end_value": 100}); rejectedFor(err, "must be less than") {
		t.Errorf("a valid range should pass validation: %v", err)
	}
}

func TestProgressColoursMustBeHex(t *testing.T) {
	err := saveField(t, "progress", map[string]interface{}{
		"start_value": 0, "end_value": 100,
		"color_ranges": []interface{}{map[string]interface{}{"from": 0, "to": 50, "color": "red"}},
	})
	if !rejectedFor(err, "hex code") {
		t.Errorf("a non-hex colour should be rejected, got %v", err)
	}
}

func TestTagAndPriorityNeedOptions(t *testing.T) {
	for _, dt := range []string{"tag", "priority"} {
		if err := saveField(t, dt, nil); !rejectedFor(err, "at least one option") {
			t.Errorf("%s with no options should be rejected, got %v", dt, err)
		}
		err := saveField(t, dt, map[string]interface{}{
			"options": []interface{}{map[string]interface{}{"name": "High", "color": "red"}},
		})
		if !rejectedFor(err, "hex code") {
			t.Errorf("%s with a non-hex option colour should be rejected, got %v", dt, err)
		}
	}
}

func TestRatingNeedsAnEndValue(t *testing.T) {
	if err := saveField(t, "rating", nil); !rejectedFor(err, "end_value is mandatory") {
		t.Errorf("rating with no end_value should be rejected, got %v", err)
	}
}

func TestAttachmentNeedsAStorage(t *testing.T) {
	if err := saveField(t, "attachment", nil); !rejectedFor(err, "storage_name is mandatory") {
		t.Errorf("attachment with no storage should be rejected, got %v", err)
	}
}

func TestObjectNeedsANestedEntity(t *testing.T) {
	for _, dt := range []string{"object_single_select", "object_multi_select"} {
		if err := saveField(t, dt, nil); !rejectedFor(err, "nested_entity is mandatory") {
			t.Errorf("%s with no nested entity should be rejected, got %v", dt, err)
		}
	}
}

func TestHyperlinkNeedsItsText(t *testing.T) {
	if err := saveField(t, "website", map[string]interface{}{"is_hyp": true}); !rejectedFor(err, "hypl_nm is mandatory") {
		t.Errorf("a hyperlink with no text should be rejected, got %v", err)
	}
}

// The keys these datatypes carry have to be expressible at all — before this they
// were absent from the params struct, so the model could only create bare fields.
func TestTypeSpecificKeysAreInTheSchema(t *testing.T) {
	var schema eru_models.JSONSchema
	for _, a := range processoToolActions {
		if a.ActionName == ProcessoSaveField && a.GetParameters != nil {
			schema = a.GetParameters()
		}
	}
	field := schema.Properties["field"]
	for _, key := range []string{
		"start_value", "end_value", "emoji_value", "color_ranges", "decimal", "seperator",
		"symbol", "symbol_field", "storage_name", "folder_name", "value_true", "value_false",
		"nested_entity", "multiple", "rich_text", "df_fields", "is_hyp", "hypl_nm",
		"display_number_as", "num_val", "dynamic_number", "default_value_offset_days",
	} {
		if _, ok := field.Properties[key]; !ok {
			t.Errorf("the field payload cannot express %q", key)
		}
	}
}

// The enum tag is what makes the provider reject a bad value against the tool
// call, instead of the service rejecting it after the round trip.
func TestEnumsReachTheGeneratedSchema(t *testing.T) {
	var schema eru_models.JSONSchema
	for _, a := range processoToolActions {
		if a.ActionName == ProcessoSaveField && a.GetParameters != nil {
			schema = a.GetParameters()
		}
	}
	field := schema.Properties["field"]
	if got := field.Properties["datatype"].Enum; len(got) != 24 {
		t.Errorf("datatype should enumerate the 24 offered types, got %d", len(got))
	}
	if got := field.Properties["option_type"].Enum; len(got) != 3 {
		t.Errorf("option_type should enumerate 3 values, got %v", got)
	}
	// On a list the enum constrains the items, not the array.
	allow := field.Properties["allow_days"]
	if allow.Items == nil || len(allow.Items.Enum) != 7 {
		t.Errorf("allow_days items should enumerate the 7 days, got %+v", allow.Items)
	}
	if len(allow.Enum) != 0 {
		t.Error("the enum belongs on the items, not on the array itself")
	}
}

// ------------------------------------------------- names and reserved names

// processo has no rename: a badly named field can only be deleted and recreated,
// and until then the editor cannot open it. The rules come from the generated
// catalog so the tool refuses exactly what the editor refuses.
func TestFieldNameRulesComeFromTheCatalog(t *testing.T) {
	for _, bad := range []string{"dealValue", "deal value", "deal1", "Deal", ""} {
		if err := saveField(t, "textbox", map[string]interface{}{"name": bad}); err == nil ||
			!(strings.Contains(err.Error(), "does not match") || strings.Contains(err.Error(), "mandatory")) {
			t.Errorf("field name %q should be rejected, got %v", bad, err)
		}
	}
	if err := saveField(t, "textbox", map[string]interface{}{"name": "deal_value"}); rejectedFor(err, "does not match") {
		t.Errorf("deal_value is a valid field name: %v", err)
	}
}

func TestSystemFieldNamesAreRefused(t *testing.T) {
	if err := saveField(t, "textbox", map[string]interface{}{"name": "rcd_onr_xx"}); !rejectedFor(err, "system field") {
		t.Errorf("writing a system field should be rejected, got %v", err)
	}
}

func TestEntityNamesAreChecked(t *testing.T) {
	tool := &ProcessoTool{}
	body := func(name string) map[string]interface{} {
		return map[string]interface{}{
			"org_id": "o", "process_id": "p", "process_name": "n",
			"entity_data": []interface{}{map[string]interface{}{"name": name}},
		}
	}
	for _, bad := range []string{"2deals", "deal-s", ""} {
		if _, _, _, err := tool.SaveEntity(context.Background(), "processo", "t", body(bad)); err == nil {
			t.Errorf("entity name %q should be rejected", bad)
		}
	}
	_, _, _, err := tool.SaveEntity(context.Background(), "processo", "t", body("deals2"))
	if rejectedFor(err, "does not match") {
		t.Errorf("deals2 is a valid entity name: %v", err)
	}
}

// att_rules nest one level only. The params struct enforces that structurally —
// a group's rules have no rules of their own — so there is nothing to check at
// runtime, and this asserts the shape stays that way.
func TestAttrRulesNestOnlyOneLevel(t *testing.T) {
	var schema eru_models.JSONSchema
	for _, a := range processoToolActions {
		if a.ActionName == ProcessoSavePageVisibility && a.GetParameters != nil {
			schema = a.GetParameters()
		}
	}
	items := schema.Properties["att_rules"].Items
	if items == nil {
		t.Fatal("att_rules has no item schema")
	}
	inner := items.Properties["rules"]
	if inner.Items == nil {
		t.Fatal("att_rules.rules has no item schema")
	}
	if _, nested := inner.Items.Properties["rules"]; nested {
		t.Error("att_rules should nest one level only, but a group can hold groups")
	}
}
