package catalog

import (
	"strings"
	"testing"
)

// These assert the shape the agent depends on. They are the guard that a change
// to processo's field editor cannot quietly unteach the agent: regenerate the
// catalog with sync.sh and these either still pass or tell you what moved.

func TestCatalogLoads(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.CatalogVersion == "" {
		t.Error("no catalog_version")
	}
	if c.Fingerprint() == "" {
		t.Error("no source fingerprint - sync.sh --check cannot detect staleness")
	}
	if c.Source.Repo != "processo" {
		t.Errorf("source repo = %q", c.Source.Repo)
	}
}

func TestEveryOfferedDatatypeIsUsable(t *testing.T) {
	c := Get()
	names := c.DatatypeNames()
	if len(names) < 20 {
		t.Fatalf("only %d datatypes - the extractor has probably lost the fieldTypes array", len(names))
	}
	for _, name := range names {
		if !c.IsDatatype(name) {
			t.Errorf("%s is listed but IsDatatype says no", name)
		}
		// Every offered datatype must resolve a key set, even an empty one, or
		// AllowsKey would reject everything on it.
		if !c.AllowsKey(name, "label") {
			t.Errorf("%s does not allow the common key `label` - its key set was not indexed", name)
		}
	}
}

// The datatypes the plan and the prompt lean on hardest.
func TestTheLoadBearingDatatypesAreThere(t *testing.T) {
	c := Get()
	for _, want := range []string{
		"textbox", "textarea", "number", "currency", "date", "datetime", "email",
		"dropdown_single_select", "dropdown_multi_select", "status", "people", "attachment",
	} {
		if !c.IsDatatype(want) {
			t.Errorf("datatype %s is missing", want)
		}
	}
}

func TestPerDatatypeKeysAreTheOnesTheEditorWrites(t *testing.T) {
	c := Get()
	cases := map[string][]string{
		"status":                 {"open_status", "close_status"},
		"dropdown_single_select": {"option_type", "options", "entity_name", "field_name", "api_name", "api_field"},
		"progress":               {"start_value", "end_value", "color_ranges"},
		"email":                  {"system_validate", "ssv_api"},
		"attachment":             {"storage_name", "folder_name"},
		"currency":               {"symbol", "symbol_field", "decimal"},
	}
	for datatype, keys := range cases {
		for _, key := range keys {
			if !c.AllowsKey(datatype, key) {
				t.Errorf("%s should allow %s", datatype, key)
			}
		}
	}
	// And the negative case, which is the one that matters: a key from another
	// datatype is dropped on save, so the agent must be told before it writes.
	if c.AllowsKey("textbox", "open_status") {
		t.Error("textbox should not allow open_status")
	}
	if c.AllowsKey("number", "storage_name") {
		t.Error("number should not allow storage_name")
	}
}

func TestCommonKeysCoverTheIdentityAndFlags(t *testing.T) {
	c := Get()
	common := map[string]bool{}
	for _, key := range c.Field.CommonKeys {
		common[key] = true
	}
	for _, want := range []string{"name", "label", "datatype", "mandatory", "is_hidden", "is_pii", "to_encrypt", "grid_index"} {
		if !common[want] {
			t.Errorf("%s is no longer a common key", want)
		}
	}
}

func TestNamePatternsMatchTheEditorsRules(t *testing.T) {
	c := Get()
	// A field name is lower case and underscore only - no digits. This is the
	// rule most easily broken and the most expensive, because a field cannot be
	// renamed.
	for _, ok := range []string{"deal_value", "owner", "a_b_c"} {
		if !c.ValidFieldName(ok) {
			t.Errorf("field name %q should be valid", ok)
		}
	}
	for _, bad := range []string{"dealValue", "deal value", "deal1", "Deal", ""} {
		if c.ValidFieldName(bad) {
			t.Errorf("field name %q should be rejected", bad)
		}
	}
	// An entity name may contain digits.
	for _, ok := range []string{"deals", "Deals", "deal2"} {
		if !c.ValidEntityName(ok) {
			t.Errorf("entity name %q should be valid", ok)
		}
	}
	for _, bad := range []string{"2deals", "_deals", "deal-s", ""} {
		if c.ValidEntityName(bad) {
			t.Errorf("entity name %q should be rejected", bad)
		}
	}
}

