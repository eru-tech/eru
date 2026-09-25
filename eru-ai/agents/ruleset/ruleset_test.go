package ruleset

import (
	"encoding/json"
	"strings"
	"testing"
)

func subject(name string, properties map[string]interface{}) Subject {
	return Subject{Path: "components[0]", Name: name, Properties: properties}
}

func TestRequiresIsTheDefaultKind(t *testing.T) {
	rule := Rule{
		Match: map[string]string{"type": "grid"}, When: "view_mode", Equals: []string{"board"},
		Property: "card_page_id", Code: "board_card_unset",
		Message: "{subject} is in board view but has no {property}",
	}
	broken := rule.Check(subject("grid", map[string]interface{}{"type": "grid", "view_mode": "board"}))
	if len(broken) != 1 {
		t.Fatalf("expected one finding, got %d", len(broken))
	}
	if broken[0].Message != "grid is in board view but has no card_page_id" {
		t.Errorf("message = %q", broken[0].Message)
	}
	ok := rule.Check(subject("grid", map[string]interface{}{
		"type": "grid", "view_mode": "board", "card_page_id": "p2",
	}))
	if len(ok) != 0 {
		t.Errorf("a set property must satisfy the rule: %+v", ok)
	}
}

// Match is what makes a rule apply to some objects and not others, and it is the
// difference between a rule and a blanket assertion.
func TestMatchSelectsTheSubject(t *testing.T) {
	rule := Rule{
		Match: map[string]string{"type": "grid"}, Property: "page", Code: "c",
		Message: "{subject} needs {property}",
	}
	if out := rule.Check(subject("tile", map[string]interface{}{"type": "tile"})); len(out) != 0 {
		t.Errorf("a rule matching grid must not fire on a tile: %+v", out)
	}
	if out := rule.Check(subject("grid", map[string]interface{}{"type": "grid"})); len(out) != 1 {
		t.Errorf("it must fire on a grid: %+v", out)
	}
}

func TestWhenArmsTheRule(t *testing.T) {
	rule := Rule{
		When: "data_source", Equals: []string{"query"},
		Property: "query", Code: "c", Message: "needs {property}",
	}
	if out := rule.Check(subject("tile", map[string]interface{}{"data_source": "page_data"})); len(out) != 0 {
		t.Errorf("an unarmed rule must not fire: %+v", out)
	}
	if out := rule.Check(subject("tile", map[string]interface{}{"data_source": "query"})); len(out) != 1 {
		t.Errorf("an armed rule must fire: %+v", out)
	}
}

func TestForbidden(t *testing.T) {
	rule := Rule{
		Match: map[string]string{"type": "tile"}, Kind: KindForbidden,
		Property: "query_result_path", Code: "c",
		Message: "a {subject} has no {property}; its row is unwrapped for it",
	}
	out := rule.Check(subject("tile", map[string]interface{}{
		"type": "tile", "query_result_path": "0.Results",
	}))
	if len(out) != 1 {
		t.Fatalf("a forbidden property that is set must be reported: %+v", out)
	}
	clean := rule.Check(subject("tile", map[string]interface{}{"type": "tile"}))
	if len(clean) != 0 {
		t.Errorf("absent is the correct state: %+v", clean)
	}
}

func TestPathHead(t *testing.T) {
	rule := Rule{
		Kind: KindPathHead, Property: "query_result_path",
		AllowedHeads: []string{"Results", "rows"}, Code: "c",
		Message: "{property} = {value} starts with {head}",
	}
	for _, good := range []string{"0.Results", "Results", "rows.0", "12"} {
		if out := rule.Check(subject("grid", map[string]interface{}{"query_result_path": good})); len(out) != 0 {
			t.Errorf("%q should be accepted: %+v", good, out)
		}
	}
	out := rule.Check(subject("grid", map[string]interface{}{"query_result_path": "result.0.Results"}))
	if len(out) != 1 {
		t.Fatalf("an unknown head must be reported: %+v", out)
	}
	if !strings.Contains(out[0].Message, `"result"`) {
		t.Errorf("the offending head must be quoted in the message: %s", out[0].Message)
	}
}