func TestSystemFieldsAreReserved(t *testing.T) {
	c := Get()
	names := c.SystemFieldNames()
	if len(names) < 5 {
		t.Fatalf("only %d system fields - generateDefaultSystemFields was probably not read", len(names))
	}
	if !c.IsSystemFieldName("rcd_onr_xx") {
		t.Error("rcd_onr_xx should be a reserved system field")
	}
	if c.IsSystemFieldName("deal_value") {
		t.Error("deal_value is not a system field")
	}
}

// `rm` is a case the save builder still handles but the editor no longer offers.
// Conflating it with an unknown datatype would send the model hunting a typo.
func TestRetiredDatatypesAreDistinctFromUnknownOnes(t *testing.T) {
	c := Get()
	if !c.IsRetiredDatatype("rm") {
		t.Error("rm should be flagged retired")
	}
	if c.IsDatatype("rm") {
		t.Error("rm must not be offered")
	}
	if c.IsRetiredDatatype("not_a_thing") {
		t.Error("an unknown datatype is not a retired one")
	}
}

func TestOptionTypesComeFromTheEditorsMap(t *testing.T) {
	c := Get()
	got := strings.Join(c.OptionTypes(), ",")
	for _, want := range []string{"STATIC", "ENTITY_DATA", "API"} {
		if !strings.Contains(got, want) {
			t.Errorf("option_type %s missing from %q", want, got)
		}
	}
}

func TestSuggestDatatypePointsSomewhere(t *testing.T) {
	c := Get()
	if got := c.SuggestDatatype("textbo"); got != "textbox" {
		t.Errorf("SuggestDatatype(textbo) = %q", got)
	}
	if got := c.SuggestDatatype("dropdown"); got == "" {
		t.Error("dropdown should suggest one of the dropdown datatypes")
	}
}

// ------------------------------------------------------------------ validation

func TestValidateFieldAcceptsAGoodField(t *testing.T) {
	c := Get()
	issues := c.ValidateField("field", map[string]interface{}{
		"name": "deal_value", "label": "Deal Value", "datatype": "number",
		"decimal": "2", "mandatory": true,
	})
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %s", FormatIssues(issues, 10))
	}
}

func TestValidateFieldCatchesTheNameRules(t *testing.T) {
	c := Get()
	cases := map[string]Code{
		"dealValue":  CodeFieldNameInvalid,
		"rcd_onr_xx": CodeFieldNameSystem,
		"":           CodeFieldNameMissing,
	}
	for name, want := range cases {
		issues := c.ValidateField("field", map[string]interface{}{
			"name": name, "label": "X", "datatype": "textbox",
		})
		if !hasCode(issues, want) {
			t.Errorf("name %q should raise %s, got %s", name, want, FormatIssues(issues, 5))
		}
	}
}

func TestValidateFieldCatchesTheDatatype(t *testing.T) {
	c := Get()
	base := map[string]interface{}{"name": "x_y", "label": "X"}

	missing := clone(base)
	if issues := c.ValidateField("field", missing); !hasCode(issues, CodeDatatypeMissing) {
		t.Errorf("a field with no datatype should raise %s", CodeDatatypeMissing)
	}

	unknown := clone(base)
	unknown["datatype"] = "textbo"
	issues := c.ValidateField("field", unknown)
	if !hasCode(issues, CodeDatatypeUnknown) {
		t.Errorf("unknown datatype should raise %s", CodeDatatypeUnknown)
	}
	if !strings.Contains(FormatIssues(issues, 5), "textbox") {
		t.Error("the message should suggest the nearest datatype")
	}

	retired := clone(base)
	retired["datatype"] = "rm"
	if issues := c.ValidateField("field", retired); !hasCode(issues, CodeDatatypeRetired) {
		t.Errorf("a retired datatype should raise %s", CodeDatatypeRetired)
	}
}