func TestNotRowPathReportsEveryOffendingProperty(t *testing.T) {
	rule := Rule{
		When: "data_source", Equals: []string{"query"},
		Kind: KindNotRowPath, Suffix: "_path", Except: []string{"query_result_path"},
		Code: "c", Message: "{property} = {value} is the row path, not a path into {field}",
	}
	out := rule.Check(subject("tile", map[string]interface{}{
		"data_source":         "query",
		"query_result_path":   "0.Results",
		"primary_value_field": "amt", "primary_value_path": "0.amt",
		"secondary_value_field": "cnt", "secondary_value_path": "0.cnt",
	}))
	if len(out) != 2 {
		t.Fatalf("both value paths must be reported, got %d: %+v", len(out), out)
	}
	// Stable order, so a diff of two runs is about behaviour not map iteration.
	if out[0].Property != "primary_value_path" || out[1].Property != "secondary_value_path" {
		t.Errorf("findings must be ordered by property: %+v", out)
	}
	// The excepted property is the response path and is legitimately set.
	for _, f := range out {
		if f.Property == "query_result_path" {
			t.Error("an excepted property must be skipped")
		}
	}
}

func TestAPathIntoAJsonColumnIsAllowed(t *testing.T) {
	rule := Rule{
		When: "data_source", Equals: []string{"query"},
		Kind: KindNotRowPath, Suffix: "_path", Code: "c", Message: "{property}",
	}
	out := rule.Check(subject("tile", map[string]interface{}{
		"data_source": "query", "primary_value_field": "entity_data",
		"primary_value_path": "limit.sanctioned",
	}))
	if len(out) != 0 {
		t.Errorf("digging into a JSON column is what the property is for: %+v", out)
	}
}

// A value the runtime resolves cannot be checked statically, and refusing it
// would ban a legitimate answer for being dynamic.
func TestRuntimeExpressionsSatisfyEveryRule(t *testing.T) {
	head := Rule{Kind: KindPathHead, Property: "query_result_path", Code: "c", Message: "m"}
	for _, expression := range []string{"@state.path", "{{ .path }}"} {
		if out := head.Check(subject("grid", map[string]interface{}{"query_result_path": expression})); len(out) != 0 {
			t.Errorf("%q must be accepted: %+v", expression, out)
		}
	}
}

// The whole point of the package: a rule arrives as configuration.
func TestARuleRoundTripsThroughJSON(t *testing.T) {
	raw := []byte(`{
      "match": {"type": "grid"},
      "when": "data_source", "equals": ["query"],
      "kind": "path_head", "property": "query_result_path",
      "allowed_heads": ["Results"],
      "code": "query_result_path_wrong",
      "message": "{subject}.{property} = {value} starts with {head}",
      "guidance": "query_result_path starts with an index."
    }`)
	var rule Rule
	if err := json.Unmarshal(raw, &rule); err != nil {
		t.Fatal(err)
	}
	if rule.Kind != KindPathHead || rule.Property != "query_result_path" {
		t.Fatalf("decoded wrong: %+v", rule)
	}
	out := rule.Check(subject("grid", map[string]interface{}{
		"type": "grid", "data_source": "query", "query_result_path": "result.0.Results",
	}))
	if len(out) != 1 {
		t.Fatalf("a configured rule must enforce: %+v", out)
	}
	if out[0].Code != "query_result_path_wrong" {
		t.Errorf("code = %q", out[0].Code)
	}
}

// An unknown kind must do nothing rather than panic or silently behave as
// "requires" - a typo in configuration should be inert, not surprising.
func TestAnUnknownKindIsInert(t *testing.T) {
	rule := Rule{Kind: Kind("no_such_kind"), Property: "x", Code: "c", Message: "m"}
	if out := rule.Check(subject("tile", map[string]interface{}{})); len(out) != 0 {
		t.Errorf("an unknown kind must not fire: %+v", out)
	}
}

func TestCheckRunsEveryRuleOverEverySubject(t *testing.T) {
	rules := []Rule{
		{Property: "a", Code: "need_a", Message: "needs a"},
		{Property: "b", Code: "need_b", Message: "needs b"},
	}
	subjects := []Subject{
		{Path: "c[0]", Name: "tile", Properties: map[string]interface{}{"a": "set"}},
		{Path: "c[1]", Name: "grid", Properties: map[string]interface{}{}},
	}
	out := Check(rules, subjects)
	if len(out) != 3 {
		t.Fatalf("expected 3 findings (b on the first, a and b on the second), got %d: %+v", len(out), out)
	}
}

// The prompt is generated from the same declarations, so what the model is told
// and what it is judged on cannot drift.
func TestPromptStatesTheGuidance(t *testing.T) {
	rules := []Rule{
		{Match: map[string]string{"type": "grid"}, Code: "c", Message: "m", Guidance: "A grid needs a card page."},
		{Code: "c2", Message: "m2", Guidance: "Every component states its source."},
		{Code: "c3", Message: "m3"}, // no guidance: nothing to say before the fact
	}
	prompt := Prompt(rules, "RULES")
	for _, want := range []string{"RULES", "grid:", "A grid needs a card page.", "every component:", "Every component states its source."} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestPromptIsEmptyWithoutGuidance(t *testing.T) {
	if got := Prompt([]Rule{{Code: "c", Message: "m"}}, "RULES"); got != "" {
		t.Errorf("a table with nothing to say must render nothing, got %q", got)
	}
}

func TestRequiresAnyIsSatisfiedByOne(t *testing.T) {
	rule := Rule{
		Kind: KindRequiresAny, Properties: []string{"query", "entity_name", "page_data_key"},
		Code: "unbound", Message: "{subject} is bound to nothing; set one of {property}",
	}
	if out := rule.Check(subject("tile", map[string]interface{}{"entity_name": "invoices"})); len(out) != 0 {
		t.Errorf("one is enough: %+v", out)
	}
	out := rule.Check(subject("tile", map[string]interface{}{"title": "Total"}))
	if len(out) != 1 {
		t.Fatalf("none of them set must be reported: %+v", out)
	}
	if !strings.Contains(out[0].Message, "query, entity_name, page_data_key") {
		t.Errorf("the message must name the alternatives: %s", out[0].Message)
	}
}

// An empty list is the shape the fault actually arrives in: the key is present,
// so a bare presence check passes it, and the grid still shows raw headings.
func TestAnEmptyCollectionCountsAsUnset(t *testing.T) {
	rule := Rule{Property: "column_overrides", Code: "c", Message: "{subject} needs {property}"}
	for name, value := range map[string]interface{}{
		"an empty list":   []interface{}{},
		"an empty object": map[string]interface{}{},
		"absent":          nil,
	} {
		out := rule.Check(subject("grid", map[string]interface{}{"column_overrides": value}))
		if len(out) != 1 {
			t.Errorf("%s should be reported: %+v", name, out)
		}
	}
	filled := rule.Check(subject("grid", map[string]interface{}{
		"column_overrides": []interface{}{map[string]interface{}{"field": "an", "label": "Account"}},
	}))
	if len(filled) != 0 {
		t.Errorf("a populated list satisfies it: %+v", filled)
	}
}

func TestNotWithNameLike(t *testing.T) {
	rule := Rule{
		Kind: KindNotWithNameLike, Property: "is_currency",
		Field: "primary_value_field", Patterns: []string{"cnt", "count", "_no"},
		Code: "currency_on_a_count", Message: "{subject} formats {field} as money but {value} reads like a count",
	}
	out := rule.Check(subject("tile", map[string]interface{}{
		"is_currency": true, "primary_value_field": "iff_cnt",
	}))
	if len(out) != 1 {
		t.Fatalf("money formatting on a count must be reported: %+v", out)
	}
	// The string form is what a page written by a model actually carries.
	asText := rule.Check(subject("tile", map[string]interface{}{
		"is_currency": "true", "primary_value_field": "LOAN_COUNT",
	}))
	if len(asText) != 1 {
		t.Errorf(`"true" and case differences must be handled: %+v`, asText)
	}
	for name, props := range map[string]map[string]interface{}{
		"switched off":     {"is_currency": false, "primary_value_field": "iff_cnt"},
		"a genuine amount": {"is_currency": true, "primary_value_field": "os_amt"},
		"no field at all":  {"is_currency": true},
	} {
		if out := rule.Check(subject("tile", props)); len(out) != 0 {
			t.Errorf("%s must not be reported: %+v", name, out)
		}
	}
}

// A caller may only enforce the severity it is entitled to: the validator
// rejects on errors, the quality gate marks down on quality.
func TestFilterSeparatesTheTables(t *testing.T) {
	all := []Rule{
		{Code: "hard", Message: "m"},
		{Code: "soft", Message: "m", Severity: SeverityQuality},
		{Code: "hard2", Message: "m", Severity: SeverityError},
	}
	if hard := Filter(all, SeverityError); len(hard) != 2 {
		t.Errorf("expected 2 error rules, got %d", len(hard))
	}
	soft := Filter(all, SeverityQuality)
	if len(soft) != 1 || soft[0].Code != "soft" {
		t.Errorf("expected only the quality rule, got %+v", soft)
	}
}

// A finding has to carry its severity, or a caller checking a mixed table
// cannot tell which findings may reject the answer.
func TestAFindingCarriesItsSeverity(t *testing.T) {
	rule := Rule{Property: "x", Code: "c", Message: "m", Severity: SeverityQuality}
	out := rule.Check(subject("tile", map[string]interface{}{}))
	if len(out) != 1 || out[0].Severity != SeverityQuality {
		t.Errorf("severity lost on the way out: %+v", out)
	}
}

// Severity defaults to error, so a rule author who says nothing gets the safe
// behaviour rather than a silently unenforced constraint.
func TestSeverityDefaultsToError(t *testing.T) {
	if (Rule{}).Severity != SeverityError {
		t.Error("the zero severity must be the rejecting one")
	}
}

func TestWhenSetArmsOnlyWhileTheComponentIsDoingTheThing(t *testing.T) {
	rule := Rule{
		WhenSet: []string{"currency_symbol", "secondary_value_field"},
		Kind:    KindNotWithNameLike, Property: "secondary_is_currency", DefaultOn: true,
		Field: "secondary_value_field", Patterns: []string{"cnt"},
		Code: "c", Message: "{subject} puts money on {value}",
	}
	// No currency symbol at all: there is no fault to have.
	if out := rule.Check(subject("tile", map[string]interface{}{
		"secondary_value_field": "loan_cnt",
	})); len(out) != 0 {
		t.Errorf("unarmed without a currency symbol: %+v", out)
	}
	out := rule.Check(subject("tile", map[string]interface{}{
		"currency_symbol": "GBP", "secondary_value_field": "loan_cnt",
	}))
	if len(out) != 1 {
		t.Fatalf("armed, and the property defaults on, so this must be reported: %+v", out)
	}
}

// The case the rule exists for is the page that never mentions the property,
// which is most of them. Without DefaultOn the check is silent exactly there.
func TestDefaultOnCatchesTheAbsentProperty(t *testing.T) {
	rule := Rule{
		Kind: KindNotWithNameLike, Property: "secondary_is_currency", DefaultOn: true,
		Field: "secondary_value_field", Patterns: []string{"cnt"},
		Code: "c", Message: "m",
	}
	if out := rule.Check(subject("tile", map[string]interface{}{"secondary_value_field": "loan_cnt"})); len(out) != 1 {
		t.Errorf("an absent default-on property must read as on: %+v", out)
	}
	// Turned off explicitly is the author having decided, and must be respected.
	if out := rule.Check(subject("tile", map[string]interface{}{
		"secondary_is_currency": false, "secondary_value_field": "loan_cnt",
	})); len(out) != 0 {
		t.Errorf("an explicit false must win over the default: %+v", out)
	}
	if out := rule.Check(subject("tile", map[string]interface{}{
		"secondary_is_currency": "false", "secondary_value_field": "loan_cnt",
	})); len(out) != 0 {
		t.Errorf(`the string "false" must also win: %+v`, out)
	}
}

// Without DefaultOn an absent property is off, which is the safe reading for
// every rule that does not opt in.
func TestWithoutDefaultOnAnAbsentPropertyIsOff(t *testing.T) {
	rule := Rule{
		Kind: KindNotWithNameLike, Property: "is_currency",
		Field: "f", Patterns: []string{"cnt"}, Code: "c", Message: "m",
	}
	if out := rule.Check(subject("tile", map[string]interface{}{"f": "loan_cnt"})); len(out) != 0 {
		t.Errorf("absent means off: %+v", out)
	}
}