// A key from another datatype is silently dropped on save, so the field arrives
// looking plain and nothing says why. That is the failure this catches.
func TestValidateFieldCatchesKeysFromAnotherDatatype(t *testing.T) {
	c := Get()
	issues := c.ValidateField("field", map[string]interface{}{
		"name": "stage", "label": "Stage", "datatype": "textbox",
		"open_status": []interface{}{},
	})
	if !hasCode(issues, CodeKeyNotOnDatatype) {
		t.Fatalf("expected %s, got %s", CodeKeyNotOnDatatype, FormatIssues(issues, 5))
	}
	// And it should say where the key does belong.
	if !strings.Contains(FormatIssues(issues, 5), "status") {
		t.Error("the message should name the datatype the key belongs to")
	}
}

// A field is described in two shapes: the payload that goes to save_field, where
// every key is a field key, and a report that it was written, which also carries
// which entity and what happened. Holding a report to the payload's key set
// would reject every honest report.
func TestValidateFieldIdentityIgnoresKeysThatAreNotPayload(t *testing.T) {
	c := Get()
	report := map[string]interface{}{
		"entity_name": "deals", "name": "deal_value", "datatype": "currency",
		"action": "created", "note": "",
	}
	if issues := c.ValidateFieldIdentity("fields[0]", report); len(issues) != 0 {
		t.Errorf("a report should pass the identity check, got %s", FormatIssues(issues, 5))
	}
	// The payload form still catches them, because there they would be dropped.
	if issues := c.ValidateField("field", report); !hasCode(issues, CodeKeyNotOnDatatype) {
		t.Error("the payload check should still flag reporting keys")
	}
}

func TestValidateFieldIdentityStillCatchesTheIdentity(t *testing.T) {
	c := Get()
	issues := c.ValidateFieldIdentity("fields[0]", map[string]interface{}{
		"entity_name": "deals", "name": "dealValue", "datatype": "number", "action": "created",
	})
	if !hasCode(issues, CodeFieldNameInvalid) {
		t.Errorf("expected %s, got %s", CodeFieldNameInvalid, FormatIssues(issues, 5))
	}
}

func TestValidateEntityChecksTheName(t *testing.T) {
	c := Get()
	if issues := c.ValidateEntity("entity", map[string]interface{}{"name": "deals"}); len(issues) != 0 {
		t.Errorf("deals should be valid, got %s", FormatIssues(issues, 5))
	}
	if issues := c.ValidateEntity("entity", map[string]interface{}{"name": "2deals"}); !hasCode(issues, CodeEntityNameInvalid) {
		t.Error("2deals should be rejected")
	}
	if issues := c.ValidateEntity("entity", map[string]interface{}{}); !hasCode(issues, CodeEntityNameMissing) {
		t.Error("an entity with no name should be rejected")
	}
}

// ---------------------------------------------------------------------- prompt

func TestContractNamesEveryDatatype(t *testing.T) {
	c := Get()
	contract := c.Contract()
	for _, name := range c.DatatypeNames() {
		if !strings.Contains(contract, name) {
			t.Errorf("the contract does not mention %s", name)
		}
	}
	// And it must warn off the retired one rather than leaving it unmentioned.
	if !strings.Contains(contract, "rm") {
		t.Error("the contract should name the retired datatype so the model knows to avoid it")
	}
	if !strings.Contains(contract, "rcd_onr_xx") {
		t.Error("the contract should list the reserved system field names")
	}
}

func TestSpecAnswersForOneDatatype(t *testing.T) {
	c := Get()
	spec := c.Spec([]string{"status"})
	for _, want := range []string{"status", "open_status", "close_status"} {
		if !strings.Contains(spec, want) {
			t.Errorf("the status spec does not mention %s", want)
		}
	}
	if strings.Contains(spec, "storage_name") {
		t.Error("the status spec should not carry another datatype's keys")
	}
}

func TestSpecCorrectsAnUnknownDatatype(t *testing.T) {
	c := Get()
	if spec := c.Spec([]string{"textbo"}); !strings.Contains(spec, "textbox") {
		t.Errorf("an unknown datatype should be corrected, got %q", spec)
	}
}

func hasCode(issues []Issue, code Code) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func clone(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
